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

	surfaceSpace lipgloss.Style
	frame        lipgloss.Style
	surfaceMuted lipgloss.Style
	bodySpace    lipgloss.Style
	activeSpace  lipgloss.Style
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
		frame:        h.On(surface, string(p.Border)),
		surfaceMuted: h.On(surface, string(p.Muted)),
		bodySpace:    lipgloss.NewStyle().Background(bg),
		activeSpace:  lipgloss.NewStyle().Background(secondary),
	}
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
