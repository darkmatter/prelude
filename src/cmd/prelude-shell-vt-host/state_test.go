package main

import "testing"

func TestCyclingThePinRequestsACapture(t *testing.T) {
	next, request, spawn := newState(80, 24, 10).cyclePin()

	if !spawn {
		t.Fatal("cycling into a pinned mode must request a capture")
	}
	if request.mode != pinMotd {
		t.Fatalf("request mode = %q, want motd", request.mode.label())
	}
	if request.generation != next.generation {
		t.Fatalf("request generation %d does not match state %d", request.generation, next.generation)
	}
	if next.phase != panelLoading {
		t.Fatalf("phase = %q, want loading", next.phase)
	}
	if want := next.layout.panelBody().height; request.rows != want {
		t.Fatalf("request rows = %d, want the panel body height %d", request.rows, want)
	}
}

func TestTurningThePinOffClearsThePanelWithoutACapture(t *testing.T) {
	state, _, _ := newState(80, 24, 10).cyclePin() // motd
	state = state.applyPanel(panelResult{generation: state.generation, surface: newSurface(4, 1, pinCell)})

	// motd -> x -> docs -> off; the last step is the one under test.
	state, _, _ = state.cyclePin()
	state, _, _ = state.cyclePin()
	state, _, spawn := state.cyclePin()

	if state.pin != pinOff {
		t.Fatalf("pin = %q, want off", state.pin.label())
	}
	if spawn {
		t.Fatal("turning the pin off must not spawn a capture")
	}
	if state.panel != nil {
		t.Fatal("panel surface must be released when the pin turns off")
	}
	if state.phase != panelOff {
		t.Fatalf("phase = %q, want off", state.phase)
	}
}

func TestResizeThatLeavesThePinBandUnchangedDoesNotRespawn(t *testing.T) {
	// Only the width below the pin changes here... nothing does, in fact: the
	// same size arrives twice, which is what a spurious SIGWINCH looks like.
	state, _, _ := newState(80, 24, 10).cyclePin()
	generationBefore := state.generation

	state, _, spawn := state.resize(80, 24)

	if spawn {
		t.Fatal("an identical resize must not respawn the capture command")
	}
	if state.generation != generationBefore {
		t.Fatalf("generation moved from %d to %d on a no-op resize", generationBefore, state.generation)
	}
}

func TestResizeThatChangesThePinBandRespawns(t *testing.T) {
	state, _, _ := newState(80, 24, 10).cyclePin()
	generationBefore := state.generation

	state, request, spawn := state.resize(100, 30)

	if !spawn {
		t.Fatal("a resize that changes the pin band must respawn the capture")
	}
	if state.generation <= generationBefore {
		t.Fatalf("generation %d did not advance past %d", state.generation, generationBefore)
	}
	if request.cols != 100 {
		t.Fatalf("request cols = %d, want 100", request.cols)
	}
}

func TestStaleCaptureResultsAreDiscarded(t *testing.T) {
	state, stale, _ := newState(80, 24, 10).cyclePin()
	state, _, _ = state.resize(100, 30) // supersedes the in-flight request

	late := state.applyPanel(panelResult{
		generation: stale.generation,
		surface:    newSurface(80, 12, pinCell),
	})

	if late.panel != nil {
		t.Fatal("a result from a superseded generation must not be painted")
	}
	if late.phase != panelLoading {
		t.Fatalf("phase = %q, want the newer capture to stay loading", late.phase)
	}
}

func TestCaptureFailureKeepsAnyPartialSurface(t *testing.T) {
	state, request, _ := newState(80, 24, 10).cyclePin()
	partial := newSurface(80, 12, pinCell)

	state = state.applyPanel(panelResult{
		generation: request.generation,
		surface:    partial,
		err:        "motd: exit status 1",
	})

	if state.phase != panelFailed {
		t.Fatalf("phase = %q, want failed", state.phase)
	}
	if state.panel != partial {
		t.Fatal("a partial image must survive so the operator sees real output")
	}
	if state.panelErr == "" {
		t.Fatal("the failure text must be retained for the status row")
	}
}

func TestZeroGenerationResultsAreIgnored(t *testing.T) {
	// The zero value of panelResult must never be mistaken for a real capture.
	state, _, _ := newState(80, 24, 10).cyclePin()
	if got := state.applyPanel(panelResult{}); got.phase != panelLoading {
		t.Fatalf("phase = %q, want the zero-value result to be ignored", got.phase)
	}
}

func TestShortFrameSkipsTheCaptureEntirely(t *testing.T) {
	// Three rows leave a header-only pin: no body, so no child process.
	state, _, spawn := newState(80, 3, 10).cyclePin()

	if spawn {
		t.Fatal("a pin with no body rows must not spawn a capture")
	}
	if state.phase != panelOff {
		t.Fatalf("phase = %q, want off", state.phase)
	}
}

