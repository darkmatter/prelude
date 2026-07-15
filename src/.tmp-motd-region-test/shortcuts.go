package main

import (
	"charm.land/lipgloss/v2"
)

// renderShortcuts paints a quiet right-aligned discoverability line inside the
// horizontally padded content area (respects padding.x / left / right).
func (r renderer) renderShortcuts() string {
	if len(r.cfg.Shortcuts) == 0 {
		return ""
	}

	item := func(command, alias string) string {
		out := inline(r.st.muted).Bold(true).Render(command)
		if alias != "" {
			out += inline(r.st.dim).Render(" (" + alias + ")")
		}
		return out
	}

	var content string
	for i, s := range r.cfg.Shortcuts {
		if i > 0 {
			content += inline(r.st.dim).Render("  ·  ")
		}
		content += item(s.Command, s.Alias)
	}

	// Right-align within the content band, then restore left/right card padding.
	padLeft := max(r.cfg.Padding.Left, 0)
	inner := r.st.blockFill.Width(r.contentWidth).Align(lipgloss.Right).Render(content)
	if padLeft == 0 {
		return r.fillCardLine(inner)
	}
	left := r.st.blockFill.Width(padLeft).Render("")
	return r.fillCardLine(left + inner)
}
