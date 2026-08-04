package main

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/screen"
)

// pinKey cycles the pinned surface and focusKey moves input between the shell
// and a live pinned pane. Both are intercepted before the child sees them;
// `ctrl+v` first (readline's quoted insert) passes a literal through.
const (
	pinKey   = "ctrl+g"
	focusKey = "ctrl+o"
)

const quotedInsertKey = "ctrl+v"

// paneScrollback is small on purpose: a live pane is a viewport onto a running
// program, not a transcript to page back through.
const paneScrollback = 200

// frameMsg asks for a repaint. It carries nothing: the child's state already
// lives in the virtual terminal, and the throttle behind it guarantees a burst
// of PTY writes costs one frame, not one frame per write.
type frameMsg struct{}

type childGoneMsg struct{ err error }

// paneGoneMsg reports that a live pane's child exited. It names the child,
// because "a pane exited" is not by itself actionable: every pane the host
// closes on purpose emits one too, and it arrives after the fact.
//
// Without the identity a shrink that retires the pane would look like the user
// quitting Docs and would unpin it, so growing the terminal back would not
// bring it back; and a late message from a closed pane would tear down a newer
// one that had since opened in its place.
type paneGoneMsg struct{ child *shell }

type panelMsg struct{ result panelResult }

type host struct {
	shell *shell
	state state
	env   []string

	// pane is the child of a live pin mode, nil whenever no live surface is
	// on screen. It is a full shell: same drawing, resizing, scrollback, and
	// shutdown, differing only in which band it lands in and that its band is
	// chrome and therefore composed opaquely.
	pane *shell

	// docsCommand is the program a live docs pin runs. Overridable so the
	// host is testable without the real docs binary installed.
	docsCommand string

	// repaint and send let a pane's own output and exit reach the program the
	// same way the shell's do. They are injected once the program exists.
	repaint func()
	send    func(tea.Msg)

	// frame is reused between renders and reallocated only when the terminal
	// changes size.
	frame *uv.ScreenBuffer

	// terminalFocused mirrors the outer window's focus. Combined with the
	// keyboard focus it decides which child believes it is active.
	terminalFocused bool

	quoted bool
	fatal  error
}

func newHost(child *shell, initial state, env []string, docsCommand string) *host {
	return &host{
		shell:       child,
		state:       initial,
		env:         append([]string(nil), env...),
		docsCommand: docsCommand,
		// Assume focus until the terminal says otherwise; a host that starts
		// blurred would leave the shell's cursor dormant on terminals that
		// never report focus at all.
		terminalFocused: true,
		repaint:         func() {},
		send:            func(tea.Msg) {},
	}
}

// attach wires the host to its running program so a live pane can drive frames
// and report its own exit.
func (h *host) attach(repaint func(), send func(tea.Msg)) {
	h.repaint = repaint
	h.send = send
}

func (h *host) Init() tea.Cmd { return nil }

func (h *host) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		next, request, spawn := h.state.resize(message.Width, message.Height)
		return h, h.adopt(next, request, spawn)

	case tea.KeyPressMsg:
		return h, h.handleKey(message)

	case tea.PasteMsg:
		// Paste is input, so it follows the keyboard like everything else.
		if h.state.focus == focusShell {
			h.state = h.state.followTail()
		}
		h.focused().paste(message.Content)
		return h, nil

	case tea.FocusMsg:
		h.terminalFocused = true
		h.syncChildFocus()
		return h, nil

	case tea.BlurMsg:
		h.terminalFocused = false
		h.syncChildFocus()
		return h, nil

	case tea.MouseMsg:
		h.forwardMouse(message)
		return h, nil

	case panelMsg:
		h.state = h.state.applyPanel(message.result)
		return h, nil

	case frameMsg:
		// The virtual terminal already holds the new state; returning is
		// enough to make Bubble Tea render it.
		return h, nil

	case paneGoneMsg:
		// Only the pane currently on screen can speak for itself. Anything
		// else is the echo of a pane the host already retired.
		if message.child == nil || message.child != h.pane {
			return h, nil
		}
		// Its program quit on its own, so drop the pin rather than leaving a
		// frozen last frame pretending to be a running one.
		h.closePane()
		return h, h.adopt(h.state.setPin(pinOff))

	case childGoneMsg:
		h.fatal = message.err
		return h, tea.Quit
	}
	return h, nil
}

