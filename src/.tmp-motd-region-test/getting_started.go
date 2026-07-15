package main

import (
	"charm.land/lipgloss/v2"
)

// renderGettingStarted unifies commands + recipes under dim sub-labels only
// (no top-level "Getting Started" heading — keeps the banner quieter).
func (r renderer) renderGettingStarted() []string {
	hasCommands := len(r.cfg.Commands) > 0
	hasRecipes := len(r.cfg.Recipes) > 0
	if !hasCommands && !hasRecipes {
		return nil
	}

	gs := r.cfg.GettingStarted
	commandsLabel := gs.CommandsLabel
	if commandsLabel == "" {
		commandsLabel = "commands"
	}
	examplesLabel := gs.ExamplesLabel
	if examplesLabel == "" {
		examplesLabel = "examples"
	}

	var out []string
	if hasCommands {
		out = append(out, r.subLabel(commandsLabel), r.blankContentLine())
		out = append(out, r.renderCommandRows()...)
	}
	if hasCommands && hasRecipes {
		out = append(out, r.blankContentLine())
	}
	if hasRecipes {
		out = append(out, r.subLabel(examplesLabel), r.blankContentLine())
		out = append(out, r.renderRecipeBlocks()...)
	}
	return out
}

func (r renderer) subLabel(text string) string {
	return r.st.blockFill.Width(r.contentWidth).Render(
		inline(r.st.dim).Render(text),
	)
}

func (r renderer) centeredHeading(text string) string {
	return r.st.blockFill.Width(r.contentWidth).Align(lipgloss.Center).Render(
		inline(r.st.fgBold).Render(text),
	)
}
