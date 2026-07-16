package motd

import (
	"strings"

	"charm.land/lipgloss/v2"

	"prelude/pkg/ui"
)

// Description is a React-style, one-component-per-file presentation of the motd
// description section. It paints onboarding prose (markdown via glamour) and
// optional tip lines. The component is stateless and uses the resolved renderer
// context for MOTD-specific config, styles, dimensions, and runtime data.
type Description struct {
	r renderer
}

// Render paints onboarding prose, then optional tip lines.
// The prose is markdown, rendered with glamour and themed from the palette
// (plus the section's style overrides). Tips may wrap `command` spans in
// backticks for accent highlighting (playground description tips).
func (x Description) Render() []string {
	d := x.r.cfg.Description
	if d.Text == "" && len(d.Tips) == 0 {
		return nil
	}

	var lines ui.Block
	if d.Text != "" {
		fillStyle := x.descFill(d)
		for _, line := range (Markdown{r: x.r}).Render(d.Text, d, x.r.contentWidth) {
			lines.Write(fillStyle.Render(line))
		}
	}

	if len(d.Tips) > 0 {
		if d.Text != "" {
			lines.Write(x.r.st.blockFill.Width(x.r.contentWidth).Render(""))
		}
		for _, tip := range d.Tips {
			for _, row := range x.renderTipLine(tip) {
				lines.Write(row)
			}
		}
	}
	return ui.SplitLines(lines.String())
}

func (x Description) descFill(d StyledText) lipgloss.Style {
	fillStyle := lipgloss.NewStyle().Width(x.r.contentWidth).MaxWidth(x.r.contentWidth)
	if d.Background != "" {
		return fillStyle.Background(lipgloss.Color(d.Background))
	}
	if x.r.cfg.Background != "" {
		return fillStyle.Background(x.r.st.blockBg)
	}
	return fillStyle
}

// renderTipLine paints one tip, highlighting `backtick` spans as accent bold.
// Leading dim role applies until the first backtick; after that, non-code text
// is muted — matching the playground tip cadence.
func (x Description) renderTipLine(tip string) []string {
	// Build a single styled string, then wrap on display width.
	var b strings.Builder
	leading := true
	rest := tip
	for {
		start := strings.Index(rest, "`")
		if start < 0 {
			if rest != "" {
				role := x.r.st.dim
				if !leading {
					role = x.r.st.muted
				}
				b.WriteString(ui.Inline(role).Render(rest))
			}
			break
		}
		if start > 0 {
			role := x.r.st.dim
			if !leading {
				role = x.r.st.muted
			}
			b.WriteString(ui.Inline(role).Render(rest[:start]))
		}
		rest = rest[start+1:]
		end := strings.Index(rest, "`")
		if end < 0 {
			// Unclosed backtick: paint remainder as muted.
			b.WriteString(ui.Inline(x.r.st.muted).Render("`" + rest))
			break
		}
		code := rest[:end]
		b.WriteString(ui.Inline(x.r.st.accent).Bold(true).Render(code))
		rest = rest[end+1:]
		leading = false
	}

	return (Env{r: x.r}).WrapAndFill(b.String(), x.r.contentWidth)
}
