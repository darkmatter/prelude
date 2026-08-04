package main

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/screen"
	"github.com/charmbracelet/x/ansi"
)

var (
	outerBackground = color.RGBA{R: 9, G: 9, B: 11, A: 255}
	pinBackground   = color.RGBA{R: 15, G: 23, B: 42, A: 255}
	pinHeader       = color.RGBA{R: 30, G: 41, B: 59, A: 255}
	pinForeground   = color.RGBA{R: 203, G: 213, B: 225, A: 255}
	accent          = color.RGBA{R: 129, G: 140, B: 248, A: 255}
	statusBg        = color.RGBA{R: 24, G: 24, B: 27, A: 255}
	statusFg        = color.RGBA{R: 161, G: 161, B: 170, A: 255}
)

const focusToggleKey = "ctrl+g"

type ptyOutputMsg struct {
	data []byte
	err  error
}

type pinControlMsg struct {
	target string
	err    error
}

type childExitMsg struct{ err error }

type livePanelStartedMsg struct {
	generation uint64
	session    *livePanelSession
	err        error
}

type livePanelOutputMsg struct {
	generation uint64
	session    *livePanelSession
	data       []byte
	err        error
}

type livePanelExitMsg struct {
	generation uint64
	session    *livePanelSession
	err        error
}

type hostModel struct {
	session       *shellSession
	panel         *livePanelSession
	state         hostState
	env           []string
	windowFocused bool
	exitErr       error
}

func newHostModel(session *shellSession, shellRows int, env []string) *hostModel {
	return &hostModel{
		session:       session,
		state:         initialHostState(shellRows),
		env:           append([]string(nil), env...),
		windowFocused: true,
	}
}

func (model *hostModel) Init() tea.Cmd {
	return tea.Batch(
		readPTYCmd(model.session),
		readPinControlCmd(model.session),
	)
}

func (model *hostModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		var request *panelRequest
		model.state, request = transition(model.state, stateEvent{
			kind:   resizeEvent,
			width:  message.Width,
			height: message.Height,
		})
		model.recordResizeError()
		return model, panelCommand(request, model.env)

	case tea.KeyPressMsg:
		if message.String() == focusToggleKey && model.canToggleInput() {
			model.state, _ = transition(model.state, stateEvent{kind: toggleInputEvent})
			model.applyInputFocus()
			return model, nil
		}
		model.forwardKey(message)
		return model, nil

	case tea.PasteMsg:
		model.forwardPaste(message.Content)
		return model, nil

	case tea.FocusMsg:
		model.windowFocused = true
		model.applyInputFocus()
		return model, nil

	case tea.BlurMsg:
		model.windowFocused = false
		model.blurInputFocus()
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
		}
		if message.err != nil {
			return model, waitForChildCmd(model.session)
		}
		return model, readPTYCmd(model.session)

	case pinControlMsg:
		if message.err != nil {
			return model, nil
		}
		mode, ok := parsePinMode(message.target)
		if !ok {
			model.state.lastRuntimeError = fmt.Sprintf("unsupported pin target %q", message.target)
			return model, readPinControlCmd(model.session)
		}

		model.closeLivePanel()
		var request *panelRequest
		model.state, request = transition(model.state, stateEvent{
			kind: setPinEvent,
			pin:  mode,
		})
		model.recordResizeError()
		model.applyInputFocus()
		return model, tea.Batch(
			panelCommand(request, model.env),
			readPinControlCmd(model.session),
		)

	case childExitMsg:
		model.closeLivePanel()
		model.exitErr = message.err
		return model, tea.Quit

	case panelResultMsg:
		model.state = acceptPanelResult(model.state, message)
		return model, nil

	case livePanelStartedMsg:
		if message.generation != model.state.panelGeneration || model.state.pin != pinDocs {
			message.session.Close()
			return model, nil
		}
		if message.err != nil {
			model.state = failLivePanel(model.state, message.generation, message.err)
			model.applyInputFocus()
			return model, nil
		}
		model.closeLivePanel()
		model.panel = message.session
		model.recordResizeError()
		model.applyInputFocus()
		return model, readLivePanelCmd(model.panel, message.generation)

	case livePanelOutputMsg:
		if message.session != model.panel || message.generation != model.state.panelGeneration {
			return model, nil
		}
		if len(message.data) > 0 {
			if _, err := model.panel.emulator.Write(message.data); err != nil {
				model.failAndCloseLivePanel(message.generation, fmt.Errorf("panel virtual terminal: %w", err))
				return model, nil
			}
			model.state = acceptLivePanelOutput(model.state, message.generation)
		}
		if message.err != nil {
			return model, waitForLivePanelCmd(model.panel, message.generation)
		}
		return model, readLivePanelCmd(model.panel, message.generation)

	case livePanelExitMsg:
		if message.session != model.panel || message.generation != model.state.panelGeneration {
			return model, nil
		}
		model.closeLivePanel()
		if message.err == nil {
			model.state, _ = transition(model.state, stateEvent{kind: setPinEvent, pin: pinOff})
		} else {
			model.state = failLivePanel(model.state, message.generation, message.err)
		}
		model.recordResizeError()
		model.applyInputFocus()
		return model, nil
	}

	return model, nil
}

