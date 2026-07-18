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
	kind      Kind
	mode      string
	styles    styles
}

func (sb statusBarView) View() string {
	space, text := sb.styles.statusChrome(sb.kind)

	position := fmt.Sprintf("%d%%", sb.scroll*100/max(sb.maxScroll, 1))
	switch {
	case sb.maxScroll == 0:
		position = "all"
	case sb.scroll == 0:
		position = "top"
	case sb.scroll >= sb.maxScroll:
		position = "bot"
	}

	// Mode chip (DOCS / HELP) is the primary differentiator in the footer.
	left := text.Bold(true).PaddingLeft(2).Render(sb.mode) + text.Render(" :"+sb.section)
	if sb.jumpCount > 0 {
		unit := "sections"
		if sb.kind == KindDocs {
			unit = "pages"
		}
		left += text.Faint(true).Render(fmt.Sprintf("  ·  1-%d %s · j/k scroll · q quit", sb.jumpCount, unit))
	}
	right := text.Faint(true).PaddingRight(2).Render(position)
	remaining := max(sb.width-lipgloss.Width(left), 0)
	right = lipgloss.PlaceHorizontal(
		remaining,
		lipgloss.Right,
		right,
		lipgloss.WithWhitespaceStyle(space),
	)
	return lipgloss.PlaceHorizontal(
		sb.width,
		lipgloss.Left,
		left+right,
		lipgloss.WithWhitespaceStyle(space),
	)
}
