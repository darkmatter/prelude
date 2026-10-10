package main

import (
	"fmt"
	"image"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (h *host) layout() workspaceLayout {
	return h.completionLayout(computeLayout(h.cols, h.rows, h.placement, h.paneVisible && h.surface != surfaceNone))
}

func (h *host) setPlacement(where placement) tea.Cmd {
	h.dismissCompletion()
	h.placement = where
	h.notice = "Placement: " + where.String()
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
	h.dismissCompletion()
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
	h.dismissCompletion()
	p := h.pane
	h.pane, h.surface, h.paneError = nil, surfaceNone, nil
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
	if err := h.closeCompletion(); err != nil {
		return h.fail(err)
	}
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
			}
		}
		// Legacy Ctrl+[ arrives as Esc, so only a distinct enhanced key can
		// cycle backward. Ctrl+\ is unambiguous on legacy terminals too.
		switch message.String() {

		case "ctrl+]":
			h.leader = false
			return h.setPlacement(h.placement.next())
		case "ctrl+[", "ctrl+\\":
			h.leader = false
			return h.setPlacement(h.placement.prev())
		}
		if message.String() == "ctrl+g" {
			h.leader = false // Child-owned even while command mode was unlocked.
		}
		if h.leader {
			h.leader = false
			switch message.String() {
			case "ctrl+p":
				return nil
			case "esc":
				return nil
			case "tab":
				if !h.paneInteractive() {
					h.focusPane = false
					h.notice = "No live interactive pane to focus; Alt+x menu or Alt+d docs"
					return h.syncFocus()
				}
				h.focusPane = !h.focusPane
				return h.syncFocus()
			}
			switch chordText(message.Key()) {
			case "m":
				return h.showMotd()
			case "x":
				return h.chooseSurface(surfaceMenu)
			case "d":
				return h.chooseSurface(surfaceDocs)

			case "t":
				return h.togglePane()
			case "v":
				return h.setPlacement(h.placement.next())
			case "c":
				return h.closePane()
			}
			h.notice = fmt.Sprintf("Unknown Ctrl+P chord %q; %s", message.String(), leaderHints)
			return nil
		}
		switch message.String() {
		case "ctrl+p":
			if err := h.closeCompletion(); err != nil {
				return h.fail(err)
			}
			h.leader, h.notice = true, ""
			return nil

		case "ctrl+v":
			h.quoted = true
		}
	}
	h.notice = ""
	if !quoted && !h.paneFocused() {
		if handled, cmd := h.completionKey(message); handled {
			return cmd
		}
	} else if err := h.closeCompletion(); err != nil {
		return h.fail(err)
	}
	// Ordinary keys and Ctrl+G belong to the focused child outside command mode.
	if h.paneFocused() {
		return h.forwardPane(h.pane.terminal.Key(message.Key()))
	}
	h.completionWanted = !quoted && message.String() == "tab"
	data, err := h.terminal.Key(message.Key())
	if err == nil {
		h.noteShellInput(data, quoted)
	}
	return h.forward(data, err)
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
	if point.In(h.completionArea(layout)) {
		return nil // This spike's completion chooser is keyboard-only.
	}
	_, clicked := message.(tea.MouseClickMsg)
	if point.In(layout.Panel) {
		if !point.In(layout.Body) || !h.paneInteractive() {
			return nil // borders, static/final images, and hidden panes are not input
		}
		if clicked && !h.focusPane {
			if err := h.closeCompletion(); err != nil {
				return h.fail(err)
			}
			layout = h.layout()
			h.focusPane = true
			if cmd := h.syncFocus(); cmd != nil {
				return cmd
			}
			if !h.paneInteractive() {
				return nil
			}
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

		nativeY := mouse.Y - layout.Shell.Min.Y + layout.ShellScroll
		if nativeY >= h.frame.Rows {
			return nil
		}
		return h.forward(h.terminal.Mouse(localMouse(message,
			mouse.X-layout.Shell.Min.X, nativeY)))
	}
	return nil // footer/outside the workspace
}
