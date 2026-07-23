package motd

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/ui"
)

// sections holds the shallow MOTD view adapters that map resolved model data
// into shared pkg/ui widgets. Collapsing them into one module keeps the layout
// composer from bouncing across a dozen single-method files.
type sections struct{ r renderer }

// commands renders the MOTD's next-step command list with dotted leaders.
func (s sections) commands() []string {
	if len(s.r.model.Config.Commands) == 0 {
		return nil
	}
	var out []string
	for _, cmd := range s.r.model.Config.Commands {
		out = append(out, s.commandRow(cmd.Command, cmd.Description))
	}
	return out
}

func (s sections) commandRow(command, description string) string {
	prefix := ""
	if remainder, ok := strings.CutPrefix(command, "x "); ok {
		prefix = "x "
		command = remainder
	}
	return ui.CommandRow{
		Context:     s.r.blockUI,
		Prefix:      prefix,
		Command:     command,
		Description: description,
		Width:       s.r.contentWidth,
	}.Render()
}

// recipes renders each recipe as a top-rule-fade codeblock.
func (s sections) recipes() []string {
	if len(s.r.model.Config.Recipes) == 0 {
		return nil
	}
	content := ui.Surface{Context: s.r.blockUI, Width: s.r.contentWidth}
	var out []string
	for i, recipe := range s.r.model.Config.Recipes {
		if i > 0 {
			out = append(out, content.Blank())
		}
		out = append(out, s.codeblock(recipe)...)
	}
	return out
}

func (s sections) codeblock(recipe Recipe) []string {
	surface := s.r.st.codeBg
	lines := make([]string, 0, len(recipe.Steps))
	for _, step := range recipe.Steps {
		lines = append(lines, s.codeblockStep(step, surface))
	}

	return ui.CodeBlock{
		Context: ui.NewContext(s.r.model.Config.Palette, surface, false),
		Title:   recipe.Title,
		Lines:   lines,
		Indent:  s.r.st.fill(surface).Render("  "),
		Width:   s.r.contentWidth,
		Rule: ui.FadingRule{
			Frame: s.r.st.frameC,
			Fade:  true,
		},
	}.Render()
}

func (s sections) codeblockStep(step RecipeStep, surface color.Color) string {
	muted := s.r.st.h.Color(string(s.r.st.pal.Muted))
	fg := s.r.st.h.Color(string(s.r.st.pal.Fg))
	if step.Command == "" {
		if step.Comment == "" {
			return ""
		}
		return ui.Inline(s.r.st.on(surface, muted)).Render("# " + step.Comment)
	}
	return ui.Inline(s.r.st.on(surface, fg).Bold(true)).Render(step.Command)
}

// links renders configured terminal hyperlinks.
func (s sections) links() []string {
	if len(s.r.model.Config.Links) == 0 {
		return nil
	}

	surface := ui.Surface{Context: s.r.blockUI, Width: s.r.contentWidth}
	var lines []string
	for _, link := range s.r.model.Config.Links {
		for _, labelLine := range ui.WrapText(link.Label, s.r.contentWidth) {
			rendered := (ui.Link{
				Context: s.r.blockUI,
				Label:   labelLine,
				URL:     link.URL,
			}).Render()
			if rendered != "" {
				lines = append(lines, surface.Fill(rendered))
			}
		}
	}
	return lines
}

// shortcuts renders the enable-derived component navigation chips.
func (s sections) shortcuts() string {
	items := make([]ui.Shortcut, len(s.r.model.Config.Shortcuts))
	for i, shortcut := range s.r.model.Config.Shortcuts {
		items[i] = ui.Shortcut{
			Command: shortcut.Command,
			Alias:   shortcut.Alias,
		}
	}

	return ui.ShortcutList{
		Context: s.r.blockUI,
		Items:   items,
	}.Render()
}

