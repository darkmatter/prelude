package menu

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

// resizeChrome propagates a geometry change to every element whose width
// derives from the panel's inner width. Called from newModel and on every
// WindowSizeMsg so the title, status, frame, prompt, list, and args stay in
// sync without each element reaching back into the root model for its width.
func (m *model) resizeChrome() {
	inner := m.innerWidth()
	m.title = m.title.WithSize(inner)
	m.status = m.status.WithSize(inner)
	m.frame = m.frame.WithSize(inner)
	m.prompt = m.prompt.WithSize(inner)
	m.list = m.list.WithSize(inner)
	m.args = m.args.WithSize(inner)
}

// syncList recomputes the list body's rows and scroll offset from the
// root-owned selection/match/geometry state, caching them so the next View()
// is a pure return. Called after filter, selection, expand, and resize —
// never from View. This is the update-time home for the computation that the
// old renderRows used to do inside View (a bubbletea anti-pattern).
func (m *model) syncList() {
	m.list = m.list.Sync(m.flat, m.matches, m.sel, m.expanded, m.listHeight(), m.prompt.Value(), m.frame)
}

func (m model) View() tea.View {
	if m.mode == modeHelp {
		return m.help.View()
	}

	var body string
	if m.mode == modeArgs {
		body = m.viewArgs()
	} else {
		body = m.viewList()
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
	cursor := m.prompt.Cursor()
	if cursor != nil {
		// lipgloss.Place centers by assigning the odd remainder to the right
		// and bottom, so integer division reproduces its left/top offsets.
		cursor.Position.X += max((m.width-lipgloss.Width(body))/2, 0)
		cursor.Position.Y += max((m.height-lipgloss.Height(body))/2, 0) + titleRows
		view.Cursor = cursor
	}
	return view
}
