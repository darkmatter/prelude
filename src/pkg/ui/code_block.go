package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/screen"
)

// FadingRule renders a labeled horizontal rule that fades toward Context's
// surface. Frame, Label, and Surface are optional overrides for components that
// need a distinct code or inset surface.
type FadingRule struct {
	Context Context
	Width   int
	Surface color.Color
	Frame   color.Color
	Label   *lipgloss.Style
	Fade    bool
}

func (r FadingRule) surface() color.Color {
	if r.Surface != nil {
		return r.Surface
	}
	return r.Context.Background
}

func (r FadingRule) frame() color.Color {
	if r.Frame != nil {
		return r.Frame
	}
	return r.Context.Color(r.Context.Palette.Border)
}

func (r FadingRule) labelStyle() lipgloss.Style {
	if r.Label != nil {
		return *r.Label
	}
	return r.Context.Accent().Bold(true)
}

func ultravioletStyle(style lipgloss.Style) uv.Style {
	var attrs uint8
	if style.GetBold() {
		attrs |= uv.AttrBold
	}
	if style.GetFaint() {
		attrs |= uv.AttrFaint
	}
	if style.GetItalic() {
		attrs |= uv.AttrItalic
	}
	if style.GetBlink() {
		attrs |= uv.AttrBlink
	}
	if style.GetReverse() {
		attrs |= uv.AttrReverse
	}
	if style.GetStrikethrough() {
		attrs |= uv.AttrStrikethrough
	}
	return uv.Style{
		Fg:             style.GetForeground(),
		Bg:             style.GetBackground(),
		UnderlineColor: style.GetUnderlineColor(),
		Underline:      style.GetUnderlineStyle(),
		Attrs:          attrs,
	}
}

// Render paints title at the leading edge when present, filling the remaining
// width with a styled horizontal rule.
func (r FadingRule) Render(title string) string {
	if r.Width <= 0 {
		return ""
	}

	surface, frame := r.surface(), r.frame()
	colors := []color.Color{frame}
	if r.Fade {
		// BorderForegroundBlend spans an implicit full perimeter, even for a
		// top-only border. Render the horizontal gradient directly so its final
		// cell reaches the surface color at the right edge.
		colors = lipgloss.Blend1D(r.Width, frame, surface)
	}

	var rule strings.Builder
	for column := 0; column < r.Width; column++ {
		foreground := colors[min(column, len(colors)-1)]
		rule.WriteString(lipgloss.NewStyle().
			Foreground(foreground).
			Background(surface).
			Render("─"))
	}
	if title == "" {
		return rule.String()
	}

	// Canvas is backed by Ultraviolet's cell buffer. DrawString segments the
	// complete label into grapheme clusters and clips it by terminal cells.
	canvas := lipgloss.NewCanvas(r.Width, 1).Compose(lipgloss.NewLayer(rule.String()))
	labelStyle := r.labelStyle().Background(surface)
	label := screen.NewContext(canvas).WithStyle(ultravioletStyle(labelStyle))
	if link, params := labelStyle.GetHyperlink(); link != "" {
		label.SetURL(link, params)
	}
	label.DrawString(" "+title+" ", 1, 0)
	return canvas.Render()
}

// CodeBlock renders a framed title-and-lines terminal block. Context supplies
// default surface and rule colors while callers may override them for a code
// surface distinct from the surrounding card.
type CodeBlock struct {
	Context Context
	Title   string
	Lines   []string
	Indent  string
	Width   int
	Surface color.Color
	Rule    FadingRule
}

func (b CodeBlock) surface() color.Color {
	if b.Surface != nil {
		return b.Surface
	}
	return b.Context.Background
}

// Render returns the top rule, filled content lines, and bottom rule.
func (b CodeBlock) Render() []string {
	surface := b.surface()
	rule := b.Rule
	rule.Context = b.Context
	rule.Width = b.Width
	if rule.Surface == nil {
		rule.Surface = surface
	}

	out := []string{rule.Render(b.Title)}
	for _, line := range b.Lines {
		out = append(out, FillLine(b.Indent+line, b.Width, surface))
	}
	return append(out, rule.Render(""))
}