// gettingStarted unifies commands + recipes under dim sub-labels.
func (s sections) gettingStarted() []string {
	hasCommands := len(s.r.model.Config.Commands) > 0
	hasRecipes := len(s.r.model.Config.Recipes) > 0
	if !hasCommands && !hasRecipes {
		return nil
	}

	gs := s.r.model.Config.GettingStarted
	commandsLabel := gs.CommandsLabel
	if commandsLabel == "" {
		commandsLabel = "commands"
	}
	examplesLabel := gs.ExamplesLabel
	if examplesLabel == "" {
		examplesLabel = "examples"
	}

	content := ui.Surface{Context: s.r.blockUI, Width: s.r.contentWidth}
	var out []string
	if hasCommands {
		out = append(out, s.subLabel(commandsLabel), content.Blank())
		out = append(out, s.commands()...)
	}
	if hasCommands && hasRecipes {
		out = append(out, content.Blank())
	}
	if hasRecipes {
		out = append(out, s.subLabel(examplesLabel), content.Blank())
		out = append(out, s.recipes()...)
	}
	return out
}

func (s sections) subLabel(text string) string {
	return s.r.st.blockFill.Width(s.r.contentWidth).Render(
		ui.Inline(s.r.st.dim).Render(text),
	)
}

// description renders onboarding prose and optional tip lines.
func (s sections) description() []string {
	d := s.r.model.Config.Description
	if d.Text == "" && len(d.Tips) == 0 {
		return nil
	}

	var lines ui.Block
	if d.Text != "" {
		fillStyle := s.descFill(d)
		for _, line := range (Markdown{r: s.r}).Render(d.Text, d, s.r.contentWidth) {
			lines.Write(fillStyle.Render(line))
		}
	}

	if len(d.Tips) > 0 {
		if d.Text != "" {
			lines.Write(s.r.st.blockFill.Width(s.r.contentWidth).Render(""))
		}
		for _, tip := range d.Tips {
			for _, row := range s.renderTipLine(tip) {
				lines.Write(row)
			}
		}
	}
	return ui.SplitLines(lines.String())
}

func (s sections) descFill(d StyledText) lipgloss.Style {
	fillStyle := lipgloss.NewStyle().Width(s.r.contentWidth).MaxWidth(s.r.contentWidth)
	if s.r.model.DescriptionBackground != "" {
		return fillStyle.Background(lipgloss.Color(s.r.model.DescriptionBackground))
	}
	if s.r.model.Background != "" {
		return fillStyle.Background(s.r.st.blockBg)
	}
	return fillStyle
}

func (s sections) renderTipLine(tip string) []string {
	var b strings.Builder
	leading := true
	rest := tip
	for {
		start := strings.Index(rest, "`")
		if start < 0 {
			if rest != "" {
				role := s.r.st.dim
				if !leading {
					role = s.r.st.muted
				}
				b.WriteString(ui.Inline(role).Render(rest))
			}
			break
		}
		if start > 0 {
			role := s.r.st.dim
			if !leading {
				role = s.r.st.muted
			}
			b.WriteString(ui.Inline(role).Render(rest[:start]))
		}
		rest = rest[start+1:]
		end := strings.Index(rest, "`")
		if end < 0 {
			b.WriteString(ui.Inline(s.r.st.muted).Render("`" + rest))
			break
		}
		code := rest[:end]
		b.WriteString(ui.Inline(s.r.st.accent).Bold(true).Render(code))
		rest = rest[end+1:]
		leading = false
	}

	return s.wrapAndFill(b.String(), s.r.contentWidth)
}

func (s sections) wrapAndFill(value string, width int) []string {
	st := s.r.st.blockFill.Width(width).MaxWidth(width)
	var bl ui.Block
	for _, line := range strings.Split(lipgloss.Wrap(value, width, ""), "\n") {
		bl.Write(st.Render(line))
	}
	return ui.SplitLines(bl.String())
}

// env renders tool versions as one flowing row of chips.
func (s sections) env() []string {
	var row strings.Builder
	for _, item := range s.r.model.Env {
		if rendered, ok := s.renderEnvItem(item); ok {
			row.WriteString(rendered)
		}
	}

	if strings.TrimSpace(ansi.Strip(row.String())) == "" {
		return nil
	}
	return s.wrapAndFill(row.String(), s.r.contentWidth)
}

