package main

import (
	"image/color"

	uv "github.com/charmbracelet/ultraviolet"
)

// Chrome colours. The pin and status bands are opaque so the child shell can
// never bleed through them, and so a captured panel that leaves cells unset
// still lands on a deliberate background rather than the terminal default.
var (
	shellBackground  = color.RGBA{R: 9, G: 9, B: 11, A: 255}
	pinBackground    = color.RGBA{R: 15, G: 23, B: 42, A: 255}
	pinHeaderBg      = color.RGBA{R: 30, G: 41, B: 59, A: 255}
	pinForeground    = color.RGBA{R: 203, G: 213, B: 225, A: 255}
	accentColor      = color.RGBA{R: 129, G: 140, B: 248, A: 255}
	warnColor        = color.RGBA{R: 248, G: 113, B: 113, A: 255}
	statusBackground = color.RGBA{R: 24, G: 24, B: 27, A: 255}
	statusForeground = color.RGBA{R: 161, G: 161, B: 170, A: 255}
)

var (
	pinStyle    = uv.Style{Fg: pinForeground, Bg: pinBackground}
	headerStyle = uv.Style{Fg: accentColor, Bg: pinHeaderBg, Attrs: uv.AttrBold}
	statusStyle = uv.Style{Fg: statusForeground, Bg: statusBackground, Attrs: uv.AttrFaint}
	errorStyle  = uv.Style{Fg: warnColor, Bg: statusBackground, Attrs: uv.AttrBold}

	pinCell    = uv.Cell{Content: " ", Width: 1, Style: pinStyle}
	headerCell = uv.Cell{Content: " ", Width: 1, Style: headerStyle}
	statusCell = uv.Cell{Content: " ", Width: 1, Style: statusStyle}
	shellCell  = uv.Cell{Content: " ", Width: 1, Style: uv.Style{Bg: shellBackground}}
)

// paint is how one band's cells reach the frame: the backdrop to use where
// there is nothing to draw, and whether cells that declined to set their own
// colours adopt it.
//
// The distinction is not cosmetic. A virtual terminal reports unset colours as
// nil, meaning "the renderer's default". For the shell band that default
// should stay unset — a child using the terminal's own palette must look the
// way it would in any other terminal, including on a light theme. For a band
// that is chrome sitting on top of the shell, the same nil would resolve
// against the outer terminal and let the user's theme show through, so those
// cells adopt the band's colours instead.
type paint struct {
	fill   uv.Cell
	opaque bool
}

// apply resolves one source cell against the band's policy.
func (p paint) apply(cell uv.Cell) uv.Cell {
	if !p.opaque {
		return cell
	}
	if cell.Style.Fg == nil {
		cell.Style.Fg = p.fill.Style.Fg
	}
	if cell.Style.Bg == nil {
		cell.Style.Bg = p.fill.Style.Bg
	}
	return cell
}

var (
	// The shell is the child's own screen and keeps the terminal's defaults.
	shellPaint = paint{fill: shellCell}

	// The pin band and its header are chrome and must be opaque.
	pinPaint    = paint{fill: pinCell, opaque: true}
	headerPaint = paint{fill: headerCell, opaque: true}
)
