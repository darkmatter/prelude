package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/screen"
	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/ui"
)

type ptyOutputMsg struct {
	data []byte
	err  error
}

type childExitMsg struct{ err error }

type hostModel struct {
	session *shellSession
	state   hostState
	catalog menuCatalog
	exitErr error
}

func newHostModel(session *shellSession, state hostState, catalog menuCatalog) *hostModel {
	return &hostModel{session: session, state: state, catalog: catalog}
}

func (model *hostModel) Init() tea.Cmd {
	return readPTYCmd(model.session)
}

func (model *hostModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.state = model.state.resize(message.Width, message.Height)
		model.recordResizeError()
		return model, nil

	case tea.KeyPressMsg:
		if model.handleScrollKey(message) {
			return model, nil
		}
		// Any non-scroll key returns the viewport to the live tail before being
		// forwarded to the child. The host keeps no line-edit mirror: ble.sh
		// owns completion, descriptions, ghost text, and syntax validation.
		model.state = model.state.followTail()
		model.forwardKey(message)
		return model, nil

	case tea.PasteMsg:
		model.state = model.state.followTail()
		model.session.Paste(message.Content)
		return model, nil

	case tea.FocusMsg:
		model.session.Focus()
		return model, nil

	case tea.BlurMsg:
		model.session.Blur()
		return model, nil

	case tea.MouseMsg:
		model.forwardMouse(message)
		return model, nil

	case ptyOutputMsg:
		if len(message.data) > 0 {
			if _, err := model.session.emulator.Write(message.data); err != nil {
				model.exitErr = fmt.Errorf("virtual terminal: %w", err)
				return model, tea.Quit
			}
			model.state = model.state.scrollBy(0, model.session.emulator.ScrollbackLen())
		}
		if message.err != nil {
			return model, waitForChildCmd(model.session)
		}
		return model, readPTYCmd(model.session)

	case childExitMsg:
		model.exitErr = message.err
		return model, tea.Quit
	}
	return model, nil
}

func (model *hostModel) handleScrollKey(message tea.KeyPressMsg) bool {
	if model.session.emulator.IsAltScreen() {
		return false
	}
	history := model.session.emulator.ScrollbackLen()
	if history == 0 {
		return false
	}
	switch message.String() {
	case "shift+pgup":
		model.state = model.state.scrollBy(model.pageSize(), history)
		return true
	case "shift+pgdown":
		model.state = model.state.scrollBy(-model.pageSize(), history)
		return true
	default:
		return false
	}
}

func (model *hostModel) forwardKey(message tea.KeyPressMsg) {
	key := message.Key()
	if key.Text != "" {
		if key.Mod.Contains(tea.ModAlt) {
			model.session.SendText("\x1b")
		}
		model.session.SendText(key.Text)
		return
	}
	model.session.SendKey(uv.KeyPressEvent(uv.Key(key)))
}

func (model *hostModel) forwardMouse(message tea.MouseMsg) {
	mouse := message.Mouse()
	if mouse.X < 0 || mouse.X >= model.state.layout.cols ||
		mouse.Y < 0 || mouse.Y >= model.state.layout.shellRows {
		return
	}
	translated := uv.Mouse{X: mouse.X, Y: mouse.Y, Button: mouse.Button, Mod: mouse.Mod}
	switch message.(type) {
	case tea.MouseClickMsg:
		model.session.SendMouse(uv.MouseClickEvent(translated))
	case tea.MouseReleaseMsg:
		model.session.SendMouse(uv.MouseReleaseEvent(translated))
	case tea.MouseWheelMsg:
		model.session.SendMouse(uv.MouseWheelEvent(translated))
	case tea.MouseMotionMsg:
		model.session.SendMouse(uv.MouseMotionEvent(translated))
	}
}

func (model *hostModel) recordResizeError() {
	err := model.session.Resize(model.state.layout.cols, model.state.layout.shellRows)
	if err != nil {
		model.state.lastRuntimeError = err.Error()
		return
	}
	model.state.lastRuntimeError = ""
}

func (model *hostModel) pageSize() int { return max(model.state.layout.shellRows-1, 1) }

