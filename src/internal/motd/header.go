package motd

import (
	"strings"

	"charm.land/lipgloss/v2"

	"prelude/pkg/ui"
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

// HeaderView renders the MOTD title and divider chrome.
type HeaderView struct{ r renderer }

// Render paints only the region above the divider. The divider itself belongs
// to the window/default surface, while activation text and everything below it
// belong to the container surface.
func (x HeaderView) Render() string {
	if x.r.model.Config.Title != "" {
		return strings.Join(x.renderGeneratedTitle(), "\n")
	}
	style := strings.ToLower(x.r.model.Config.Header.TitleStyle)
	if style == titleStyleInline {
		return ""
	}
	return strings.Join(x.renderRowHeader(style), "\n")
}

func (x HeaderView) Divider() string {
	if x.r.model.Config.Title == "" && strings.ToLower(x.r.model.Config.Header.TitleStyle) == titleStyleInline {
		return x.inlineTitleRule(x.r.model.Config.Project)
	}
	return x.headerUnderline()
}

func (x HeaderView) renderGeneratedTitle() []string {
	titleBlock := strings.TrimRight(x.r.model.Config.Title, "\n")
	lines := strings.Split(titleBlock, "\n")
	titleWidth := lipgloss.Width(titleBlock)
	offset := titleBlockOffset(x.r.cardWidth, titleWidth, x.r.model.Config.TitleAlign)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < titleWidth {
			// Normalize every FIGlet row to the block width before applying the
			// outer alignment. Otherwise left/right alignment moves each row by
			// its own width and destroys the wordmark's geometry.
			line += strings.Repeat(" ", titleWidth-lineWidth)
		}
		title := strings.Repeat(" ", offset) + line
		title = ui.Inline(x.r.st.headerAccent).Bold(true).Render(title)
		out = append(out, x.fillHeaderLine(title, x.r.cardWidth))
	}
	return out
}

func titleBlockOffset(cardWidth, titleWidth int, align string) int {
	available := max(cardWidth-titleWidth, 0)
	switch strings.ToLower(align) {
	case "right":
		return available
	case "center":
		return available / 2
	default:
		return 0
	}
}

// renderRowHeader renders the title and its trailing space. Status items belong
// to FooterView so their placement stays consistent across title variants.
func (x HeaderView) renderRowHeader(style string) []string {
	title := sections{r: x.r}.headerTitle(style)
	return []string{x.fillHeaderLine(title, x.r.cardWidth), x.BlankLine()}
}

// inlineTitleRule is a full-width accent glow with the title centered in a break
// (playground headingRule(text, -1), using ━ like the header underline).
func (x HeaderView) inlineTitleRule(title string) string {
	label := " " + title + " "
	labelWidth := lipgloss.Width(label)
	left, ruleWidth := x.gradientRuleGeometry()
	start := max((ruleWidth-labelWidth)/2, 0)
	peak := x.r.st.headerUnderlinePk
	base := x.r.st.blockBg
	grad := lipgloss.Blend2D(ruleWidth, 1, 0, base, peak, base)

	var b strings.Builder
	labelRunes := []rune(label)
	for col := range ruleWidth {
		if col >= start && col < start+labelWidth {
			ch := string(labelRunes[col-start])
			if ch == " " {
				b.WriteString(x.r.st.blockFill.Render(" "))
			} else {
				b.WriteString(ui.Inline(x.r.st.onBlock(x.r.st.h.Color(string(x.r.st.pal.Fg))).Bold(true)).Render(ch))
			}
			continue
		}
		b.WriteString(x.r.st.onBlock(grad[col]).Inline(true).Render("━"))
	}
	return x.padGradientRule(b.String(), left)
}

// headerUnderline is the accent glow rule under the wordmark. Peak is a
// slightly darkened accent so the center reads softer than pure accent.
func (x HeaderView) headerUnderline() string {
	left, ruleWidth := x.gradientRuleGeometry()
	grad := lipgloss.Blend2D(ruleWidth, 1, 0, x.r.st.blockBg, x.r.st.headerUnderlinePk, x.r.st.blockBg)
	var b strings.Builder
	for col := range ruleWidth {
		b.WriteString(x.r.st.onBlock(grad[col]).Render("━"))
	}
	return x.padGradientRule(b.String(), left)
}

func (x HeaderView) gradientRuleGeometry() (left, width int) {
	cardWidth := max(x.r.cardWidth, 0)
	left = min(max(x.r.model.Padding.Left, 0), cardWidth)
	right := min(max(x.r.model.Padding.Right, 0), cardWidth-left)
	return left, cardWidth - left - right
}

func (x HeaderView) padGradientRule(rule string, left int) string {
	return lipgloss.PlaceHorizontal(
		x.r.cardWidth,
		lipgloss.Left,
		strings.Repeat(" ", left)+rule,
		lipgloss.WithWhitespaceStyle(x.r.st.blockFill),
	)
}

func (x HeaderView) BlankLine() string {
	return x.r.st.headerFill.Width(x.r.cardWidth).Render("")
}

// fillHeaderLine pads the title row to width on the header surface.
func (x HeaderView) fillHeaderLine(content string, width int) string {
	return lipgloss.PlaceHorizontal(
		width,
		lipgloss.Left,
		content,
		lipgloss.WithWhitespaceStyle(x.r.st.headerFill),
	)
}
