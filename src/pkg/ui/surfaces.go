package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// Surface renders fixed-width terminal rows against a shared visual context.
// Fill optionally overrides the context-derived surface fill.
type Surface struct {
	Context   Context
	Width     int
	FillStyle *lipgloss.Style
}

func (s Surface) fill() lipgloss.Style {
	if s.FillStyle != nil {
		return *s.FillStyle
	}
	return s.Context.Fill()
}

// Blank returns one Width-wide row filled with the surface fill.
func (s Surface) Blank() string {
	return s.fill().Width(s.Width).Render("")
}

// Fill pads content on the right to Width, painting the added cells with the
// surface fill.
func (s Surface) Fill(content string) string {
	return lipgloss.PlaceHorizontal(
		s.Width,
		lipgloss.Left,
		content,
		lipgloss.WithWhitespaceStyle(s.fill()),
	)
}

// JoinVertical stacks non-empty parts and fills every resulting line to Width.
func (s Surface) JoinVertical(parts ...string) string {
	var block Block
	for _, part := range parts {
		if part == "" {
			continue
		}
		for _, line := range SplitLines(part) {
			block.Write(s.Fill(line))
		}
	}
	return strings.TrimSuffix(block.String(), "\n")
}

// FillLine pads content on the right to width, painting added cells with bg.
func FillLine(content string, width int, bg color.Color) string {
	return lipgloss.PlaceHorizontal(
		width,
		lipgloss.Left,
		content,
		lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(bg)),
	)
}

// PadBlock applies horizontal padding to content within width. The supplied
// fill style paints the block background, including padding and unused width.
func PadBlock(content string, width, left, right int, fill lipgloss.Style) string {
	if content == "" {
		return ""
	}
	return fill.
		Width(width).
		Padding(0, max(right, 0), 0, max(left, 0)).
		Render(content)
}

// PlaceContentLine places styled content in a content-width band after left
// padding, aligns it within that band, and fills the full card width. The fill
// style paints all whitespace introduced by the placement.
func PlaceContentLine(styled string, cardWidth, contentWidth, leftPadding int, align lipgloss.Position, fill lipgloss.Style) string {
	whitespace := lipgloss.WithWhitespaceStyle(fill)
	left := lipgloss.PlaceHorizontal(
		max(leftPadding, 0),
		lipgloss.Left,
		"",
		whitespace,
	)
	content := lipgloss.PlaceHorizontal(
		contentWidth,
		align,
		styled,
		whitespace,
	)
	return lipgloss.PlaceHorizontal(
		cardWidth,
		lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, left, content),
		whitespace,
	)
}

// Window places a rendered body inside a fixed-width terminal window. Offset
// indents body rows; TopMargin and BottomMargin add blank rows on Context's
// surface. Fill optionally overrides the context-derived fill.
type Window struct {
	Context      Context
	Width        int
	Offset       int
	TopMargin    int
	BottomMargin int
	Fill         *lipgloss.Style
}

func (w Window) fill() lipgloss.Style {
	if w.Fill != nil {
		return *w.Fill
	}
	return w.Context.Fill()
}

// Render places each body row at Offset and surrounds it with configured
// margin rows. The returned string retains a trailing newline.
func (w Window) Render(body string) string {
	var block Block
	for range max(w.TopMargin, 0) {
		block.Write(w.blank())
	}
	for _, line := range SplitLines(body) {
		block.Write(w.place(line))
	}
	for range max(w.BottomMargin, 0) {
		block.Write(w.blank())
	}
	return block.String()
}

func (w Window) place(line string) string {
	whitespace := lipgloss.WithWhitespaceStyle(w.fill())
	left := lipgloss.PlaceHorizontal(max(w.Offset, 0), lipgloss.Left, "", whitespace)
	return lipgloss.PlaceHorizontal(w.Width, lipgloss.Left, left+line, whitespace)
}

func (w Window) blank() string {
	return w.fill().Width(w.Width).Render("")
}