func (model *hostModel) View() tea.View {
	theme := model.catalog.statusTheme()
	frame := uv.NewScreenBuffer(model.state.layout.cols, model.state.layout.height)
	frame.Fill(&uv.Cell{Content: " ", Width: 1, Style: uv.Style{Bg: theme.outer}})
	model.drawShell(&frame)
	model.drawStatus(&frame)

	view := tea.NewView(frame.Render())
	view.AltScreen = true
	view.BackgroundColor = theme.outer
	view.ReportFocus = true
	view.MouseMode = tea.MouseModeAllMotion
	view.WindowTitle = model.session.title
	view.Cursor = model.cursor()
	return view
}

func (model *hostModel) drawShell(frame *uv.ScreenBuffer) {
	history := model.session.emulator.ScrollbackLen()
	scroll := clamp(model.state.scroll, 0, history)
	if model.session.emulator.IsAltScreen() {
		scroll = 0
	}
	first := history - scroll
	for y := 0; y < model.state.layout.shellRows; y++ {
		row := first + y
		for x := 0; x < model.state.layout.cols; x++ {
			var cell *uv.Cell
			if row < history {
				cell = model.session.emulator.ScrollbackCellAt(x, row)
			} else {
				cell = model.session.emulator.CellAt(x, row-history)
			}
			if cell != nil {
				frame.SetCell(x, y, cell.Clone())
			}
		}
	}
}

func (model *hostModel) drawStatus(frame *uv.ScreenBuffer) {
	if model.state.layout.statusRows == 0 {
		return
	}
	theme := model.catalog.statusTheme()
	ctx := screen.NewContext(frame)
	background := uv.Style{Fg: theme.fg, Bg: theme.bg}
	ctx.SetStyle(background)
	for row := 0; row < model.state.layout.statusRows; row++ {
		ctx.DrawString(strings.Repeat(" ", model.state.layout.cols), 0, model.state.layout.statusTop+row)
	}

	content := model.statusContent(theme)
	drawSplitStatusRow(ctx, content.primary, model.state.layout.statusTop, model.state.layout.cols)
	if model.state.layout.statusRows > 1 {
		drawMenuFooterLine(frame, model.state.layout.statusTop+1, model.state.layout.cols, theme, content.footer)
	}
}

type statusPart struct {
	text  string
	style uv.Style
}

type statusRow struct {
	left  []statusPart
	right []statusPart
}

type statusContent struct {
	primary statusRow
	footer  footerContent
}

type footerContent struct {
	hints  []ui.KeyHint
	status string
}

func drawSplitStatusRow(ctx *screen.Context, row statusRow, y, width int) {
	rightWidth := statusPartsWidth(row.right)
	if rightWidth >= width {
		drawStatusParts(ctx, row.right, 0, y, width)
		return
	}
	rightX := width - rightWidth
	leftWidth := rightX
	if len(row.left) > 0 && len(row.right) > 0 {
		leftWidth = max(leftWidth-1, 0)
	}
	drawStatusParts(ctx, row.left, 0, y, leftWidth)
	drawStatusParts(ctx, row.right, rightX, y, rightWidth)
}

func drawStatusParts(ctx *screen.Context, parts []statusPart, x, y, width int) {
	end := x + max(width, 0)
	for _, part := range parts {
		if x >= end {
			return
		}
		text := ansi.Truncate(part.text, end-x, "…")
		if text == "" {
			continue
		}
		ctx.SetStyle(part.style)
		ctx.DrawString(text, x, y)
		x += ansi.StringWidth(text)
	}
}

func statusPartsWidth(parts []statusPart) int {
	width := 0
	for _, part := range parts {
		width += ansi.StringWidth(part.text)
	}
	return width
}

