package main

import (
	"fmt"
	"image"
	"strings"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// wheelScrollLines is one wheel notch's worth of history, matching the step
// size most terminal emulators use for their own scrollback.
const wheelScrollLines = 3

func (h *host) layout() workspaceLayout {
	return computeLayout(h.cols, h.rows, h.placement, h.paneVisible && h.surface != surfaceNone)
}

func (h *host) setPlacement(where placement) tea.Cmd {
	h.placement = where
	h.notice = ""
	wasPane := h.focusPane
	if err := h.resizeWorkspace(); err != nil {
		return h.fail(err)
	}
	if wasPane != h.focusPane {
		return h.syncFocus()
	}
	return nil
}

func (h *host) currentPane(p *pane) bool {
	return p != nil && p == h.pane && !p.closed && !p.done
}

func (h *host) paneInteractive() bool {
	return h.currentPane(h.pane) && h.pane.terminal != nil &&
		h.paneError == nil && !h.layout().Body.Empty()
}

func (h *host) paneFocused() bool { return h.focusPane && h.paneInteractive() }

func (h *host) forwardPane(data []byte, err error) tea.Cmd {
	if err := h.pane.forward(data, err); err != nil {
		return h.failPane(err)
	}
	return nil
}

// Pane failures keep the final image/error visible and return input to Bash.
// Only a failure in the main shell's own engine/process can quit the host.
func (h *host) failPane(err error) tea.Cmd {
	if h.pane != nil {
		if finishErr := h.pane.finish(err); finishErr != nil {
			err = finishErr // finish already includes the failure and cleanup errors
		}
	}
	h.paneError, h.focusPane = err, false
	h.notice = fmt.Sprintf("%s error: %v", h.surface, err)
	if h.terminal != nil {
		return h.forward(h.terminal.Focus(h.terminalFocused))
	}
	return nil
}

// Focus loss is sent before focus gain. Inactive children never receive a gain,
// including while the outer terminal is blurred or the pane is suspended.
func (h *host) syncFocus() tea.Cmd {
	paneFocused := h.paneFocused()
	paneFocus := func(focused bool) tea.Cmd {
		if h.currentPane(h.pane) && h.pane.terminal != nil {
			return h.forwardPane(h.pane.terminal.Focus(focused))
		}
		return nil
	}
	shellFocus := func(focused bool) tea.Cmd {
		if h.terminal != nil {
			return h.forward(h.terminal.Focus(focused))
		}
		return nil
	}
	if paneFocused {
		if cmd := shellFocus(false); cmd != nil {
			return cmd
		}
		return paneFocus(h.terminalFocused)
	}
	if cmd := paneFocus(false); cmd != nil {
		return cmd
	}
	return shellFocus(h.terminalFocused)
}

// Hiding only changes composition and focus. The current pane keeps its PTY,
// engine, and output reader, so showing it restores its live state, not a restart.
func (h *host) togglePane() tea.Cmd {
	if h.surface == surfaceNone {
		return h.chooseSurface(surfaceMenu)
	}
	h.paneVisible = !h.paneVisible
	h.focusPane, h.notice = false, ""
	if err := h.resizeWorkspace(); err != nil {
		return h.fail(err)
	}
	h.focusPane = h.paneInteractive()
	return h.syncFocus()
}

func (h *host) closePane() tea.Cmd {
	p := h.pane
	h.pane, h.surface, h.paneError = nil, surfaceNone, nil
	h.leader = false // Layout commands need a window.
	h.paneVisible, h.focusPane, h.notice = false, false, ""
	if p != nil {
		if err := p.close(); err != nil {
			h.notice = fmt.Sprintf("close %s: %v", p.kind, err)
		}
	}
	if err := h.resizeWorkspace(); err != nil {
		return h.fail(err)
	}
	return h.syncFocus()
}

func (h *host) chooseSurface(kind surfaceKind) tea.Cmd {
	if kind == surfaceMenu && h.pendingCommand != "" {
		h.notice = "A menu command is waiting for the next Bash prompt"
		return nil
	}
	if h.surface == kind && h.currentPane(h.pane) && h.paneError == nil {
		return h.togglePane()
	}
	p := h.pane
	h.pane, h.surface, h.paneError = nil, kind, nil
	h.paneVisible, h.focusPane, h.notice = true, false, ""
	if p != nil {
		if err := p.close(); err != nil {
			h.notice = fmt.Sprintf("close %s: %v", p.kind, err)
		}
	}
	if err := h.resizeWorkspace(); err != nil {
		return h.fail(err)
	}
	body := h.layout().Body
	// Opening in a tiny window still creates a valid terminal; its process is
	// retained until a usable Body returns, without accepting hidden input.
	p, err := startPane(kind, max(1, body.Dx()), max(1, body.Dy()), h.env)
	if err != nil {
		h.paneError = err
		h.notice = fmt.Sprintf("%s error: %v", kind, err)
		return h.syncFocus()
	}
	h.pane = p
	h.focusPane = h.paneInteractive()
	if cmd := h.syncFocus(); cmd != nil {
		return cmd
	}
	// The picker publishes shell source separately and exits. Only then may
	// the UI-loop owner hand the selection to the main Bash at a primary prompt.
	return p.nextOutput
}

func chordText(key tea.Key) string {
	if key.Mod & ^(tea.ModShift|tea.ModCapsLock|tea.ModNumLock) != 0 {
		return ""
	}
	if key.Text != "" {
		return key.Text
	}
	if key.ShiftedCode != 0 {
		return string(key.ShiftedCode)
	}
	if key.Mod&tea.ModShift != 0 {
		if key.Code == '/' || key.Code == '?' {
			return "?"
		}
		return ""
	}
	return string(key.Code)
}

func (h *host) keyInput(message tea.KeyPressMsg) tea.Cmd {
	quoted := h.quoted
	h.quoted = false
	if !quoted {
		key := message.Key()
		if message.String() == "alt+tab" {
			h.leader = false
			return h.toggleFocus()
		}
		if key.Mod&tea.ModAlt != 0 {
			key.Mod &^= tea.ModAlt
			switch strings.ToLower(chordText(key)) {
			case "m":
				h.leader = false
				return h.showMotd()
			case "x":
				h.leader = false
				return h.chooseSurface(surfaceMenu)
			case "d":
				h.leader = false
				return h.chooseSurface(surfaceDocs)
			case "w":
				h.leader = false
				return h.togglePane()
			}
		}
		// Legacy Ctrl+[ arrives as Esc, so only a distinct enhanced key can
		// cycle backward. Ctrl+\ is unambiguous on legacy terminals too.
		// Layout keys leave command mode as it was.
		switch message.String() {
		case "ctrl+]":
			return h.setPlacement(h.placement.next())
		case "ctrl+[", "ctrl+\\":
			return h.setPlacement(h.placement.prev())
		}
		if message.String() == "ctrl+g" {
			h.leader = false // Child-owned even while command mode was unlocked.
		}
		// Unlocked command mode persists across its commands, so layouts can be
		// cycled and the window toggled repeatedly; only Esc or Ctrl+P lock.
		if h.leader {
			switch message.String() {
			case "ctrl+p", "esc":
				h.leader = false
				return nil
			case "tab":
				return h.toggleFocus()
			}
			switch chordText(message.Key()) {
			case "]":
				return h.setPlacement(h.placement.next())
			case "[":
				return h.setPlacement(h.placement.prev())
			}
			h.notice = fmt.Sprintf("Unknown key %q; Esc locks", message.String())
			return nil
		}
		switch message.String() {
		case "ctrl+p":
			// Layout commands need a window; otherwise Ctrl+P stays Readline's.
			if !h.windowOpen() {
				break
			}
			h.leader, h.notice = true, ""
			return nil

		case "ctrl+v":
			h.quoted = true
		}
	}
	h.notice = ""
	// Ordinary keys and Ctrl+G belong to the focused child outside command mode.
	if h.paneFocused() {
		if cmd := h.followPane(); cmd != nil {
			return cmd
		}
		return h.forwardPane(h.pane.terminal.Key(message.Key()))
	}
	if cmd := h.followShell(); cmd != nil {
		return cmd
	}
	data, err := h.terminal.Key(message.Key())
	if err == nil {
		h.noteShellInput(data, quoted)
	}
	return h.forward(data, err)
}

// toggleFocus moves input between Bash and a live interactive pane.
func (h *host) toggleFocus() tea.Cmd {
	if !h.paneInteractive() {
		h.focusPane = false
		h.notice = "No live interactive pane to focus; Alt+x menu or Alt+d docs"
		return h.syncFocus()
	}
	h.focusPane = !h.focusPane
	return h.syncFocus()
}

// wheelDelta is a vertical wheel notch in viewport lines, negative upward.
// Horizontal wheel events and other buttons do not scroll history.
func wheelDelta(message tea.MouseMsg) (int, bool) {
	if _, ok := message.(tea.MouseWheelMsg); !ok {
		return 0, false
	}
	switch message.Mouse().Button {
	case tea.MouseWheelUp:
		return -wheelScrollLines, true
	case tea.MouseWheelDown:
		return wheelScrollLines, true
	}
	return 0, false
}

// scrollShell moves the Bash viewport and repaints only when the snapshot
// would actually change, so wheel ticks at the ends of history stay free.
func (h *host) scrollShell(delta int) tea.Cmd {
	moved, err := h.terminal.Scroll(delta)
	if err != nil {
		return h.fail(err)
	}
	if !moved {
		return nil
	}
	if err := h.snapshot(); err != nil {
		return h.fail(err)
	}
	return nil
}

// wheelArrow is the key an outer terminal would send for a wheel notch on the
// alternate screen, where there is no scrollback to move.
func wheelArrow(delta int) tea.Key {
	if delta < 0 {
		return tea.Key{Code: uv.KeyUp}
	}
	return tea.Key{Code: uv.KeyDown}
}

// wheelShell leaves a full-screen child's paging to the child, preserving the
// arrow-key behaviour the outer terminal had before the host claimed the wheel.
func (h *host) wheelShell(delta int) tea.Cmd {
	if !h.frame.AltScreen {
		return h.scrollShell(delta)
	}
	data, err := h.terminal.Key(wheelArrow(delta))
	if err != nil {
		return h.forward(nil, err)
	}
	return h.forward(data, nil)
}

func (h *host) wheelPane(delta int) tea.Cmd {
	if !h.pane.frame.AltScreen {
		return h.scrollPane(delta)
	}
	data, err := h.pane.terminal.Key(wheelArrow(delta))
	if err != nil {
		return h.forwardPane(nil, err)
	}
	return h.forwardPane(data, nil)
}

func (h *host) scrollPane(delta int) tea.Cmd {
	moved, err := h.pane.terminal.Scroll(delta)
	if err != nil {
		return h.failPane(err)
	}
	if !moved {
		return nil
	}
	if err := h.pane.snapshot(); err != nil {
		return h.failPane(err)
	}
	return nil
}

// Typing returns the viewport to the live screen, the way a scrolled-back
// terminal snaps to the bottom on input.
func (h *host) followShell() tea.Cmd {
	moved, err := h.terminal.ScrollBottom()
	if err != nil {
		return h.forward(nil, err)
	}
	if moved {
		if err := h.snapshot(); err != nil {
			return h.fail(err)
		}
	}
	return nil
}

func (h *host) followPane() tea.Cmd {
	moved, err := h.pane.terminal.ScrollBottom()
	if err != nil {
		return h.forwardPane(nil, err)
	}
	if moved {
		if err := h.pane.snapshot(); err != nil {
			return h.failPane(err)
		}
	}
	return nil
}

func localMouse(message tea.MouseMsg, x, y int) tea.MouseMsg {
	mouse := message.Mouse()
	mouse.X, mouse.Y = x, y
	switch message.(type) {
	case tea.MouseClickMsg:
		return tea.MouseClickMsg(mouse)
	case tea.MouseReleaseMsg:
		return tea.MouseReleaseMsg(mouse)
	case tea.MouseWheelMsg:
		return tea.MouseWheelMsg(mouse)
	case tea.MouseMotionMsg:
		return tea.MouseMotionMsg(mouse)
	}
	return message
}

func (h *host) mouseInput(message tea.MouseMsg) tea.Cmd {
	mouse := message.Mouse()
	point, layout := image.Pt(mouse.X, mouse.Y), h.layout()
	_, clicked := message.(tea.MouseClickMsg)
	if point.In(layout.Panel) {
		if !point.In(layout.Body) || !h.paneInteractive() {
			return nil // borders, static/final images, and hidden panes are not input
		}
		if clicked && !h.focusPane {
			h.focusPane = true
			if cmd := h.syncFocus(); cmd != nil {
				return cmd
			}
			if !h.paneInteractive() {
				return nil
			}
		}
		if delta, wheel := wheelDelta(message); wheel && !h.pane.frame.MouseTracking {
			return h.wheelPane(delta)
		}
		return h.forwardPane(h.pane.terminal.Mouse(localMouse(message,
			mouse.X-layout.Body.Min.X, mouse.Y-layout.Body.Min.Y)))
	}
	if point.In(layout.Shell) {
		if clicked && h.focusPane {
			h.focusPane = false
			if cmd := h.syncFocus(); cmd != nil {
				return cmd
			}
		}

		if delta, wheel := wheelDelta(message); wheel && !h.frame.MouseTracking {
			return h.wheelShell(delta)
		}

		nativeY := mouse.Y - layout.Shell.Min.Y
		if nativeY >= h.frame.Rows {
			return nil
		}
		return h.forward(h.terminal.Mouse(localMouse(message,
			mouse.X-layout.Shell.Min.X, nativeY)))
	}
	return nil // footer/outside the workspace
}
