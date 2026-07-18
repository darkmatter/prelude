package motd

import (
	"image/color"

	"prelude/pkg/ui"
)

// Codeblock renders a single recipe as a top-rule-fade code block by mapping
// MOTD recipe data and resolved styles into the shared ui.CodeBlock.
type Codeblock struct{ r renderer }

// Render maps MOTD recipe data and resolved styles into the shared ui.CodeBlock.
func (cb Codeblock) Render(recipe Recipe) []string {
	surface := cb.r.st.codeBg
	lines := make([]string, 0, len(recipe.Steps))
	for _, step := range recipe.Steps {
		lines = append(lines, cb.stepLine(step, surface))
	}

	return ui.CodeBlock{
		Context: ui.NewContext(cb.r.cfg.Palette, surface, false),
		Title:   recipe.Title,
		Lines:   lines,
		Indent:  cb.r.st.fill(surface).Render("  "),
		Width:   cb.r.contentWidth,
		Rule: ui.FadingRule{
			Frame: cb.r.st.frameC,
			Fade:  true,
		},
		HeaderTransparent: true,
	}.Render()
}

func (cb Codeblock) stepLine(s RecipeStep, surface color.Color) string {
	if s.Command == "" {
		if s.Comment == "" {
			return ""
		}
		return ui.Inline(cb.r.st.on(surface, cb.r.st.h.Color(string(cb.r.st.pal.Muted)))).Render("# " + s.Comment)
	}
	return ui.Inline(cb.r.st.on(surface, cb.r.st.h.Color(string(cb.r.st.pal.Fg))).Bold(true)).Render(s.Command)
}
