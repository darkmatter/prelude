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
// need a distinct code or inset surface. Transparent (defaulting to the context
// flag) skips painting cell backgrounds; the surface color then only anchors the
// fade endpoints.
type FadingRule struct {
	Context     Context
	Width       int
	Surface     color.Color
	Frame       color.Color
	Label       *lipgloss.Style
	Fade        bool
	Transparent *bool
}

func (r FadingRule) surface() color.Color {
	if r.Surface != nil {
		return r.Surface
	}
	return r.Context.Background
}

func (r FadingRule) transparent() bool {
	if r.Transparent != nil {
		return *r.Transparent
	}
	return r.Context.Transparent
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
	transparent := r.transparent()
	colors := []color.Color{frame}
	if r.Fade {
		// BorderForegroundBlend spans an implicit full perimeter, even for a
		// top-only border. Render the horizontal gradient directly so its final
		// cell reaches the surface color at the right edge.
		colors = lipgloss.Blend1D(r.Width, frame, surface)
	}

	// dashStyle returns the style for a rule dash at the given column.
	dashStyle := func(column int) lipgloss.Style {
		style := lipgloss.NewStyle().Foreground(colors[min(column, len(colors)-1)])
		if !transparent {
			style = style.Background(surface)
		}
		return style
	}

	if title == "" {
		var rule strings.Builder
		for column := 0; column < r.Width; column++ {
			rule.WriteString(dashStyle(column).Render("─"))
		}
		return rule.String()
	}

	if transparent {
		// When transparent, render column-by-column so the title region omits
		// both dashes and backgrounds — no canvas compositing (which defaults
		// empty cells to a black background rather than truly transparent).
		return r.renderTransparent(title, dashStyle)
	}

	// Canvas is backed by Ultraviolet's cell buffer. DrawString segments the
	// complete label into grapheme clusters and clips it by terminal cells.
	var rule strings.Builder
	for column := 0; column < r.Width; column++ {
		rule.WriteString(dashStyle(column).Render("─"))
	}
	canvas := lipgloss.NewCanvas(r.Width, 1).Compose(lipgloss.NewLayer(rule.String()))
	labelStyle := r.labelStyle().Background(surface)
	label := screen.NewContext(canvas).WithStyle(ultravioletStyle(labelStyle))
	if link, params := labelStyle.GetHyperlink(); link != "" {
		label.SetURL(link, params)
	}
	label.DrawString(" "+title+" ", 1, 0)
	return canvas.Render()
}

// renderTransparent renders the rule with a title inset, column-by-column,
// without painting any cell backgrounds. The title region uses the label
// foreground style; dashes outside it follow the fade gradient.
func (r FadingRule) renderTransparent(title string, dashStyle func(int) lipgloss.Style) string {
	label := r.labelStyle().UnsetBackground().Inline(true)
	labelText := " " + title + " "
	labelRunes := []rune(labelText)

	var out strings.Builder
	column := 0
	for _, ru := range labelRunes {
		w := lipgloss.Width(string(ru))
		if column >= r.Width {
			break
		}
		if ru == ' ' {
			for range w {
				if column >= r.Width {
					break
				}
				out.WriteString(" ")
				column++
			}
		} else {
			out.WriteString(label.Render(string(ru)))
			column += w
		}
	}
	for ; column < r.Width; column++ {
		out.WriteString(dashStyle(column).Render("─"))
	}
	return out.String()
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
	// HeaderTransparent, when true, omits the background fill behind the top
	// rule and title so the surrounding card background shows through.
	HeaderTransparent bool
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

	topRule := rule
	if b.HeaderTransparent {
		t := true
		topRule.Transparent = &t
	}
	out := []string{topRule.Render(b.Title)}
	for _, line := range b.Lines {
		out = append(out, FillLine(b.Indent+line, b.Width, surface))
	}
	return append(out, rule.Render(""))
}