// adopt installs a new state, keeps both children in step with it, and starts
// a panel capture when the pin band changed shape.
func (h *host) adopt(next state, request panelRequest, spawn bool) tea.Cmd {
	h.state = next
	if _, err := h.shell.resize(next.layout.cols, next.layout.shell.height); err != nil {
		h.state.shellErr = err.Error()
	} else {
		h.state.shellErr = ""
	}
	h.reconcilePane()

	if !spawn {
		return nil
	}
	env := h.env
	return func() tea.Msg {
		image, err := loadPanel(request, env)
		result := panelResult{generation: request.generation, surface: image}
		if err != nil {
			result.err = err.Error()
		}
		return panelMsg{result: result}
	}
}

// reconcilePane brings the live pane into line with the state: started when a
// live mode has rows to occupy, resized when the band moves, stopped as soon
// as it has nowhere to be.
//
// Process lifecycle lives here rather than in the state machine so the state
// stays pure and testable, and so a pane that fails to start degrades into a
// visible error instead of a broken transition.
func (h *host) reconcilePane() {
	if !h.state.paneAvailable() {
		h.closePane()
		return
	}

	body := h.state.layout.panelBody()
	if h.pane != nil {
		if _, err := h.pane.resize(h.state.layout.cols, body.height); err != nil {
			h.state.panelErr = err.Error()
		}
		return
	}

	command, args := h.paneCommand(h.state.pin)
	child, err := startShell(shellSpec{
		command:    command,
		args:       args,
		cols:       h.state.layout.cols,
		rows:       body.height,
		scrollback: paneScrollback,
		env:        h.env,
	})
	if err != nil {
		h.state.phase = panelFailed
		h.state.panelErr = err.Error()
		return
	}

	h.pane = child
	h.state.phase = panelReady
	h.state.panelErr = ""

	// The pane drives frames exactly like the shell. Its exit is routed
	// through the program rather than mutating the host from a goroutine.
	repaint, send := h.repaint, h.send
	child.pump(repaint, func(error) { send(paneGoneMsg{child: child}) })

	// A new pane starts unfocused, so the shell keeps the keyboard and must
	// keep believing it has focus.
	h.syncChildFocus()
}

// paneCommand names the child behind a live pin mode.
func (h *host) paneCommand(mode pinMode) (string, []string) {
	if mode == pinDocs {
		return h.docsCommand, nil
	}
	return "", nil
}

// closePane stops the live child and gives its band back to the placeholder.
//
// The pane is detached before it is stopped. Stopping kills the PTY, which
// ends the child's pump and emits a paneGoneMsg; detaching first means that
// message can no longer match h.pane, so a close the host asked for cannot be
// mistaken for the user quitting the program.
func (h *host) closePane() {
	child := h.pane
	if child == nil {
		return
	}
	h.pane = nil
	child.stop()
	if h.state.phase == panelReady {
		h.state.phase = panelOff
	}
	h.syncChildFocus()
}

func (h *host) handleKey(message tea.KeyPressMsg) tea.Cmd {
	name := message.String()
	quoted := h.quoted
	h.quoted = false

	if !quoted {
		switch name {
		case pinKey:
			return h.adopt(h.state.cyclePin())
		case focusKey:
			h.state = h.state.toggleFocus()
			h.syncChildFocus()
			return nil
		}

		// Scrollback keys drive the shell's viewport, which is only meaningful
		// while the shell owns input. Focused on a pane they belong to the
		// pane, whose own program may well page with them.
		if h.state.focus == focusShell {
			switch name {
			case "shift+pgup":
				h.state = h.state.scrollBy(h.pageSize(), h.shell.view().history)
				return nil
			case "shift+pgdown":
				h.state = h.state.scrollBy(-h.pageSize(), h.shell.view().history)
				return nil
			}
		}
		h.quoted = name == quotedInsertKey
	}

	// Typing returns the shell viewport to the live screen, the way a
	// scrolled-back terminal snaps to the bottom on input. A pane has its own
	// viewport and does not disturb this one.
	if h.state.focus == focusShell {
		h.state = h.state.followTail()
	}

	target := h.focused()
	key := message.Key()
	if key.Text != "" {
		// Bubble Tea reports the printable result of a key in Text; Code alone
		// loses shifted characters. Alt is delivered as an ESC prefix, which
		// is what every shell readline expects.
		if key.Mod.Contains(tea.ModAlt) {
			target.sendText("\x1b")
		}
		target.sendText(key.Text)
		return nil
	}
	target.sendKey(uv.KeyPressEvent(uv.Key(key)))
	return nil
}

