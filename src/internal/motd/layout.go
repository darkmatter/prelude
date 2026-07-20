package motd

import "prelude/pkg/ui"

// Layout constants encode hard-coded column geometry — not configuration.
const (
	minimumCardWidth = 10
	headerRightPad   = 2 // keep status off the header edge
)

// renderer is immutable render context for one MOTD pass. It carries resolved
// configuration, palette styles, and geometry; named UI components receive this
// context and own presentation in their own files.
type renderer struct {
	cfg              Config
	st               styles
	blockUI          ui.Context
	headerUI         ui.Context
	windowUI         ui.Context
	terminalWidth    int
	terminalHeight   int
	cardWidth        int
	contentWidth     int
	horizontalOffset int
}

func newRenderer(cfg Config, terminalWidth, terminalHeight int) renderer {
	height := max(terminalHeight, 1)
	// Height-gated spacing resolves once here so every consumer (margins,
	// card padding, offsets) sees the same effective values.
	cfg.Margin = cfg.Margin.collapseVertical(height)
	cfg.Padding = cfg.Padding.collapseVertical(height)
	cardWidth := resolveCardWidth(cfg.Width, cfg.MaxWidth, terminalWidth)
	padLeft := max(cfg.Padding.Left, 0)
	padRight := max(cfg.Padding.Right, 0)
	st := newStyles(cfg)
	r := renderer{
		cfg:            cfg,
		st:             st,
		blockUI:        ui.NewContext(cfg.Palette, st.blockBg, st.blockTransparent),
		headerUI:       ui.NewContext(cfg.Palette, st.headerBg, st.headerTransparent),
		windowUI:       ui.NewContext(cfg.Palette, st.windowBg, st.windowTransparent),
		terminalWidth:  max(terminalWidth, 1),
		terminalHeight: height,
		cardWidth:      cardWidth,
		contentWidth:   max(cardWidth-padLeft-padRight, 1),
	}
	r.horizontalOffset = r.resolveHorizontalOffset()
	return r
}

// resolveCardWidth applies width / maxWidth / terminal policy.
func resolveCardWidth(width, maxWidth, terminalWidth int) int {
	cardWidth := width
	if cardWidth == 0 {
		cardWidth = terminalWidth
	}
	if maxWidth > 0 && cardWidth > maxWidth {
		cardWidth = maxWidth
	}
	return max(cardWidth, minimumCardWidth)
}

func (r renderer) resolveHorizontalOffset() int {
	switch r.cfg.Align {
	case "right":
		return max(r.terminalWidth-r.cardWidth-r.cfg.Margin.Right, 0)
	case "center":
		return max((r.terminalWidth-r.cardWidth)/2+r.cfg.Margin.Left-r.cfg.Margin.Right, 0)
	default:
		return max(r.cfg.Margin.Left, 0)
	}
}