func (model *hostModel) View() tea.View {
	frame := uv.NewScreenBuffer(model.state.layout.cols, model.state.layout.height)
	frame.Fill(&uv.Cell{
		Content: " ",
		Width:   1,
		Style:   uv.Style{Bg: outerBackground},
	})
	model.drawPin(&frame)
	model.drawShell(&frame)
	model.drawStatus(&frame)

	view := tea.NewView(frame.Render())
	view.AltScreen = true
	view.BackgroundColor = outerBackground
	view.ReportFocus = true
	view.MouseMode = tea.MouseModeAllMotion
	view.WindowTitle = model.session.title
	view.Cursor = model.cursor()
	return view
}

func (model *hostModel) recordResizeError() {
	var failures []string
	if err := model.session.Resize(model.state.layout.cols, model.state.layout.shellRows); err != nil {
		failures = append(failures, err.Error())
	}
	if model.panel != nil {
		if err := model.panel.Resize(model.state.layout.cols, model.state.layout.pinHeight); err != nil {
			failures = append(failures, err.Error())
		}
	}
	model.state.lastRuntimeError = strings.Join(failures, "; ")
}

func (model *hostModel) canToggleInput() bool {
	return model.panel != nil &&
		model.state.pin == pinDocs &&
		model.state.panelPhase == panelReady
}

func (model *hostModel) docsOwnsInput() bool {
	return model.canToggleInput() && model.state.input == inputDocs
}

func (model *hostModel) forwardKey(message tea.KeyPressMsg) {
	key := message.Key()
	if key.Text != "" {
		if key.Mod.Contains(tea.ModAlt) {
			model.sendText("\x1b")
		}
		model.sendText(key.Text)
		return
	}

	event := uv.KeyPressEvent(uv.Key(key))
	if model.docsOwnsInput() {
		model.panel.SendKey(event)
		return
	}
	model.session.SendKey(event)
}

func (model *hostModel) sendText(text string) {
	if model.docsOwnsInput() {
		model.panel.SendText(text)
		return
	}
	model.session.SendText(text)
}

func (model *hostModel) forwardPaste(content string) {
	if model.docsOwnsInput() {
		model.panel.Paste(content)
		return
	}
	model.session.Paste(content)
}

func (model *hostModel) applyInputFocus() {
	if !model.windowFocused {
		return
	}
	if model.docsOwnsInput() {
		model.session.Blur()
		model.panel.Focus()
		return
	}
	if model.panel != nil {
		model.panel.Blur()
	}
	model.session.Focus()
}

func (model *hostModel) blurInputFocus() {
	model.session.Blur()
	if model.panel != nil {
		model.panel.Blur()
	}
}

func (model *hostModel) closeLivePanel() {
	panel := model.panel
	model.panel = nil
	if panel == nil {
		return
	}
	panel.Blur()
	panel.Close()
}

func (model *hostModel) failAndCloseLivePanel(generation uint64, err error) {
	model.closeLivePanel()
	model.state = failLivePanel(model.state, generation, err)
	model.applyInputFocus()
}

