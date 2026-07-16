package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// KeyHint is one keyboard shortcut and its description in a KeyHintsFooter.
type KeyHint struct {
	Key  string
	Text string
}

// KeyHintsFooter renders a three-row footer: a half-cell transition from Open
// to Context's surface, a row of key hints and a right-aligned status, and a
// half-cell transition from Context's surface to Outer. Context provides
// semantic defaults; individual styles are optional overrides.
type KeyHintsFooter struct {
	Context           Context
	Width             int
	Outer             color.Color
	Open              color.Color
	Muted             *lipgloss.Style
	Key               *lipgloss.Style
	Text              *lipgloss.Style
	Status            *lipgloss.Style
	HorizontalPadding int
}

func (f KeyHintsFooter) muted() lipgloss.Style {
	if f.Muted != nil {
		return *f.Muted
	}
	return f.Context.Muted()
}

func (f KeyHintsFooter) key() lipgloss.Style {
	if f.Key != nil {
		return *f.Key
	}
	return f.Context.Accent2().Bold(true)
}

func (f KeyHintsFooter) text() lipgloss.Style {
	if f.Text != nil {
		return *f.Text
	}
	return f.Context.Muted()
}

func (f KeyHintsFooter) status() lipgloss.Style {
	if f.Status != nil {
		return *f.Status
	}
	return f.Context.Accent().Bold(true)
}

// Render returns the footer for hints and status.
func (f KeyHintsFooter) Render(hints []KeyHint, status string) string {
	fill := f.Context.Fill()
	muted := f.muted()
	key := f.key()
	text := f.text()

	var b strings.Builder
	b.WriteString(fill.PaddingLeft(f.HorizontalPadding).Render(""))
	for i, hint := range hints {
		if i > 0 {
			b.WriteString(muted.Render("  "))
		}
		b.WriteString(key.Render(" "+hint.Key+" ") + fill.Render(" ") + text.Render(hint.Text))
	}
	left := b.String()

	right := f.status().Render(status) + fill.PaddingRight(f.HorizontalPadding).Render("")
	line := PlaceRight(f.Width, left, right, fill)
	keymap := fill.Width(f.Width).MaxWidth(f.Width).Render(ansi.Truncate(line, f.Width, ""))

	topHalfPad := lipgloss.NewStyle().
		Foreground(f.Context.Background).
		Background(f.Open).
		Render(strings.Repeat("▄", f.Width))
	bottomHalfPad := lipgloss.NewStyle().
		Foreground(f.Context.Background).
		Background(f.Outer).
		Render(strings.Repeat("▀", f.Width))
	return lipgloss.JoinVertical(lipgloss.Left, topHalfPad, keymap, bottomHalfPad)
}
