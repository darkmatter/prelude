package main

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// renderCommandRows paints next-step commands with dotted leaders to a
// right-aligned description (playground CommandsLeaders).
func (r renderer) renderCommandRows() []string {
	if len(r.cfg.Commands) == 0 {
		return nil
	}
	var out []string
	for _, cmd := range r.cfg.Commands {
		out = append(out, r.commandRow(cmd.Command, cmd.Description))
	}
	return out
}

func (r renderer) commandRow(cmd, desc string) string {
	left := inline(r.st.accent).Render("$ ") +
		inline(r.st.fgBold).Render(cmd)
	right := inline(r.st.muted).Render(desc)

	// Dotted leaders bridge command → right-aligned description.
	dots := max(r.contentWidth-2-ansi.StringWidth(cmd)-ansi.StringWidth(desc)-2, 1)
	leader := inline(r.st.dim).Render(" " + strings.Repeat("·", dots) + " ")
	line := left + leader + right
	return r.st.blockFill.Width(r.contentWidth).Render(line)
}
