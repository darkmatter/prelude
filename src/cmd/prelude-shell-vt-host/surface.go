package main

import (
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/screen"
	"github.com/charmbracelet/x/ansi"
)

// surface is a finished rectangular cell image — the single currency this host
// composes in. A captured panel, a placeholder, and the child screen are all
// just cells, so none of them needs its escape sequences rewritten into outer
// terminal coordinates.
//
// Cells are stored by value in one allocation. [uv.Buffer.SetCell] copies what
// it is handed, so blitting can pass interior pointers and never clone.
type surface struct {
	cols  int
	rows  int
	cells []uv.Cell
}

func newSurface(cols, rows int, fill uv.Cell) *surface {
	cols = max(cols, 0)
	rows = max(rows, 0)
	cells := make([]uv.Cell, cols*rows)
	for i := range cells {
		cells[i] = fill
	}
	return &surface{cols: cols, rows: rows, cells: cells}
}

func (s *surface) at(x, y int) *uv.Cell {
	if s == nil || x < 0 || x >= s.cols || y < 0 || y >= s.rows {
		return nil
	}
	return &s.cells[y*s.cols+x]
}

// setComposedCell writes one cell into dst and reports how many columns it
// consumed.
//
// A zero-width cell is the trailing half of a wide grapheme. Ultraviolet
// writes those placeholders itself when the wide cell lands, and writing one
// explicitly makes Line.Set walk back and blank the very glyph it continues —
// so composition must step over them, not through them. A zero-width cell can
// still be reached first when a viewport is clipped mid-grapheme; there is no
// glyph to show in that column, so it renders as a blank.
func setComposedCell(dst *uv.ScreenBuffer, x, y int, cell *uv.Cell) int {
	if cell.Width < 1 {
		dst.SetCell(x, y, &uv.EmptyCell)
		return 1
	}
	dst.SetCell(x, y, cell)
	return cell.Width
}

// blit copies the whole surface into dst with its top-left corner at
// (originX, originY). Cells outside dst are dropped by dst itself.
func (s *surface) blit(dst *uv.ScreenBuffer, originX, originY int) {
	if s == nil {
		return
	}
	for y := range s.rows {
		row := s.cells[y*s.cols : (y+1)*s.cols]
		for x := 0; x < len(row); {
			x += setComposedCell(dst, originX+x, originY+y, &row[x])
		}
	}
}

// cellSource is the read side of any virtual terminal screen.
type cellSource interface {
	CellAt(x, y int) *uv.Cell
}

// captureSurface freezes a cols x rows window of src. The copy matters: the
// emulator it came from is closed as soon as its child exits, and the result
// outlives it as immutable state.
//
// A virtual terminal hands back a blank cell rather than nothing for every
// coordinate the child never touched, so the band's fill only survives where
// the source has no cell at all. Everything else goes through the band's
// paint policy, which is what keeps chrome opaque.
func captureSurface(src cellSource, cols, rows int, band paint) *surface {
	result := newSurface(cols, rows, band.fill)
	for y := range rows {
		for x := range cols {
			cell := src.CellAt(x, y)
			if cell == nil {
				continue
			}
			result.cells[y*cols+x] = band.apply(*cell)
		}
	}
	return result
}

// textSurface renders plain lines into a surface, for placeholders and error
// reports that never run a child command. The result is chrome, so it is
// composed opaquely against its own style.
func textSurface(lines []string, style uv.Style, cols, rows int) *surface {
	buffer := uv.NewScreenBuffer(cols, rows)
	band := paint{fill: uv.Cell{Content: " ", Width: 1, Style: style}, opaque: true}
	buffer.Fill(&band.fill)

	ctx := screen.NewContext(&buffer)
	ctx.SetStyle(style)
	for row, line := range lines {
		if row >= rows {
			break
		}
		drawLine(ctx, line, 0, row, cols)
	}
	return captureSurface(&buffer, cols, rows, band)
}

// drawLine writes one line of chrome, truncating at the given width. Callers
// fill the band first, so no padding is written back.
func drawLine(ctx *screen.Context, text string, x, y, width int) {
	if width <= 0 || text == "" {
		return
	}
	if strings.ContainsRune(text, '\t') {
		text = strings.ReplaceAll(text, "\t", "    ")
	}
	if ansi.StringWidth(text) > width {
		text = ansi.Truncate(text, width, "…")
	}
	ctx.DrawString(text, x, y)
}
