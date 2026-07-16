package menu

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"prelude/pkg/shared"
	"prelude/pkg/ui"
)

// styles derives every lipgloss style from the theme palette.
//
// Layering matches the command-menu playground:
//
//	bg          window + details inset + help body
//	body        picker/form panel between bg and chrome
//	open        filter input + command preview
//	chrome      title row + keymap footer (theme surface)
//	secondary   option chips
//	accent      selection bar
//
// lipgloss does not re-apply a parent background after a child style's
// reset, so every segment style carries its own background. The plain
// (background-free) styles serve non-TUI output: `menu list` and the
// post-quit `$ command` preview.
type styles struct {
	pal shared.Palette

	bodyColor   color.Color
	openColor   color.Color
	chromeColor color.Color
	secondary   color.Color
	bgColor     color.Color
	accentC     color.Color
	borderC     color.Color

	bodyUI   ui.Context
	openUI   ui.Context
	chromeUI ui.Context
	windowUI ui.Context

	windowBg lipgloss.Style // fills the terminal canvas outside the panel

	// plain, no background (list output, exec preview)
	fg      lipgloss.Style
	muted   lipgloss.Style
	dim     lipgloss.Style
	accent  lipgloss.Style
	accent2 lipgloss.Style
	errText lipgloss.Style

	// on body (panel interior)
	sFg      lipgloss.Style
	sMuted   lipgloss.Style
	sDim     lipgloss.Style
	sAccent  lipgloss.Style
	sAccent2 lipgloss.Style
	sErr     lipgloss.Style
	sp       lipgloss.Style // body filler
	frame    lipgloss.Style // window border chars on body

	// on open surface (filter + live preview)
	openSp      lipgloss.Style
	openFg      lipgloss.Style
	openMuted   lipgloss.Style
	openDim     lipgloss.Style
	openAccent  lipgloss.Style
	openAccent2 lipgloss.Style

	// on chrome (title + keymap)
	chromeSp    lipgloss.Style
	chromeMuted lipgloss.Style

	// chips
	keyChip lipgloss.Style // key accelerator rails on body
	kbdChip lipgloss.Style // footer key labels
	optChip lipgloss.Style // arg option chips

	// selection bar (accent bg)
	selText lipgloss.Style
	selDim  lipgloss.Style
	selChip lipgloss.Style // outlined hotkey on the selection bar
	selSp   lipgloss.Style
}

func newStyles(cfg *Config) styles {
	p := cfg.Palette
	h := shared.NewPaletteHelper(p)
	bgColor := h.Color(string(p.Bg))
	chrome := h.Color(string(p.Surface))
	secondary := h.Color(string(p.Secondary))
	// Body sits between window bg and chrome surface — same split as the playground.
	body := lipgloss.Lighten(bgColor, 0.02)
	open := lipgloss.Darken(body, 0.01)
	accentC := h.Color(string(p.Accent))
	borderC := h.Color(string(p.Border))
	selFg := h.Color(string(p.SelectionFg))

	plain := func(fg shared.Color) lipgloss.Style { return h.Plain(string(fg)) }
	on := func(bg color.Color, fg shared.Color) lipgloss.Style {
		return h.On(bg, string(fg))
	}

	return styles{
		pal:         p,
		bodyUI:      ui.NewContext(p, body, false),
		openUI:      ui.NewContext(p, open, false),
		chromeUI:    ui.NewContext(p, chrome, false),
		windowUI:    ui.NewContext(p, bgColor, false),
		bodyColor:   body,
		openColor:   open,
		chromeColor: chrome,
		secondary:   secondary,
		bgColor:     bgColor,
		accentC:     accentC,
		borderC:     borderC,
		windowBg:    lipgloss.NewStyle().Background(bgColor),

		fg:      plain(p.Fg),
		muted:   plain(p.Muted),
		dim:     plain(p.Dim),
		accent:  plain(p.Accent),
		accent2: plain(p.Accent2),
		errText: plain(p.Error),

		sFg:      on(body, p.Fg),
		sMuted:   on(body, p.Muted),
		sDim:     on(body, p.Dim),
		sAccent:  on(body, p.Accent),
		sAccent2: on(body, p.Accent2),
		sErr:     on(body, p.Error),
		sp:       lipgloss.NewStyle().Background(body),
		frame:    on(body, p.Border),

		openSp:      lipgloss.NewStyle().Background(open),
		openFg:      on(open, p.Fg),
		openMuted:   on(open, p.Muted),
		openDim:     on(open, p.Dim),
		openAccent:  on(open, p.Accent),
		openAccent2: on(open, p.Accent2),

		chromeSp:    lipgloss.NewStyle().Background(chrome),
		chromeMuted: on(chrome, p.Muted),

		// Amber glyphs on body with left/right border rails only (one terminal row).
		keyChip: on(body, p.Accent2).Bold(true).
			Border(lipgloss.RoundedBorder(), false, true, false, true).
			BorderForeground(borderC),
		kbdChip: on(bgColor, p.Accent2).Bold(true),
		optChip: on(secondary, p.Fg),

		// High-contrast selection: bright row, dark foreground.
		selText: on(accentC, p.SelectionFg),
		selDim:  lipgloss.NewStyle().Foreground(lipgloss.Lighten(selFg, 0.18)).Background(accentC),
		// Active keycaps avoid Lip Gloss borders: border cells can fall back to
		// the terminal background and cut bars through the accent row.
		selChip: on(accentC, p.SelectionFg).Bold(true),
		selSp:   lipgloss.NewStyle().Background(accentC),
	}
}

// inset returns an arbitrary foreground on the bg (details inset) background.
func (s styles) inset(fg shared.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(string(fg))).Background(s.bgColor)
}