// drawMenuFooterLine consumes the center row of the same KeyHintsFooter used
// by menu. KeyHintsFooter.Render joins three rows (top ▄ transition, the
// hints+status row, a bottom ▀ transition); the host's single footer line only
// has room for the center row, so the half-cell transitions are discarded.
// Ultraviolet parses the center row's ANSI output back into styled cells, so
// layout, key chips, spacing, and right-aligned status stay shared with menu.
func drawMenuFooterLine(frame *uv.ScreenBuffer, y, width int, theme statusTheme, content footerContent) {
	context := ui.NewContext(ui.Palette{}, theme.bg, false)
	mutedStyle := lipgloss.NewStyle().Foreground(theme.muted).Background(theme.bg)
	keyStyle := lipgloss.NewStyle().Foreground(theme.accent2).Background(theme.outer).Bold(true)
	statusStyle := lipgloss.NewStyle().Foreground(theme.success).Background(theme.bg).Bold(true)
	switch {
	case strings.Contains(content.status, "error"):
		statusStyle = lipgloss.NewStyle().Foreground(theme.error).Background(theme.bg).Bold(true)
	case strings.Contains(content.status, "unavailable"):
		statusStyle = lipgloss.NewStyle().Foreground(theme.warning).Background(theme.bg).Bold(true)
	}
	footer := ui.KeyHintsFooter{
		Context:           context,
		Width:             width,
		Muted:             &mutedStyle,
		Key:               &keyStyle,
		Text:              &mutedStyle,
		Status:            &statusStyle,
		HorizontalPadding: 2,
	}
	rows := strings.Split(footer.Render(content.hints, content.status), "\n")
	center := rows[len(rows)/2]
	uv.NewStyledString(center).Draw(frame, uv.Rect(0, y, width, 1))
}

// statusContent renders the thinned chrome. ble.sh owns argument metadata and
// completion descriptions, so the first row is only the idle MOTD lifecycle
// selection (or an error/catalogue-unavailable notice); the second row is the
// shared menu footer with scrollback key hints and a live status word.
func (model *hostModel) statusContent(theme statusTheme) statusContent {
	muted := uv.Style{Fg: theme.muted, Bg: theme.bg}
	dim := uv.Style{Fg: theme.dim, Bg: theme.bg, Attrs: uv.AttrFaint}
	if model.state.lastRuntimeError != "" {
		return statusContent{
			primary: statusRow{left: []statusPart{{text: " error: " + model.state.lastRuntimeError, style: uv.Style{Fg: theme.error, Bg: theme.bg}}}},
			footer:  footerContent{status: "● error"},
		}
	}
	if len(model.catalog.commands) == 0 {
		return statusContent{
			primary: statusRow{left: []statusPart{
				{text: " Prelude menu catalogue unavailable", style: muted},
				{text: "  Enter nix develop before running this prototype.", style: dim},
			}},
			footer: footerContent{status: "● unavailable"},
		}
	}

	parts := []statusPart{{text: " ", style: dim}}
	for index, command := range model.catalog.MOTDCommands {
		if index > 0 {
			parts = append(parts, statusPart{text: " · ", style: dim})
		}
		parts = append(parts, statusPart{text: command.Command, style: muted})
	}
	return statusContent{
		primary: statusRow{left: parts},
		footer: footerContent{
			hints:  []ui.KeyHint{{Key: " ⇧⇩ ", Text: "scroll"}, {Key: "⇥", Text: "complete"}},
			status: "● ready",
		},
	}
}

func (model *hostModel) cursor() *tea.Cursor {
	if model.state.scroll > 0 && !model.session.emulator.IsAltScreen() || !model.session.cursor.visible {
		return nil
	}
	position := model.session.emulator.CursorPosition()
	if position.X < 0 || position.X >= model.state.layout.cols ||
		position.Y < 0 || position.Y >= model.state.layout.shellRows {
		return nil
	}
	cursor := tea.NewCursor(position.X, position.Y)
	cursor.Shape = tea.CursorShape(model.session.cursor.style)
	cursor.Blink = model.session.cursor.blink
	return cursor
}

func readPTYCmd(session *shellSession) tea.Cmd {
	return func() tea.Msg {
		buffer := make([]byte, 32*1024)
		count, err := session.pty.Read(buffer)
		return ptyOutputMsg{data: append([]byte(nil), buffer[:count]...), err: err}
	}
}

func waitForChildCmd(session *shellSession) tea.Cmd {
	return func() tea.Msg { return childExitMsg{err: session.Wait()} }
}
