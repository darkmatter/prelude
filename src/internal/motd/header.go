package motd

import (
	"strings"

	"charm.land/lipgloss/v2"

	"prelude/pkg/ui"
)

// HeaderView renders the MOTD header chrome. React-style, one component per
// file: it owns title, divider, and header-status presentation.
type HeaderView struct{ r renderer }

// Render paints only the region above the divider. The divider itself belongs
// to the window/default surface, while activation text and everything below it
// belong to the container surface.
func (x HeaderView) Render() string {
	if x.r.cfg.Title != "" {
		return strings.Join(x.renderGeneratedTitle(), "\n")
	}
	style := strings.ToLower(x.r.cfg.Header.TitleStyle)
	if style == titleStyleInline {
		return strings.Join(x.renderInlineHeader(), "\n")
	}
	return strings.Join(x.renderRowHeader(style), "\n")
}

func (x HeaderView) Divider() string {
	if x.r.cfg.Title == "" && strings.ToLower(x.r.cfg.Header.TitleStyle) == titleStyleInline {
		return x.inlineTitleRule(x.r.cfg.Project)
	}
	return x.headerUnderline()
}

func (x HeaderView) renderGeneratedTitle() []string {
	lines := strings.Split(strings.TrimRight(x.r.cfg.Title, "\n"), "\n")
	out := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		title := ui.Inline(x.r.st.headerAccent).Bold(true).Render(line)
		out = append(out, x.titleHeaderLine(title))
	}
	return out
}

func (x HeaderView) titleHeaderLine(content string) string {
	position := lipgloss.Left
	switch strings.ToLower(x.r.cfg.TitleAlign) {
	case "center":
		position = lipgloss.Center
	case "right":
		position = lipgloss.Right
	}
	return lipgloss.PlaceHorizontal(
		x.r.cardWidth,
		position,
		content,
		lipgloss.WithWhitespaceStyle(x.r.st.headerFill),
	)
}

// renderRowHeader is the header-owned title/status row plus its trailing space.
// The divider and lower spacing are composed separately.
func (x HeaderView) renderRowHeader(style string) []string {
	title := (HeaderTitle{r: x.r}).Render(style)

	contentWidth := x.r.cardWidth - headerRightPad
	statusItems := StatusItems{r: x.r}
	if x.r.cfg.Header.StatusHintLayout == "inline" {
		if statusRow := statusItems.InlineHint(x.r.cfg.Header.Status, contentWidth, true); statusRow != "" {
			return []string{
				x.fillHeaderLine(title, x.r.cardWidth),
				x.BlankLine(),
				statusRow + x.headerGap(headerRightPad),
				x.BlankLine(),
			}
		}
	}
	info := statusItems.Render(x.r.cfg.Header.Status, false)
	if info != "" && lipgloss.Width(title)+2+lipgloss.Width(info) > contentWidth {
		info = statusItems.Render(x.r.cfg.Header.Status, true)
	}

	var row string
	if info == "" {
		row = x.fillHeaderLine(title, x.r.cardWidth)
	} else {
		gap := max(contentWidth-lipgloss.Width(title)-lipgloss.Width(info), 1)
		row = x.fillHeaderLine(title+x.headerGap(gap)+info+x.headerGap(headerRightPad), x.r.cardWidth)
	}
	rows := []string{row}
	if hint := (StatusItems{r: x.r}).Hint(true); hint != "" {
		rows = append(rows, x.BlankLine(), lipgloss.PlaceHorizontal(
			x.r.cardWidth-headerRightPad,
			lipgloss.Right,
			hint,
			lipgloss.WithWhitespaceStyle(x.r.st.headerFill),
		)+x.headerGap(headerRightPad))
	}
	return append(rows, x.BlankLine())
}

// renderInlineHeader centers the project name inside the accent gradient rule
// (playground headingRule with start=-1). Status chips sit on a quiet row above.
func (x HeaderView) renderInlineHeader() []string {
	var parts []string
	statusItems := StatusItems{r: x.r}
	if info := statusItems.Render(x.r.cfg.Header.Status, false); info != "" {
		if x.r.cfg.Header.StatusHintLayout == "inline" {
			if row := statusItems.InlineHint(x.r.cfg.Header.Status, x.r.cardWidth, true); row != "" {
				return []string{row, x.BlankLine()}
			}
		}
		// Right-align status above the rule so it doesn't fight the centered title.
		row := lipgloss.PlaceHorizontal(
			x.r.cardWidth,
			lipgloss.Right,
			info,
			lipgloss.WithWhitespaceStyle(x.r.st.headerFill),
		)
		parts = append(parts, row)
		if hint := (StatusItems{r: x.r}).Hint(true); hint != "" {
			parts = append(parts, x.BlankLine(), lipgloss.PlaceHorizontal(
				x.r.cardWidth,
				lipgloss.Right,
				hint,
				lipgloss.WithWhitespaceStyle(x.r.st.headerFill),
			))
		}
		parts = append(parts, x.BlankLine())
	}
	return parts
}

// inlineTitleRule is a full-width accent glow with the title centered in a break
// (playground headingRule(text, -1), using ━ like the header underline).
func (x HeaderView) inlineTitleRule(title string) string {
	label := " " + title + " "
	labelWidth := lipgloss.Width(label)
	start := max((x.r.cardWidth-labelWidth)/2, 0)
	peak := x.r.st.headerUnderlinePk
	base := x.r.st.windowBg
	grad := lipgloss.Blend2D(x.r.cardWidth, 1, 0, base, peak, base)

	var b strings.Builder
	labelRunes := []rune(label)
	for col := 0; col < x.r.cardWidth; col++ {
		if col >= start && col < start+labelWidth {
			ch := string(labelRunes[col-start])
			if ch == " " {
				b.WriteString(x.r.st.windowFill.Render(" "))
			} else {
				b.WriteString(ui.Inline(x.r.st.onWindow(x.r.st.h.Color(string(x.r.st.pal.Fg))).Bold(true)).Render(ch))
			}
			continue
		}
		b.WriteString(x.r.st.onWindow(grad[col]).Inline(true).Render("━"))
	}
	return b.String()
}

// headerUnderline is the accent glow rule under the wordmark. Peak is a
// slightly darkened accent so the center reads softer than pure accent.
func (x HeaderView) headerUnderline() string {
	grad := lipgloss.Blend2D(x.r.cardWidth, 1, 0, x.r.st.windowBg, x.r.st.headerUnderlinePk, x.r.st.windowBg)
	var b strings.Builder
	for col := range x.r.cardWidth {
		b.WriteString(x.r.st.onWindow(grad[col]).Render("━"))
	}
	return b.String()
}

func (x HeaderView) BlankLine() string {
	return x.r.st.headerFill.Width(x.r.cardWidth).Render("")
}

func (x HeaderView) headerGap(n int) string {
	return lipgloss.PlaceHorizontal(
		n,
		lipgloss.Left,
		"",
		lipgloss.WithWhitespaceStyle(x.r.st.headerFill),
	)
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
