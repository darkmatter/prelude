package menu

import (
	"charm.land/lipgloss/v2"

	"prelude/pkg/ui"
)

// Frame adapts menu-specific styles to the shared ui.PanelFrame renderer.
// It preserves the menu's existing construction and method API.
type Frame struct {
	st   styles
	base ui.PanelFrame
}

func (f Frame) panel() ui.PanelFrame {
	panel := f.base
	panel.Context = f.st.bodyUI
	return panel
}

// Paint pads a styled line to the inner width with the given filler and wraps
// it in the frame's side rails.
func (f Frame) Paint(content string, filler lipgloss.Style) string {
	return f.panel().Paint(content, filler)
}

// Top is the rounded top cap: ╭─…─╮.
func (f Frame) Top() string { return f.panel().Top() }

// Divider is the framed horizontal divider: ├─…─┤.
func (f Frame) Divider() string { return f.panel().Divider() }

// Bottom is the rounded bottom cap: ╰─…─╯.
func (f Frame) Bottom() string { return f.panel().Bottom() }

// Blank is a frame-wrapped empty row on the body filler.
func (f Frame) Blank() string { return f.panel().Blank() }

// WithSize returns a copy with the panel's inner width, set on WindowSizeMsg.
func (f Frame) WithSize(inner int) Frame {
	f.base = f.base.WithSize(inner)
	return f
}
