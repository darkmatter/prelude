package motd

import (
	"charm.land/lipgloss/v2"

	"prelude/pkg/ui"
)

// GettingStartedView renders the MOTD's unified commands and recipes section
// under dim sub-labels. It composes MOTD-specific child components from the
// resolved renderer context.
type GettingStartedView struct{ r renderer }

// Render unifies commands + recipes under dim sub-labels only (no top-level
// "Getting Started" heading — keeps the banner quieter).
func (x GettingStartedView) Render() []string {
	hasCommands := len(x.r.cfg.Commands) > 0
	hasRecipes := len(x.r.cfg.Recipes) > 0
	if !hasCommands && !hasRecipes {
		return nil
	}

	gs := x.r.cfg.GettingStarted
	commandsLabel := gs.CommandsLabel
	if commandsLabel == "" {
		commandsLabel = "commands"
	}
	examplesLabel := gs.ExamplesLabel
	if examplesLabel == "" {
		examplesLabel = "examples"
	}

	content := ui.Surface{Context: x.r.blockUI, Width: x.r.contentWidth}
	var out []string
	if hasCommands {
		out = append(out, x.subLabel(commandsLabel), content.Blank())
		out = append(out, Commands{r: x.r}.Render()...)
	}
	if hasCommands && hasRecipes {
		out = append(out, content.Blank())
	}
	if hasRecipes {
		out = append(out, x.subLabel(examplesLabel), content.Blank())
		out = append(out, Recipes{r: x.r}.Render()...)
	}
	return out
}

func (x GettingStartedView) subLabel(text string) string {
	return x.r.st.blockFill.Width(x.r.contentWidth).Render(
		ui.Inline(x.r.st.dim).Render(text),
	)
}

func (x GettingStartedView) centeredHeading(text string) string {
	return x.r.st.blockFill.Width(x.r.contentWidth).Align(lipgloss.Center).Render(
		ui.Inline(x.r.st.fgBold).Render(text),
	)
}
