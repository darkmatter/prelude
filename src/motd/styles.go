package main

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"prelude/shared"
)

// Shade amounts for lipgloss.Lighten / Darken (0–1). Match the playground.
const (
	headerLighten    = 0.05 // header bar vs block background
	codeblockDarken  = 0.05 // recipe block surface vs block background
	frameLighten     = 0.12 // rule/frame color vs codeblock surface
	dividerPeakDark  = 0.35 // glow-rule peak vs pure accent
)

// styles holds every lipgloss style derived from the theme palette.
// Presentation only — no layout widths or content decisions live here.
type styles struct {
	pal shared.Palette
	h   shared.PaletteHelper

	blockBg   color.Color
	headerBg  color.Color
	codeBg    color.Color
	frameC    color.Color
	dividerPk color.Color
	windowBg  color.Color

	blockFill  lipgloss.Style
	windowFill lipgloss.Style
	headerFill lipgloss.Style

	// Block layer (page background).
	dim    lipgloss.Style
	muted  lipgloss.Style
	fg     lipgloss.Style
	fgBold lipgloss.Style
	accent lipgloss.Style
	amber  lipgloss.Style // accent2 — git branch / warnings

	// Header layer (shared surface for wordmark + status).
	headerDim    lipgloss.Style
	headerMuted  lipgloss.Style
	headerFg     lipgloss.Style
	headerAccent lipgloss.Style
}

func newStyles(cfg Config) styles {
	p := cfg.Palette
	h := shared.NewPaletteHelper(p)

	blockBg := h.Color(cfg.Background)
	baseBg := blockBg
	if cfg.Background == "" {
		baseBg = h.Color(string(p.Bg))
	}

	headerBg := lipgloss.Lighten(baseBg, headerLighten)
	codeBg := lipgloss.Darken(baseBg, codeblockDarken)
	frameC := lipgloss.Lighten(codeBg, frameLighten)
	dividerPk := lipgloss.Darken(h.Color(string(p.Accent)), dividerPeakDark)

	onBlock := func(fg shared.Color) lipgloss.Style {
		return h.On(blockBg, string(fg))
	}
	onHeader := func(fg shared.Color) lipgloss.Style {
		return h.On(headerBg, string(fg))
	}

	blockFill := lipgloss.NewStyle()
	if cfg.Background != "" {
		blockFill = blockFill.Background(blockBg)
	}
	windowFill := lipgloss.NewStyle()
	if cfg.WindowBackground != "" {
		windowFill = windowFill.Background(h.Color(cfg.WindowBackground))
	}
	headerFill := lipgloss.NewStyle().Background(headerBg)

	return styles{
		pal: p,
		h:   h,

		blockBg:   blockBg,
		headerBg:  headerBg,
		codeBg:    codeBg,
		frameC:    frameC,
		dividerPk: dividerPk,
		windowBg:  h.Color(cfg.WindowBackground),

		blockFill:  blockFill,
		windowFill: windowFill,
		headerFill: headerFill,

		dim:    onBlock(p.Dim),
		muted:  onBlock(p.Muted),
		fg:     onBlock(p.Fg),
		fgBold: onBlock(p.Fg).Bold(true),
		accent: onBlock(p.Accent),
		amber:  onBlock(p.Accent2),

		headerDim:    onHeader(p.Dim),
		headerMuted:  onHeader(p.Muted),
		headerFg:     onHeader(p.Fg),
		headerAccent: onHeader(p.Accent),
	}
}

// on returns a style with background then foreground (playground order).
func (s styles) on(bg, fg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(fg).Background(bg)
}

// fill is a background-only style.
func (s styles) fill(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Background(c)
}

// inline strips layout so only paint applies when joining segments.
func inline(st lipgloss.Style) lipgloss.Style {
	return st.Inline(true)
}
