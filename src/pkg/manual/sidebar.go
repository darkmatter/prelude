package manual

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// sidebarView renders the CONTENTS sidebar as a self-contained column.
// The viewer constructs one per render with its current state.
type sidebarView struct {
	sections []Section
	active   int
	styles   styles
	width    int
	height   int // viewH — total rows including header and border
}

func (s sidebarView) View() string {
	border := lipgloss.NormalBorder()
	pad := func(content string) string {
		return s.styles.surfaceSpace.Inline(true).Width(s.width).MaxWidth(s.width).Render(content)
	}

	lines := make([]string, 0, s.height)
	// Row 0: breathing room above the CONTENTS label.
	lines = append(lines, pad(""))
	// Row 1: CONTENTS label.
	lines = append(lines, pad(s.styles.surfaceMuted.PaddingLeft(2).Render("CONTENTS")))
	// Row 2: sidebar internal border separating label from items.
	lines = append(lines, s.styles.frame.Render(strings.Repeat(border.Top, s.width)))

	// Row 3+: section entries.
	for i := range s.sections {
		lines = append(lines, s.item(i, pad))
	}

	// Pad to the body height.
	for len(lines) < s.height {
		lines = append(lines, pad(""))
	}
	return strings.Join(lines, "\n")
}

func (s sidebarView) item(index int, pad func(string) string) string {
	if index < 0 || index >= len(s.sections) {
		return pad("")
	}
	title := s.sections[index].Title
	if index == s.active {
		line := s.styles.onActive(s.styles.pal.Accent).PaddingLeft(2).Render("❯") +
			s.styles.onActive(s.styles.pal.Fg).PaddingLeft(1).Render(title)
		return s.styles.activeSpace.Inline(true).Width(s.width).MaxWidth(s.width).Render(line)
	}
	line := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(s.styles.pal.Dim))).
		Background(s.styles.surface).
		PaddingLeft(2).
		Render(fmt.Sprintf("%d", index+1)) +
		s.styles.surfaceMuted.PaddingLeft(1).Render(title)
	return pad(line)
}