// focused is the child that keystrokes reach. State clamps focus to the shell
// whenever no pane is on screen, so the nil check is belt and braces rather
// than a second policy.
func (h *host) focused() *shell {
	if h.state.focus == focusPane && h.pane != nil {
		return h.pane
	}
	return h.shell
}

// syncChildFocus tells each child whether it is the active one. Only the child
// holding the keyboard is focused, and only while the outer terminal itself
// is: a pane reporting focus while the shell owns input would blink a cursor
// nobody is driving, and programs that gate behaviour on focus reports would
// act on a lie.
//
// Call this after anything that moves the keyboard: a focus toggle, a pane
// opening or closing, or the terminal window gaining or losing focus.
func (h *host) syncChildFocus() {
	active := h.focused()
	h.shell.setFocus(h.terminalFocused && active == h.shell)
	if h.pane != nil {
		h.pane.setFocus(h.terminalFocused && active == h.pane)
	}
}

func (h *host) pageSize() int { return max(h.state.layout.shell.height-1, 1) }

// forwardMouse sends an event to the child whose band contains the pointer,
// translated into that child's own coordinates.
//
// Position is the targeting here, not the keyboard: a wheel over the pinned
// pane should scroll the pane even while the shell owns input, which is how
// every other split-pane terminal behaves. A child that never asked for
// tracking has its emulator drop the event, so dispatching by band cannot
// invent input for a program that does not want it.
func (h *host) forwardMouse(message tea.MouseMsg) {
	mouse := message.Mouse()
	if mouse.X < 0 || mouse.X >= h.state.layout.cols {
		return
	}

	target, area := h.shell, h.state.layout.shell
	if body := h.state.layout.panelBody(); h.pane != nil && body.contains(mouse.Y) {
		target, area = h.pane, body
	}
	if !area.contains(mouse.Y) {
		return
	}

	translated := uv.Mouse{
		X:      mouse.X,
		Y:      mouse.Y - area.top,
		Button: mouse.Button,
		Mod:    mouse.Mod,
	}
	switch message.(type) {
	case tea.MouseClickMsg:
		target.sendMouse(uv.MouseClickEvent(translated))
	case tea.MouseReleaseMsg:
		target.sendMouse(uv.MouseReleaseEvent(translated))
	case tea.MouseWheelMsg:
		target.sendMouse(uv.MouseWheelEvent(translated))
	case tea.MouseMotionMsg:
		target.sendMouse(uv.MouseMotionEvent(translated))
	}
}

func (h *host) View() tea.View {
	layout := h.state.layout
	if h.frame == nil || h.frame.Width() != layout.cols || h.frame.Height() != layout.rows {
		buffer := uv.NewScreenBuffer(layout.cols, layout.rows)
		h.frame = &buffer
	}

	child := h.shell.view()
	h.drawPin()
	h.shell.draw(h.frame, layout.shell, h.state.scroll, shellPaint)
	h.drawStatus(child)

	view := tea.NewView(h.frame.Render())
	view.AltScreen = true
	view.BackgroundColor = shellBackground
	view.ReportFocus = true
	view.WindowTitle = child.title
	// Mirror the children instead of grabbing the mouse unconditionally: with
	// no tracking requested anywhere, the outer terminal keeps native
	// selection and copy. Either child asking is enough, because events are
	// dispatched by band and the other's emulator drops what it never wanted.
	view.MouseMode = tea.MouseModeNone
	if child.mouseTracking || (h.pane != nil && h.pane.view().mouseTracking) {
		view.MouseMode = tea.MouseModeAllMotion
	}
	view.Cursor = h.cursor(child)
	return view
}

