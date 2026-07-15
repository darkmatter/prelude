package main

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Title style variants for the header wordmark.
const (
	titleStylePlain     = "plain"
	titleStyleSpine     = "spine"
	titleStyleBracketed = "bracketed"
	titleStyleLabel     = "label"
	titleStyleInline    = "inline"   // project name centered in the accent gradient rule
	titleStyleInverted  = "inverted" // solid accent chip, selectionFg text
)

// renderHeader paints only the region above the divider. The divider itself
// belongs to the window/default surface, while activation text and everything
// below it belong to the container surface.
func (r renderer) renderHeader() string {
	style := strings.ToLower(r.cfg.Header.TitleStyle)
	if style == titleStyleInline {
		return join(r.renderInlineHeader()...)
	}
	return join(r.renderRowHeader(style)...)
}

func (r renderer) renderHeaderDivider() string {
	if strings.ToLower(r.cfg.Header.TitleStyle) == titleStyleInline {
		return r.inlineTitleRule(r.cfg.Project)
	}
	return r.headerUnderline()
}

// renderRowHeader is the header-owned title/status row plus its trailing space.
// The divider and lower spacing are composed separately.
func (r renderer) renderRowHeader(style string) []string {
	title := r.headerTitle(style)

	contentWidth := r.cardWidth - headerRightPad
	info := r.renderStatusItems(r.cfg.Header.Status, false)
	if info != "" && lipgloss.Width(title)+2+lipgloss.Width(info) > contentWidth {
		info = r.renderStatusItems(r.cfg.Header.Status, true)
	}

	var row string
	if info == "" {
		row = r.fillHeaderLine(title, r.cardWidth)
	} else {
		gap := max(contentWidth-lipgloss.Width(title)-lipgloss.Width(info), 1)
		row = r.fillHeaderLine(title+r.headerGap(gap)+info+r.headerGap(headerRightPad), r.cardWidth)
	}
	return []string{row, r.headerBlankLine()}
}

// renderInlineHeader centers the project name inside the accent gradient rule
// (playground headingRule with start=-1). Status chips sit on a quiet row above.
func (r renderer) renderInlineHeader() []string {
	var parts []string
	if info := r.renderStatusItems(r.cfg.Header.Status, false); info != "" {
		// Right-align status above the rule so it doesn't fight the centered title.
		row := r.st.headerFill.Width(r.cardWidth).Align(lipgloss.Right).Render(info)
		if r.st.headerTransparent {
			// headerFill may not pad when transparent; place manually.
			w := lipgloss.Width(info)
			if w < r.cardWidth {
				row = strings.Repeat(" ", r.cardWidth-w) + info
			} else {
				row = info
			}
		}
		parts = append(parts, row, r.headerBlankLine())
	}
	return parts
}

// inlineTitleRule is a full-width accent glow with the title centered in a break
// (playground headingRule(text, -1), using ━ like the header underline).
func (r renderer) inlineTitleRule(title string) string {
	label := " " + title + " "
	labelWidth := lipgloss.Width(label)
	start := max((r.cardWidth-labelWidth)/2, 0)
	peak := r.st.headerUnderlinePk
	base := r.st.windowBg
	grad := lipgloss.Blend2D(r.cardWidth, 1, 0, base, peak, base)

	var b strings.Builder
	labelRunes := []rune(label)
	for col := 0; col < r.cardWidth; col++ {
		if col >= start && col < start+labelWidth {
			ch := string(labelRunes[col-start])
			if ch == " " {
				b.WriteString(r.st.windowFill.Render(" "))
			} else {
				b.WriteString(inline(r.st.onWindow(r.st.h.Color(string(r.st.pal.Fg))).Bold(true)).Render(ch))
			}
			continue
		}
		b.WriteString(r.st.onWindow(grad[col]).Inline(true).Render("━"))
	}
	return b.String()
}

// headerUnderline is the accent glow rule under the wordmark. Peak is a
// slightly darkened accent so the center reads softer than pure accent.
func (r renderer) headerUnderline() string {
	grad := lipgloss.Blend2D(r.cardWidth, 1, 0, r.st.windowBg, r.st.headerUnderlinePk, r.st.windowBg)
	var b strings.Builder
	for col := range r.cardWidth {
		b.WriteString(r.st.onWindow(grad[col]).Render("━"))
	}
	return b.String()
}

func (r renderer) headerBlankLine() string {
	return r.st.headerFill.Width(r.cardWidth).Render("")
}

func (r renderer) headerGap(n int) string {
	pad := strings.Repeat(" ", n)
	if !r.st.headerTransparent {
		return r.st.headerFill.Render(pad)
	}
	return pad
}

// fillHeaderLine pads the title row to width on the header surface.
func (r renderer) fillHeaderLine(content string, width int) string {
	w := lipgloss.Width(content)
	if w >= width {
		return content
	}
	pad := strings.Repeat(" ", width-w)
	if !r.st.headerTransparent {
		return content + r.st.headerFill.Render(pad)
	}
	return content + pad
}

