package main

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"prelude/shared"
)

// styles derives every lipgloss style from the theme palette.
//
// The TUI window is painted like the mock: the panel sits on `surface`
// (--card), the title/status bars on `secondary`, the details inset on
// `bg`, chips on `secondary`/`bg`, and the selection bar on `accent`.
// lipgloss does not re-apply a parent background after a child style's
// reset, so every segment style carries its own background. The plain
// (background-free) styles serve non-TUI output: `menu list` and the
// post-quit `$ command` preview.
type styles struct {
	pal shared.Palette

	surface   color.Color
	secondary color.Color
	bgColor   color.Color
	accentC   color.Color

	windowBg lipgloss.Style // fills the terminal canvas outside the panel

	// plain, no background (list output, exec preview)
	fg      lipgloss.Style
	muted   lipgloss.Style
	dim     lipgloss.Style
	accent  lipgloss.Style
	accent2 lipgloss.Style
	errText lipgloss.Style

	// on surface (panel body)
	sFg      lipgloss.Style
	sMuted   lipgloss.Style
	sDim     lipgloss.Style
	sAccent  lipgloss.Style
	sAccent2 lipgloss.Style
	sErr     lipgloss.Style
	sp       lipgloss.Style // surface filler
	frame    lipgloss.Style // window border chars

	// on secondary (title + status bars)
	barMuted lipgloss.Style
	barDim   lipgloss.Style
	barSp    lipgloss.Style

	// chips
	keyChip lipgloss.Style // key accelerator square (secondary bg, accent2)
	kbdChip lipgloss.Style // footer kbd hints (bg, accent2)
	optChip lipgloss.Style // arg option chips (secondary bg, fg)

	// selection bar (accent bg)
	selText lipgloss.Style
	selDim  lipgloss.Style
	selChip lipgloss.Style // inverted mini chip on the selection bar
	selSp   lipgloss.Style
}

func newStyles(cfg *Config) styles {
	p := cfg.Palette
	h := shared.NewPaletteHelper(p)
	surface := h.Color(string(p.Surface))
	secondary := h.Color(string(p.Secondary))

	plain := func(fg shared.Color) lipgloss.Style { return h.Plain(string(fg)) }
	on := func(fg shared.Color) lipgloss.Style { return h.On(surface, string(fg)) }
	onBar := func(fg shared.Color) lipgloss.Style { return h.On(secondary, string(fg)) }

	return styles{
		pal:       p,
		surface:   surface,
		secondary: secondary,
		bgColor:   h.Color(string(p.Bg)),
		accentC:   h.Color(string(p.Accent)),
		windowBg:  lipgloss.NewStyle().Background(h.Color(string(p.Bg))),

		fg:      plain(p.Fg),
		muted:   plain(p.Muted),
		dim:     plain(p.Dim),
		accent:  plain(p.Accent),
		accent2: plain(p.Accent2),
		errText: plain(p.Error),

		sFg:      on(p.Fg),
		sMuted:   on(p.Muted),
		sDim:     on(p.Dim),
		sAccent:  on(p.Accent),
		sAccent2: on(p.Accent2),
		sErr:     on(p.Error),
		sp:       lipgloss.NewStyle().Background(surface),
		frame:    on(p.Border),

		barMuted: onBar(p.Muted),
		barDim:   onBar(p.Dim),
		barSp:    lipgloss.NewStyle().Background(secondary),

		keyChip: onBar(p.Accent2),
		kbdChip: plain(p.Accent2).Background(h.Color(string(p.Bg))),
		optChip: onBar(p.Fg),

		selText: lipgloss.NewStyle().Background(h.Color(string(p.Accent))).Foreground(h.Color(string(p.SelectionFg))),
		selDim:  lipgloss.NewStyle().Background(h.Color(string(p.Accent))).Foreground(h.Color(string(p.SelectionFg))).Faint(true),
		selChip: lipgloss.NewStyle().Background(h.Color(string(p.SelectionFg))).Foreground(h.Color(string(p.Accent))),
		selSp:   lipgloss.NewStyle().Background(h.Color(string(p.Accent))),
	}
}

// bar returns an arbitrary foreground on the secondary (bar) background.
func (s styles) bar(fg shared.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(string(fg))).Background(s.secondary)
}

// inset returns an arbitrary foreground on the bg (details inset) background.
func (s styles) inset(fg shared.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(string(fg))).Background(s.bgColor)
}
