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
// Semantic styles (fg/muted/dim/…) are derived from a surface → ui.Context
// map so each layer reuses Context.Style/Fill instead of hand-rolling on()/
// plain() pairs. Exceptional styles (chips, selection bar, plain no-bg
// output) stay explicit.
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

	// surfaces maps layer name → render context. Looked up once in newStyles;
	// the named *UI fields below are convenience aliases for existing callers.
	surfaces map[string]ui.Context

	bodyUI   ui.Context
	openUI   ui.Context
	chromeUI ui.Context
	windowUI ui.Context
	plainUI  ui.Context // transparent surface for non-TUI output

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

	// Surface map is the single source for semantic styles. Transparent
	// "plain" is the no-background export surface used by `menu list`.
	surfaces := map[string]ui.Context{
		"window": ui.NewContext(p, bgColor, false),
		"body":   ui.NewContext(p, body, false),
		"open":   ui.NewContext(p, open, false),
		"chrome": ui.NewContext(p, chrome, false),
		"plain":  ui.NewContext(p, nil, true),
	}
	windowUI := surfaces["window"]
	bodyUI := surfaces["body"]
	openUI := surfaces["open"]
	chromeUI := surfaces["chrome"]
	plainUI := surfaces["plain"]

	return styles{
		pal:         p,
		surfaces:    surfaces,
		bodyUI:      bodyUI,
		openUI:      openUI,
		chromeUI:    chromeUI,
		windowUI:    windowUI,
		plainUI:     plainUI,
		bodyColor:   body,
		openColor:   open,
		chromeColor: chrome,
		secondary:   secondary,
		bgColor:     bgColor,
		accentC:     accentC,
		borderC:     borderC,
		windowBg:    windowUI.Fill(),

		// plain export surface
		fg:      plainUI.Foreground(),
		muted:   plainUI.Muted(),
		dim:     plainUI.Dim(),
		accent:  plainUI.Accent(),
		accent2: plainUI.Accent2(),
		errText: plainUI.Error(),

		// body surface
		sFg:      bodyUI.Foreground(),
		sMuted:   bodyUI.Muted(),
		sDim:     bodyUI.Dim(),
		sAccent:  bodyUI.Accent(),
		sAccent2: bodyUI.Accent2(),
		sErr:     bodyUI.Error(),
		sp:       bodyUI.Fill(),
		frame:    bodyUI.Border(),

		// open surface
		openSp:      openUI.Fill(),
		openFg:      openUI.Foreground(),
		openMuted:   openUI.Muted(),
		openDim:     openUI.Dim(),
		openAccent:  openUI.Accent(),
		openAccent2: openUI.Accent2(),

		// chrome surface
		chromeSp:    chromeUI.Fill(),
		chromeMuted: chromeUI.Muted(),

		// Amber glyphs on body with left/right border rails only (one terminal row).
		keyChip: bodyUI.Accent2().Bold(true).
			Border(lipgloss.RoundedBorder(), false, true, false, true).
			BorderForeground(borderC),
		kbdChip: windowUI.Accent2().Bold(true),
		optChip: lipgloss.NewStyle().Foreground(h.Color(string(p.Fg))).Background(secondary),

		// High-contrast selection: bright row, dark foreground. Selection is
		// its own accent-bg surface, not a palette Context layer.
		selText: lipgloss.NewStyle().Foreground(selFg).Background(accentC),
		selDim:  lipgloss.NewStyle().Foreground(lipgloss.Lighten(selFg, 0.18)).Background(accentC),
		// Active keycaps avoid Lip Gloss borders: border cells can fall back to
		// the terminal background and cut bars through the accent row.
		selChip: lipgloss.NewStyle().Foreground(selFg).Background(accentC).Bold(true),
		selSp:   lipgloss.NewStyle().Background(accentC),
	}
}

// surface returns the named surface Context from the derivation map.
func (s styles) surface(name string) ui.Context {
	if c, ok := s.surfaces[name]; ok {
		return c
	}
	return s.bodyUI
}

// inset returns an arbitrary foreground on the bg (details inset) background.
func (s styles) inset(fg shared.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(string(fg))).Background(s.bgColor)
}
