package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// GlowRule renders a full-width glyph rule with a symmetric gradient from its
// context surface to the accent token and back. Background and Peak optionally
// override those defaults.
type GlowRule struct {
	Context    Context
	Width      int
	Glyph      string
	Background color.Color
	Peak       color.Color
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

// Render returns the styled gradient rule.
func (r GlowRule) Render() string {
	background := r.background()
	gradient := lipgloss.Blend2D(r.Width, 1, 0, background, r.peak(), background)
	glyph := r.Glyph
	if glyph == "" {
		glyph = "┄"
	}

	var out strings.Builder
	for column := 0; column < r.Width; column++ {
		out.WriteString(lipgloss.NewStyle().Foreground(gradient[column]).Background(background).Render(glyph))
	}
	return out.String()
}

// CenteredGlowRule renders a glow rule broken around a centered label. Context
// provides default accent label, fill, and gradient colors. Fields provide
// caller overrides for exceptional surfaces.
type CenteredGlowRule struct {
	Context     Context
	Width       int
	Glyph       string
	Background  color.Color
	Peak        color.Color
	Label       string
	LabelStyle  *lipgloss.Style
	FillStyle   *lipgloss.Style
	Transparent *bool
}

func (r CenteredGlowRule) background() color.Color {
	if r.Background != nil {
		return r.Background
	}
	return r.Context.Background
}

func (r CenteredGlowRule) peak() color.Color {
	if r.Peak != nil {
		return r.Peak
	}
	return r.Context.Color(r.Context.Palette.Accent)
}

func (r CenteredGlowRule) labelStyle() lipgloss.Style {
	if r.LabelStyle != nil {
		return *r.LabelStyle
	}
	return r.Context.Accent().Bold(true)
}

func (r CenteredGlowRule) fillStyle() lipgloss.Style {
	if r.FillStyle != nil {
		return *r.FillStyle
	}
	return r.Context.Fill()
}

func (r CenteredGlowRule) transparent() bool {
	if r.Transparent != nil {
		return *r.Transparent
	}
	return r.Context.Transparent
}

// Render returns the centered labeled gradient rule.
func (r CenteredGlowRule) Render() string {
	glyph := r.Glyph
	if glyph == "" {
		glyph = "┄"
	}
	background := r.background()
	label := " " + r.Label + " "
	labelWidth := lipgloss.Width(label)
	start := max((r.Width-labelWidth)/2, 0)
	gradient := lipgloss.Blend2D(r.Width, 1, 0, background, r.peak(), background)
	labelRunes := []rune(label)
	transparent := r.transparent()

	var out strings.Builder
	for column := 0; column < r.Width; column++ {
		if column >= start && column < start+labelWidth {
			char := string(labelRunes[column-start])
			if char == " " {
				if transparent {
					out.WriteString(" ")
				} else {
					out.WriteString(r.fillStyle().Render(" "))
				}
			} else {
				out.WriteString(Inline(r.labelStyle()).Render(char))
			}
			continue
		}

		if transparent {
			out.WriteString(lipgloss.NewStyle().Foreground(gradient[column]).Inline(true).Render(glyph))
		} else {
			out.WriteString(lipgloss.NewStyle().Foreground(gradient[column]).Background(background).Render(glyph))
		}
	}
	return out.String()
}
