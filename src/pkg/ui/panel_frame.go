package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// PanelFrame renders a rounded terminal panel border around fixed-width content.
// Context supplies default border and fill styles; callers may override either
// style for a deliberate local exception.
type PanelFrame struct {
	Context    Context
	InnerWidth int
	Border     *lipgloss.Style
	Fill       *lipgloss.Style
}

// WithSize returns a copy with its content area set to inner columns wide.
func (f PanelFrame) WithSize(inner int) PanelFrame {
	f.InnerWidth = inner
	return f
}

func (f PanelFrame) border() lipgloss.Style {
	if f.Border != nil {
		return *f.Border
	}
	return f.Context.Border()
}

func (f PanelFrame) fill() lipgloss.Style {
	if f.Fill != nil {
		return *f.Fill
	}
	return f.Context.Fill()
}

// Paint pads content to the panel's inner width with filler and wraps it in
// border-painted side rails.
func (f PanelFrame) Paint(content string, filler lipgloss.Style) string {
	content = filler.MaxWidth(f.InnerWidth).Width(f.InnerWidth).Render(content)
	rail := f.border().Render("│")
	return rail + content + rail
}

// Top renders the rounded top border.
func (f PanelFrame) Top() string {
	return f.border().Render("╭" + strings.Repeat("─", f.InnerWidth) + "╮")
}

// Divider renders a horizontal divider between panel sections.
func (f PanelFrame) Divider() string {
	return f.border().Render("├" + strings.Repeat("─", f.InnerWidth) + "┤")
}

// Bottom renders the rounded bottom border.
func (f PanelFrame) Bottom() string {
	return f.border().Render("╰" + strings.Repeat("─", f.InnerWidth) + "╯")
}

// Blank renders an empty, fill-painted panel row.
func (f PanelFrame) Blank() string {
	return f.Paint("", f.fill())
}
