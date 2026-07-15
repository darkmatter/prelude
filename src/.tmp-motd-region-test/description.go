package main

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// renderDescription paints onboarding prose, then optional tip lines.
// The prose is markdown, rendered with glamour and themed from the palette
// (plus the section's style overrides). Tips may wrap `command` spans in
// backticks for accent highlighting (playground description tips).
func (r renderer) renderDescription() []string {
	d := r.cfg.Description
	if d.Text == "" && len(d.Tips) == 0 {
		return nil
	}

	var lines block
	if d.Text != "" {
		fillStyle := r.descFill(d)
		for _, line := range r.renderMarkdownBlock(d.Text, d, r.contentWidth) {
			lines.write(fillStyle.Render(line))
		}
	}

	if len(d.Tips) > 0 {
		if d.Text != "" {
			lines.write(r.st.blockFill.Width(r.contentWidth).Render(""))
		}
		for _, tip := range d.Tips {
			for _, row := range r.renderTipLine(tip) {
				lines.write(row)
			}
		}
	}
	return splitLines(lines.String())
}

func (r renderer) descFill(d StyledText) lipgloss.Style {
	fillStyle := lipgloss.NewStyle().Width(r.contentWidth).MaxWidth(r.contentWidth)
	if d.Background != "" {
		return fillStyle.Background(lipgloss.Color(d.Background))
	}
	if r.cfg.Background != "" {
		return fillStyle.Background(r.st.blockBg)
	}
	return fillStyle
}

// renderTipLine paints one tip, highlighting `backtick` spans as accent bold.
// Leading dim role applies until the first backtick; after that, non-code text
// is muted — matching the playground tip cadence.
func (r renderer) renderTipLine(tip string) []string {
	// Build a single styled string, then wrap on display width.
	var b strings.Builder
	leading := true
	rest := tip
	for {
		start := strings.Index(rest, "`")
		if start < 0 {
			if rest != "" {
				role := r.st.dim
				if !leading {
					role = r.st.muted
				}
				b.WriteString(inline(role).Render(rest))
			}
			break
		}
		if start > 0 {
			role := r.st.dim
			if !leading {
				role = r.st.muted
			}
			b.WriteString(inline(role).Render(rest[:start]))
		}
		rest = rest[start+1:]
		end := strings.Index(rest, "`")
		if end < 0 {
			// Unclosed backtick: paint remainder as muted.
			b.WriteString(inline(r.st.muted).Render("`" + rest))
			break
		}
		code := rest[:end]
		b.WriteString(inline(r.st.accent).Bold(true).Render(code))
		rest = rest[end+1:]
		leading = false
	}

	return r.wrapAndFill(b.String(), r.contentWidth)
}
