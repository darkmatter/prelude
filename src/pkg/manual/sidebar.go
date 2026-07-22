package manual

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// sidebarView renders the navigation column as a self-contained panel.
// The viewer constructs one per render with its current state.
type sidebarView struct {
	label     string
	rows      []treeRow // visible (windowed) tree rows
	cursor    int       // cursor index within rows (window-local)
	focusSide bool      // sidebar focused
	styles    styles
	width     int
	height    int // viewH — total rows including header and border
}

// itemsTop is the number of header rows before the first nav entry.
const itemsTop = 3 // blank + label + border

// Fixed leading columns so selection never shifts text.
// Left gutter is blank indent (no selection caret); digits stay optional.
const (
	indentWidth = 2 // leading blank indent
	digitWidth  = 2 // "1 " / "  "
)

func (s sidebarView) View() string {
	pad := func(content string) string {
		return lipgloss.PlaceHorizontal(
			s.width,
			lipgloss.Left,
			content,
			lipgloss.WithWhitespaceStyle(s.styles.surfaceSpace),
		)
	}

	// Docs label uses accent2.
	labelFG := s.styles.pal.Accent2
	label := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(labelFG))).
		Background(s.styles.surface).
		Bold(true).
		PaddingLeft(2).
		Render(s.label)

	lines := make([]string, 0, s.height)
	lines = append(lines, pad(""))
	lines = append(lines, pad(label))
	// Top rule under the label; focused pane paints it accent in render().
	frame := s.styles.frame
	lines = append(lines, frame.Render(strings.Repeat(lipgloss.NormalBorder().Top, s.width)))

	for i := range s.rows {
		lines = append(lines, s.treeItem(i, pad))
	}

	for len(lines) < s.height {
		lines = append(lines, pad(""))
	}
	return strings.Join(lines, "\n")
}

func (s sidebarView) treeItem(index int, pad func(string) string) string {
	if index < 0 || index >= len(s.rows) {
		return pad("")
	}
	row := s.rows[index]
	if row.separator {
		return pad("")
	}

	active := index == s.cursor

	digitGlyph := ""
	if row.digit > 0 {
		digitGlyph = fmt.Sprintf("%d", row.digit)
	}

	restBudget := s.width - indentWidth - digitWidth
	if restBudget < 4 {
		restBudget = 4
	}

	var body string
	if row.depth == 0 {
		mark := ""
		if row.node.IsGroup() {
			if row.expanded {
				mark = "v " // ASCII expand marker (width-stable)
			} else {
				mark = "> "
			}
		}
		title := truncateTitle(row.node.Title, restBudget-lipgloss.Width(mark))
		body = mark + title
	} else {
		title := truncateTitle(row.node.Title, restBudget-lipgloss.Width(row.branch))
		body = row.branch + title
	}

	if active && s.focusSide {
		// Nav focused: full-row highlight on secondary (prior selection color).
		// Accent stays on the top border only.
		indentCell := s.styles.onActive(s.styles.pal.Fg).Width(indentWidth).Render("")
		digitCell := s.styles.onActive(s.styles.pal.Dim).Width(digitWidth).Render(digitGlyph)
		bodyCell := s.styles.onActive(s.styles.pal.Fg).Bold(true).Render(body)
		line := indentCell + digitCell + bodyCell
		return lipgloss.PlaceHorizontal(
			s.width,
			lipgloss.Left,
			line,
			lipgloss.WithWhitespaceStyle(s.styles.activeSpace),
		)
	}
	if active {
		// Nav unfocused: lighten + bold on surface.
		indentCell := lipgloss.NewStyle().
			Background(s.styles.surface).
			Width(indentWidth).
			Render("")
		digitCell := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(s.styles.pal.Fg))).
			Background(s.styles.surface).
			Bold(true).
			Width(digitWidth).
			Render(digitGlyph)
		bodyCell := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(s.styles.pal.Fg))).
			Background(s.styles.surface).
			Bold(true).
			Render(body)
		return pad(indentCell + digitCell + bodyCell)
	}

	indentCell := lipgloss.NewStyle().
		Background(s.styles.surface).
		Width(indentWidth).
		Render("")
	digitCell := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(s.styles.pal.Dim))).
		Background(s.styles.surface).
		Width(digitWidth).
		Render(digitGlyph)
	titleStyle := s.styles.surfaceMuted
	if row.depth > 0 {
		titleStyle = s.styles.surfaceDim
	}
	return pad(indentCell + digitCell + titleStyle.Render(body))
}

func truncateTitle(title string, maxW int) string {
	if maxW < 4 {
		maxW = 4
	}
	if lipgloss.Width(title) <= maxW {
		return title
	}
	var b strings.Builder
	w := 0
	for _, r := range title {
		rw := lipgloss.Width(string(r))
		if w+rw+1 > maxW {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
}

// plainTitleWidth helper for layout (strip not needed on raw titles).
func plainTitleWidth(s string) int {
	return lipgloss.Width(ansi.Strip(s))
}
