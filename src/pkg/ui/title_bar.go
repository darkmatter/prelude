package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// TitleBar renders a three-row title treatment: a half-cell rise from Outer
// into Context's surface, a centered and truncated title, and a half-cell drop
// into Below. Text optionally overrides the context's muted default.
type TitleBar struct {
	Context           Context
	Width             int
	Outer             color.Color
	Below             color.Color
	Text              *lipgloss.Style
	HorizontalPadding int
}

func (b TitleBar) textStyle() lipgloss.Style {
	if b.Text != nil {
		return *b.Text
	}
	return b.Context.Muted()
}

// Render returns the title bar for title.
func (b TitleBar) Render(title string) string {
	halfPad := lipgloss.NewStyle().
		Foreground(b.Context.Background).
		Background(b.Outer).
		Render(strings.Repeat("▄", b.Width))
	row := b.textStyle().
		Background(b.Context.Background).
		Width(b.Width).
		MaxWidth(b.Width).
		Align(lipgloss.Center).
		Render(ansi.Truncate(title, b.Width-2*b.HorizontalPadding-2, "…"))
	bottomHalfPad := lipgloss.NewStyle().
		Foreground(b.Context.Background).
		Background(b.Below).
		Render(strings.Repeat("▀", b.Width))
	return lipgloss.JoinVertical(lipgloss.Left, halfPad, row, bottomHalfPad)
}
