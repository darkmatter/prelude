package main

import (
	"charm.land/lipgloss/v2"
)

// renderShortcuts paints a quiet right-aligned discoverability line.
// Replaces the old inverted footer bar.
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
	return r.st.blockFill.Width(r.cardWidth).Align(lipgloss.Right).Render(content)
}
