package menu

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/ui"
)

// ListView is the scrolling command list body — the central pane of the menu
// panel. It owns the vertical scroll offset and caches the visible rows
// computed at Sync time; View is a pure return of that cache.
//
// Like the other menu components, ListView has a focused ownership boundary.
// It is stateful: it owns the scroll offset and cached row window. The root
// model owns the match indices, selection index, expanded flag, and filter
// query, and passes them to Sync; ListView does not reach back into the root.
type ListView struct {
	st     styles
	inner  int      // framed body width (set via WithSize)
	offset int      // scroll offset
	rows   []string // cached visible window, populated by Sync, returned by View
}

// newListView builds the list body element with the given styles and initial
// inner width.
func newListView(st styles, inner int) *ListView {
	return &ListView{st: st, inner: inner}
}

// WithSize sets the panel's inner width and returns the receiver. Called on
// WindowSizeMsg so the list stays sized without reaching back into the root.
func (l *ListView) WithSize(inner int) *ListView {
	l.inner = inner
	return l
}

// Sync recomputes the list rows and scroll offset from the root-owned state
// (match indices, selection, expanded flag, filter query) and the panel
// geometry (height, frame). The result is cached; View returns it without
// recomputing. This fixes the bubbletea anti-pattern where the old renderRows
// mutated the scroll offset inside View — Sync is the update-time home for
// that computation.
//
// flat is the full flattened task list; matches are indices into flat; sel is
// the index into matches; filter is the prompt's current value (used for the
// "no commands match %q" message); frame provides Paint/Blank for the panel
// rails.
func (l *ListView) Sync(flat []Task, matches []int, sel int, expanded bool, height int, filter string, frame Frame) *ListView {
	l.rows = l.renderRows(flat, matches, sel, expanded, height, filter, frame)
	return l
}

// View returns the cached visible window as a single newline-joined string.
// Pure: it does not recompute or mutate state.
func (l ListView) View() string {
	return strings.Join(l.rows, "\n")
}

// Height returns the number of cached visible rows. The root model uses this
// to position the status layer below the list.
func (l ListView) Height() int {
	return len(l.rows)
}

// scrollTo adjusts the offset so that line is visible within height, given
// the total number of lines. When total fits within height, the offset
// resets to zero.
func (l *ListView) scrollTo(line, total, height int) {
	if total <= height {
		l.offset = 0
		return
	}
	if line < l.offset {
		l.offset = line
	}
	if line >= l.offset+height {
		l.offset = line - height + 1
	}
	l.offset = max(0, min(l.offset, total-height))
}

// visible returns the visible portion of lines, padded with blank to height.
func (l *ListView) visible(lines []string, height int, blank string) []string {
	end := min(l.offset+height, len(lines))
	vis := append([]string{}, lines[l.offset:end]...)
	for len(vis) < height {
		vis = append(vis, blank)
	}
	return vis
}

