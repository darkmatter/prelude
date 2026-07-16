package motd

import "prelude/pkg/ui"

// Commands renders the MOTD's next-step command list with dotted leaders by
// mapping MOTD command data and resolved styles into the shared ui.CommandRow.
type Commands struct{ r renderer }

// Render paints next-step commands with dotted leaders to a right-aligned
// description (playground CommandsLeaders).
func (x Commands) Render() []string {
	if len(x.r.cfg.Commands) == 0 {
		return nil
	}
	var out []string
	for _, cmd := range x.r.cfg.Commands {
		out = append(out, x.commandRow(cmd.Command, cmd.Description))
	}
	return out
}

func (x Commands) commandRow(command, description string) string {
	return ui.CommandRow{
		Context:     x.r.blockUI,
		Command:     command,
		Description: description,
		Width:       x.r.contentWidth,
	}.Render()
}
