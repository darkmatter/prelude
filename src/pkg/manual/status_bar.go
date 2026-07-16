package manual

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

// statusBarView renders the footer status bar as a self-contained row.
type statusBarView struct {
	width     int
	scroll    int
	maxScroll int
	section   string
	jumpCount int
	styles    styles
}

func (sb statusBarView) View() string {
	foreground := lipgloss.Color(string(sb.styles.pal.Fg))
	space := lipgloss.NewStyle().Background(foreground)
	text := lipgloss.NewStyle().Background(foreground).Foreground(sb.styles.bg)

	position := fmt.Sprintf("%d%%", sb.scroll*100/max(sb.maxScroll, 1))
	switch {
	case sb.maxScroll == 0:
		position = "all"
	case sb.scroll == 0:
		position = "top"
	case sb.scroll >= sb.maxScroll:
		position = "bot"
	}

	left := text.Bold(true).PaddingLeft(2).Render("NORMAL") + text.Render(" :" + sb.section)
	if sb.jumpCount > 0 {
		left += text.Faint(true).Render(fmt.Sprintf("  ·  1-%d jump · j/k scroll · q quit", sb.jumpCount))
	}
	right := text.Faint(true).PaddingRight(2).Render(position)
	remaining := max(sb.width-lipgloss.Width(left), 0)
	right = lipgloss.PlaceHorizontal(
		remaining,
		lipgloss.Right,
		right,
		lipgloss.WithWhitespaceStyle(space),
	)
	return lipgloss.NewStyle().Inline(true).MaxWidth(sb.width).Render(left + right)
}
