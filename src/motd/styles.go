package main

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"prelude/shared"
)

// Shade amounts for lipgloss.Lighten / Darken (0–1). Match the playground.
const (
	headerLighten         = 0.05 // header bar vs block background
	codeblockDarken       = 0.05 // recipe block surface vs block background
	frameLighten          = 0.12 // rule/frame color vs codeblock surface
	dividerPeakDark       = 0.35 // section glow-rule peak vs pure accent
	headerUnderlineDarken = 0.22 // header ━ underline peak vs pure accent
)

// styles holds every lipgloss style derived from the theme palette.
// Presentation only — no layout widths or content decisions live here.
type styles struct {
	pal shared.Palette
	h   shared.PaletteHelper

	blockBg           color.Color
	blockTransparent  bool
	windowBg          color.Color
	windowTransparent bool
	headerBg          color.Color
	headerTransparent bool
	codeBg            color.Color
	frameC            color.Color
	dividerPk         color.Color
	headerUnderlinePk color.Color // dimmed accent peak for the ━ hero rule

	blockFill  lipgloss.Style
	windowFill lipgloss.Style
	headerFill lipgloss.Style

	// Block layer (page background).
	dim    lipgloss.Style
	muted  lipgloss.Style
	fg     lipgloss.Style
	fgBold lipgloss.Style
	accent lipgloss.Style
	amber  lipgloss.Style // accent2
	err    lipgloss.Style

	// Header layer (shared surface for wordmark + status).
	headerDim    lipgloss.Style
	headerMuted  lipgloss.Style
	headerFg     lipgloss.Style
	headerAccent lipgloss.Style
	headerAmber  lipgloss.Style
	headerErr    lipgloss.Style
}

func newStyles(cfg Config) styles {
	p := cfg.Palette
	h := shared.NewPaletteHelper(p)

	hasBlockBg := cfg.Background != ""
	blockBg := h.Color(string(p.Bg))
	if hasBlockBg {
		blockBg = h.Color(cfg.Background)
	}
	baseBg := blockBg

	headerTransparent := false
	var headerBg color.Color
	switch {
	case cfg.Header.Background != "":
		headerBg = h.Color(cfg.Header.Background)
	case cfg.Header.BackgroundRaised:
		headerBg = lipgloss.Lighten(baseBg, headerLighten)
	default:
		headerTransparent = true
		headerBg = baseBg
	}

	codeBg := lipgloss.Darken(baseBg, codeblockDarken)
	frameC := lipgloss.Lighten(codeBg, frameLighten)
	accentC := h.Color(string(p.Accent))
	dividerPk := lipgloss.Darken(accentC, dividerPeakDark)
	headerUnderlinePk := lipgloss.Darken(accentC, headerUnderlineDarken)

	onBlock := func(fg shared.Color) lipgloss.Style {
		if !hasBlockBg {
			return h.Plain(string(fg))
		}
		return h.On(blockBg, string(fg))
	}
	onHeader := func(fg shared.Color) lipgloss.Style {
		if headerTransparent {
			return h.Plain(string(fg))
		}
		return h.On(headerBg, string(fg))
	}

	blockFill := lipgloss.NewStyle()
	if hasBlockBg {
		blockFill = blockFill.Background(blockBg)
	}
	windowTransparent := cfg.WindowBackground == ""
	windowBg := h.Color(string(p.Bg))
	windowFill := lipgloss.NewStyle()
	if !windowTransparent {
		windowBg = h.Color(cfg.WindowBackground)
		windowFill = windowFill.Background(windowBg)
	}
	headerFill := lipgloss.NewStyle()
	if !headerTransparent {
		headerFill = headerFill.Background(headerBg)
	}

	return styles{
		pal: p,
		h:   h,

		blockBg:           blockBg,
		blockTransparent:  !hasBlockBg,
		windowBg:          windowBg,
		windowTransparent: windowTransparent,
		headerBg:          headerBg,
		headerTransparent: headerTransparent,
		codeBg:            codeBg,
		frameC:            frameC,
		dividerPk:         dividerPk,
		headerUnderlinePk: headerUnderlinePk,

		blockFill:  blockFill,
		windowFill: windowFill,
		headerFill: headerFill,

		dim:    onBlock(p.Dim),
		muted:  onBlock(p.Muted),
		fg:     onBlock(p.Fg),
		fgBold: onBlock(p.Fg).Bold(true),
		accent: onBlock(p.Accent),
		amber:  onBlock(p.Accent2),
		err:    onBlock(p.Error),

		headerDim:    onHeader(p.Dim),
		headerMuted:  onHeader(p.Muted),
		headerFg:     onHeader(p.Fg),
		headerAccent: onHeader(p.Accent),
		headerAmber:  onHeader(p.Accent2),
		headerErr:    onHeader(p.Error),
	}
}

// on returns a style with background then foreground (playground order).
func (s styles) on(bg, fg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(fg).Background(bg)
}

func (s styles) onWindow(fg color.Color) lipgloss.Style {
	style := lipgloss.NewStyle().Foreground(fg)
	if !s.windowTransparent {
		style = style.Background(s.windowBg)
	}
	return style
}

// fill is a background-only style. No-op background when c is unused by caller.
func (s styles) fill(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Background(c)
}

// inline strips layout so only paint applies when joining segments.
func inline(st lipgloss.Style) lipgloss.Style {
	return st.Inline(true)
}
