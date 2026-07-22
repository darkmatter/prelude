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
	border    color.Color
	accent    color.Color

	// bodySpace / surfaceSpace are pure fills used with
	// lipgloss.WithWhitespaceStyle — the project-standard way to paint
	// placement padding without inventing per-cell background hacks.
	surfaceSpace lipgloss.Style
	bodySpace    lipgloss.Style
	activeSpace  lipgloss.Style // selected-row fill when nav focused (secondary)
	frame        lipgloss.Style // idle top chrome on surface
	frameAccent  lipgloss.Style // focused nav top chrome (accent on surface)
	surfaceMuted lipgloss.Style
	surfaceDim   lipgloss.Style // nested tree rows — slightly dimmer than muted
	divider      lipgloss.Style // idle top/junction on body bg
	topAccent    lipgloss.Style // focused body top chrome (accent on body bg)
	scrollTrack  lipgloss.Style // body scrollbar track (very faint)
	scrollThumb  lipgloss.Style // body scrollbar thumb (muted)
}

func newStyles(p shared.Palette) styles {
	h := shared.NewPaletteHelper(p)
	surface := h.Color(string(p.Surface))
	secondary := h.Color(string(p.Secondary))
	bg := h.Color(string(p.Bg))
	border := h.Color(string(p.Border))
	accent := h.Color(string(p.Accent))
	return styles{
		pal:          p,
		surface:      surface,
		secondary:    secondary,
		bg:           bg,
		border:       border,
		accent:       accent,
		surfaceSpace: lipgloss.NewStyle().Background(surface),
		bodySpace:    lipgloss.NewStyle().Background(bg),
		activeSpace:  lipgloss.NewStyle().Background(secondary),
		frame:        lipgloss.NewStyle().Foreground(border).Background(surface),
		frameAccent:  lipgloss.NewStyle().Foreground(accent).Background(surface),
		surfaceMuted: h.On(surface, string(p.Muted)),
		surfaceDim:   h.On(surface, string(p.Dim)),
		divider:      lipgloss.NewStyle().Foreground(border).Background(bg),
		topAccent:    lipgloss.NewStyle().Foreground(accent).Background(bg),
		// Scrollbar: dim track, muted thumb — readable but not loud.
		scrollTrack: lipgloss.NewStyle().
			Foreground(h.Color(string(p.Dim))).
			Background(bg).
			Faint(true),
		scrollThumb: lipgloss.NewStyle().
			Foreground(h.Color(string(p.Muted))).
			Background(bg),
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

// onActive paints selection on the secondary row fill (nav focused).
func (s styles) onActive(fg shared.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(string(fg))).Background(s.secondary)
}

// statusChrome returns inverted status-bar styles (accent2 bar for docs).
func (s styles) statusChrome() (space, text lipgloss.Style) {
	bar := lipgloss.Color(string(s.pal.Accent2))
	space = lipgloss.NewStyle().Background(bar)
	text = lipgloss.NewStyle().Background(bar).Foreground(s.bg)
	return space, text
}
