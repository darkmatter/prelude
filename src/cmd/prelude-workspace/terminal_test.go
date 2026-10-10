package main

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

func ghosttyTestTerminal(t *testing.T, cols, rows int) *terminal {
	t.Helper()
	term, err := newTerminal(cols, rows)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(term.Close)
	return term
}

func ghosttyWrite(t *testing.T, term *terminal, text string) []byte {
	t.Helper()
	replies, err := term.Write([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return replies
}

func ghosttySnapshot(t *testing.T, term *terminal) terminalFrame {
	t.Helper()
	frame, err := term.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.Cells) != frame.Cols*frame.Rows {
		t.Fatalf("snapshot has %d cells for %dx%d", len(frame.Cells), frame.Cols, frame.Rows)
	}
	return frame
}

func TestGhosttyDefaultCombiningAccent(t *testing.T) {
	// Do not negotiate mode 2027: ordinary combining marks work in default mode.
	for name, writes := range map[string][]string{
		"one write":       {"e\u0301"},
		"split grapheme":  {"e", "\u0301"},
		"split UTF-8":     {"e", "\xcc", "\x81"},
		"cursor movement": {"e\u0301", "\x1b[3;1H"},
	} {
		t.Run(name, func(t *testing.T) {
			term := ghosttyTestTerminal(t, 80, 24)
			if got := string(ghosttyWrite(t, term, "\x1b[?2027$p")); got != "\x1b[?2027;2$y" {
				t.Fatalf("expected default grapheme mode to be reset, got %q", got)
			}
			_ = ghosttySnapshot(t, term)
			for _, write := range writes {
				ghosttyWrite(t, term, write)
				_ = ghosttySnapshot(t, term)
			}
			for range 2 {
				frame := ghosttySnapshot(t, term)
				if got := frame.Cells[0]; got.Content != "e\u0301" || got.Width != 1 {
					t.Fatalf("default-mode cell = %q (%U), width %d; want e+U+0301, width 1", got.Content, []rune(got.Content), got.Width)
				}
			}
		})
	}
}

func TestGhosttyDefaultUnicodeSmokeFrame(t *testing.T) {
	output := "\x1b[2J\x1b[H\x1b[3;1HVISIBLE:界e\u0301🙂END\x1b[12;40HUNDERLAY-CONTENT\x1b[18;1H"
	for _, fragmented := range []bool{false, true} {
		t.Run(fmt.Sprintf("fragmented=%v", fragmented), func(t *testing.T) {
			term := ghosttyTestTerminal(t, 80, 24)
			_ = ghosttySnapshot(t, term)
			if fragmented {
				for _, b := range []byte(output) {
					ghosttyWrite(t, term, string([]byte{b}))
					_ = ghosttySnapshot(t, term)
				}
			} else {
				ghosttyWrite(t, term, output)
			}
			frame := ghosttySnapshot(t, term)
			accent := frame.Cells[2*frame.Cols+10]
			if accent.Content != "e\u0301" || accent.Width != 1 {
				t.Fatalf("smoke accent cell = %q (%U), width %d", accent.Content, []rune(accent.Content), accent.Width)
			}
			if frame.CursorX != 0 || frame.CursorY != 17 {
				t.Fatalf("smoke cursor = %d,%d", frame.CursorX, frame.CursorY)
			}
			buffer := uv.NewScreenBuffer(frame.Cols, frame.Rows)
			buffer.Fill(&uv.EmptyCell)
			for y := range frame.Rows {
				for x := 0; x < frame.Cols; {
					cell := frame.Cells[y*frame.Cols+x]
					if cell.Width <= 0 {
						x++
						continue
					}
					buffer.SetCell(x, y, &cell)
					x += cell.Width
				}
			}
			// Check the cell-rendering boundary before BubbleTea's independent string reparse.
			if raw := buffer.Render(); !strings.Contains(raw, "VISIBLE:界e\u0301🙂END") || !strings.Contains(raw, "UNDERLAY-CONTENT") {
				t.Fatalf("cell-buffer rendering lost smoke text: %q", raw)
			}
		})
	}
}

func TestGhosttyStyledGraphemesAndOwnedSnapshots(t *testing.T) {
	term := ghosttyTestTerminal(t, 12, 3)
	// Joined emoji use the terminal's negotiated grapheme-cluster mode.
	ghosttyWrite(t, term, "\x1b[?2027h\x1b]2;workspace title\x1b\\A\x1b[1;2;3;4:3;5;7;8;9;38;2;12;34;56;48;2;65;43;21;58;2;7;8;9mé\x1b[0me\u0301界👩‍💻")
	frame := ghosttySnapshot(t, term)
	want := []struct {
		text  string
		width int
	}{{"A", 1}, {"é", 1}, {"e\u0301", 1}, {"界", 2}, {"", 0}, {"👩‍💻", 2}, {"", 0}}
	for i, expected := range want {
		if got := frame.Cells[i]; got.Content != expected.text || got.Width != expected.width {
			t.Fatalf("cell %d = %q width %d, want %q width %d", i, got.Content, got.Width, expected.text, expected.width)
		}
	}
	if frame.Cells[0].Style.Fg != nil || frame.Cells[0].Style.Bg != nil {
		t.Fatal("default colors must remain nil")
	}
	style := frame.Cells[1].Style
	if style.Fg != (color.RGBA{12, 34, 56, 255}) || style.Bg != (color.RGBA{65, 43, 21, 255}) ||
		style.UnderlineColor != (color.RGBA{7, 8, 9, 255}) || style.Underline != uv.UnderlineCurly {
		t.Fatalf("styled cell = %+v", style)
	}
	attrs := uint8(uv.AttrBold | uv.AttrFaint | uv.AttrItalic | uv.AttrBlink | uv.AttrReverse | uv.AttrConceal | uv.AttrStrikethrough)
	if style.Attrs != attrs {
		t.Fatalf("attributes = %#x, want %#x", style.Attrs, attrs)
	}
	if frame.Title != "workspace title" {
		t.Fatalf("title = %q", frame.Title)
	}
	ghosttyWrite(t, term, "\x1b[2J\x1b[Hchanged\x1b]2;new title\x07")
	_ = ghosttySnapshot(t, term)
	if frame.Cells[1].Content != "é" || frame.Title != "workspace title" {
		t.Fatal("a later write/snapshot mutated the owned snapshot")
	}
	ghosttyWrite(t, term, "\x1b[2J\x1b[H\x1b[48;2;1;2;3m\x1b[K")
	blank := ghosttySnapshot(t, term).Cells[0]
	if blank.Content != " " || blank.Width != 1 || blank.Style.Bg != (color.RGBA{1, 2, 3, 255}) {
		t.Fatalf("background-only erased cell = %+v", blank)
	}
}

func TestGhosttyResizeReflowAndAlternateScreen(t *testing.T) {
	term := ghosttyTestTerminal(t, 8, 4)
	ghosttyWrite(t, term, "abcdef界XYZ")
	if err := term.Resize(4, 4); err != nil {
		t.Fatal(err)
	}
	frame := ghosttySnapshot(t, term)
	var text strings.Builder
	for _, cell := range frame.Cells {
		if cell.Width != 0 {
			text.WriteString(cell.Content)
		}
	}
	if got := text.String(); !strings.HasPrefix(got, "abcdef界XYZ") {
		t.Fatalf("reflow lost/reordered content: %q", got)
	}
	before := frame
	ghosttyWrite(t, term, "\x1b[?1049h\x1b[Halt")
	alt := ghosttySnapshot(t, term)
	if !alt.AltScreen || alt.Cells[0].Content != "a" || alt.Cells[1].Content != "l" {
		t.Fatal("alternate screen did not activate")
	}
	ghosttyWrite(t, term, "\x1b[?1049l")
	after := ghosttySnapshot(t, term)
	if after.AltScreen || after.CursorX != before.CursorX || after.CursorY != before.CursorY {
		t.Fatal("primary screen/cursor was not restored")
	}
	for i := range before.Cells {
		if !before.Cells[i].Equal(&after.Cells[i]) {
			t.Fatalf("primary cell %d was not restored", i)
		}
	}
	ghosttyWrite(t, term, "\x1b[2J\x1b[Habc界")
	wrapped := ghosttySnapshot(t, term)
	if wrapped.Cells[3].Content != " " || wrapped.Cells[3].Width != 1 ||
		wrapped.Cells[4].Content != "界" || wrapped.Cells[4].Width != 2 || wrapped.Cells[5].Width != 0 {
		t.Fatal("wide wrap-head spacer or tail continuation was not preserved")
	}
}

func TestGhosttyRepliesCursorAndCapabilities(t *testing.T) {
	term := ghosttyTestTerminal(t, 10, 4)
	frame := ghosttySnapshot(t, term)
	if !frame.CursorVisible || frame.CursorStyle != 1 || !frame.CursorBlink {
		t.Fatalf("initial cursor must be a blinking bar: %+v", frame)
	}
	replies := ghosttyWrite(t, term, "\x1b[2;3H\x1b[5n\x1b[6n\x1b[18t\x1b[14t\x1b[16t\x1b[c")
	want := "\x1b[0n\x1b[2;3R\x1b[8;4;10t\x1b[4;0;0t\x1b[6;0;0t\x1b[?62;22c"
	if string(replies) != want {
		t.Fatalf("replies = %q, want %q", replies, want)
	}
	if extra := ghosttyWrite(t, term, ""); len(extra) != 0 {
		t.Fatalf("replies were not drained: %q", extra)
	}
	ghosttyWrite(t, term, "\x1b[6 q\x1b[?25l\x1b[?1000h")
	frame = ghosttySnapshot(t, term)
	if frame.CursorX != 2 || frame.CursorY != 1 || frame.CursorVisible ||
		frame.CursorStyle != 1 || frame.CursorBlink || !frame.MouseTracking {
		t.Fatalf("cursor/mouse state = %+v", frame)
	}
	ghosttyWrite(t, term, "\x1b[3 q\x1b[?25h")
	frame = ghosttySnapshot(t, term)
	if !frame.CursorVisible || frame.CursorStyle != 2 || !frame.CursorBlink {
		t.Fatal("blinking underline cursor was not mapped")
	}
	ghosttyWrite(t, term, "\x1b[?2048h")
	if err := term.Resize(12, 5); err != nil {
		t.Fatal(err)
	}
	if got := string(ghosttyWrite(t, term, "")); got != "\x1b[48;5;12;0;0t" {
		t.Fatalf("queued resize reply = %q", got)
	}
	if string(replies) != want {
		t.Fatal("returned replies retained the C buffer")
	}
	for _, switchScreen := range []string{"", "\x1b[?1049h", "\x1b[?1049l", "\x1b[!p", "\x1bc"} {
		// Keep the screen change and query in one write to exercise the capability boundary.
		query := "\x1b_Gi=31,a=q,t=d,f=24,s=1,v=1;AAAA\x1b\\\x1b]52;c;?\x1b\\"
		if got := ghosttyWrite(t, term, switchScreen+query); len(got) != 0 {
			t.Fatalf("graphics/clipboard query after %q = %q", switchScreen, got)
		}
	}
	ghosttyWrite(t, term, "\x1b")
	if got := ghosttyWrite(t, term, "c\x1b_Gi=31,a=q,t=d,f=24,s=1,v=1;AAAA\x1b\\"); len(got) != 0 {
		t.Fatalf("graphics query after fragmented RIS = %q", got)
	}
}

func TestGhosttyInputEncoders(t *testing.T) {
	term := ghosttyTestTerminal(t, 10, 4)
	for _, test := range []struct {
		key  tea.Key
		want string
	}{
		{tea.Key{Code: 'é', Text: "é"}, "é"},
		{tea.Key{Code: uv.KeyExtended, Text: "e\u0301"}, "e\u0301"},
		{tea.Key{Code: 'x', Text: "x", Mod: uv.ModAlt}, "\x1bx"},
		{tea.Key{Code: 'c', Mod: uv.ModCtrl}, "\x03"},
		{tea.Key{Code: ' ', Mod: uv.ModCtrl}, "\x00"},
		{tea.Key{Code: 'a', ShiftedCode: 'A', Text: "A", Mod: uv.ModShift}, "A"},
		{tea.Key{Code: uv.KeyUp}, "\x1b[A"},
		{tea.Key{Code: uv.KeyEnter}, "\r"},
		{tea.Key{Code: uv.KeyBackspace}, "\x7f"},
	} {
		got, err := term.Key(test.key)
		if err != nil || string(got) != test.want {
			t.Fatalf("key %+v = %q, %v; want %q", test.key, got, err, test.want)
		}
	}
	ghosttyWrite(t, term, "\x1b[?1h")
	if got, err := term.Key(tea.Key{Code: uv.KeyUp}); err != nil || string(got) != "\x1bOA" {
		t.Fatalf("application cursor key = %q, %v", got, err)
	}
	if got, err := term.Paste("one\ntwo"); err != nil || string(got) != "one\rtwo" {
		t.Fatalf("ordinary paste = %q, %v", got, err)
	}
	if got, err := term.Focus(true); err != nil || len(got) != 0 {
		t.Fatalf("disabled focus report = %q, %v", got, err)
	}
	ghosttyWrite(t, term, "\x1b[?2004h\x1b[?1004h")
	paste, err := term.Paste("one\ntwo")
	if err != nil || string(paste) != "\x1b[200~one\ntwo\x1b[201~" {
		t.Fatalf("bracketed paste = %q, %v", paste, err)
	}
	for focused, want := range map[bool]string{true: "\x1b[I", false: "\x1b[O"} {
		if got, err := term.Focus(focused); err != nil || string(got) != want {
			t.Fatalf("focus %v = %q, %v", focused, got, err)
		}
	}
	if string(paste) != "\x1b[200~one\ntwo\x1b[201~" {
		t.Fatal("encoded input retained a reused C buffer")
	}
	click := tea.MouseClickMsg{X: 2, Y: 1, Button: tea.MouseLeft}
	if got, err := term.Mouse(click); err != nil || len(got) != 0 {
		t.Fatalf("disabled mouse report = %q, %v", got, err)
	}
	ghosttyWrite(t, term, "\x1b[?1000h\x1b[?1006h")
	if got, err := term.Mouse(click); err != nil || string(got) != "\x1b[<0;3;2M" {
		t.Fatalf("SGR mouse press = %q, %v", got, err)
	}
	if got, err := term.Mouse(tea.MouseReleaseMsg(click)); err != nil || string(got) != "\x1b[<0;3;2m" {
		t.Fatalf("SGR mouse release = %q, %v", got, err)
	}
	if got, err := term.Mouse(tea.MouseWheelMsg{X: 2, Y: 1, Button: tea.MouseWheelUp}); err != nil || string(got) != "\x1b[<64;3;2M" {
		t.Fatalf("SGR mouse wheel = %q, %v", got, err)
	}
	ghosttyWrite(t, term, "\x1b[>1u")
	if got, err := term.Key(tea.Key{Code: 'c', Mod: uv.ModCtrl}); err != nil || string(got) != "\x1b[99;5u" {
		t.Fatalf("negotiated Kitty key = %q, %v", got, err)
	}
}

func TestGhosttyGeometryAndClose(t *testing.T) {
	for _, size := range [][2]int{{0, 1}, {1, 0}, {-1, 1}, {65536, 1}} {
		if term, err := newTerminal(size[0], size[1]); err == nil {
			term.Close()
			t.Fatalf("accepted invalid geometry %v", size)
		}
	}
	term := ghosttyTestTerminal(t, 2, 2)
	if err := term.Resize(0, 2); err == nil {
		t.Fatal("accepted zero resize geometry")
	}
	term.Close()
	term.Close()
	if _, err := term.Write(nil); err == nil {
		t.Fatal("write after close succeeded")
	}
	if _, err := term.Snapshot(); err == nil {
		t.Fatal("snapshot after close succeeded")
	}
}
