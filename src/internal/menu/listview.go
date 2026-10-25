package menu

import (
	"fmt"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/ui"
)

// ListView is the scrolling command list body — the central pane of the menu
// panel. It owns a bubbles viewport for scroll state and caches the visible
// window at Sync time; View is a pure return of that cache.
//
// Like the other menu components, ListView has a focused ownership boundary.
// It is stateful: it owns the viewport (offset + content). The root model owns
// the match indices, selection index, expanded flag, and filter query, and
// passes them to Sync; ListView does not reach back into the root.
type ListView struct {
	st       styles
	inner    int    // framed body width (set via WithSize)
	height   int    // configured body height from the last Sync
	header   string // pinned expanded detail header; empty in list mode
	viewport viewport.Model
}

// newListView builds the list body element with the given styles and initial
// inner width.
func newListView(st styles, inner int) *ListView {
	vp := viewport.New(
		viewport.WithWidth(max(inner+2, 1)), // rails add one cell each side
		viewport.WithHeight(4),
	)
	vp.MouseWheelEnabled = true
	vp.SoftWrap = false
	vp.FillHeight = true
	// Selection movement is owned by the root model; the viewport only scrolls
	// content. Disable its own key bindings so ↑/↓ never double-move.
	vp.KeyMap.Down.SetEnabled(false)
	vp.KeyMap.Up.SetEnabled(false)
	vp.KeyMap.PageDown.SetEnabled(false)
	vp.KeyMap.PageUp.SetEnabled(false)
	vp.KeyMap.HalfPageDown.SetEnabled(false)
	vp.KeyMap.HalfPageUp.SetEnabled(false)
	return &ListView{st: st, inner: inner, viewport: vp}
}

// WithSize sets the panel's inner width and returns the receiver. Called on
// WindowSizeMsg so the list stays sized without reaching back into the root.
func (l *ListView) WithSize(inner int) *ListView {
	l.inner = inner
	// Frame rails are painted into each content line, so the viewport width is
	// the framed row width (inner + 2).
	l.viewport.SetWidth(max(inner+2, 1))
	return l
}

// Sync recomputes the list rows and scroll offset from the root-owned state
// (match indices, selection, expanded flag, filter query) and the panel
// geometry (height, frame). The result is cached in the viewport; View returns
// it without recomputing. This keeps scroll mutation out of View.
//
// flat is the full flattened task list; matches are indices into flat; sel is
// the index into matches; filter is the prompt's current value (used for the
// "no commands match %q" message); frame provides Paint/Blank for the panel
// rails.
func (l *ListView) Sync(flat []Task, matches []int, sel int, expanded bool, height int, filter string, frame Frame) *ListView {
	if expanded && len(matches) > 0 {
		return l.syncExpanded(flat, matches, sel, height, frame)
	}

	l.header = ""
	lines, selLine, selEnd := l.renderRows(flat, matches, sel, height, filter, frame)

	// The list never grows the panel: padding rows use the panel style and
	// renderRows has already filled its own inset where needed.
	targetH := height
	for len(lines) < targetH {
		lines = append(lines, frame.Blank())
	}
	l.height = targetH
	l.viewport.SetHeight(max(targetH, 1))
	l.viewport.SetWidth(max(l.inner+2, 1))
	l.viewport.SetContentLines(lines)
	// Reveal the selected command when it fits, or its first screenful.
	l.viewport.EnsureVisible(min(selEnd, selLine+targetH-1), 0, 0)
	l.viewport.EnsureVisible(selLine, 0, 0)
	return l
}

// syncExpanded pins the detail header above a viewport that contains only the
// inset content. Up/Down and the mouse wheel therefore scroll details without
// moving the command title off screen. The header deliberately has no
// selection bar: the downward caret is the only cue that this row is open.
func (l *ListView) syncExpanded(flat []Task, matches []int, sel int, height int, frame Frame) *ListView {
	sel = min(max(sel, 0), len(matches)-1)
	t := flat[matches[sel]]
	detailH := max(height-1, 0)
	l.header = l.renderExpandedHeader(t, frame)
	l.height = height
	l.viewport.SetHeight(detailH)
	l.viewport.SetWidth(max(l.inner+2, 1))
	l.viewport.SetContentLines(l.renderDetails(t, detailH))
	l.viewport.SetYOffset(0)
	return l
}

// Update forwards Bubble Tea messages the viewport owns (mouse wheel).
func (l *ListView) Update(msg tea.Msg) (*ListView, tea.Cmd) {
	var cmd tea.Cmd
	l.viewport, cmd = l.viewport.Update(msg)
	return l, cmd
}

// ScrollUp moves the expanded detail content up without moving its header.
func (l *ListView) ScrollUp(n int) {
	l.viewport.ScrollUp(n)
}

// ScrollDown moves the expanded detail content down without moving its header.
func (l *ListView) ScrollDown(n int) {
	l.viewport.ScrollDown(n)
}

