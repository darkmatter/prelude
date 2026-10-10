#include "bridge.h"

#include <ghostty/vt/focus.h>
#include <ghostty/vt/paste.h>

#include <stdlib.h>
#include <string.h>

/* One instance belongs to one Go event loop. Synchronous native callbacks use
   this C allocation as userdata; no Go pointer is retained across the FFI. */
struct bridge_terminal {
    GhosttyTerminal terminal;
    GhosttyRenderState render;
    GhosttyRenderStateRowIterator rows;
    GhosttyRenderStateRowCells cells;
    GhosttyKeyEncoder keys;
    GhosttyMouseEncoder mouse;
    uint16_t cols, height, pressed_buttons;
    bool previous_escape;
    char *replies, *encoded;
    size_t replies_len, replies_cap, encoded_cap;
    GhosttyResult reply_error;
    bridge_cell *snapshot_cells;
    size_t snapshot_cap;
    uint32_t *text;
    size_t text_cap;
};

#define TRY(call) do { GhosttyResult r = (call); if (r != GHOSTTY_SUCCESS) return r; } while (0)

static GhosttyResult reserve(void **buffer, size_t *capacity, size_t count, size_t size) {
    if (count <= *capacity) return GHOSTTY_SUCCESS;
    size_t limit = SIZE_MAX / size;
    if (count > limit) return GHOSTTY_OUT_OF_MEMORY;
    size_t next_capacity = count;
    if (*capacity <= limit / 2 && count < *capacity * 2)
        next_capacity = *capacity * 2;
    void *next = realloc(*buffer, next_capacity * size);
    if (!next) return GHOSTTY_OUT_OF_MEMORY;
    *buffer = next;
    *capacity = next_capacity;
    return GHOSTTY_SUCCESS;
}

/* Called while Ghostty processes output. Collect answers without blocking on
   PTY I/O or calling into Go; bridge_write returns them after parsing finishes. */
static void write_pty(GhosttyTerminal terminal, void *userdata,
                      const uint8_t *data, size_t len) {
    (void)terminal;
    bridge_terminal *t = userdata;
    if (!len || t->reply_error != GHOSTTY_SUCCESS) return;
    if (len > SIZE_MAX - t->replies_len) {
        t->reply_error = GHOSTTY_OUT_OF_MEMORY;
        return;
    }
    size_t needed = t->replies_len + len;
    t->reply_error = reserve((void **)&t->replies, &t->replies_cap, needed, 1);
    if (t->reply_error != GHOSTTY_SUCCESS) return;
    memcpy(t->replies + t->replies_len, data, len);
    t->replies_len = needed;
}

static void title_changed(GhosttyTerminal terminal, void *userdata) {
    (void)terminal;
    (void)userdata;
    /* Installing the effect lets the terminal retain OSC 0/2 titles. */
}

static GhosttyString version(GhosttyTerminal terminal, void *userdata) {
    (void)terminal;
    (void)userdata;
    static const char name[] = "prelude-workspace";
    return (GhosttyString){.ptr = (const uint8_t *)name, .len = sizeof(name) - 1};
}

static bool device_attributes(GhosttyTerminal terminal, void *userdata,
                              GhosttyDeviceAttributes *out) {
    (void)terminal;
    (void)userdata;
    *out = (GhosttyDeviceAttributes){
        .primary = {
            .conformance_level = GHOSTTY_DA_CONFORMANCE_VT220,
            .features = {GHOSTTY_DA_FEATURE_ANSI_COLOR},
            .num_features = 1,
        },
        .secondary = {.device_type = GHOSTTY_DA_DEVICE_TYPE_VT220},
    };
    return true;
}

static bool terminal_size(GhosttyTerminal terminal, void *userdata,
                          GhosttySizeReportSize *out) {
    (void)terminal;
    bridge_terminal *t = userdata;
    /* The host knows cells, not pixels: zero is unknown, not a fictitious font size. */
    *out = (GhosttySizeReportSize){.rows = t->height, .columns = t->cols};
    return true;
}

/* Parsing a protocol does not mean the host can display it. This cell-only
   renderer has no image compositor; also disallow child-selected image files
   and shared memory rather than inheriting a graphical terminal's defaults. */