func (model *hostModel) Close() {
	model.closeLivePanel()
}

func panelCommand(request *panelRequest, env []string) tea.Cmd {
	if request == nil {
		return nil
	}
	if request.mode == pinDocs {
		return startLivePanelCmd(*request, env)
	}
	return loadPanelCmd(*request, env)
}

func (model *hostModel) drawPin(frame *uv.ScreenBuffer) {
	if model.state.layout.pinHeight == 0 {
		return
	}
	frame.FillArea(&uv.Cell{
		Content: " ",
		Width:   1,
		Style:   uv.Style{Fg: pinForeground, Bg: pinBackground},
	}, uv.Rect(0, 0, model.state.layout.cols, model.state.layout.pinHeight))
	if model.state.pin == pinDocs && model.state.panelPhase == panelReady && model.panel != nil {
		for y := 0; y < model.state.layout.pinHeight; y++ {
			for x := 0; x < model.state.layout.cols; x++ {
				if cell := model.panel.emulator.CellAt(x, y); cell != nil {
					frame.SetCell(x, y, cell.Clone())
				}
			}
		}
		return
	}

	ctx := screen.NewContext(frame)
	header := fmt.Sprintf(
		" pin:%s · generation:%d · panel:%s ",
		model.state.pin.label(),
		model.state.panelGeneration,
		model.state.panelPhase,
	)
	ctx.SetStyle(uv.Style{Fg: accent, Bg: pinHeader, Attrs: uv.AttrBold})
	ctx.DrawString(fitLine(header, model.state.layout.cols), 0, 0)

	ctx.SetStyle(uv.Style{Fg: pinForeground, Bg: pinBackground})
	if model.state.panelPhase == panelReady && model.state.panelSnapshot != nil {
		for y := 0; y < model.state.layout.pinHeight; y++ {
			for x := 0; x < model.state.layout.cols; x++ {
				if cell := model.state.panelSnapshot.cellAt(x, y); cell != nil {
					frame.SetCell(x, y, cell.Clone())
				}
			}
		}
		return
	}

	lines := []string{"Loading pinned surface asynchronously…"}
	if model.state.panelPhase == panelFailed {
		lines = []string{"Panel capture failed.", model.state.panelError}
	}
	for row := 1; row < model.state.layout.pinHeight; row++ {
		line := ""
		if row-1 < len(lines) {
			line = lines[row-1]
		}
		ctx.DrawString(fitLine(line, model.state.layout.cols), 0, row)
	}
}

func (model *hostModel) drawShell(frame *uv.ScreenBuffer) {
	for y := 0; y < model.state.layout.shellRows; y++ {
		for x := 0; x < model.state.layout.cols; x++ {
			cell := model.session.emulator.CellAt(x, y)
			if cell == nil {
				continue
			}
			frame.SetCell(x, model.state.layout.shellTop+y, cell.Clone())
		}
	}
}

func (model *hostModel) drawStatus(frame *uv.ScreenBuffer) {
	screenKind := "main"
	if model.session.emulator.IsAltScreen() {
		screenKind = "alt"
	}
	status := fmt.Sprintf(
		" vt spike · pin:%s · input:%s · shell:%dx%d@%d · child:%s · panel:%s#%d · ^G input · vtpin motd|docs|off ",
		model.state.pin.label(),
		model.state.input.label(),
		model.state.layout.cols,
		model.state.layout.shellRows,
		model.state.layout.shellTop,
		screenKind,
		model.state.panelPhase,
		model.state.panelGeneration,
	)
	if model.state.lastRuntimeError != "" {
		status = " error: " + model.state.lastRuntimeError
	}
	ctx := screen.NewContext(frame)
	ctx.SetStyle(uv.Style{Fg: statusFg, Bg: statusBg, Attrs: uv.AttrFaint})
	ctx.DrawString(fitLine(status, model.state.layout.cols), 0, model.state.layout.statusRow)
}

