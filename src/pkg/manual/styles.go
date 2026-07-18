package manual

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"prelude/pkg/shared"
)

type styles struct {
	pal shared.Palette

	surface   color.Color
	secondary color.Color
	bg        color.Color

	// bodySpace / surfaceSpace are pure fills used with
	// lipgloss.WithWhitespaceStyle — the project-standard way to paint
	// placement padding without inventing per-cell background hacks.
	surfaceSpace lipgloss.Style
	bodySpace    lipgloss.Style
	activeSpace  lipgloss.Style
	frame        lipgloss.Style
	surfaceMuted lipgloss.Style
}

func newStyles(p shared.Palette) styles {
	h := shared.NewPaletteHelper(p)
	surface := h.Color(string(p.Surface))
	secondary := h.Color(string(p.Secondary))
	bg := h.Color(string(p.Bg))
	return styles{
		pal:          p,
		surface:      surface,
		secondary:    secondary,
		bg:           bg,
		surfaceSpace: lipgloss.NewStyle().Background(surface),
		bodySpace:    lipgloss.NewStyle().Background(bg),
		activeSpace:  lipgloss.NewStyle().Background(secondary),
		frame:        h.On(surface, string(p.Border)),
		surfaceMuted: h.On(surface, string(p.Muted)),
	}
}

// fillLine pads content to width with the body fill, using lipgloss placement
// whitespace styling so trailing cells always carry the theme background.
func (s styles) fillLine(content string, width int) string {
	return lipgloss.PlaceHorizontal(
		width,
		lipgloss.Left,
		content,
		lipgloss.WithWhitespaceStyle(s.bodySpace),
	)
}

// blankLine is one full-width body row with no content.
func (s styles) blankLine(width int) string {
	return s.bodySpace.Width(width).Render("")
}

// indent returns width cells of body fill (styled left padding).
func (s styles) indent(width int) string {
	if width <= 0 {
		return ""
	}
	return s.bodySpace.Width(width).Render("")
}

func (s styles) onBody(role Role, bold bool) lipgloss.Style {
	fg := s.pal.Fg
	switch role {
	case Muted:
		fg = s.pal.Muted
	case Dim:
		fg = s.pal.Dim
	case Accent:
		fg = s.pal.Accent
	case Accent2:
		fg = s.pal.Accent2
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(fg))).
		Background(s.bg).
		Bold(bold)
}

func (s styles) onActive(fg shared.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(string(fg))).Background(s.secondary)
}

// statusChrome returns inverted status-bar styles. Docs uses accent2 so the
// footer chip reads differently from the help manual's accent bar.
func (s styles) statusChrome(kind Kind) (space, text lipgloss.Style) {
	bar := lipgloss.Color(string(s.pal.Accent))
	if kind == KindDocs {
		bar = lipgloss.Color(string(s.pal.Accent2))
	}
	space = lipgloss.NewStyle().Background(bar)
	text = lipgloss.NewStyle().Background(bar).Foreground(s.bg)
	return space, text
}