static GhosttyResult restrict_capabilities(bridge_terminal *t) {
    uint64_t no_images = 0;
    bool disabled = false;
    size_t apc_limit = 4096;
    TRY(ghostty_terminal_set(t->terminal, GHOSTTY_TERMINAL_OPT_KITTY_IMAGE_STORAGE_LIMIT, &no_images));
    TRY(ghostty_terminal_set(t->terminal, GHOSTTY_TERMINAL_OPT_KITTY_IMAGE_MEDIUM_FILE, &disabled));
    TRY(ghostty_terminal_set(t->terminal, GHOSTTY_TERMINAL_OPT_KITTY_IMAGE_MEDIUM_TEMP_FILE, &disabled));
    TRY(ghostty_terminal_set(t->terminal, GHOSTTY_TERMINAL_OPT_KITTY_IMAGE_MEDIUM_SHARED_MEM, &disabled));
    TRY(ghostty_terminal_set(t->terminal, GHOSTTY_TERMINAL_OPT_APC_MAX_BYTES, &apc_limit));
    return GHOSTTY_SUCCESS;
}

static GhosttyResult initialize(bridge_terminal *t) {
    TRY(ghostty_terminal_new(NULL, &t->terminal, (GhosttyTerminalOptions){
        .cols = t->cols, .rows = t->height, .max_scrollback = 1000,
    }));
    TRY(ghostty_terminal_set(t->terminal, GHOSTTY_TERMINAL_OPT_USERDATA, t));
    TRY(ghostty_terminal_set(t->terminal, GHOSTTY_TERMINAL_OPT_WRITE_PTY, (const void *)write_pty));
    TRY(ghostty_terminal_set(t->terminal, GHOSTTY_TERMINAL_OPT_TITLE_CHANGED, (const void *)title_changed));
    TRY(ghostty_terminal_set(t->terminal, GHOSTTY_TERMINAL_OPT_XTVERSION, (const void *)version));
    TRY(ghostty_terminal_set(t->terminal, GHOSTTY_TERMINAL_OPT_DEVICE_ATTRIBUTES, (const void *)device_attributes));
    TRY(ghostty_terminal_set(t->terminal, GHOSTTY_TERMINAL_OPT_SIZE, (const void *)terminal_size));
    TRY(restrict_capabilities(t));
    /* Seed terminal state rather than forcing View: children can override it. */
    static const uint8_t cursor[] = "\x1b[5 q";
    ghostty_terminal_vt_write(t->terminal, cursor, sizeof(cursor) - 1);
    TRY(ghostty_render_state_row_iterator_new(NULL, &t->rows));
    TRY(ghostty_render_state_row_cells_new(NULL, &t->cells));
    TRY(ghostty_key_encoder_new(NULL, &t->keys));
    TRY(ghostty_mouse_encoder_new(NULL, &t->mouse));
    return GHOSTTY_SUCCESS;
}

GhosttyResult bridge_new(uint16_t cols, uint16_t rows, bridge_terminal **out) {
    *out = NULL;
    if (!cols || !rows) return GHOSTTY_INVALID_VALUE;
    bridge_terminal *t = calloc(1, sizeof(*t));
    if (!t) return GHOSTTY_OUT_OF_MEMORY;
    t->cols = cols;
    t->height = rows;
    GhosttyResult r = initialize(t);
    if (r != GHOSTTY_SUCCESS) {
        bridge_free(t);
        return r;
    }
    *out = t;
    return GHOSTTY_SUCCESS;
}

void bridge_free(bridge_terminal *t) {
    if (!t) return;
    ghostty_mouse_encoder_free(t->mouse);
    ghostty_key_encoder_free(t->keys);
    ghostty_render_state_row_cells_free(t->cells);
    ghostty_render_state_row_iterator_free(t->rows);
    ghostty_render_state_free(t->render);
    ghostty_terminal_free(t->terminal);
    free(t->replies);
    free(t->encoded);
    free(t->snapshot_cells);
    free(t->text);
    free(t);
}

GhosttyResult bridge_write(bridge_terminal *t, const char *data, size_t len,
                           bridge_bytes *out) {
    /* This pinned library's RIS resets graphics options. Split at ESC c and
       restore bounds before subsequent input, including across Write calls.
       A match inside a control string only reapplies the same graphics bounds;
       all VT parsing remains in the library and no bytes are removed. */
    GhosttyResult r = GHOSTTY_SUCCESS;
    size_t start = 0;
    for (size_t i = 0; i < len; i++) {
        bool ris = t->previous_escape && data[i] == 'c';
        t->previous_escape = data[i] == '\x1b';
        if (!ris) continue;
        ghostty_terminal_vt_write(t->terminal, (const uint8_t *)data + start, i + 1 - start);
        start = i + 1;
        r = restrict_capabilities(t);
        if (r != GHOSTTY_SUCCESS) break;
    }
    if (r == GHOSTTY_SUCCESS && start < len)
        ghostty_terminal_vt_write(t->terminal, (const uint8_t *)data + start, len - start);
    *out = (bridge_bytes){.data = t->replies, .len = t->replies_len};
    if (r == GHOSTTY_SUCCESS) r = t->reply_error;
    t->replies_len = 0;
    t->reply_error = GHOSTTY_SUCCESS;
    return r;
}