func (model *hostModel) cursor() *tea.Cursor {
	if model.docsOwnsInput() {
		if !model.panel.cursor.visible {
			return nil
		}
		position := model.panel.emulator.CursorPosition()
		if position.X < 0 || position.X >= model.state.layout.cols ||
			position.Y < 0 || position.Y >= model.state.layout.pinHeight {
			return nil
		}
		cursor := tea.NewCursor(position.X, position.Y)
		cursor.Shape = tea.CursorShape(model.panel.cursor.style)
		cursor.Blink = model.panel.cursor.blink
		return cursor
	}
	if !model.session.cursor.visible {
		return nil
	}
	position := model.session.emulator.CursorPosition()
	if position.X < 0 || position.X >= model.state.layout.cols ||
		position.Y < 0 || position.Y >= model.state.layout.shellRows {
		return nil
	}
	cursor := tea.NewCursor(position.X, model.state.layout.shellTop+position.Y)
	cursor.Shape = tea.CursorShape(model.session.cursor.style)
	cursor.Blink = model.session.cursor.blink
	return cursor
}

func (model *hostModel) forwardMouse(message tea.MouseMsg) {
	mouse := message.Mouse()
	if model.docsOwnsInput() {
		if mouse.X < 0 || mouse.X >= model.state.layout.cols ||
			mouse.Y < 0 || mouse.Y >= model.state.layout.pinHeight {
			return
		}
		translated := uv.Mouse{
			X:      mouse.X,
			Y:      mouse.Y,
			Button: mouse.Button,
			Mod:    mouse.Mod,
		}
		switch message.(type) {
		case tea.MouseClickMsg:
			model.panel.SendMouse(uv.MouseClickEvent(translated))
		case tea.MouseReleaseMsg:
			model.panel.SendMouse(uv.MouseReleaseEvent(translated))
		case tea.MouseWheelMsg:
			model.panel.SendMouse(uv.MouseWheelEvent(translated))
		case tea.MouseMotionMsg:
			model.panel.SendMouse(uv.MouseMotionEvent(translated))
		}
		return
	}
	if mouse.X < 0 || mouse.X >= model.state.layout.cols ||
		mouse.Y < model.state.layout.shellTop ||
		mouse.Y >= model.state.layout.shellTop+model.state.layout.shellRows {
		return
	}
	translated := uv.Mouse{
		X:      mouse.X,
		Y:      mouse.Y - model.state.layout.shellTop,
		Button: mouse.Button,
		Mod:    mouse.Mod,
	}
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

func readPTYCmd(session *shellSession) tea.Cmd {
	return func() tea.Msg {
		buffer := make([]byte, 32*1024)
		count, err := session.pty.Read(buffer)
		return ptyOutputMsg{data: append([]byte(nil), buffer[:count]...), err: err}
	}
}

func readPinControlCmd(session *shellSession) tea.Cmd {
	return func() tea.Msg {
		target, err := session.ReadPinTarget()
		return pinControlMsg{target: target, err: err}
	}
}

func waitForChildCmd(session *shellSession) tea.Cmd {
	return func() tea.Msg { return childExitMsg{err: session.Wait()} }
}

func startLivePanelCmd(request panelRequest, env []string) tea.Cmd {
	return func() tea.Msg {
		session, err := startLivePanelCommand("docs", nil, request.cols, request.rows, env)
		return livePanelStartedMsg{
			generation: request.generation,
			session:    session,
			err:        err,
		}
	}
}

func readLivePanelCmd(session *livePanelSession, generation uint64) tea.Cmd {
	return func() tea.Msg {
		buffer := make([]byte, 32*1024)
		count, err := session.pty.Read(buffer)
		return livePanelOutputMsg{
			generation: generation,
			session:    session,
			data:       append([]byte(nil), buffer[:count]...),
			err:        err,
		}
	}
}

func waitForLivePanelCmd(session *livePanelSession, generation uint64) tea.Cmd {
	return func() tea.Msg {
		return livePanelExitMsg{
			generation: generation,
			session:    session,
			err:        session.Wait(),
		}
	}
}

func fitLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	line = strings.ReplaceAll(line, "\t", "    ")
	if ansi.StringWidth(line) > width {
		line = ansi.Truncate(line, width, "…")
	}
	return line + strings.Repeat(" ", max(width-ansi.StringWidth(line), 0))
}
