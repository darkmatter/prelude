package main

/*
#cgo pkg-config: libghostty-vt
#include "bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"image/color"
	"unicode"
	"unicode/utf8"
	"unsafe"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// terminalFrame is a complete Go-owned viewport, not a view into native buffers.
// Text, widths, styles, and cursor state travel together so View needs no native
// calls and cannot retain memory that a later Write/Snapshot would invalidate.
type terminalFrame struct {
	Cols, Rows    int
	Cells         []uv.Cell
	CursorX       int
	CursorY       int
	CursorVisible bool
	CursorStyle   int // 0 block, 1 bar, 2 underline
	CursorBlink   bool
	Title         string
	AltScreen     bool
	MouseTracking bool
}

// The host event loop exclusively owns this adapter; snapshots and returned bytes
// are Go-owned, but the terminal itself must not be used concurrently.
type terminal struct {
	native *C.bridge_terminal
}

func newTerminal(cols, rows int) (*terminal, error) {
	if err := terminalGeometry(cols, rows); err != nil {
		return nil, err
	}
	var native *C.bridge_terminal
	if err := terminalResult("create", C.bridge_new(C.uint16_t(cols), C.uint16_t(rows), &native)); err != nil {
		return nil, err
	}
	return &terminal{native: native}, nil
}

func terminalGeometry(cols, rows int) error {
	if cols < 1 || rows < 1 || cols > 65535 || rows > 65535 {
		return fmt.Errorf("ghostty terminal geometry must be in 1..65535 cells: %dx%d", cols, rows)
	}
	return nil
}

func terminalResult(operation string, result C.GhosttyResult) error {
	if result == C.GHOSTTY_SUCCESS {
		return nil
	}
	return fmt.Errorf("ghostty terminal %s failed (native result %d)", operation, int(result))
}

func (t *terminal) open() error {
	if t == nil || t.native == nil {
		return errors.New("ghostty terminal is closed")
	}
	return nil
}

// The bridge reuses reply/encoder storage on subsequent calls. Returning a
// borrowed slice would silently change input or replies retained by Go callers.
func cloneTerminalBytes(data C.bridge_bytes) []byte {
	if data.len == 0 {
		return nil
	}
	return bytes.Clone(unsafe.Slice((*byte)(unsafe.Pointer(data.data)), int(data.len)))
}

// Write consumes child output, not keystrokes. Returned bytes are terminal
// answers going in the reverse direction; the host must send them to the PTY
// so programs waiting for cursor/mode/color reports can make progress.
func (t *terminal) Write(data []byte) ([]byte, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	var input unsafe.Pointer
	if len(data) != 0 {
		input = C.CBytes(data)
		defer C.free(input)
	}
	var output C.bridge_bytes
	result := C.bridge_write(t.native, (*C.char)(input), C.size_t(len(data)), &output)
	return cloneTerminalBytes(output), terminalResult("write", result)
}

// Mode-2048 resize replies are queued until Write, including an empty Write.
func (t *terminal) Resize(cols, rows int) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := terminalGeometry(cols, rows); err != nil {
		return err
	}
	return terminalResult("resize", C.bridge_resize(t.native, C.uint16_t(cols), C.uint16_t(rows)))
}

// An unset color inherits the outer terminal's defaults; it is not black.
// Keeping nil distinct from explicit RGB avoids repainting the shell as chrome.
func terminalColor(value C.bridge_color) color.Color {
	if !bool(value.set) {
		return nil
	}
	return color.RGBA{R: uint8(value.rgb.r), G: uint8(value.rgb.g), B: uint8(value.rgb.b), A: 255}
}

// Snapshot copies the bridge's borrowed cells, graphemes, and title before any
// later native call can resize/reuse their storage. No C pointers escape in the
// returned frame, including strings that refer to native text arenas.
func (t *terminal) Snapshot() (terminalFrame, error) {
	if err := t.open(); err != nil {
		return terminalFrame{}, err
	}
	var native C.bridge_frame
	if err := terminalResult("snapshot", C.bridge_snapshot(t.native, &native)); err != nil {
		return terminalFrame{}, err
	}
	frame := terminalFrame{
		Cols: int(native.cols), Rows: int(native.rows),
		CursorX: int(native.cursor_x), CursorY: int(native.cursor_y),
		CursorVisible: bool(native.cursor_visible), CursorStyle: int(native.cursor_style),
		CursorBlink: bool(native.cursor_blink), AltScreen: bool(native.alt_screen),
		MouseTracking: bool(native.mouse_tracking),
		Title:         string(unsafe.Slice((*byte)(unsafe.Pointer(native.title.ptr)), int(native.title.len))),
	}
	frame.Cells = make([]uv.Cell, frame.Cols*frame.Rows)
	cells := unsafe.Slice(native.cells, len(frame.Cells))
	text := unsafe.Slice(native.text, int(native.text_len))
	for i, source := range cells {
		cell := &frame.Cells[i]
		cell.Width = int(source.width)
		if cell.Width != 0 {
			cell.Content = " "
			if source.text_len != 0 {
				start, length := int(source.text_offset), int(source.text_len)
				runes := make([]rune, length)
				for j := range runes {
					runes[j] = rune(text[start+j])
				}
				cell.Content = string(runes)
			}
		}
		cell.Style = uv.Style{
			Fg: terminalColor(source.fg), Bg: terminalColor(source.bg),
			UnderlineColor: terminalColor(source.underline), Underline: uv.Underline(source.style.underline),
		}
		for _, attribute := range []struct {
			enabled C.bool
			mask    uint8
		}{
			{source.style.bold, uv.AttrBold}, {source.style.faint, uv.AttrFaint},
			{source.style.italic, uv.AttrItalic}, {source.style.blink, uv.AttrBlink},
			{source.style.inverse, uv.AttrReverse}, {source.style.invisible, uv.AttrConceal},
			{source.style.strikethrough, uv.AttrStrikethrough},
		} {
			if bool(attribute.enabled) {
				cell.Style.Attrs |= attribute.mask
			}
		}
		// UV has no overline attribute; all representable Ghostty styles are copied.
	}
	return frame, nil
}

func terminalMods(mods tea.KeyMod) (C.GhosttyMods, error) {
	const supported = uv.ModShift | uv.ModCtrl | uv.ModAlt | uv.ModSuper | uv.ModCapsLock | uv.ModNumLock
	if mods & ^supported != 0 {
		return 0, fmt.Errorf("ghostty terminal cannot encode modifier mask %#x", int(mods))
	}
	var native C.GhosttyMods
	for _, modifier := range []struct {
		mask tea.KeyMod
		flag C.GhosttyMods
	}{
		{uv.ModShift, C.GHOSTTY_MODS_SHIFT}, {uv.ModCtrl, C.GHOSTTY_MODS_CTRL},
		{uv.ModAlt, C.GHOSTTY_MODS_ALT}, {uv.ModSuper, C.GHOSTTY_MODS_SUPER},
		{uv.ModCapsLock, C.GHOSTTY_MODS_CAPS_LOCK}, {uv.ModNumLock, C.GHOSTTY_MODS_NUM_LOCK},
	} {
		if mods&modifier.mask != 0 {
			native |= modifier.flag
		}
	}
	return native, nil
}

var terminalKeys = map[rune]C.GhosttyKey{
	uv.KeyEnter: C.GHOSTTY_KEY_ENTER, uv.KeyTab: C.GHOSTTY_KEY_TAB,
	uv.KeyEscape: C.GHOSTTY_KEY_ESCAPE, uv.KeyBackspace: C.GHOSTTY_KEY_BACKSPACE,
	uv.KeyUp: C.GHOSTTY_KEY_ARROW_UP, uv.KeyDown: C.GHOSTTY_KEY_ARROW_DOWN,
	uv.KeyLeft: C.GHOSTTY_KEY_ARROW_LEFT, uv.KeyRight: C.GHOSTTY_KEY_ARROW_RIGHT,
	uv.KeyHome: C.GHOSTTY_KEY_HOME, uv.KeyEnd: C.GHOSTTY_KEY_END,
	uv.KeyInsert: C.GHOSTTY_KEY_INSERT, uv.KeyDelete: C.GHOSTTY_KEY_DELETE,
	uv.KeyPgUp: C.GHOSTTY_KEY_PAGE_UP, uv.KeyPgDown: C.GHOSTTY_KEY_PAGE_DOWN,
	uv.KeyKpEnter: C.GHOSTTY_KEY_NUMPAD_ENTER, uv.KeyKpEqual: C.GHOSTTY_KEY_NUMPAD_EQUAL,
	uv.KeyKpMultiply: C.GHOSTTY_KEY_NUMPAD_MULTIPLY, uv.KeyKpPlus: C.GHOSTTY_KEY_NUMPAD_ADD,
	uv.KeyKpComma: C.GHOSTTY_KEY_NUMPAD_COMMA, uv.KeyKpMinus: C.GHOSTTY_KEY_NUMPAD_SUBTRACT,
	uv.KeyKpDecimal: C.GHOSTTY_KEY_NUMPAD_DECIMAL, uv.KeyKpDivide: C.GHOSTTY_KEY_NUMPAD_DIVIDE,
	uv.KeyKpSep: C.GHOSTTY_KEY_NUMPAD_SEPARATOR, uv.KeyKpBegin: C.GHOSTTY_KEY_NUMPAD_BEGIN,
	uv.KeyKpUp: C.GHOSTTY_KEY_NUMPAD_UP, uv.KeyKpDown: C.GHOSTTY_KEY_NUMPAD_DOWN,
	uv.KeyKpLeft: C.GHOSTTY_KEY_NUMPAD_LEFT, uv.KeyKpRight: C.GHOSTTY_KEY_NUMPAD_RIGHT,
	uv.KeyKpHome: C.GHOSTTY_KEY_NUMPAD_HOME, uv.KeyKpEnd: C.GHOSTTY_KEY_NUMPAD_END,
	uv.KeyKpPgUp: C.GHOSTTY_KEY_NUMPAD_PAGE_UP, uv.KeyKpPgDown: C.GHOSTTY_KEY_NUMPAD_PAGE_DOWN,
	uv.KeyKpInsert: C.GHOSTTY_KEY_NUMPAD_INSERT, uv.KeyKpDelete: C.GHOSTTY_KEY_NUMPAD_DELETE,
	' ': C.GHOSTTY_KEY_SPACE,
	'`': C.GHOSTTY_KEY_BACKQUOTE, '~': C.GHOSTTY_KEY_BACKQUOTE,
	'\\': C.GHOSTTY_KEY_BACKSLASH, '|': C.GHOSTTY_KEY_BACKSLASH,
	'[': C.GHOSTTY_KEY_BRACKET_LEFT, '{': C.GHOSTTY_KEY_BRACKET_LEFT,
	']': C.GHOSTTY_KEY_BRACKET_RIGHT, '}': C.GHOSTTY_KEY_BRACKET_RIGHT,
	',': C.GHOSTTY_KEY_COMMA, '<': C.GHOSTTY_KEY_COMMA,
	'.': C.GHOSTTY_KEY_PERIOD, '>': C.GHOSTTY_KEY_PERIOD,
	'=': C.GHOSTTY_KEY_EQUAL, '+': C.GHOSTTY_KEY_EQUAL,
	'-': C.GHOSTTY_KEY_MINUS, '_': C.GHOSTTY_KEY_MINUS,
	'\'': C.GHOSTTY_KEY_QUOTE, '"': C.GHOSTTY_KEY_QUOTE,
	';': C.GHOSTTY_KEY_SEMICOLON, ':': C.GHOSTTY_KEY_SEMICOLON,
	'/': C.GHOSTTY_KEY_SLASH, '?': C.GHOSTTY_KEY_SLASH,
	')': C.GHOSTTY_KEY_DIGIT_0, '!': C.GHOSTTY_KEY_DIGIT_1, '@': C.GHOSTTY_KEY_DIGIT_2,
	'#': C.GHOSTTY_KEY_DIGIT_3, '$': C.GHOSTTY_KEY_DIGIT_4, '%': C.GHOSTTY_KEY_DIGIT_5,
	'^': C.GHOSTTY_KEY_DIGIT_6, '&': C.GHOSTTY_KEY_DIGIT_7, '*': C.GHOSTTY_KEY_DIGIT_8,
	'(': C.GHOSTTY_KEY_DIGIT_9,
}

func terminalKeyCode(code rune) (C.GhosttyKey, error) {
	if key, ok := terminalKeys[code]; ok {
		return key, nil
	}
	code = unicode.ToLower(code)
	switch {
	case code >= 'a' && code <= 'z':
		return C.GHOSTTY_KEY_A + C.GhosttyKey(code-'a'), nil
	case code >= '0' && code <= '9':
		return C.GHOSTTY_KEY_DIGIT_0 + C.GhosttyKey(code-'0'), nil
	case code >= uv.KeyF1 && code <= uv.KeyF25:
		return C.GHOSTTY_KEY_F1 + C.GhosttyKey(code-uv.KeyF1), nil
	case code >= uv.KeyKp0 && code <= uv.KeyKp9:
		return C.GHOSTTY_KEY_NUMPAD_0 + C.GhosttyKey(code-uv.KeyKp0), nil
	case code == 0 || code == uv.KeyExtended || unicode.IsPrint(code):
		return C.GHOSTTY_KEY_UNIDENTIFIED, nil
	default:
		return 0, fmt.Errorf("ghostty terminal cannot encode key code %U", code)
	}
}

func (t *terminal) Key(key tea.Key) ([]byte, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	mods, err := terminalMods(key.Mod)
	if err != nil {
		return nil, err
	}
	// Key identity and printable text are separate. Keep both: shifted
	// characters and composed Unicode cannot be recovered from Code alone.
	physical := key.Code
	if key.BaseCode != 0 {
		physical = key.BaseCode
	}
	code, err := terminalKeyCode(physical)
	if err != nil {
		return nil, err
	}
	text := key.Text
	if text == "" && unicode.IsPrint(key.Code) {
		text = string(key.Code)
		if unicode.IsPrint(key.ShiftedCode) {
			text = string(key.ShiftedCode)
		}
	}
	if !utf8.ValidString(text) {
		return nil, errors.New("ghostty terminal key text is not valid UTF-8")
	}
	for _, r := range text {
		if r < 32 || r == 127 {
			return nil, errors.New("ghostty terminal key text must not contain control characters")
		}
	}
	unshifted := unicode.ToLower(key.Code)
	if unshifted == uv.KeyExtended || unshifted == 0 {
		unshifted, _ = utf8.DecodeRuneInString(text)
	}
	if unshifted > unicode.MaxRune || unshifted == utf8.RuneError {
		unshifted = 0
	}
	input := C.CString(text)
	defer C.free(unsafe.Pointer(input))
	var output C.bridge_bytes
	result := C.bridge_key(t.native, code, mods, C.uint32_t(unshifted), input,
		C.size_t(len(text)), C.bool(key.IsRepeat), &output)
	return cloneTerminalBytes(output), terminalResult("key", result)
}

func (t *terminal) Paste(text string) ([]byte, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	input := C.CString(text)
	defer C.free(unsafe.Pointer(input))
	var output C.bridge_bytes
	result := C.bridge_paste(t.native, input, C.size_t(len(text)), &output)
	return cloneTerminalBytes(output), terminalResult("paste", result)
}

func (t *terminal) Focus(focused bool) ([]byte, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	var output C.bridge_bytes
	result := C.bridge_focus(t.native, C.bool(focused), &output)
	return cloneTerminalBytes(output), terminalResult("focus", result)
}

func (t *terminal) Mouse(message tea.MouseMsg) ([]byte, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if message == nil {
		return nil, errors.New("ghostty terminal mouse message is nil")
	}
	mouse := message.Mouse()
	mods, err := terminalMods(mouse.Mod)
	if err != nil {
		return nil, err
	}
	action := C.GhosttyMouseAction(C.GHOSTTY_MOUSE_ACTION_PRESS)
	switch message.(type) {
	case tea.MouseClickMsg, tea.MouseWheelMsg:
	case tea.MouseReleaseMsg:
		action = C.GHOSTTY_MOUSE_ACTION_RELEASE
	case tea.MouseMotionMsg:
		action = C.GHOSTTY_MOUSE_ACTION_MOTION
	default:
		return nil, fmt.Errorf("ghostty terminal cannot encode mouse message %T", message)
	}
	buttons := map[tea.MouseButton]C.GhosttyMouseButton{
		tea.MouseNone: C.GHOSTTY_MOUSE_BUTTON_UNKNOWN, tea.MouseLeft: C.GHOSTTY_MOUSE_BUTTON_LEFT,
		tea.MouseMiddle: C.GHOSTTY_MOUSE_BUTTON_MIDDLE, tea.MouseRight: C.GHOSTTY_MOUSE_BUTTON_RIGHT,
		tea.MouseWheelUp: C.GHOSTTY_MOUSE_BUTTON_FOUR, tea.MouseWheelDown: C.GHOSTTY_MOUSE_BUTTON_FIVE,
		tea.MouseWheelLeft: C.GHOSTTY_MOUSE_BUTTON_SIX, tea.MouseWheelRight: C.GHOSTTY_MOUSE_BUTTON_SEVEN,
		tea.MouseBackward: C.GHOSTTY_MOUSE_BUTTON_EIGHT, tea.MouseForward: C.GHOSTTY_MOUSE_BUTTON_NINE,
		uv.MouseButton10: C.GHOSTTY_MOUSE_BUTTON_TEN, uv.MouseButton11: C.GHOSTTY_MOUSE_BUTTON_ELEVEN,
	}
	button, ok := buttons[mouse.Button]
	if !ok {
		return nil, fmt.Errorf("ghostty terminal cannot encode mouse button %d", mouse.Button)
	}
	var output C.bridge_bytes
	result := C.bridge_mouse(t.native, action, button, mods, C.int(mouse.X), C.int(mouse.Y), &output)
	if result == C.GHOSTTY_NO_VALUE {
		return nil, errors.New("ghostty terminal pixel mouse reporting requires pixel geometry unavailable to this host")
	}
	return cloneTerminalBytes(output), terminalResult("mouse", result)
}

func (t *terminal) Close() {
	if t != nil && t.native != nil {
		C.bridge_free(t.native)
		t.native = nil
	}
}