GhosttyResult bridge_resize(bridge_terminal *t, uint16_t cols, uint16_t rows) {
    /* Resize can generate mode-2048 replies; Write (including Write(nil)) drains them. */
    TRY(ghostty_terminal_resize(t->terminal, cols, rows, 0, 0));
    t->cols = cols;
    t->height = rows;
    ghostty_mouse_encoder_reset(t->mouse);
    return GHOSTTY_SUCCESS;
}

static bridge_color resolve_color(GhosttyStyleColor color, const GhosttyColorRgb *palette) {
    switch (color.tag) {
    case GHOSTTY_STYLE_COLOR_RGB:
        return (bridge_color){.set = true, .rgb = color.value.rgb};
    case GHOSTTY_STYLE_COLOR_PALETTE:
        return (bridge_color){.set = true, .rgb = palette[color.value.palette]};
    default:
        return (bridge_color){0};
    }
}

GhosttyResult bridge_snapshot(bridge_terminal *t, bridge_frame *out) {
    /* This pinned library's incremental render cache misses appended combining
       marks after a prior update, even with DIRTY_FULL. The native grid retains
       them in default mode. Full owned snapshots require a fresh render state. */
    GhosttyRenderState render;
    TRY(ghostty_render_state_new(NULL, &render));
    GhosttyResult result = ghostty_render_state_update(render, t->terminal);
    if (result != GHOSTTY_SUCCESS) {
        ghostty_render_state_free(render);
        return result;
    }
    ghostty_render_state_free(t->render);
    t->render = render;
    *out = (bridge_frame){.cols = t->cols, .rows = t->height};
    GhosttyTerminalScreen screen;
    GhosttyRenderStateCursorVisualStyle cursor_style;
    TRY(ghostty_terminal_get(t->terminal, GHOSTTY_TERMINAL_DATA_ACTIVE_SCREEN, &screen));
    TRY(ghostty_terminal_get(t->terminal, GHOSTTY_TERMINAL_DATA_MOUSE_TRACKING, &out->mouse_tracking));
    TRY(ghostty_terminal_get(t->terminal, GHOSTTY_TERMINAL_DATA_TITLE, &out->title));
    TRY(ghostty_terminal_get(t->terminal, GHOSTTY_TERMINAL_DATA_CURSOR_X, &out->cursor_x));
    TRY(ghostty_terminal_get(t->terminal, GHOSTTY_TERMINAL_DATA_CURSOR_Y, &out->cursor_y));
    TRY(ghostty_render_state_get(t->render, GHOSTTY_RENDER_STATE_DATA_CURSOR_VISIBLE, &out->cursor_visible));
    TRY(ghostty_render_state_get(t->render, GHOSTTY_RENDER_STATE_DATA_CURSOR_BLINKING, &out->cursor_blink));
    TRY(ghostty_render_state_get(t->render, GHOSTTY_RENDER_STATE_DATA_CURSOR_VISUAL_STYLE, &cursor_style));
    out->alt_screen = screen == GHOSTTY_TERMINAL_SCREEN_ALTERNATE;
    out->cursor_style = cursor_style == GHOSTTY_RENDER_STATE_CURSOR_VISUAL_STYLE_BAR ? 1 :
                        cursor_style == GHOSTTY_RENDER_STATE_CURSOR_VISUAL_STYLE_UNDERLINE ? 2 : 0;
    GhosttyColorRgb palette[256];
    TRY(ghostty_render_state_get(t->render, GHOSTTY_RENDER_STATE_DATA_COLOR_PALETTE, palette));
    size_t count = (size_t)t->cols * t->height;
    TRY(reserve((void **)&t->snapshot_cells, &t->snapshot_cap, count, sizeof(bridge_cell)));
    /* These data getters take the address of a preallocated handle. */
    TRY(ghostty_render_state_get(t->render, GHOSTTY_RENDER_STATE_DATA_ROW_ITERATOR, &t->rows));
    /* Flatten the viewport into row-major cells plus a codepoint arena. Text
       offsets survive arena reallocations; Go copies both before the next call. */
    size_t i = 0, text_len = 0;
    while (ghostty_render_state_row_iterator_next(t->rows)) {
        TRY(ghostty_render_state_row_get(t->rows, GHOSTTY_RENDER_STATE_ROW_DATA_CELLS, &t->cells));
        while (ghostty_render_state_row_cells_next(t->cells)) {
            if (i >= count) return GHOSTTY_INVALID_VALUE;
            bridge_cell *cell = &t->snapshot_cells[i++];
            *cell = (bridge_cell){.text_offset = text_len, .width = 1};
            cell->style = GHOSTTY_INIT_SIZED(GhosttyStyle);
            GhosttyCell raw;
            GhosttyCellWide wide;
            TRY(ghostty_render_state_row_cells_get(t->cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_RAW, &raw));
            TRY(ghostty_cell_get(raw, GHOSTTY_CELL_DATA_WIDE, &wide));
            TRY(ghostty_render_state_row_cells_get(t->cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_STYLE, &cell->style));
            cell->fg.set = ghostty_render_state_row_cells_get(t->cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_FG_COLOR, &cell->fg.rgb) == GHOSTTY_SUCCESS;
            cell->bg.set = ghostty_render_state_row_cells_get(t->cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_BG_COLOR, &cell->bg.rgb) == GHOSTTY_SUCCESS;
            cell->underline = resolve_color(cell->style.underline_color, palette);
            if (wide == GHOSTTY_CELL_WIDE_SPACER_TAIL) {
                cell->width = 0;
                continue;
            }
            /* A wrap-head spacer is a real blank column, not a wide-cell continuation. */
            if (wide == GHOSTTY_CELL_WIDE_SPACER_HEAD) continue;
            cell->width = wide == GHOSTTY_CELL_WIDE_WIDE ? 2 : 1;
            TRY(ghostty_render_state_row_cells_get(t->cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_GRAPHEMES_LEN, &cell->text_len));
            if (!cell->text_len) continue;
            if (cell->text_len > SIZE_MAX - text_len) return GHOSTTY_OUT_OF_MEMORY;
            TRY(reserve((void **)&t->text, &t->text_cap, text_len + cell->text_len, sizeof(uint32_t)));
            TRY(ghostty_render_state_row_cells_get(t->cells, GHOSTTY_RENDER_STATE_ROW_CELLS_DATA_GRAPHEMES_BUF, t->text + text_len));
            text_len += cell->text_len;
        }
    }
    if (i != count) return GHOSTTY_INVALID_VALUE;
    out->cells = t->snapshot_cells;
    out->text = t->text;
    out->text_len = text_len;
    return GHOSTTY_SUCCESS;
}

