package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// ---------------------------------------------------------------------------
// view: a hand-drawn window frame so section dividers junction into the
// border (├──┤) and every interior cell is painted like the mock.
// ---------------------------------------------------------------------------

const padX = 2 // horizontal padding inside the frame

func (m model) innerWidth() int {
	w := m.width - 2 // frame borders
	if m.cfg.MaxWidth > 0 && w > m.cfg.MaxWidth {
		w = m.cfg.MaxWidth
	}
	return max(w, 40)
}

// paint pads a styled line to the inner width with the given filler and
// wraps it in frame borders.
func (m model) paint(content string, filler lipgloss.Style, inner int) string {
	content = filler.MaxWidth(inner).Width(inner).Render(content)
	v := m.st.frame.Render("│")
	return v + content + v
}

func (m model) frameTop(inner int) string {
	return m.st.frame.Render("╭" + strings.Repeat("─", inner) + "╮")
}

func (m model) frameDiv(inner int) string {
	return m.st.frame.Render("├" + strings.Repeat("─", inner) + "┤")
}

func (m model) frameBottom(inner int) string {
	return m.st.frame.Render("╰" + strings.Repeat("─", inner) + "╯")
}

// letterSpace renders the mock's tracked section labels: "PROJECT" → "P R O J E C T".
func letterSpace(s string) string {
	runes := []rune(strings.ToUpper(s))
	out := make([]string, len(runes))
	for i, r := range runes {
		out[i] = string(r)
	}
	return strings.Join(out, "")
}

// titleBar renders the secondary-colored top bar with the centered title.
func (m model) titleBar(title string, inner int) string {
	st := m.st
	mid := st.barMuted.Render(ansi.Truncate(title, inner-2*padX, "…"))
	line := lipgloss.PlaceHorizontal(inner, lipgloss.Center, mid, lipgloss.WithWhitespaceStyle(st.barSp))
	return m.paint(line, st.barSp, inner)
}

// statusBar renders the secondary-colored bottom bar: kbd hint chips left,
// mode indicator right.
func (m model) statusBar(hints [][2]string, status string, inner int) string {
	st := m.st
	var b strings.Builder
	b.WriteString(st.barSp.Render(strings.Repeat(" ", padX)))
	for i, h := range hints {
		if i > 0 {
			b.WriteString(st.barSp.Render("  "))
		}
		b.WriteString(st.kbdChip.Render(" "+h[0]+" ") + st.barSp.Render(" ") + st.barMuted.Render(h[1]))
	}
	left := b.String()
	right := status + st.barSp.Render(strings.Repeat(" ", padX))
	line := lipgloss.JoinHorizontal(lipgloss.Top,
		left,
		lipgloss.PlaceHorizontal(inner-lipgloss.Width(left)-lipgloss.Width(right), lipgloss.Right, right, lipgloss.WithWhitespaceStyle(st.barSp)),
	)
	return m.paint(line, st.barSp, inner)
}

// promptLine renders the input row: `<project> ❯ [task] <input>`.
func (m model) promptLine(inner int, taskName string) string {
	st := m.st
	line := st.sp.Render(strings.Repeat(" ", padX)) +
		st.sMuted.Render(m.cfg.Project) + st.sp.Render(" ") + st.sAccent2.Render("❯") + st.sp.Render(" ")
	if taskName != "" {
		line += st.sAccent.Render(taskName) + st.sp.Render(" ")
	}
	line += m.input.View()
	return m.paint(line, st.sp, inner)
}

func (m model) blank(inner int) string {
	return m.paint("", m.st.sp, inner)
}

func (m model) View() tea.View {
	// Help mode is full-bleed and mouse-aware; it builds its own canvas.
	if m.mode == modeHelp {
		view := tea.NewView(m.viewHelp())
		view.BackgroundColor = m.st.bgColor
		view.AltScreen = true
		view.MouseMode = tea.MouseModeCellMotion
		return view
	}

	inner := m.innerWidth()

	var body string
	if m.mode == modeArgs {
		body = m.viewArgs(inner)
	} else {
		body = m.viewList(inner)
	}

	// BackgroundColor controls Bubble Tea's default SGR background, but cells
	// outside the rendered content can remain untouched by the renderer. Emit a
	// terminal-sized canvas with explicitly styled whitespace so every cell,
	// including the margins below and beside the panel, receives the theme bg.
	content := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Top,
		body,
		lipgloss.WithWhitespaceStyle(m.st.windowBg),
	)
	view := tea.NewView(content)
	view.BackgroundColor = m.st.bgColor
	view.AltScreen = true
	return view
}