func TestScrollIsClampedToRetainedHistory(t *testing.T) {
	state := newState(80, 24, 10)

	state = state.scrollBy(50, 12)
	if state.scroll != 12 {
		t.Fatalf("scroll = %d, want it clamped to the 12 retained lines", state.scroll)
	}

	// History shrinks as old lines are evicted; the offset must follow.
	state = state.scrollBy(0, 4)
	if state.scroll != 4 {
		t.Fatalf("scroll = %d, want re-clamping to 4", state.scroll)
	}

	state = state.scrollBy(-100, 4)
	if state.scroll != 0 {
		t.Fatalf("scroll = %d, want 0", state.scroll)
	}
}

func TestFollowTailReturnsToTheLiveScreen(t *testing.T) {
	state := newState(80, 24, 10).scrollBy(5, 20)
	if got := state.followTail().scroll; got != 0 {
		t.Fatalf("scroll = %d, want 0 after following the tail", got)
	}
}

// pinTo cycles until the requested mode is reached, so tests name the mode
// they mean instead of counting ^G presses.
func pinTo(t *testing.T, start state, mode pinMode) state {
	t.Helper()
	current := start
	for range len(pinLabels) + 1 {
		if current.pin == mode {
			return current
		}
		current, _, _ = current.cyclePin()
	}
	t.Fatalf("never reached pin mode %q", mode.label())
	return current
}

func TestLiveModesNeverRequestACapture(t *testing.T) {
	// Docs is a running child in the band, not a photograph, so the capture
	// machinery must stay out of its way entirely.
	docs := pinTo(t, newState(80, 24, 10), pinDocs)

	if docs.phase != panelOff {
		t.Fatalf("phase = %q, want off for a live mode", docs.phase)
	}
	if docs.panel != nil {
		t.Fatal("a live mode must not hold a captured surface")
	}

	// Resizing must not start one either.
	if _, _, spawn := docs.resize(100, 30); spawn {
		t.Fatal("resizing a live pane must not spawn a capture")
	}
}

func TestSwitchingFromACaptureToALiveModeDropsTheImage(t *testing.T) {
	captured := pinTo(t, newState(80, 24, 10), pinMenu)
	captured = captured.applyPanel(panelResult{
		generation: captured.inFlight.generation,
		surface:    newSurface(80, 12, pinCell),
	})
	if captured.panel == nil {
		t.Fatal("test setup: the capture was not adopted")
	}

	docs := pinTo(t, captured, pinDocs)
	if docs.panel != nil {
		t.Fatal("the previous mode's image must not linger under a live pane")
	}
}

func TestFocusStartsOnTheShell(t *testing.T) {
	if got := newState(80, 24, 10).focus; got != focusShell {
		t.Fatalf("focus = %q, want the shell", got)
	}
}

func TestFocusMovesToALivePaneOnlyWhenAsked(t *testing.T) {
	// Cycling past docs must not steal input; you pass through it on the way
	// to off, and having the shell go deaf mid-cycle would be hostile.
	docs := pinTo(t, newState(80, 24, 10), pinDocs)
	if docs.focus != focusShell {
		t.Fatalf("focus = %q, want the shell until focus is requested", docs.focus)
	}

	focused := docs.toggleFocus()
	if focused.focus != focusPane {
		t.Fatalf("focus = %q, want the pane after toggling", focused.focus)
	}
	if back := focused.toggleFocus(); back.focus != focusShell {
		t.Fatalf("focus = %q, want the shell after toggling back", back.focus)
	}
}

func TestFocusCannotLandOnAModeWithNoPane(t *testing.T) {
	captured := pinTo(t, newState(80, 24, 10), pinMotd)
	if got := captured.toggleFocus().focus; got != focusShell {
		t.Fatalf("focus = %q, want the shell: a captured surface takes no input", got)
	}

	off := newState(80, 24, 10)
	if got := off.toggleFocus().focus; got != focusShell {
		t.Fatalf("focus = %q, want the shell with no pin at all", got)
	}
}

func TestUnpinningReturnsFocusToTheShell(t *testing.T) {
	focused := pinTo(t, newState(80, 24, 10), pinDocs).toggleFocus()
	if focused.focus != focusPane {
		t.Fatal("test setup: focus never reached the pane")
	}

	off, _, _ := focused.cyclePin() // docs -> off
	if off.pin != pinOff {
		t.Fatalf("pin = %q, want off", off.pin.label())
	}
	if off.focus != focusShell {
		t.Fatalf("focus = %q, want the shell once the pane is gone", off.focus)
	}
}

func TestShrinkingPastThePaneReturnsFocusToTheShell(t *testing.T) {
	// A frame with room for only a header row has nowhere to run a child, so
	// keystrokes must stop being routed at one.
	focused := pinTo(t, newState(80, 24, 10), pinDocs).toggleFocus()
	if focused.focus != focusPane {
		t.Fatal("test setup: focus never reached the pane")
	}

	tiny, _, _ := focused.resize(80, 3)
	if tiny.paneAvailable() {
		t.Fatal("test setup: a 3-row frame should leave no pane rows")
	}
	if tiny.focus != focusShell {
		t.Fatalf("focus = %q, want the shell once the pane no longer fits", tiny.focus)
	}
}
