package motd

import "prelude/pkg/ui"

// Shortcuts is a React-style, one-component-per-file presentation of the motd
// shortcuts row. It maps configured shortcuts and resolved styles into the
// shared ui.ShortcutList. The component is stateless and uses the resolved
// renderer context for MOTD-specific data and layout.
type Shortcuts struct {
	r renderer
}

// Render builds the shortcuts row as one flowing line of command chips. Returns
// an empty string when no shortcuts are configured.
func (x Shortcuts) Render() string {
	items := make([]ui.Shortcut, len(x.r.cfg.Shortcuts))
	for i, shortcut := range x.r.cfg.Shortcuts {
		items[i] = ui.Shortcut{
			Command: shortcut.Command,
			Alias:   shortcut.Alias,
		}
	}

	return ui.ShortcutList{
		Context: x.r.blockUI,
		Items:   items,
	}.Render()
}
