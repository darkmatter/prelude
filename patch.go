package motd

import (
	"github.com/charmbracelet/lipgloss"
	"prelude/pkg/ui"
)

func (r renderer) renderShortcutItems() string {
	if len(r.cfg.Shortcuts) == 0 {
		return ""
	}

	item := func(command, alias string) string {
		out := ui.Inline(r.st.muted).Bold(true).Render(command)
		if alias != "" {
			out += ui.Inline(r.st.dim).Render(" (" + alias + ")")
		}
		return out
	}

	var items []string
	for _, s := range r.cfg.Shortcuts {
		items = append(items, item(s.Command, s.Alias))
	}

	sep := ui.Inline(r.st.dim).Render("  ·  ")
	return lipgloss.JoinHorizontal(lipgloss.Top, lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(items, sep)))
}