GhosttyResult bridge_key(bridge_terminal *t, GhosttyKey key, GhosttyMods mods,
                         uint32_t codepoint, const char *text, size_t len,
                         bool repeat, bridge_bytes *out) {
    TRY(reserve((void **)&t->encoded, &t->encoded_cap, 128, 1));
    GhosttyKeyEvent event;
    TRY(ghostty_key_event_new(NULL, &event));
    ghostty_key_event_set_action(event, repeat ? GHOSTTY_KEY_ACTION_REPEAT : GHOSTTY_KEY_ACTION_PRESS);
    ghostty_key_event_set_key(event, key);
    ghostty_key_event_set_mods(event, mods);
    ghostty_key_event_set_consumed_mods(event, len ? mods & GHOSTTY_MODS_SHIFT : 0);
    ghostty_key_event_set_unshifted_codepoint(event, codepoint);
    ghostty_key_event_set_utf8(event, text, len);
    /* Read modes per event: a child TUI may change cursor/key protocols between
       keystrokes, so caching encoder settings would send the wrong bytes. */
    ghostty_key_encoder_setopt_from_terminal(t->keys, t->terminal);
    GhosttyOptionAsAlt alt = GHOSTTY_OPTION_AS_ALT_TRUE;
    ghostty_key_encoder_setopt(t->keys, GHOSTTY_KEY_ENCODER_OPT_MACOS_OPTION_AS_ALT, &alt);
    size_t written = 0;
    GhosttyResult r = ghostty_key_encoder_encode(t->keys, event, t->encoded, t->encoded_cap, &written);
    if (r == GHOSTTY_OUT_OF_SPACE) {
        r = reserve((void **)&t->encoded, &t->encoded_cap, written, 1);
        if (r == GHOSTTY_SUCCESS)
            r = ghostty_key_encoder_encode(t->keys, event, t->encoded, t->encoded_cap, &written);
    }
    ghostty_key_event_free(event);
    if (r == GHOSTTY_SUCCESS) *out = (bridge_bytes){.data = t->encoded, .len = written};
    return r;
}

