package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

func TestRendererPromptLeftShift(t *testing.T) {
	const cols, rows, row = 160, 28, 26
	rowText := func(e *vt.Emulator) string {
		var text strings.Builder
		for x := range cols {
			if cell := e.CellAt(x, row); cell != nil {
				text.WriteString(cell.Content)
			} else {
				text.WriteByte(' ')
			}
		}
		return strings.TrimRight(text.String(), " ")
	}
	for _, tc := range []struct {
		name, prompt, input string
		backspace           bool
	}{
		{"CJKBackspace", "界❯", "echo parked-界-h", true},
		{"CJKCSI", "界❯", "echo parked-界-h", false},
		{"Emoji", "🙂❯", "echo parked-界-h", true},
		{"Combining", "e\u0301❯", "echo parked-input-h", true},
		{"ASCII", "X>", "echo parked-input-h", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := tc.prompt + " " + tc.input
			var output bytes.Buffer
			renderer := uv.NewTerminalRenderer(&output, []string{"TERM=xterm-256color"})
			renderer.SetFullscreen(true)
			renderer.SetBackspace(tc.backspace)
			renderer.SetScrollOptim(false)
			drawAt := func(x int) *uv.RenderBuffer {
				frame := uv.NewScreenBuffer(cols, rows)
				frame.Fill(&uv.EmptyCell)
				uv.NewStyledString(text).Draw(&frame, uv.Rect(x, row, cols-x, 1))
				return frame.RenderBuffer
			}
			actual, reference := vt.NewEmulator(cols, rows), vt.NewEmulator(cols, rows)
			t.Cleanup(func() { _ = actual.Close(); _ = reference.Close() })
			// A direct, correct paint provides the same emulator's cell/cursor
			// baseline, independently of the renderer's incremental repaint.
			if _, err := reference.Write([]byte(fmt.Sprintf("\x1b[%d;1H%s", row+1, text))); err != nil {
				t.Fatal(err)
			}
			for _, x := range []int{80, 0} {
				output.Reset()
				renderer.Render(drawAt(x))
				if err := renderer.Flush(); err != nil {
					t.Fatal(err)
				}
				if _, err := actual.Write(output.Bytes()); err != nil {
					t.Fatal(err)
				}
			}
			if got, want := rowText(actual), rowText(reference); got != want {
				t.Errorf("repainted row %d = %q, want %q; repaint=%q", row, got, want, output.String())
			}
			for x := range ansi.StringWidth(text) {
				got, want := actual.CellAt(x, row), reference.CellAt(x, row)
				if (got == nil) != (want == nil) || got != nil && (got.Content != want.Content || got.Width != want.Width) {
					t.Errorf("repainted cell (%d,%d) = %+v, want %+v", x, row, got, want)
					break
				}
			}
			if got, want := actual.CursorPosition(), reference.CursorPosition(); got != want {
				t.Errorf("repainted cursor = %v, want %v", got, want)
			}
			// x/vt drops decomposed accents when parsing. The public output must
			// nevertheless retain the combining cluster for a real terminal.
			if strings.Contains(tc.prompt, "e\u0301") && !strings.Contains(output.String(), "e\u0301") {
				t.Errorf("repaint lost combining cluster: %q", output.String())
			}
		})
	}
}
