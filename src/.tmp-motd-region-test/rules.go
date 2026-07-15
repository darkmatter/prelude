package main

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// glowRule renders a full-width rule with a soft symmetric glow:
// bg → peak → bg. Used for section dividers.
func (r renderer) glowRule(char string, peak color.Color, width int) string {
	grad := lipgloss.Blend2D(width, 1, 0, r.st.blockBg, peak, r.st.blockBg)
	var b strings.Builder
	for col := 0; col < width; col++ {
		b.WriteString(r.st.on(r.st.blockBg, grad[col]).Render(char))
	}
	return b.String()
}

// divider is the quiet dashed glow used between major regions (card width).
func (r renderer) divider() string {
	return r.glowRule("┄", r.st.dividerPk, r.cardWidth)
}

// contentDivider is the same glow sized to contentWidth.
func (r renderer) contentDivider() string {
	return r.glowRule("┄", r.st.dividerPk, r.contentWidth)
}

// centeredContentHeadingRule breaks the content divider around a centered,
// bold heading. This is intentionally hardcoded for quick visual evaluation.
func (r renderer) centeredContentHeadingRule(text string) string {
	label := " " + text + " "
	labelWidth := lipgloss.Width(label)
	start := max((r.contentWidth-labelWidth)/2, 0)
	base := r.st.blockBg
	grad := lipgloss.Blend2D(r.contentWidth, 1, 0, base, r.st.dividerPk, base)

	var b strings.Builder
	labelRunes := []rune(label)
	for col := 0; col < r.contentWidth; col++ {
		if col >= start && col < start+labelWidth {
			ch := string(labelRunes[col-start])
			if ch == " " {
				if r.st.blockTransparent {
					b.WriteString(" ")
				} else {
					b.WriteString(r.st.blockFill.Render(" "))
				}
			} else {
				b.WriteString(inline(r.st.fgBold).Render(ch))
			}
			continue
		}
		if r.st.blockTransparent {
			b.WriteString(lipgloss.NewStyle().Foreground(grad[col]).Inline(true).Render("┄"))
		} else {
			b.WriteString(r.st.on(base, grad[col]).Render("┄"))
		}
	}
	return b.String()
}

// fadeRule paints a horizontal rule that optionally dissolves toward the
// right edge. Non-empty title text is inlined starting at column 1.
func (r renderer) fadeRule(title string, fade bool, surface, frame color.Color, width int) string {
	var grad []color.Color
	if fade {
		// Fade ~70% of the way toward the surface so the right end stays visible.
		fadeEnd := lipgloss.Blend2D(10, 1, 0, frame, surface)[7]
		grad = lipgloss.Blend2D(width, 1, 0, frame, fadeEnd)
	}

	ruleColor := func(col int) color.Color {
		if fade {
			return grad[col]
		}
		return frame
	}

	var titleRunes []rune
	lw, titleStart := 0, 1
	if title != "" {
		label := " " + title + " "
		titleRunes = []rune(label)
		lw = lipgloss.Width(label)
	}

	var b strings.Builder
	for col := 0; col < width; col++ {
		if lw > 0 && col >= titleStart && col < titleStart+lw {
			ch := string(titleRunes[col-titleStart])
			b.WriteString(inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Accent))).Bold(true)).Render(ch))
			continue
		}
		b.WriteString(inline(r.st.on(surface, ruleColor(col))).Render("─"))
	}
	return r.fillLine(b.String(), width, surface)
}