// renderStatusItems paints resolved status badges. compact drops labels and
// keeps only the indicator + status text. Dot color reflects Level:
// success/static → accent, warning → accent2, error → error.
func (r renderer) renderStatusItems(items []HeaderStatus, compact bool) string {
	var parts []string
	for _, it := range items {
		label, status := it.Label, it.Status
		if label == "" && status == "" {
			continue
		}
		dot := r.st.headerAccent
		switch it.Level {
		case "error":
			dot = r.st.headerErr
		case "warning":
			dot = r.st.headerAmber
		}
		var chip string
		if compact {
			if status != "" {
				chip = inline(dot).Render("● ") + inline(r.st.headerMuted).Render(status)
			} else if label != "" {
				chip = inline(r.st.headerDim).Render(label)
			}
		} else {
			if label != "" {
				chip += inline(r.st.headerDim).Render(label)
				if status != "" {
					chip += inline(r.st.headerDim).Render("  ")
				}
			}
			if status != "" {
				chip += inline(dot).Render("● ") + inline(r.st.headerMuted).Render(status)
			}
		}
		if chip != "" {
			parts = append(parts, chip)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	sep := inline(r.st.headerDim).Render("  ·  ")
	return strings.Join(parts, sep)
}

// renderActivation paints the post-underline block inspired by voy activate.sh:
// bold accent2 title + optional faint muted subtitle.
// Layout/align come from header.taglineLayout / header.taglineAlign.
func (r renderer) renderActivation(tagline, subtitle string) []string {
	layout := strings.ToLower(r.cfg.Header.TaglineLayout)
	if layout == "" {
		layout = "stack"
	}
	align := strings.ToLower(r.cfg.Header.TaglineAlign)
	if align == "" {
		align = "left"
	}

	title := ""
	if tagline != "" {
		title = inline(r.st.amber).Bold(true).Render(tagline)
	}
	sub := ""
	if subtitle != "" {
		sub = inline(r.st.muted).Faint(true).Render(subtitle)
	}

	if layout == "inline" && title != "" && sub != "" {
		sep := inline(r.st.dim).Render("  ·  ")
		return []string{r.padContentLine(title+sep+sub, align)}
	}

	var out []string
	if title != "" {
		out = append(out, r.padContentLine(title, align))
	}
	if sub != "" {
		out = append(out, r.padContentLine(sub, align))
	}
	return out
}

// padContentLine places a styled fragment in the content band (left or center)
// and fills out to cardWidth.
func (r renderer) padContentLine(styled, align string) string {
	padLeft := max(r.cfg.Padding.Left, 0)
	left := ""
	if padLeft > 0 {
		if r.cfg.Background != "" {
			left = r.st.blockFill.Width(padLeft).Render("")
		} else {
			left = strings.Repeat(" ", padLeft)
		}
	}

	alignPos := lipgloss.Left
	if align == "center" {
		alignPos = lipgloss.Center
	}
	text := r.st.blockFill.Width(r.contentWidth).Align(alignPos).Render(styled)
	if r.cfg.Background == "" {
		// Manual place when blockFill has no background.
		w := lipgloss.Width(styled)
		pad := max(r.contentWidth-w, 0)
		switch align {
		case "center":
			leftPad := pad / 2
			text = strings.Repeat(" ", leftPad) + styled + strings.Repeat(" ", pad-leftPad)
		default:
			text = styled + strings.Repeat(" ", pad)
		}
	}

	line := left + text
	if r.cfg.Background == "" {
		w := lipgloss.Width(line)
		if w < r.cardWidth {
			return line + strings.Repeat(" ", r.cardWidth-w)
		}
		return line
	}
	return r.fillLine(line, r.cardWidth, r.st.blockBg)
}

// headerTitle renders the project wordmark on the card/page surface for
// non-inline styles.
func (r renderer) headerTitle(style string) string {
	name := r.cfg.Project
	dim, fg, accent := r.st.headerDim, r.st.headerFg, r.st.headerAccent
	switch strings.ToLower(style) {
	case titleStylePlain:
		return inline(accent).Bold(true).Render("  " + name + "  ")
	case titleStyleBracketed:
		return inline(dim).Render("  [ ") +
			inline(accent).Bold(true).Render(name) +
			inline(dim).Render(" ]  ")
	case titleStyleLabel:
		return inline(dim).Render("  devshell / ") +
			inline(fg).Bold(true).Render(name) +
			inline(dim).Render("  ")
	case titleStyleInverted:
		// Solid accent chip with dark/selection text — playground TitleInverted.
		chipFg := r.st.h.Color(string(r.st.pal.SelectionFg))
		if string(r.st.pal.SelectionFg) == "" {
			chipFg = r.st.h.Color(string(r.st.pal.Bg))
		}
		chipBg := r.st.h.Color(string(r.st.pal.Accent))
		return inline(lipgloss.NewStyle().Foreground(chipFg).Background(chipBg).Bold(true)).
			Render("  " + name + "  ")
	default: // spine
		return inline(accent).Render("  ▌ ") +
			inline(fg).Bold(true).Render(name) +
			inline(dim).Render("  ")
	}
}