func (h *host) drawPin() {
	layout := h.state.layout
	if layout.pin.height == 0 {
		return
	}

	context := screen.NewContext(h.frame)
	h.frame.FillArea(&headerCell, uv.Rect(0, layout.pin.top, layout.cols, 1))
	context.SetStyle(headerStyle)
	drawLine(context, h.pinHeader(), 0, layout.pin.top, layout.cols)

	body := layout.panelBody()
	if body.height == 0 {
		return
	}

	// A live pane draws itself straight from its own child, opaquely: the band
	// is chrome over the shell, so cells that left their colours unset adopt
	// the pin palette rather than the outer terminal's.
	if h.pane != nil {
		h.pane.draw(h.frame, body, 0, pinPaint)
		return
	}

	panel := h.state.panel
	if panel != nil && panel.cols == layout.cols && panel.rows == body.height {
		panel.blit(h.frame, 0, body.top)
		return
	}

	h.frame.FillArea(&pinCell, uv.Rect(0, body.top, layout.cols, body.height))
	if panel != nil {
		// Geometry drifted between capture and render; show what we have.
		panel.blit(h.frame, 0, body.top)
		return
	}
	context.SetStyle(pinStyle)
	for offset, line := range h.pinPlaceholder() {
		if offset >= body.height {
			break
		}
		drawLine(context, line, 0, body.top+offset, layout.cols)
	}
}

func (h *host) pinHeader() string {
	// A live pane says whether it holds the keyboard, because that is the one
	// thing a user cannot infer from the picture.
	if h.pane != nil {
		input := "shell"
		if h.state.focus == focusPane {
			input = "PANE"
		}
		return fmt.Sprintf(
			" %s · live · input:%s · %s focus ",
			strings.ToUpper(h.state.pin.label()), input, focusKey,
		)
	}
	return fmt.Sprintf(
		" %s · panel:%s · generation:%d ",
		strings.ToUpper(h.state.pin.label()),
		h.state.phase,
		h.state.generation,
	)
}

func (h *host) pinPlaceholder() []string {
	switch h.state.phase {
	case panelFailed:
		return []string{"Panel capture failed.", h.state.panelErr}
	case panelLoading:
		return []string{"Capturing pinned surface…"}
	}
	return []string{"No pinned surface."}
}

func (h *host) drawStatus(child shellView) {
	layout := h.state.layout
	h.frame.FillArea(&statusCell, uv.Rect(0, layout.statusRow, layout.cols, 1))

	context := screen.NewContext(h.frame)
	if failure := h.statusFailure(); failure != "" {
		context.SetStyle(errorStyle)
		drawLine(context, " "+failure+" ", 0, layout.statusRow, layout.cols)
		return
	}

	screenKind := "main"
	if child.altScreen {
		screenKind = "alt"
	}
	position := "live"
	if h.state.scroll > 0 {
		position = fmt.Sprintf("-%d/%d", h.state.scroll, child.history)
	}
	status := fmt.Sprintf(
		" vt host · pin:%s · shell:%dx%d@%d · child:%s · view:%s · ^G pin · shift+pgup scroll · exit shell ",
		h.state.pin.label(),
		layout.cols,
		layout.shell.height,
		layout.shell.top,
		screenKind,
		position,
	)
	context.SetStyle(statusStyle)
	drawLine(context, status, 0, layout.statusRow, layout.cols)
}

func (h *host) statusFailure() string {
	switch {
	case h.state.shellErr != "":
		return "shell: " + h.state.shellErr
	case h.state.phase == panelFailed && h.state.panel == nil:
		return "panel: " + h.state.panelErr
	}
	return ""
}

// cursor places the outer terminal's cursor over whichever child holds the
// keyboard, and only while that child's viewport follows its live screen: a
// cursor drawn into scrollback would be pointing at history.
//
// There is one cursor and one focus, so a pinned pane taking input also takes
// the cursor. That is the clearest signal available that keystrokes have
// moved, short of drawing a border.
func (h *host) cursor(child shellView) *tea.Cursor {
	target, area, scroll := child, h.state.layout.shell, h.state.scroll
	if h.state.focus == focusPane && h.pane != nil {
		target, area, scroll = h.pane.view(), h.state.layout.panelBody(), 0
	}

	if !target.cursor.visible || scroll > 0 {
		return nil
	}
	position := target.cursor.position
	if position.X < 0 || position.X >= h.state.layout.cols ||
		position.Y < 0 || position.Y >= area.height {
		return nil
	}
	cursor := tea.NewCursor(position.X, area.top+position.Y)
	cursor.Shape = tea.CursorShape(target.cursor.style)
	cursor.Blink = target.cursor.blink
	return cursor
}
