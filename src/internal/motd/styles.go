package motd

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"prelude/pkg/shared"
	"prelude/pkg/ui"
)

// Color adjustment amounts (0–1). Match the playground.
const (
	headerLighten         = 0.05 // header bar vs block background
	codeblockSurfaceBlend = 0.50 // palette surface contribution over block background
	frameLighten          = 0.12 // rule/frame color vs codeblock surface
	dividerPeakDark       = 0.35 // section glow-rule peak vs pure accent
	headerUnderlineDarken = 0.22 // header ━ underline peak vs pure accent
)

// styles holds every lipgloss style derived from the theme palette.
// Presentation only — no layout widths or content decisions live here.
//
// Semantic styles come from a surface → ui.Context map (block / header /
// window). Fills and exceptional colors (codeBg, frame, divider peaks) stay
// explicit because they are blended surfaces, not palette tokens.
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

	// surfaces maps layer name → render context. Named *UI fields on the
	// renderer are built from these; styles keeps the map for derivation.
	surfaces map[string]ui.Context

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

	// Header layer (shared surface for wordmark + status).
	headerDim     lipgloss.Style
	headerMuted   lipgloss.Style
	headerFg      lipgloss.Style
	headerAccent  lipgloss.Style
	headerSuccess lipgloss.Style
	headerWarning lipgloss.Style
	headerInfo    lipgloss.Style
	headerError   lipgloss.Style
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

	blockContext := ui.NewContext(p, baseBg, !hasBlockBg)
	codeBg := blockContext.BlendBackground(h.Color(string(p.Surface)), codeblockSurfaceBlend)
	frameC := lipgloss.Lighten(codeBg, frameLighten)
	accentC := h.Color(string(p.Accent))
	dividerPk := lipgloss.Darken(accentC, dividerPeakDark)
	headerUnderlinePk := lipgloss.Darken(accentC, headerUnderlineDarken)

	windowTransparent := cfg.WindowBackground == ""
	windowBg := h.Color(string(p.Bg))
	if !windowTransparent {
		windowBg = h.Color(cfg.WindowBackground)
	}

	// Surface map owns semantic styles. Header falls through to block when
	// transparent so raised/transparent headers share one derivation path.
	headerCtxBg := headerBg
	headerCtxTransparent := headerTransparent && !hasBlockBg
	if headerTransparent && hasBlockBg {
		headerCtxBg = blockBg
		headerCtxTransparent = false
	}
	surfaces := map[string]ui.Context{
		"block":  blockContext,
		"header": ui.NewContext(p, headerCtxBg, headerCtxTransparent),
		"window": ui.NewContext(p, windowBg, windowTransparent),
	}
	blockUI := surfaces["block"]
	headerUI := surfaces["header"]

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

		surfaces: surfaces,

		blockFill:  blockUI.Fill(),
		windowFill: surfaces["window"].Fill(),
		headerFill: headerUI.Fill(),

		dim:    blockUI.Dim(),
		muted:  blockUI.Muted(),
		fg:     blockUI.Foreground(),
		fgBold: blockUI.Foreground().Bold(true),
		accent: blockUI.Accent(),
		amber:  blockUI.Accent2(),

		headerDim:     headerUI.Dim(),
		headerMuted:   headerUI.Muted(),
		headerFg:      headerUI.Foreground(),
		headerAccent:  headerUI.Accent(),
		headerSuccess: headerUI.Success(),
		headerWarning: headerUI.Warning(),
		headerInfo:    headerUI.Info(),
		headerError:   headerUI.Error(),
	}
}

// surface returns the named surface Context from the derivation map.
func (s styles) surface(name string) ui.Context {
	if c, ok := s.surfaces[name]; ok {
		return c
	}
	return s.surfaces["block"]
}

// on returns a style with background then foreground (playground order).
func (s styles) on(bg, fg color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(fg).Background(bg)
}

func (s styles) onWindow(fg color.Color) lipgloss.Style {
	if s.windowTransparent {
		return lipgloss.NewStyle().Foreground(fg)
	}
	return s.on(s.windowBg, fg)
}

// fill is a background-only style. No-op background when c is unused by caller.
func (s styles) fill(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Background(c)
}
