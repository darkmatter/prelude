package motd

import "prelude/pkg/ui"

// Recipes renders the MOTD's step-by-step command-bridge sections by composing
// the MOTD-specific Codeblock adapter.
type Recipes struct{ r renderer }

// Render paints each recipe as a top-rule-fade codeblock
// (playground CodeblockTopRuleFade). Chrome is accent-free; the title is
// the sole accented element.
func (x Recipes) Render() []string {
	if len(x.r.cfg.Recipes) == 0 {
		return nil
	}
	content := ui.Surface{Context: x.r.blockUI, Width: x.r.contentWidth}
	var out []string
	for i, recipe := range x.r.cfg.Recipes {
		if i > 0 {
			out = append(out, content.Blank())
		}
		out = append(out, Codeblock{r: x.r}.Render(recipe)...)
	}
	return out
}