func (s sections) renderEnvItem(item ResolvedEnv) (string, bool) {
	if item.Value == "" {
		return "", false
	}
	return ui.Inline(s.r.st.dim).Render(item.Label+" ") +
		ui.Inline(s.r.st.fgBold).Render(item.Value+"   "), true
}

// activation renders the post-underline tagline/subtitle/shortcuts block.
func (s sections) activation(tagline, subtitle, shortcuts string) []string {
	layout := strings.ToLower(s.r.model.Config.Header.TaglineLayout)
	if layout == "" {
		layout = "stack"
	}
	align := lipgloss.Left
	switch strings.ToLower(s.r.model.Config.Header.TaglineAlign) {
	case "center":
		align = lipgloss.Center
	case "right":
		align = lipgloss.Right
	}

	title := ""
	if tagline != "" {
		title = ui.Inline(s.r.st.amber).Bold(true).Render(tagline)
	}
	sub := ""
	if subtitle != "" {
		sub = ui.Inline(s.r.st.muted).Faint(true).Render(subtitle)
	}

	place := func(content string) string {
		return ui.PlaceContentLine(content, s.r.cardWidth, s.r.contentWidth, s.r.model.Padding.Left, align, s.r.st.blockFill)
	}
	inline := title
	if layout == "inline" && sub != "" {
		if inline != "" {
			inline += ui.Inline(s.r.st.dim).Render("  ·  ")
		}
		inline += sub
		sub = ""
	}

	var out []string
	if shortcuts != "" && inline != "" && lipgloss.Width(inline)+1+lipgloss.Width(shortcuts) <= s.r.contentWidth {
		out = append(out, place(ui.PlaceRight(s.r.contentWidth, inline, shortcuts, s.r.blockUI.Fill())))
	} else {
		for _, line := range s.wrapInline(inline) {
			out = append(out, place(line))
		}
		for _, line := range s.wrapInline(shortcuts) {
			out = append(out, ui.PlaceContentLine(line, s.r.cardWidth, s.r.contentWidth, s.r.model.Padding.Left, lipgloss.Right, s.r.st.blockFill))
		}
	}
	if sub != "" {
		if layout != "inline" {
			out = append(out, place(""))
		}
		out = append(out, place(sub))
	}
	return out
}

func (s sections) wrapInline(content string) []string {
	var out strings.Builder
	writer := lipgloss.NewWrapWriter(&out)
	defer writer.Close() //nolint:errcheck
	_, _ = writer.Write([]byte(ansi.Wrap(content, s.r.contentWidth, "")))
	return ui.SplitLines(out.String())
}

// headerTitle renders the project wordmark for the given style variant.
func (s sections) headerTitle(style string) string {
	name := s.r.model.Config.Project
	dim, fg, accent := s.r.st.headerDim, s.r.st.headerFg, s.r.st.headerAccent
	switch strings.ToLower(style) {
	case titleStylePlain:
		return ui.Inline(accent).Bold(true).Render("  " + name + "  ")
	case titleStyleBracketed:
		return ui.Inline(dim).Render("  [ ") +
			ui.Inline(accent).Bold(true).Render(name) +
			ui.Inline(dim).Render(" ]  ")
	case titleStyleLabel:
		return ui.Inline(dim).Render("  devshell / ") +
			ui.Inline(fg).Bold(true).Render(name) +
			ui.Inline(dim).Render("  ")
	case titleStyleInverted:
		chipFg := s.r.st.h.Color(string(s.r.st.pal.SelectionFg))
		if string(s.r.st.pal.SelectionFg) == "" {
			chipFg = s.r.st.h.Color(string(s.r.st.pal.Bg))
		}
		chipBg := s.r.st.h.Color(string(s.r.st.pal.Accent))
		return ui.Inline(lipgloss.NewStyle().Foreground(chipFg).Background(chipBg).Bold(true)).
			Render("  " + name + "  ")
	default: // spine
		return ui.Inline(accent).Render("  ▌ ") +
			ui.Inline(fg).Bold(true).Render(name) +
			ui.Inline(dim).Render("  ")
	}
}
