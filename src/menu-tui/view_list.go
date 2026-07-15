package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// --- list view ---------------------------------------------------------------

func (m model) listHeight() int {
	return max(min(m.cfg.Height, m.height-chromeRows), 4)
}

func (m model) viewList(inner int) string {
	title := fmt.Sprintf("%s — command menu — %d of %d", m.cfg.Project, len(m.matches), len(m.flat))
	rows := m.renderRows(inner)

	parts := []string{
		m.mutedTitleRow(title, inner),
		m.promptLine(inner, ""),
		m.frameTop(inner),
	}
	parts = append(parts, rows...)
	parts = append(parts,
		m.statusBar([][2]string{
			{"↑ ↓", "navigate"}, {"⇥", "details"}, {"↵", "run"}, {"esc", "clear"},
		}, "● ready", inner),
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
		lines[h/2] = m.paint(lipgloss.PlaceHorizontal(inner, lipgloss.Center, msg, lipgloss.WithWhitespaceStyle(m.st.sp)), m.st.sp, inner)
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
	for pos, fi := range m.matches {
		t := m.flat[fi]
		if t.group != lastGroup {
			lastGroup = t.group
			if t.group != "" {
				// paint adds the frame's left rail; subtract that cell so group
				// labels align with the unframed ~/project context above.
				label := m.st.sp.Render(strings.Repeat(" ", max(padX-1, 0))) +
					m.st.sMuted.Render(letterSpace(t.group))
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

	// Scroll to keep the selected row visible. Expanded menus grow past the
	// configured height so the full disclosure stays on screen.
	targetH := h
	if m.expanded {
		targetH = max(targetH, len(lines))
	}
	offset := m.offset
	if selLine < offset {
		offset = selLine
	}
	if selLine >= offset+targetH {
		offset = selLine - targetH + 1
	}
	offset = max(0, min(offset, max(0, len(lines)-targetH)))

	visible := lines[offset:min(offset+targetH, len(lines))]
	for len(visible) < targetH {
		visible = append(visible, m.blank(inner))
	}
	return visible
}

func (m model) renderRow(t Task, active bool, nameW, inner int) string {
	st := m.st

	keyLabel := ""
	if t.Key != "" {
		keyLabel = t.Key
	}

	if active {
		// Compact columns: caret then command name; the hotkey keycap sits in
		// the right lane.
		caretCol := st.selText.Bold(true).Width(2).Render("❯")
		chip := ""
		if t.Key != "" {
			// Glyph rails keep the outlined keycap while every cell stays on
			// the active row's accent background.
			chip = st.selChip.Render("│" + keyLabel + "│")
		}
		name := st.selText.Bold(true).Width(nameW).Render(t.Name)
		used := (padX - 1) + 2 + nameW + 1 + lipgloss.Width(chip) + 1 + padX
		desc := st.selText.Render(ansi.Truncate(t.Description, max(inner-used, 4), "…"))
		line := st.selSp.Render(strings.Repeat(" ", padX-1)) + caretCol +
			name + st.selSp.Render(" ") + desc
		pad := inner - lipgloss.Width(line) - lipgloss.Width(chip) - padX
		line += st.selSp.Render(strings.Repeat(" ", max(pad, 1))) + chip +
			st.selSp.Render(strings.Repeat(" ", padX))
		return m.paint(line, st.selSp, inner)
	}

	caretCol := st.sp.Width(2).Render("")
	chip := ""
	if t.Key != "" {
		chip = st.keyChip.Render(keyLabel)
	}
	used := (padX - 1) + 2 + nameW + 1 + lipgloss.Width(chip) + 1 + padX
	desc := st.sMuted.Render(ansi.Truncate(t.Description, max(inner-used, 4), "…"))
	line := st.sp.Render(strings.Repeat(" ", padX-1)) + caretCol +
		st.sFg.Bold(true).Width(nameW).Render(t.Name) + st.sp.Render(" ") + desc
	pad := inner - lipgloss.Width(line) - lipgloss.Width(chip) - padX
	line += st.sp.Render(strings.Repeat(" ", max(pad, 1))) + chip +
		st.sp.Render(strings.Repeat(" ", padX))
	return m.paint(line, st.sp, inner)
}

// renderDetails draws the expanded panel on the darker bg inset, framed with
// the same side rails as the picker so the disclosure stays aligned.
func (m model) renderDetails(t Task, inner int) []string {
	st := m.st
	insetSp := lipgloss.NewStyle().Background(st.bgColor)
	// Align disclosure content with the caret column of the item rows.
	detailIndent := padX - 1
	indent := insetSp.Render(strings.Repeat(" ", detailIndent))

	paintInset := func(content string) string {
		panel := insetSp.Width(inner).MaxWidth(inner).Render(content)
		return st.frame.Render("│") + panel + st.frame.Render("│")
	}

	var out []string
	out = append(out, paintInset(""))
	if t.Details != "" {
		wrapW := inner - detailIndent - padX
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