GhosttyResult bridge_paste(bridge_terminal *t, char *text, size_t len, bridge_bytes *out) {
    bool bracketed;
    TRY(ghostty_terminal_mode_get(t->terminal, GHOSTTY_MODE_BRACKETED_PASTE, &bracketed));
    if (len > SIZE_MAX - 12) return GHOSTTY_OUT_OF_MEMORY;
    TRY(reserve((void **)&t->encoded, &t->encoded_cap, len + 12, 1));
    size_t written = 0;
    TRY(ghostty_paste_encode(text, len, bracketed, t->encoded, t->encoded_cap, &written));
    *out = (bridge_bytes){.data = t->encoded, .len = written};
    return GHOSTTY_SUCCESS;
}

GhosttyResult bridge_focus(bridge_terminal *t, bool focused, bridge_bytes *out) {
    bool enabled;
    TRY(ghostty_terminal_mode_get(t->terminal, GHOSTTY_MODE_FOCUS_EVENT, &enabled));
    *out = (bridge_bytes){0};
    if (!enabled) return GHOSTTY_SUCCESS;
    TRY(reserve((void **)&t->encoded, &t->encoded_cap, 3, 1));
    size_t written = 0;
    TRY(ghostty_focus_encode(focused ? GHOSTTY_FOCUS_GAINED : GHOSTTY_FOCUS_LOST,
                            t->encoded, t->encoded_cap, &written));
    *out = (bridge_bytes){.data = t->encoded, .len = written};
    return GHOSTTY_SUCCESS;
}

GhosttyResult bridge_mouse(bridge_terminal *t, GhosttyMouseAction action,
                           GhosttyMouseButton button, GhosttyMods mods, int x, int y,
                           bridge_bytes *out) {
    *out = (bridge_bytes){0};
    if (x < 0 || y < 0 || x >= t->cols || y >= t->height) return GHOSTTY_SUCCESS;
    bool pixel_mode;
    TRY(ghostty_terminal_mode_get(t->terminal, GHOSTTY_MODE_SGR_PIXELS_MOUSE, &pixel_mode));
    if (pixel_mode) return GHOSTTY_NO_VALUE;
    TRY(reserve((void **)&t->encoded, &t->encoded_cap, 128, 1));
    GhosttyMouseEvent event;
    TRY(ghostty_mouse_event_new(NULL, &event));
    ghostty_mouse_event_set_action(event, action);
    if (button != GHOSTTY_MOUSE_BUTTON_UNKNOWN) ghostty_mouse_event_set_button(event, button);
    ghostty_mouse_event_set_mods(event, mods);
    ghostty_mouse_event_set_position(event, (GhosttyMousePosition){.x = x, .y = y});
    bool wheel = button >= GHOSTTY_MOUSE_BUTTON_FOUR && button <= GHOSTTY_MOUSE_BUTTON_SEVEN;
    if (!wheel) {
        uint16_t bit = button == GHOSTTY_MOUSE_BUTTON_UNKNOWN ? 0 : (uint16_t)(1u << button);
        if (action == GHOSTTY_MOUSE_ACTION_PRESS) t->pressed_buttons |= bit;
        if (action == GHOSTTY_MOUSE_ACTION_RELEASE) t->pressed_buttons &= (uint16_t)~bit;
        if (button == GHOSTTY_MOUSE_BUTTON_UNKNOWN) t->pressed_buttons = 0;
        if (action == GHOSTTY_MOUSE_ACTION_MOTION) t->pressed_buttons |= bit;
    }
    bool pressed = t->pressed_buttons != 0;
    /* Unit cells are an encoder coordinate system only; never reported as pixels. */
    GhosttyMouseEncoderSize size = GHOSTTY_INIT_SIZED(GhosttyMouseEncoderSize);
    size.screen_width = t->cols;
    size.screen_height = t->height;
    size.cell_width = size.cell_height = 1;
    ghostty_mouse_encoder_setopt_from_terminal(t->mouse, t->terminal);
    ghostty_mouse_encoder_setopt(t->mouse, GHOSTTY_MOUSE_ENCODER_OPT_SIZE, &size);
    ghostty_mouse_encoder_setopt(t->mouse, GHOSTTY_MOUSE_ENCODER_OPT_ANY_BUTTON_PRESSED, &pressed);
    size_t written = 0;
    GhosttyResult r = ghostty_mouse_encoder_encode(t->mouse, event, t->encoded, t->encoded_cap, &written);
    ghostty_mouse_event_free(event);
    if (r == GHOSTTY_SUCCESS) *out = (bridge_bytes){.data = t->encoded, .len = written};
    return r;
}
