package main

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
)

// Wide graphemes occupy two columns: the glyph in the first and a zero-width
// placeholder in the second. Composition has to step over the placeholder,
// because writing one back makes ultraviolet blank the glyph it continues.
const (
	wideCJK   = "日本語"
	wideEmoji = "🚀🌍"
)

func TestBlitPreservesWideGraphemes(t *testing.T) {
	source := vt.NewEmulator(12, 2)
	if _, err := source.WriteString(wideCJK + "\r\n" + wideEmoji); err != nil {
		t.Fatalf("write to emulator: %v", err)
	}
	captured := captureSurface(source, 12, 2, pinPaint)

	frame := uv.NewScreenBuffer(12, 2)
	captured.blit(&frame, 0, 0)

	if got := rowText(&frame, 0); !strings.HasPrefix(got, wideCJK) {
		t.Fatalf("CJK row = %q, want it to start with %q", got, wideCJK)
	}
	if got := rowText(&frame, 1); !strings.HasPrefix(got, wideEmoji) {
		t.Fatalf("emoji row = %q, want it to start with %q", got, wideEmoji)
	}
}

func TestBlitKeepsWideCellPlaceholdersIntact(t *testing.T) {
	// The glyph lands in one column and the next stays a zero-width
	// continuation, which is what stops the renderer emitting a stray column.
	source := vt.NewEmulator(6, 1)
	if _, err := source.WriteString("日x"); err != nil {
		t.Fatalf("write to emulator: %v", err)
	}
	frame := uv.NewScreenBuffer(6, 1)
	captureSurface(source, 6, 1, pinPaint).blit(&frame, 0, 0)

	glyph := frame.CellAt(0, 0)
	if glyph == nil || glyph.Content != "日" {
		t.Fatalf("cell 0 = %+v, want the wide glyph", glyph)
	}
	if glyph.Width != 2 {
		t.Fatalf("cell 0 width = %d, want 2", glyph.Width)
	}
	if placeholder := frame.CellAt(1, 0); placeholder == nil || placeholder.Width != 0 {
		t.Fatalf("cell 1 = %+v, want a zero-width continuation", placeholder)
	}
	if next := frame.CellAt(2, 0); next == nil || next.Content != "x" {
		t.Fatalf("cell 2 = %+v, want the following narrow cell", next)
	}
}

func TestBlitBlanksAGraphemeClippedByTheOrigin(t *testing.T) {
	// Blitting a surface whose first column is a continuation cannot show half
	// a glyph, so that column must come out blank rather than corrupt.
	clipped := newSurface(2, 1, uv.Cell{})
	clipped.cells[0] = uv.Cell{} // continuation
	clipped.cells[1] = uv.Cell{Content: "x", Width: 1}

	frame := uv.NewScreenBuffer(2, 1)
	frame.Fill(&uv.Cell{Content: "#", Width: 1})
	clipped.blit(&frame, 0, 0)

	if got := frame.CellAt(0, 0); got == nil || strings.TrimSpace(got.Content) != "" {
		t.Fatalf("clipped column = %+v, want a blank", got)
	}
	if got := frame.CellAt(1, 0); got == nil || got.Content != "x" {
		t.Fatalf("column after the clip = %+v, want the narrow cell", got)
	}
}

func TestShellDrawPreservesWideGraphemes(t *testing.T) {
	// End to end through a real child: the composed band must carry the same
	// glyphs the child painted.
	child := startTestShell(t, 40, 4)

	child.run("printf '" + wideCJK + " " + wideEmoji + "\\n'")
	child.await(t, "the wide output", func(screen string) bool {
		return strings.Count(screen, wideCJK) >= 2
	})

	frame := uv.NewScreenBuffer(40, 4)
	child.draw(&frame, band{top: 0, height: 4}, 0, blankPaint)

	composed := composedText(&frame)
	if !strings.Contains(composed, wideCJK) {
		t.Fatalf("CJK did not survive composition:\n%s", composed)
	}
	if !strings.Contains(composed, wideEmoji) {
		t.Fatalf("emoji did not survive composition:\n%s", composed)
	}
}

func TestTextSurfaceHandlesWideGraphemes(t *testing.T) {
	// Three CJK glyphs need six columns; in five, the last must not be split.
	surface := textSurface([]string{wideCJK}, uv.Style{}, 5, 1)

	frame := uv.NewScreenBuffer(5, 1)
	surface.blit(&frame, 0, 0)

	row := rowText(&frame, 0)
	if !strings.HasPrefix(row, "日本") {
		t.Fatalf("row = %q, want the glyphs that fit", row)
	}
	if strings.Contains(row, "語") {
		t.Fatalf("row = %q, want the overflowing glyph dropped", row)
	}
}

func TestShellDrawKeepsTheTerminalDefaultInTheShellBand(t *testing.T) {
	// A child using the terminal's own colours must reach the frame with them
	// still unset, so the outer terminal resolves them. Forcing the host's
	// palette here would break every light-theme user.
	child := startTestShell(t, 20, 3)
	child.run("printf 'plain\\n'")
	child.await(t, "the child's output", func(screen string) bool {
		return strings.Count(screen, "plain") >= 2
	})

	frame := uv.NewScreenBuffer(20, 3)
	child.draw(&frame, band{top: 0, height: 3}, 0, shellPaint)

	cell := frame.CellAt(0, 0)
	if cell == nil {
		t.Fatal("no cell at 0,0")
	}
	if cell.Style.Bg != nil {
		t.Fatalf("background = %v, want it left unset in the shell band", cell.Style.Bg)
	}
}

func TestShellDrawIsOpaqueWhenDrawnAsAPinnedPane(t *testing.T) {
	// The same child composed into the pin band is chrome, and chrome sits on
	// top of the shell. Any cell that left its colours unset has to adopt the
	// band's, or the user's terminal theme shows through the pane.
	child := startTestShell(t, 20, 3)
	child.run("printf 'plain\\n'")
	child.await(t, "the child's output", func(screen string) bool {
		return strings.Count(screen, "plain") >= 2
	})

	frame := uv.NewScreenBuffer(20, 3)
	child.draw(&frame, band{top: 0, height: 3}, 0, pinPaint)

	for y := range 3 {
		for x := range 20 {
			cell := frame.CellAt(x, y)
			if cell == nil {
				t.Fatalf("cell %d,%d is missing", x, y)
			}
			if cell.Width == 0 {
				continue // trailing half of a wide grapheme
			}
			if cell.Style.Bg != pinBackground {
				t.Fatalf("cell %d,%d (%q) background = %v, want the opaque pin background %v",
					x, y, cell.Content, cell.Style.Bg, pinBackground)
			}
		}
	}
}
