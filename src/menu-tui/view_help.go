package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// --- help view ---------------------------------------------------------------
//
// Full-bleed, pager-like layout (no rounded frame):
//
//	──────────────┬────────────────────────────
//	              │  NAME
//	  CONTENTS    │    menu — …
//	──────────────┤  SYNOPSIS
//	 ❯ name       │    …
//	  2 synopsis  │
//	 NORMAL :name · j/k or click to navigate      (inverted bar)

// helpSidebarItemsTop is the terminal row of the first sidebar item:
// top border, blank, CONTENTS, divider.
const helpSidebarItemsTop = 4

type helpLayout struct {
	sideW int // sidebar columns (excluding the vertical divider)
	bodyW int // content pane columns
	textW int // wrap width for doc lines inside the pane
	viewH int // visible doc lines
}

func (m model) helpLayout() helpLayout {
	side := lipgloss.Width("CONTENTS")
	for _, t := range helpSectionTitles {
		side = max(side, lipgloss.Width(t)+2) // "<n> " marker column
	}
	side += 2 + 2 // left/right padding
	body := max(m.width-side-1, 20)
	return helpLayout{
		sideW: side,
		bodyW: body,
		textW: min(body-2, 96),
		viewH: max(m.height-2, 1),
	}
}

func (m model) viewHelp() string {
	st := m.st
	l := m.helpLayout()
	lines, _ := m.helpDoc(l.textW)
	maxScroll := max(0, len(lines)-l.viewH)
	scroll := min(m.helpScroll, maxScroll)

	insetSp := lipgloss.NewStyle().Background(st.bgColor)
	padSide := func(s string) string { return st.sp.MaxWidth(l.sideW).Width(l.sideW).Render(s) }
	padBody := func(s string) string { return insetSp.MaxWidth(l.bodyW).Width(l.bodyW).Render(s) }
	div := st.inset(st.pal.Border)

	rows := make([]string, 0, m.height)
	rows = append(rows, st.frame.Render(strings.Repeat("─", l.sideW))+
		div.Render("┬"+strings.Repeat("─", l.bodyW)))

	for r := 0; r < l.viewH; r++ {
		junction := "│"
		var side string
		switch r {
		case 0:
			side = padSide("")
		case 1:
			side = padSide(st.sp.Render("  ") + st.sMuted.Render("CONTENTS"))
		case 2:
			side = st.frame.Render(strings.Repeat("─", l.sideW))
			junction = "┤"
		default:
			side = m.helpSidebarItem(r-3, l.sideW, padSide)
		}

		body := ""
		if i := scroll + r; i < len(lines) {
			body = lines[i]
		}
		rows = append(rows, side+div.Render(junction)+padBody(body))
	}

	rows = append(rows, m.helpStatusBar(scroll, maxScroll))
	return strings.Join(rows, "\n")
}

// helpSidebarItem renders sidebar entry i: the active section shows a ❯
// marker on a highlighted bar, inactive ones their number.
func (m model) helpSidebarItem(i, sideW int, padSide func(string) string) string {
	st := m.st
	if i < 0 || i >= len(helpSectionTitles) {
		return padSide("")
	}
	title := helpSectionTitles[i]
	if i == m.helpActive {
		line := st.barSp.Render("  ") + st.bar(st.pal.Accent).Render("❯") +
			st.barSp.Render(" ") + st.bar(st.pal.Fg).Render(title)
		return st.barSp.MaxWidth(sideW).Width(sideW).Render(line)
	}
	line := st.sp.Render("  ") + st.sDim.Render(fmt.Sprintf("%d", i+1)) +
		st.sp.Render(" ") + st.sMuted.Render(title)
	return padSide(line)
}

// helpStatusBar renders the inverted (fg-on-bg swapped) bottom bar.
func (m model) helpStatusBar(scroll, maxScroll int) string {
	st := m.st
	fgC := lipgloss.Color(string(st.pal.Fg))
	sp := lipgloss.NewStyle().Background(fgC)
	txt := lipgloss.NewStyle().Background(fgC).Foreground(st.bgColor)

	pos := fmt.Sprintf("%d%%", scroll*100/max(maxScroll, 1))
	switch {
	case maxScroll == 0:
		pos = "all"
	case scroll == 0:
		pos = "top"
	case scroll >= maxScroll:
		pos = "bot"
	}

	left := sp.Render("  ") + txt.Bold(true).Render("NORMAL") +
		txt.Render(" :"+helpSectionTitles[m.helpActive]) +
		txt.Faint(true).Render("  ·  j/k or click to navigate")
	right := txt.Faint(true).Render(pos) + sp.Render("  ")
	pad := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	bar := left + sp.Render(strings.Repeat(" ", max(pad, 0))) + right
	return ansi.Truncate(bar, m.width, "")
}
