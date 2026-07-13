package main

import (
	"image/color"
)

// renderRecipeBlocks paints each recipe as a top-rule-fade codeblock
// (playground CodeblockTopRuleFade). Chrome is accent-free; the title is
// the sole accented element.
func (r renderer) renderRecipeBlocks() []string {
	if len(r.cfg.Recipes) == 0 {
		return nil
	}
	var out []string
	for i, recipe := range r.cfg.Recipes {
		if i > 0 {
			out = append(out, r.blankContentLine())
		}
		out = append(out, r.codeblock(recipe)...)
	}
	return out
}

func (r renderer) blankContentLine() string {
	return r.st.blockFill.Width(r.contentWidth).Render("")
}

func (r renderer) codeblock(recipe Recipe) []string {
	surface := r.st.codeBg
	frame := r.st.frameC
	width := r.contentWidth

	top := r.fadeRule(recipe.Title, true, surface, frame, width)
	bot := r.fadeRule("", true, surface, frame, width)

	out := []string{top}
	for _, step := range recipe.Steps {
		out = append(out, r.fillLine("  "+r.stepLine(step, surface), width, surface))
	}
	return append(out, bot)
}

func (r renderer) stepLine(s RecipeStep, surface color.Color) string {
	if s.Command == "" {
		if s.Comment == "" {
			return ""
		}
		return inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Dim)))).Render("# " + s.Comment)
	}
	return inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Fg))).Bold(true)).Render(s.Command)
}