// View returns the pinned detail header followed by the viewport's visible
// window. Pure: it does not recompute or mutate state.
func (l ListView) View() string {
	if l.header == "" {
		return l.viewport.View()
	}
	if l.viewport.Height() == 0 {
		return l.header
	}
	return l.header + "\n" + l.viewport.View()
}

// Height returns the configured list height (not the content length). The root
// model uses this to position the status layer below the list.
func (l ListView) Height() int {
	if l.height > 0 {
		return l.height
	}
	return l.viewport.Height()
}

// renderRows builds the full grouped result list (not yet windowed). Geometry
// comes from l.inner, styles from l.st, and root-owned state from the
// parameters. Returns the lines and the selected command's first/last line indices.
func (l *ListView) renderRows(flat []Task, matches []int, sel int, height int, filter string, frame Frame) ([]string, int, int) {
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
		return lines, 0, 0
	}

	nameW := 4
	for _, t := range flat {
		nameW = max(nameW, lipgloss.Width(t.rowName()))
	}
	nameW += 2

	var lines []string
	selLine, selEnd := 0, 0
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
		lines = append(lines, l.renderRow(t, active, nameW, frame)...)
		if active {
			selEnd = len(lines) - 1
		}
	}
	lines = append(lines, frame.Blank())
	return lines, selLine, selEnd
}

// renderRow draws one command on exactly one terminal row. Expanded mode uses
// renderExpandedHeader instead, so a row never wraps or grows the selection.
func (l *ListView) renderRow(t Task, active bool, nameW int, frame Frame) []string {
	st := l.st
	filler, nameStyle, descStyle := st.sp, st.sFg, st.sMuted
	caretCol := filler.Width(2).Render("")
	chip := ""
	if t.Key != "" {
		chip = st.keyChip.Render(t.Key)
	}
	if active {
		filler, nameStyle, descStyle = st.selSp, st.selText, st.selText
		caretCol = st.selText.Bold(true).Width(2).Render("❯")
		if t.Key != "" {
			// Glyph rails keep every keycap cell on the selection background.
			chip = st.selChip.Render("│" + t.Key + "│")
		}
	}

	prefix := filler.PaddingLeft(padX-1).Render("") + caretCol +
		nameStyle.Bold(true).Width(nameW).Render(t.rowName()) + filler.Render(" ")
	tail := chip + filler.PaddingRight(padX).Render("")
	indentW := lipgloss.Width(prefix)
	wrapW := max(l.inner-indentW-lipgloss.Width(tail)-1, 1)

	// Control whitespace would break the one-row contract; descriptions are
	// summaries, so a row treats it as space and clips the overflow.
	text := ansi.Truncate(oneLine.Replace(t.Description), wrapW, "…")
	line := ui.PlaceRight(l.inner, prefix+descStyle.Render(text), tail, filler)
	return []string{frame.Paint(line, filler)}
}

// renderExpandedHeader draws the pinned detail-page title. It intentionally
// drops the selection bar, inline description, and keycap: the downward caret
// is the only row cue, and the inset below carries the full command detail.
func (l *ListView) renderExpandedHeader(t Task, frame Frame) string {
	st := l.st
	filler := st.sp
	left := filler.PaddingLeft(padX-1).Render("") +
		st.sAccent2.Bold(true).Width(2).Render("▼") +
		st.sFg.Bold(true).Render(t.rowName())
	return frame.Paint(ui.PlaceRight(l.inner, left, "", filler), filler)
}

// renderDetails draws the expanded panel on the darker bg inset, framed with
// the same side rails as the picker. It fills the body below the pinned
// header: short disclosures pad with inset rows and longer ones scroll.
func (l *ListView) renderDetails(t Task, height int) []string {
	st := l.st
	inner := l.inner
	insetSp := st.windowUI.Fill()
	// Align disclosure content with the caret column of the item rows.
	detailIndent := padX - 1
	indent := insetSp.PaddingLeft(detailIndent).Render("")
	wrapW := max(inner-detailIndent-padX, 1)

	paintInset := func(content string) string {
		panel := insetSp.Width(inner).MaxWidth(inner).Render(content)
		return st.frame.Render("│") + panel + st.frame.Render("│")
	}

	var out []string
	out = append(out, paintInset(""))
	if t.Description != "" {
		for _, line := range ui.WrapText(t.Description, wrapW) {
			out = append(out, paintInset(indent+st.inset(st.pal.Fg).Render(line)))
		}
	}
	if t.Details != "" {
		if t.Description != "" {
			out = append(out, paintInset(""))
		}
		for _, line := range ui.WrapText(t.Details, wrapW) {
			out = append(out, paintInset(indent+st.inset(st.pal.Muted).Render(line)))
		}
	} else if t.Description == "" {
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
	for _, hidden := range t.hides {
		out = append(out, paintInset(indent+st.inset(st.pal.Dim).Render("hides ")+
			st.inset(st.pal.Muted).Render(hidden.noun+" "+hidden.name)))
	}
	out = append(out, paintInset(""))
	// Fill unused body rows with the inset rather than the outer panel so the
	// detail view reads as one surface all the way to the status row.
	for len(out) < height {
		out = append(out, paintInset(""))
	}
	return out
}
