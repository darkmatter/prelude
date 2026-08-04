package main

// Host state machine. Pure: no PTY, no subprocess, no terminal. Every
// interesting rule of the host — when a panel is reloaded, which late results
// are discarded, how far back the viewport may scroll — is decided here.

type panelPhase uint8

const (
	panelOff panelPhase = iota
	panelLoading
	panelReady
	panelFailed
)

func (p panelPhase) String() string {
	switch p {
	case panelOff:
		return "off"
	case panelLoading:
		return "loading"
	case panelReady:
		return "ready"
	case panelFailed:
		return "failed"
	}
	return "?"
}

// panelRequest names one attempt to capture a pinned surface. The generation
// counter is what lets a capture that finishes after a resize be dropped
// instead of painted at the wrong size.
type panelRequest struct {
	generation uint64
	mode       pinMode
	cols       int
	rows       int
}

// sameShape reports whether two requests would produce an interchangeable
// surface. Generation is deliberately excluded: it identifies the attempt, not
// the result.
func (r panelRequest) sameShape(other panelRequest) bool {
	return r.mode == other.mode && r.cols == other.cols && r.rows == other.rows
}

type panelResult struct {
	generation uint64
	surface    *surface
	err        string
}

// focusTarget names which child receives keystrokes. Only one can, and the
// shell is the default: a pinned pane has to be asked for explicitly, because
// stealing input while cycling past a pin would be hostile.
type focusTarget uint8

const (
	focusShell focusTarget = iota
	focusPane
)

func (f focusTarget) String() string {
	if f == focusPane {
		return "pane"
	}
	return "shell"
}

type state struct {
	cols          int
	rows          int
	wantShellRows int
	pin           pinMode
	layout        layout

	generation uint64
	phase      panelPhase
	panel      *surface
	panelErr   string
	// inFlight records the shape of the newest request so a resize that leaves
	// the pin band unchanged does not respawn the capture command.
	inFlight panelRequest

	// focus is which child keystrokes reach. Held to the invariant that only
	// a live pane with rows on screen can take it.
	focus focusTarget

	// scroll is how many lines above the live screen the shell viewport is
	// pinned. Zero follows the child.
	scroll int

	shellErr string
}

func newState(cols, rows, wantShellRows int) state {
	result := state{
		cols:          max(cols, minFrameCols),
		rows:          max(rows, minFrameRows),
		wantShellRows: max(wantShellRows, 1),
		phase:         panelOff,
	}
	result.layout = computeLayout(result.cols, result.rows, result.wantShellRows, result.pin)
	return result
}

// paneAvailable reports whether a live pane currently has somewhere to live.
// A header-only pin band has no rows for a child, so there is no pane even
// though the mode is live.
func (s state) paneAvailable() bool {
	return s.pin.live() && s.layout.panelBody().height > 0
}

// clampFocus enforces the focus invariant. Unpinning, cycling to a captured
// mode, or shrinking the terminal until the pane no longer fits all hand
// input back to the shell rather than routing it at a pane that is gone.
func (s state) clampFocus() state {
	if !s.paneAvailable() {
		s.focus = focusShell
	}
	return s
}

// toggleFocus moves input between the shell and a live pane. With no pane on
// screen there is nowhere else for keys to go.
func (s state) toggleFocus() state {
	if !s.paneAvailable() {
		return s.clampFocus()
	}
	if s.focus == focusShell {
		s.focus = focusPane
	} else {
		s.focus = focusShell
	}
	return s
}

// resize adopts a new frame size. It returns a capture request only when the
// pin band actually changed shape, so dragging a window edge does not spawn a
// process per pixel column.
func (s state) resize(cols, rows int) (state, panelRequest, bool) {
	s.cols = max(cols, minFrameCols)
	s.rows = max(rows, minFrameRows)
	return s.relayout()
}

func (s state) cyclePin() (state, panelRequest, bool) {
	return s.setPin(s.pin.next())
}

// setPin selects a mode directly, for callers that know which one they want
// rather than stepping through the cycle.
func (s state) setPin(mode pinMode) (state, panelRequest, bool) {
	s.pin = mode
	return s.relayout()
}

func (s state) relayout() (state, panelRequest, bool) {
	s.layout = computeLayout(s.cols, s.rows, s.wantShellRows, s.pin)
	next, request, spawn := s.planPanel()
	return next.clampFocus(), request, spawn
}

// planPanel decides what the pin band needs after a layout change. It only
// speaks for captured surfaces; a live pane is a running child, which is the
// host's business, not the state machine's.
func (s state) planPanel() (state, panelRequest, bool) {
	if !s.pin.on() {
		return s.clearPanel(panelRequest{}), panelRequest{}, false
	}

	// A live mode draws itself straight out of its own child, so there is no
	// capture to request and no captured image worth keeping.
	if s.pin.live() {
		return s.clearPanel(panelRequest{}), panelRequest{}, false
	}

	want := panelRequest{
		mode: s.pin,
		cols: s.layout.cols,
		rows: s.layout.panelBody().height,
	}
	if want.rows <= 0 {
		// Header-only pin: there is nowhere to put a capture, and a zero-row
		// PTY is not something to hand a child process.
		return s.clearPanel(want), panelRequest{}, false
	}
	if s.inFlight.sameShape(want) {
		return s, panelRequest{}, false
	}

	s.generation++
	want.generation = s.generation
	s.inFlight = want
	s.phase = panelLoading
	s.panel = nil
	s.panelErr = ""
	return s, want, true
}

func (s state) clearPanel(inFlight panelRequest) state {
	s.phase = panelOff
	s.panel = nil
	s.panelErr = ""
	s.inFlight = inFlight
	return s
}

// applyPanel adopts a capture result, ignoring anything the host has already
// moved past.
func (s state) applyPanel(result panelResult) state {
	if result.generation == 0 || result.generation != s.generation {
		return s
	}
	s.panel = result.surface
	s.panelErr = result.err
	if result.err == "" {
		s.phase = panelReady
	} else {
		s.phase = panelFailed
	}
	return s
}

// scrollBy moves the shell viewport back through history. limit is the number
// of scrollback lines currently retained, which shrinks as old lines are
// evicted, so the offset is re-clamped on every move.
func (s state) scrollBy(delta, limit int) state {
	s.scroll = clamp(s.scroll+delta, 0, max(limit, 0))
	return s
}

func (s state) followTail() state {
	s.scroll = 0
	return s
}
