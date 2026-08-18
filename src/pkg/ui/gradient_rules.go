package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// GlowRule renders a full-width glyph rule with a symmetric gradient from its
// context surface to the accent token and back. Background and Peak optionally
// override those defaults. Transparent (defaulting to the context flag) skips
// painting cell backgrounds; the background color then only anchors the
// gradient endpoints.
type GlowRule struct {
	Context     Context
	Width       int
	Glyph       string
	Background  color.Color
	Peak        color.Color
	Transparent *bool
}

func (r GlowRule) background() color.Color {
	if r.Background != nil {
		return r.Background
	}
	return r.Context.Background
}

func (r GlowRule) peak() color.Color {
	if r.Peak != nil {
		return r.Peak
	}
	return r.Context.Color(r.Context.Palette.Accent)
}

func (r GlowRule) transparent() bool {
	if r.Transparent != nil {
		return *r.Transparent
	}
	return r.Context.Transparent
}

// Render returns the styled gradient rule.
func (r GlowRule) Render() string {
	background := r.background()
	gradient := lipgloss.Blend2D(r.Width, 1, 0, background, r.peak(), background)
	glyph := r.Glyph
	if glyph == "" {
		glyph = "┄"
	}
	transparent := r.transparent()

	var out strings.Builder
	for column := range r.Width {
		style := lipgloss.NewStyle().Foreground(gradient[column])
		if !transparent {
			style = style.Background(background)
		}
		out.WriteString(style.Render(glyph))
	}
	return out.String()
}
