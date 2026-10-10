#ifndef PRELUDE_GHOSTTY_BRIDGE_H
#define PRELUDE_GHOSTTY_BRIDGE_H

#include <ghostty/vt/key.h>
#include <ghostty/vt/mouse.h>
#include <ghostty/vt/render.h>

/* The bridge owns emulation/encoding, not a PTY or child process. Go owns the
   event loop and OS I/O. Calls are serialized by that loop, not by C locks.
   All returned buffers are C-owned and borrowed until the next adapter
   operation; callers must copy them before retaining a result. */
typedef struct bridge_terminal bridge_terminal;

typedef struct {
    const char *data;
    size_t len;
} bridge_bytes;

/* Unset colors inherit terminal defaults; explicit RGB black is different. */
typedef struct {
    bool set;
    GhosttyColorRgb rgb;
} bridge_color;

/* Text indexes a shared UTF-32 arena, not UTF-8 bytes. A grapheme can contain
   several codepoints but occupy one/two columns; width zero is a wide tail. */
typedef struct {
    size_t text_offset;
    uint32_t text_len;
    int width;
    GhosttyStyle style;
    bridge_color fg, bg, underline;
} bridge_cell;

typedef struct {
    uint16_t cols, rows, cursor_x, cursor_y;
    bool cursor_visible, cursor_blink, alt_screen, mouse_tracking;
    int cursor_style;
    GhosttyString title;
    const bridge_cell *cells;
    const uint32_t *text;
    size_t text_len;
} bridge_frame;

GhosttyResult bridge_new(uint16_t cols, uint16_t rows, bridge_terminal **out);
void bridge_free(bridge_terminal *terminal);
GhosttyResult bridge_write(bridge_terminal *terminal, const char *data, size_t len,
                           bridge_bytes *out);
GhosttyResult bridge_resize(bridge_terminal *terminal, uint16_t cols, uint16_t rows);
GhosttyResult bridge_snapshot(bridge_terminal *terminal, bridge_frame *out);
GhosttyResult bridge_key(bridge_terminal *terminal, GhosttyKey key, GhosttyMods mods,
                         uint32_t codepoint, const char *text, size_t len,
                         bool repeat, bridge_bytes *out);
GhosttyResult bridge_paste(bridge_terminal *terminal, char *text, size_t len,
                           bridge_bytes *out);
GhosttyResult bridge_focus(bridge_terminal *terminal, bool focused, bridge_bytes *out);
GhosttyResult bridge_mouse(bridge_terminal *terminal, GhosttyMouseAction action,
                           GhosttyMouseButton button, GhosttyMods mods, int x, int y,
                           bridge_bytes *out);

#endif
