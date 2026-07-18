package motd

import "prelude/pkg/ui"

// Shortcuts renders the enable-derived component navigation supplied by Nix.
// Keeping this component stateless preserves the invariant that presentation
// cannot add, remove, or retarget built-in aliases.
type Shortcuts struct {
	r renderer
}

// Render builds the shortcuts row as one flowing line of command chips. It is
// empty only when no shortcut-bearing Prelude component is enabled.
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
