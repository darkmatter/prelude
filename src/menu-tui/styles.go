package main

import (
	"image/color"

	"charm.land/lipgloss/v2"
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
	pal Palette

	surface   color.Color
	secondary color.Color
	bgColor   color.Color
	accentC   color.Color

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
	c := func(s string) color.Color { return lipgloss.Color(s) }
	surface := c(p.Surface)
	secondary := c(p.Secondary)

	plain := func(fg string) lipgloss.Style { return lipgloss.NewStyle().Foreground(c(fg)) }
	on := func(fg string) lipgloss.Style { return plain(fg).Background(surface) }
	onBar := func(fg string) lipgloss.Style { return plain(fg).Background(secondary) }

	return styles{
		pal:       p,
		surface:   surface,
		secondary: secondary,
		bgColor:   c(p.Bg),
		accentC:   c(p.Accent),

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
		kbdChip: plain(p.Accent2).Background(c(p.Bg)),
		optChip: onBar(p.Fg),

		selText: lipgloss.NewStyle().Background(c(p.Accent)).Foreground(c(p.SelectionFg)),
		selDim:  lipgloss.NewStyle().Background(c(p.Accent)).Foreground(c(p.SelectionFg)).Faint(true),
		selChip: lipgloss.NewStyle().Background(c(p.SelectionFg)).Foreground(c(p.Accent)),
		selSp:   lipgloss.NewStyle().Background(c(p.Accent)),
	}
}

// bar returns an arbitrary foreground on the secondary (bar) background.
func (s styles) bar(fg string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).Background(s.secondary)
}

// inset returns an arbitrary foreground on the bg (details inset) background.
func (s styles) inset(fg string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fg)).Background(s.bgColor)
}