// renderRows builds the scrolling grouped result list, padded to the list
// height. Geometry comes from l.inner, styles from l.st, and root-owned state
// from the parameters.
func (l *ListView) renderRows(flat []Task, matches []int, sel int, expanded bool, height int, filter string, frame Frame) []string {
	inner := l.inner
	h := height

	if len(matches) == 0 {
		lines := make([]string, h)
		for i := range lines {
			lines[i] = frame.Blank()
		}
		msg := l.st.sMuted.Render("no commands match ") +
			l.st.sFg.Render(fmt.Sprintf("%q", filter)) +
			l.st.sMuted.Render(" — press ") + l.st.sAccent2.Render("esc") + l.st.sMuted.Render(" to reset")
		lines[h/2] = frame.Paint(lipgloss.PlaceHorizontal(inner, lipgloss.Center, msg, lipgloss.WithWhitespaceStyle(l.st.sp)), l.st.sp)
		return lines
	}

	nameW := 4
	for _, t := range flat {
		nameW = max(nameW, lipgloss.Width(t.displayName()))
	}
	nameW += 2

	var lines []string
	selLine := 0
	lastGroup := "\x00"
	for pos, fi := range matches {
		t := flat[fi]
		if t.group != lastGroup {
			lastGroup = t.group
			if t.group != "" {
				// paint adds the frame's left rail; subtract that cell so group
				// labels align with the unframed ~/project context above.
				label := l.st.sp.PaddingLeft(max(padX-1, 0)).Render("") +
					l.st.sMuted.Render(ui.LetterSpace(t.group))
				lines = append(lines, frame.Paint(label, l.st.sp))
			}
		}
		active := pos == sel
		if active {
			selLine = len(lines)
		}
		lines = append(lines, l.renderRow(t, active, nameW, frame))
		if active && expanded {
			lines = append(lines, l.renderDetails(t)...)
		}
	}
	lines = append(lines, frame.Blank())

	// Scroll to keep the selected row visible. Expanded menus grow past the
	// configured height so the full disclosure stays on screen.
	targetH := h
	if expanded {
		targetH = max(targetH, len(lines))
	}

	l.scrollTo(selLine, len(lines), targetH)
	return l.visible(lines, targetH, frame.Blank())
}

// renderRow renders one command row; frame provides Paint for the panel rails.
func (l *ListView) renderRow(t Task, active bool, nameW int, frame Frame) string {
	st := l.st
	inner := l.inner

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
		name := st.selText.Bold(true).Width(nameW).Render(t.displayName())
		used := (padX - 1) + 2 + nameW + 1 + lipgloss.Width(chip) + 1 + padX
		desc := st.selText.Render(ansi.Truncate(t.Description, max(inner-used, 4), "…"))
		line := st.selSp.PaddingLeft(padX-1).Render("") + caretCol +
			name + st.selSp.Render(" ") + desc
		tail := chip + st.selSp.PaddingRight(padX).Render("")
		line = ui.PlaceRight(inner, line, tail, st.selSp)
		return frame.Paint(line, st.selSp)
	}

	caretCol := st.sp.Width(2).Render("")
	chip := ""
	if t.Key != "" {
		chip = st.keyChip.Render(keyLabel)
	}
	used := (padX - 1) + 2 + nameW + 1 + lipgloss.Width(chip) + 1 + padX
	desc := st.sMuted.Render(ansi.Truncate(t.Description, max(inner-used, 4), "…"))
	line := st.sp.PaddingLeft(padX-1).Render("") + caretCol +
		st.sFg.Bold(true).Width(nameW).Render(t.displayName()) + st.sp.Render(" ") + desc
	tail := chip + st.sp.PaddingRight(padX).Render("")
	line = ui.PlaceRight(inner, line, tail, st.sp)
	return frame.Paint(line, st.sp)
}

// renderDetails draws the expanded panel on the darker bg inset, framed with
// the same side rails as the picker so the disclosure stays aligned. Ported
// verbatim from the former model.renderDetails in view_list.go; note that this
// uses styles.frame (a lipgloss.Style) for the inset rails, not the Frame
// struct — it needs no frame param.
func (l *ListView) renderDetails(t Task) []string {
	st := l.st
	inner := l.inner
	insetSp := lipgloss.NewStyle().Background(st.bgColor)
	// Align disclosure content with the caret column of the item rows.
	detailIndent := padX - 1
	indent := insetSp.PaddingLeft(detailIndent).Render("")

	paintInset := func(content string) string {
		panel := insetSp.Width(inner).MaxWidth(inner).Render(content)
		return st.frame.Render("│") + panel + st.frame.Render("│")
	}

	var out []string
	out = append(out, paintInset(""))
	if t.Details != "" {
		wrapW := inner - detailIndent - padX
		for _, line := range strings.Split(ansi.Wordwrap(t.Details, wrapW, ""), "\n") {
			out = append(out, paintInset(indent+st.inset(st.pal.Muted).Render(line)))
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
