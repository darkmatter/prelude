package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// ---------------------------------------------------------------------------
// view: playground-aligned chrome — open title/prompt/status outside a framed
// list body, with half-cell surface transitions into the window background.
// ---------------------------------------------------------------------------

const padX = 2 // horizontal padding inside the frame

// chromeRows is the fixed vertical cost of title(3) + prompt(1) + frameTop(1) +
// status(3). listHeight subtracts this from the terminal so the panel fits.
const chromeRows = 8

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

// letterSpace renders tracked section labels: "PROJECT" → "PROJECT" (uppercased).
// Kept as a named helper so group headers stay consistent with the playground.
func letterSpace(s string) string {
	return strings.ToUpper(s)
}

// mutedTitleRow is the open chrome title: half-cell rise from window bg into
// the chrome surface, centered muted title, half-cell drop into the open input.
func (m model) mutedTitleRow(title string, inner int) string {
	st := m.st
	width := inner + 2
	halfPad := lipgloss.NewStyle().
		Foreground(st.chromeColor).
		Background(st.bgColor).
		Render(strings.Repeat("▄", width))
	row := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(st.pal.Muted))).
		Background(st.chromeColor).
		Width(width).
		MaxWidth(width).
		Align(lipgloss.Center).
		Render(ansi.Truncate(title, width-2*padX-2, "…"))
	bottomHalfPad := lipgloss.NewStyle().
		Foreground(st.chromeColor).
		Background(st.openColor).
		Render(strings.Repeat("▀", width))
	return lipgloss.JoinVertical(lipgloss.Left, halfPad, row, bottomHalfPad)
}

// statusBar is the open chrome footer: half-cell rise from open surface, keymap
// on chrome, half-cell drop back to window bg. Hints left, live status right.
func (m model) statusBar(hints [][2]string, status string, inner int) string {
	st := m.st
	width := inner + 2
	sp := st.chromeSp
	key := st.kbdChip
	mutedText := st.chromeMuted

	var b strings.Builder
	b.WriteString(sp.Render(strings.Repeat(" ", padX)))
	for i, h := range hints {
		if i > 0 {
			b.WriteString(mutedText.Render("  "))
		}
		b.WriteString(key.Render(" "+h[0]+" ") + sp.Render(" ") + mutedText.Render(h[1]))
	}
	left := b.String()

	statusColor := lipgloss.Color(string(st.pal.Accent))
	if strings.Contains(status, "args") {
		statusColor = lipgloss.Color(string(st.pal.Accent2))
	}
	right := lipgloss.NewStyle().
		Foreground(statusColor).
		Background(st.chromeColor).
		Bold(true).
		Render(status) + sp.Render(strings.Repeat(" ", padX))

	available := max(width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	line := left + sp.Render(strings.Repeat(" ", available)) + right
	keymap := sp.Width(width).MaxWidth(width).Render(ansi.Truncate(line, width, ""))

	topHalfPad := lipgloss.NewStyle().
		Foreground(st.chromeColor).
		Background(st.openColor).
		Render(strings.Repeat("▄", width))
	bottomHalfPad := lipgloss.NewStyle().
		Foreground(st.chromeColor).
		Background(st.bgColor).
		Render(strings.Repeat("▀", width))
	return lipgloss.JoinVertical(lipgloss.Left, topHalfPad, keymap, bottomHalfPad)
}

// promptLine is the open filter/arg input row: ~/project or task name, amber
// prompt glyph, then the textinput view. Sits outside the frame on open surface.
func (m model) promptLine(inner int, taskName string) string {
	st := m.st
	line := st.openSp.Render(strings.Repeat(" ", padX))
	context := "~/" + m.cfg.Project
	if taskName != "" {
		context = taskName
	}
	line += st.openMuted.Render(context) +
		st.openSp.Render(" ") +
		st.openAccent2.Bold(true).Render("❯") +
		st.openSp.Render(" ") +
		m.input.View()
	return st.openSp.Width(inner + 2).MaxWidth(inner + 2).Render(line)
}

func (m model) blank(inner int) string {
	return m.paint("", m.st.sp, inner)
}

func (m model) View() tea.View {
	if m.mode == modeHelp {
		return m.help.View()
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
	// Horizontally and vertically center the panel in the terminal window.
	content := lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		body,
		lipgloss.WithWhitespaceStyle(m.st.windowBg),
	)
	view := tea.NewView(content)
	view.BackgroundColor = m.st.bgColor
	view.AltScreen = true
	return view
}
