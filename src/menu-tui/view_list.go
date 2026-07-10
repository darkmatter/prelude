package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// --- list view ---------------------------------------------------------------

func (m model) listHeight() int {
	// frame(2) + title(1) + div(1) + prompt(1) + div(1) + div(1) + footer(1)
	return max(min(m.cfg.Height, m.height-8), 4)
}

func (m model) viewList(inner int) string {
	title := fmt.Sprintf("%s — command menu — %d of %d", m.cfg.Project, len(m.matches), len(m.flat))
	rows := m.renderRows(inner)

	parts := []string{
		m.frameTop(inner),
		m.titleBar(title, inner),
		m.frameDiv(inner),
		m.promptLine(inner, ""),
		m.frameDiv(inner),
	}
	parts = append(parts, rows...)
	parts = append(parts,
		m.frameDiv(inner),
		m.statusBar([][2]string{
			{"↑ ↓", "navigate"}, {"⇥", "details"}, {"↵", "run"}, {"esc", "clear"},
		}, m.st.bar(m.st.pal.Accent).Render("● ready"), inner),
		m.frameBottom(inner),
	)
	return strings.Join(parts, "\n")
}

// renderRows builds the scrolling grouped result list, padded to the list
// height.
func (m model) renderRows(inner int) []string {
	h := m.listHeight()

	if len(m.matches) == 0 {
		lines := make([]string, h)
		for i := range lines {
			lines[i] = m.blank(inner)
		}
		msg := m.st.sMuted.Render("no commands match ") +
			m.st.sFg.Render(fmt.Sprintf("%q", m.input.Value())) +
			m.st.sMuted.Render(" — press ") + m.st.sAccent2.Render("esc") + m.st.sMuted.Render(" to reset")
		pad := max((inner-lipgloss.Width(msg))/2, 0)
		lines[h/2] = m.paint(m.st.sp.Render(strings.Repeat(" ", pad))+msg, m.st.sp, inner)
		return lines
	}

	nameW := 4
	for _, t := range m.flat {
		nameW = max(nameW, lipgloss.Width(t.Name))
	}
	nameW += 2

	var lines []string
	selLine := 0
	lastGroup := "\x00"
	lines = append(lines, m.blank(inner)) // py padding
	for pos, fi := range m.matches {
		t := m.flat[fi]
		if t.group != lastGroup {
			lastGroup = t.group
			if len(lines) > 1 {
				lines = append(lines, m.blank(inner))
			}
			if t.group != "" {
				label := m.st.sp.Render(strings.Repeat(" ", padX)) + m.st.sMuted.Render(letterSpace(t.group))
				lines = append(lines, m.paint(label, m.st.sp, inner))
			}
		}
		active := pos == m.sel
		if active {
			selLine = len(lines)
		}
		lines = append(lines, m.renderRow(t, active, nameW, inner))
		if active && m.expanded {
			lines = append(lines, m.renderDetails(t, inner)...)
		}
	}
	lines = append(lines, m.blank(inner))

	// Scroll to keep the selected row visible.
	offset := m.offset
	if selLine < offset {
		offset = selLine
	}
	if selLine >= offset+h {
		offset = selLine - h + 1
	}
	offset = max(0, min(offset, max(0, len(lines)-h)))

	visible := lines[offset:min(offset+h, len(lines))]
	for len(visible) < h {
		visible = append(visible, m.blank(inner))
	}
	return visible
}

func (m model) renderRow(t Task, active bool, nameW, inner int) string {
	st := m.st

	keyLabel := "   "
	if t.Key != "" {
		keyLabel = " " + t.Key + " "
	}
	marker := ""
	if len(t.Args) > 0 {
		marker = "args…"
	}
	if active && t.Details != "" {
		if m.expanded {
			marker = "⇥ less"
		} else {
			marker = "⇥ more"
		}
	}

	if active {
		gutter := st.selText.Render("❯")
		chipStr := st.selChip.Render(keyLabel)
		if t.Key == "" {
			chipStr = st.selSp.Render(keyLabel)
		}
		name := st.selText.Bold(true).Render(padRight(t.Name, nameW))
		used := padX + 1 + 1 + 3 + 1 + nameW + 1 + lipgloss.Width(marker) + 1 + padX
		desc := st.selText.Render(ansi.Truncate(t.Description, max(inner-used, 4), "…"))
		line := st.selSp.Render(strings.Repeat(" ", padX)) + gutter + st.selSp.Render(" ") +
			chipStr + st.selSp.Render(" ") + name + st.selSp.Render(" ") + desc
		pad := inner - lipgloss.Width(line) - lipgloss.Width(marker) - padX
		line += st.selSp.Render(strings.Repeat(" ", max(pad, 1))) + st.selDim.Render(marker) +
			st.selSp.Render(strings.Repeat(" ", padX))
		return m.paint(line, st.selSp, inner)
	}

	chipStr := st.keyChip.Render(keyLabel)
	if t.Key == "" {
		chipStr = st.sp.Render(keyLabel)
	}
	used := padX + 1 + 1 + 3 + 1 + nameW + 1 + lipgloss.Width(marker) + 1 + padX
	desc := st.sMuted.Render(ansi.Truncate(t.Description, max(inner-used, 4), "…"))
	line := st.sp.Render(strings.Repeat(" ", padX)) + st.sDim.Render(" ") + st.sp.Render(" ") +
		chipStr + st.sp.Render(" ") + st.sFg.Bold(true).Render(padRight(t.Name, nameW)) +
		st.sp.Render(" ") + desc
	pad := inner - lipgloss.Width(line) - lipgloss.Width(marker) - padX
	line += st.sp.Render(strings.Repeat(" ", max(pad, 1))) + st.sDim.Render(marker) +
		st.sp.Render(strings.Repeat(" ", padX))
	return m.paint(line, st.sp, inner)
}

// renderDetails draws the expanded panel on the darker `bg` inset, like the
// mock's bg-background/60 details block.
func (m model) renderDetails(t Task, inner int) []string {
	st := m.st
	insetSp := lipgloss.NewStyle().Background(st.bgColor)
	indent := insetSp.Render(strings.Repeat(" ", padX+6))

	paintInset := func(content string) string {
		return m.paint(content, insetSp, inner)
	}

	var out []string
	out = append(out, paintInset(""))
	if t.Details != "" {
		wrapW := inner - (padX + 6) - padX
		for _, l := range strings.Split(ansi.Wordwrap(t.Details, wrapW, ""), "\n") {
			out = append(out, paintInset(indent+st.inset(st.pal.Muted).Render(l)))
		}
	} else {
		out = append(out, paintInset(indent+st.inset(st.pal.Dim).Italic(true).Render("no extended description")))
	}
	if t.Usage != "" {
		out = append(out, paintInset(""),
			paintInset(indent+st.inset(st.pal.Accent).Render("$ ")+st.inset(st.pal.Fg).Render(t.Usage)))
	}
	for _, ex := range t.Examples {
		out = append(out, paintInset(indent+st.inset(st.pal.Dim).Render("example ")+
			st.inset(st.pal.Accent).Render("❯ ")+st.inset(st.pal.Muted).Render(ex)))
	}
	out = append(out, paintInset(""))
	return out
}
