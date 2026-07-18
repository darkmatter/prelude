package ui

import (
	"image/color"
	"math"

	"charm.land/lipgloss/v2"

	"prelude/pkg/shared"
)

// Palette is the common design-token palette for terminal UI components.
// It is an alias so configuration, menu, MOTD, and reusable UI widgets share
// one type while the package boundary is migrated toward ui ownership.
type Palette = shared.Palette

// Context is immutable, render-scoped visual context for a UI surface. Widgets
// use its semantic tokens for their defaults; callers can still override an
// individual widget style when a component needs an intentional exception.
type Context struct {
	Palette     Palette
	Background  color.Color
	Transparent bool
}

// NewContext creates a context for one visual surface.
func NewContext(palette Palette, background color.Color, transparent bool) Context {
	return Context{
		Palette:     palette,
		Background:  background,
		Transparent: transparent,
	}
}

// Color resolves a palette color into a Lip Gloss color.
func (c Context) Color(token shared.Color) color.Color {
	return lipgloss.Color(token.String())
}

// BlendBackground blends tint into the context background in CIELAB space.
// Weight is the tint contribution: 0 returns Background and 1 returns tint.
func (c Context) BlendBackground(tint color.Color, weight float64) color.Color {
	if c.Background == nil {
		return tint
	}
	if tint == nil {
		return c.Background
	}

	const precision = 100
	weight = max(0, min(weight, 1))
	steps := lipgloss.Blend1D(precision+1, c.Background, tint)
	return steps[int(math.Round(weight*precision))]
}

// Style renders token on this context's surface.
func (c Context) Style(token shared.Color) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(c.Color(token))
	if !c.Transparent {
		style = style.Background(c.Background)
	}
	return style
}

// Fill renders whitespace on this context's surface.
func (c Context) Fill() lipgloss.Style {
	if c.Transparent {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Background(c.Background)
}

func (c Context) Foreground() lipgloss.Style   { return c.Style(c.Palette.Fg) }
func (c Context) Muted() lipgloss.Style        { return c.Style(c.Palette.Muted) }
func (c Context) Dim() lipgloss.Style          { return c.Style(c.Palette.Dim) }
func (c Context) Accent() lipgloss.Style       { return c.Style(c.Palette.Accent) }
func (c Context) Accent2() lipgloss.Style      { return c.Style(c.Palette.Accent2) }
func (c Context) Success() lipgloss.Style      { return c.Style(c.Palette.Success) }
func (c Context) Warning() lipgloss.Style      { return c.Style(c.Palette.Warning) }
func (c Context) Info() lipgloss.Style         { return c.Style(c.Palette.Info) }
func (c Context) Error() lipgloss.Style        { return c.Style(c.Palette.Error) }
func (c Context) Border() lipgloss.Style       { return c.Style(c.Palette.Border) }
func (c Context) AccentBorder() lipgloss.Style { return c.Style(c.Palette.AccentBorder) }
