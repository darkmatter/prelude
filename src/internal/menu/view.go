package menu

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
// status(3). listHeight subtracts this from the terminal so the panel fits,
// leaving one row for the pinned script preview (see View/overlayLastRow).
const chromeRows = 8

// Layout captures the terminal and panel geometry computed once per resize.
// It is the single source of truth for placement: sub-views receive it at
// Sync/View time instead of reaching back into the root model.
type Layout struct {
	width      int // terminal width
	height     int // terminal height
	inner      int // panel inner width
	listHeight int // list body height
}

func newLayout(cfg *Config, width, height int) Layout {
	inner := width - 2 // frame borders
	if cfg.MaxWidth > 0 && inner > cfg.MaxWidth {
		inner = cfg.MaxWidth
	}
	listHeight := max(min(cfg.Height, height-chromeRows-1), 4)
	return Layout{width: width, height: height, inner: inner, listHeight: listHeight}
}

// applyLayout recomputes the single Layout value and propagates the inner
// width to every chrome element. Called from newModel and on every
// WindowSizeMsg so the title, status, frame, prompt, list, and args stay in
// sync without each element reaching back into the root model for its width.
func (m *model) applyLayout(width, height int) {
	m.layout = newLayout(m.cfg, width, height)
	m.title = m.title.WithSize(m.layout.inner)
	m.status = m.status.WithSize(m.layout.inner)
	m.frame = m.frame.WithSize(m.layout.inner)
	m.prompt = m.prompt.WithSize(m.layout.inner, m.promptCtx)
	m.list = m.list.WithSize(m.layout.inner)
	m.args = m.args.WithSize(m.layout.inner)
}

// syncList recomputes the list body's rows and scroll offset from the
// root-owned selection/match/geometry state, caching them so the next View()
// is a pure return. Called after filter, selection, expand, and resize —
// never from View. This is the update-time home for the computation that the
// old renderRows used to do inside View (a bubbletea anti-pattern).
func (m *model) syncList() {
	m.list = m.list.Sync(m.flat, m.matches, m.sel, m.expanded, m.layout.listHeight, m.prompt.Value(), m.frame)
}

func (m model) View() tea.View {
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
	// Reserve the last terminal row for the pinned script preview: place the
	// panel into height-1 rows so it cannot grow into the preview's slot,
	// then overlayLastRow stamps the preview onto the genuine last row.
	previewRow := m.layout.height - 1
	content := lipgloss.Place(
		m.layout.width,
		previewRow,
		lipgloss.Center,
		lipgloss.Center,
		body,
		lipgloss.WithWhitespaceStyle(m.st.windowBg),
	)
	// The menu is an alt-screen program, so ble.sh's prompt_status_line is
	// gone. Pin the assembled script on the last terminal row so run looks
	// like typing the command at a prompt.
	content = overlayLastRow(content, m.renderScriptPreviewRow(), m.blankWindowRow(), m.layout.height)
	view := tea.NewView(content)
	view.BackgroundColor = m.st.bgColor
	view.AltScreen = true
	cursor := m.prompt.Cursor(m.promptCtx)
	if cursor != nil {
		// lipgloss.Place centers by assigning the odd remainder to the right
		// and bottom, so integer division reproduces its left/top offsets.
		cursor.Position.X += max((m.layout.width-lipgloss.Width(body))/2, 0)
		cursor.Position.Y += max((previewRow-lipgloss.Height(body))/2, 0) + titleRows
		view.Cursor = cursor
	}
	return view
}

// invocationPreview is the shell script Enter would hand to finish(). It is
// the single source of truth for the last-row script preview: both list and
// arg mode render it via renderScriptPreviewRow, so there is no second,
// in-panel copy of the assembled command.
func (m model) invocationPreview() string {
	switch m.mode {
	case modeArgs:
		if t := m.args.Task(); t != nil {
			return assembleInvocation(*t, strings.TrimSpace(m.prompt.Value()))
		}
	default:
		if len(m.matches) == 0 {
			return ""
		}
		return assembleInvocation(m.flat[m.matches[m.sel]], "")
	}
	return ""
}

// pendingArgsHint reports whether the last-row preview should append a dim
// ellipsis. In list mode, a task with Args advertises that Enter collects
// arguments rather than running the command verbatim; the ellipsis is the same
// affordance the old in-panel ArgsView preview used. In arg mode, an empty
// argument line means the assembled command is still the bare Run string, so
// the ellipsis marks the pending argument.
func (m model) pendingArgsHint() bool {
	switch m.mode {
	case modeArgs:
		return strings.TrimSpace(m.prompt.Value()) == ""
	default:
		if len(m.matches) == 0 {
			return false
		}
		return len(m.flat[m.matches[m.sel]].Args) > 0
	}
}

func (m model) renderScriptPreviewRow() string {
	width := max(m.layout.width, 1)
	// Display-only: keep finish() on the original script, including the
	// two-line commands.gen.exec form (sync-docs / record-docs).
	script := collapsePreviewScript(m.invocationPreview())
	row := m.st.windowBg.PaddingLeft(padX).Render("")
	if script != "" {
		row += m.st.windowUI.Accent().Render("$ ") + m.st.windowUI.Foreground().Render(script)
	}
	if m.pendingArgsHint() {
		row += m.st.windowUI.Dim().Render(" …")
	}
	return m.st.windowBg.Width(width).MaxWidth(width).Render(ansi.Truncate(row, width, ""))
}

func (m model) blankWindowRow() string {
	width := max(m.layout.width, 1)
	return m.st.windowBg.Width(width).MaxWidth(width).Render("")
}

func collapsePreviewScript(script string) string {
	var b strings.Builder
	b.Grow(len(script))
	for _, r := range script {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteByte(' ')
		case r < 0x20 || r == 0x7f:
			// drop remaining C0 controls so the last-row overlay cannot wrap
		default:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func overlayLastRow(canvas, row, fill string, height int) string {
	if height < 1 {
		return canvas
	}
	lines := strings.Split(strings.TrimRight(canvas, "\n"), "\n")
	// collapsePreviewScript already stripped all CR/LF/control chars from
	// the preview, so the replacement row is a single display line. Pad
	// missing lines with themed fills so unused cells keep the window bg.
	for len(lines) < height {
		lines = append(lines, fill)
	}
	lines[height-1] = row
	return strings.Join(lines[:height], "\n")
}
