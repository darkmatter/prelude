package main

import (
	"image/color"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

func TestCaptureSurfacePreservesColour(t *testing.T) {
	// The whole reason panels go through a virtual terminal instead of
	// ansi.Strip: styling has to survive the capture.
	emulator := vt.NewEmulator(8, 1)
	if _, err := emulator.WriteString("\x1b[31mRED"); err != nil {
		t.Fatalf("write to emulator: %v", err)
	}

	captured := captureSurface(emulator, 8, 1, pinPaint)

	cell := captured.at(0, 0)
	if cell == nil {
		t.Fatal("no cell at 0,0")
	}
	if cell.Content != "R" {
		t.Fatalf("content = %q, want %q", cell.Content, "R")
	}
	// The child's own colour must win over the band fill, or a panel could
	// never say anything in red.
	if cell.Style.Fg != ansi.Red {
		t.Fatalf("foreground = %v, want the child's red %v", cell.Style.Fg, ansi.Red)
	}
	if cell.Style.Bg != pinBackground {
		t.Fatalf("background = %v, want the band fill %v", cell.Style.Bg, pinBackground)
	}
}

func TestCaptureSurfaceKeepsTheBandOpaque(t *testing.T) {
	// A virtual terminal hands back a blank cell with nil colours for every
	// coordinate the child never touched, and nil means "the renderer's
	// default" — the outer terminal's palette once these cells reach the host
	// frame. Chrome that is meant to be opaque has to carry its own colours.
	emulator := vt.NewEmulator(4, 2)
	if _, err := emulator.WriteString("ab"); err != nil {
		t.Fatalf("write to emulator: %v", err)
	}

	background := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	foreground := color.RGBA{R: 4, G: 5, B: 6, A: 255}
	band := paint{
		fill:   uv.Cell{Content: " ", Width: 1, Style: uv.Style{Fg: foreground, Bg: background}},
		opaque: true,
	}
	captured := captureSurface(emulator, 4, 2, band)

	for y := range 2 {
		for x := range 4 {
			cell := captured.at(x, y)
			if cell == nil {
				t.Fatalf("cell %d,%d is missing from the capture", x, y)
			}
			if cell.Style.Bg != background {
				t.Fatalf("cell %d,%d background = %v, want the fill %v", x, y, cell.Style.Bg, background)
			}
			if cell.Style.Fg != foreground {
				t.Fatalf("cell %d,%d foreground = %v, want the fill %v", x, y, cell.Style.Fg, foreground)
			}
		}
	}

	// The child's text still has to be there; opacity is a backdrop, not a
	// coat of paint over the content.
	if got := captured.at(0, 0).Content; got != "a" {
		t.Fatalf("cell 0,0 = %q, want the child's text", got)
	}
}

func TestCaptureSurfaceLeavesATransparentBandAlone(t *testing.T) {
	// The shell band is not chrome. A child that uses the terminal's own
	// colours has to keep them unset, or the host would force its palette on
	// a user whose terminal is, say, light.
	emulator := vt.NewEmulator(4, 1)
	if _, err := emulator.WriteString("ab"); err != nil {
		t.Fatalf("write to emulator: %v", err)
	}

	captured := captureSurface(emulator, 4, 1, shellPaint)

	cell := captured.at(0, 0)
	if cell == nil {
		t.Fatal("no cell at 0,0")
	}
	if cell.Style.Bg != nil {
		t.Fatalf("background = %v, want it left unset for the terminal default", cell.Style.Bg)
	}
	if cell.Style.Fg != nil {
		t.Fatalf("foreground = %v, want it left unset for the terminal default", cell.Style.Fg)
	}
}

func TestSurfaceAtRejectsOutOfBounds(t *testing.T) {
	bounded := newSurface(2, 2, pinCell)
	for _, point := range [][2]int{{-1, 0}, {0, -1}, {2, 0}, {0, 2}} {
		if bounded.at(point[0], point[1]) != nil {
			t.Fatalf("at(%d,%d) returned a cell outside the surface", point[0], point[1])
		}
	}

	var absent *surface
	if absent.at(0, 0) != nil {
		t.Fatal("a nil surface must report no cells")
	}
}

func TestBlitLandsAtTheOffsetAndLeavesNeighboursAlone(t *testing.T) {
	frame := uv.NewScreenBuffer(6, 4)
	marker := uv.Cell{Content: "·", Width: 1}
	frame.Fill(&marker)

	patch := newSurface(2, 2, uv.Cell{Content: "X", Width: 1})
	patch.blit(&frame, 2, 1)

	for y := range 4 {
		for x := range 6 {
			inPatch := x >= 2 && x < 4 && y >= 1 && y < 3
			want := "·"
			if inPatch {
				want = "X"
			}
			cell := frame.CellAt(x, y)
			if cell == nil {
				t.Fatalf("frame cell %d,%d missing", x, y)
			}
			if cell.Content != want {
				t.Fatalf("cell %d,%d = %q, want %q", x, y, cell.Content, want)
			}
		}
	}
}

func TestBlitClipsAtTheFrameEdge(t *testing.T) {
	frame := uv.NewScreenBuffer(3, 2)
	patch := newSurface(4, 4, uv.Cell{Content: "X", Width: 1})

	patch.blit(&frame, 2, 1) // mostly off the right and bottom edges

	if got := frame.CellAt(2, 1); got == nil || got.Content != "X" {
		t.Fatal("the visible corner of the patch was not drawn")
	}
	// Reaching this point without a panic is the assertion: SetCell clips.
}

func TestTextSurfaceRendersLinesAndStopsAtTheBottom(t *testing.T) {
	style := uv.Style{Fg: color.RGBA{R: 200, G: 200, B: 200, A: 255}}
	surface := textSurface([]string{"one", "two", "three"}, style, 5, 2)

	if surface.rows != 2 {
		t.Fatalf("rows = %d, want 2", surface.rows)
	}
	if got := surface.at(0, 0).Content; got != "o" {
		t.Fatalf("first row starts with %q, want %q", got, "o")
	}
	if got := surface.at(0, 1).Content; got != "t" {
		t.Fatalf("second row starts with %q, want %q", got, "t")
	}
}

func TestTextSurfaceTruncatesOverlongLines(t *testing.T) {
	surface := textSurface([]string{"abcdefgh"}, uv.Style{}, 4, 1)

	if surface.cols != 4 {
		t.Fatalf("cols = %d, want 4", surface.cols)
	}
	// The last visible column carries the truncation marker, not source text.
	if got := surface.at(3, 0).Content; got != "…" {
		t.Fatalf("last column = %q, want the ellipsis marker", got)
	}
}
