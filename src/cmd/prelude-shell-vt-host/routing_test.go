package main

import (
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

// routedHost is a host with two real children, so input routing is asserted
// against what the programs actually received rather than against a spy.
type routedHost struct {
	*host
	shellChild *testShell
	paneChild  *testShell
}

// newRoutedHost builds a host with a live pane already attached. It bypasses
// reconcilePane so the pane's command is a plain shell that echoes what it is
// sent, which is what makes "who got this keystroke" answerable.
func newRoutedHost(t *testing.T) *routedHost {
	t.Helper()

	shellChild := startTestShell(t, 80, 6)
	paneChild := startTestShell(t, 80, 12)

	initial := newState(80, 24, 6)
	initial = pinTo(t, initial, pinDocs)

	h := newHost(shellChild.shell, initial, nil, "/bin/sh")
	h.pane = paneChild.shell
	h.syncChildFocus()

	return &routedHost{host: h, shellChild: shellChild, paneChild: paneChild}
}

func (r *routedHost) focusPane(t *testing.T) {
	t.Helper()
	r.host.Update(tea.KeyPressMsg(uv.KeyPressEvent{Code: 'o', Mod: uv.ModCtrl}))
	if r.host.state.focus != focusPane {
		t.Fatalf("focus = %q, want the pane", r.host.state.focus)
	}
}

func TestPasteFollowsTheKeyboard(t *testing.T) {
	// Paste is input. Sending it to the shell while a pane holds the keyboard
	// would drop a wall of text into the wrong program.
	routed := newRoutedHost(t)

	routed.host.Update(tea.PasteMsg{Content: "shell-paste"})
	routed.shellChild.await(t, "the shell to receive the paste", func(screen string) bool {
		return strings.Contains(screen, "shell-paste")
	})
	if strings.Contains(routed.paneChild.text(), "shell-paste") {
		t.Fatal("the pane received a paste meant for the shell")
	}

	routed.focusPane(t)
	routed.host.Update(tea.PasteMsg{Content: "pane-paste"})
	routed.paneChild.await(t, "the pane to receive the paste", func(screen string) bool {
		return strings.Contains(screen, "pane-paste")
	})
	if strings.Contains(routed.shellChild.text(), "pane-paste") {
		t.Fatal("the shell received a paste meant for the pane")
	}
}

func TestOnlyTheFocusedChildBelievesItIsFocused(t *testing.T) {
	// Terminal focus reports have to describe one active program. A pane that
	// thinks it is focused while the shell owns input blinks a cursor nobody
	// is driving, and programs gating on focus act on a lie.
	routed := newRoutedHost(t)

	if !routed.shellChild.view().focused {
		t.Fatal("the shell holds the keyboard and must be focused")
	}
	if routed.paneChild.view().focused {
		t.Fatal("an unfocused pane must not report focus")
	}

	routed.focusPane(t)
	if routed.shellChild.view().focused {
		t.Fatal("the shell lost the keyboard and must be blurred")
	}
	if !routed.paneChild.view().focused {
		t.Fatal("the pane holds the keyboard and must be focused")
	}
}

func TestBlurringTheWindowBlursBothChildren(t *testing.T) {
	routed := newRoutedHost(t)
	routed.focusPane(t)

	routed.host.Update(tea.BlurMsg{})
	if routed.shellChild.view().focused || routed.paneChild.view().focused {
		t.Fatal("no child may report focus while the terminal itself is blurred")
	}

	// Regaining window focus restores it to the keyboard holder only.
	routed.host.Update(tea.FocusMsg{})
	if routed.shellChild.view().focused {
		t.Fatal("the shell does not hold the keyboard and must stay blurred")
	}
	if !routed.paneChild.view().focused {
		t.Fatal("the pane holds the keyboard and must regain focus")
	}
}

func TestClosingAPaneReturnsFocusToTheShell(t *testing.T) {
	routed := newRoutedHost(t)
	routed.focusPane(t)

	routed.host.closePane()

	if !routed.shellChild.view().focused {
		t.Fatal("the shell must regain focus once the pane is gone")
	}
}

func TestMouseGoesToTheBandUnderThePointer(t *testing.T) {
	// Position is the targeting for a mouse, not the keyboard: a wheel over
	// the pane should scroll the pane even while the shell owns input.
	routed := newRoutedHost(t)
	body := routed.host.state.layout.panelBody()
	shellBand := routed.host.state.layout.shell

	if body.height == 0 || shellBand.height == 0 {
		t.Fatal("test setup: both bands need rows")
	}

	// Both children must be asking for the mouse, or their emulators drop it.
	routed.shellChild.run("printf '\\033[?1003h'")
	routed.paneChild.run("printf '\\033[?1003h'")
	routed.shellChild.await(t, "the shell to request mouse tracking", func(string) bool {
		return routed.shellChild.view().mouseTracking
	})
	routed.paneChild.await(t, "the pane to request mouse tracking", func(string) bool {
		return routed.paneChild.view().mouseTracking
	})

	before := routed.paneChild.text()
	routed.host.forwardMouse(tea.MouseClickMsg(uv.MouseClickEvent{
		X: 3, Y: body.top, Button: uv.MouseLeft,
	}))
	routed.paneChild.await(t, "the pane to receive the click", func(screen string) bool {
		return screen != before
	})
}

func TestMouseModeMirrorsEitherChild(t *testing.T) {
	// The outer terminal must grab the mouse when the pane wants it, even
	// though the shell does not — otherwise a pinned pane is unclickable.
	routed := newRoutedHost(t)

	if got := routed.host.View().MouseMode; got != tea.MouseModeNone {
		t.Fatalf("mouse mode = %v, want none while neither child is tracking", got)
	}

	routed.paneChild.run("printf '\\033[?1003h'")
	routed.paneChild.await(t, "the pane to request mouse tracking", func(string) bool {
		return routed.paneChild.view().mouseTracking
	})

	if got := routed.host.View().MouseMode; got != tea.MouseModeAllMotion {
		t.Fatalf("mouse mode = %v, want tracking once the pane asks", got)
	}
}

func TestScrollKeysBelongToWhicheverChildHasTheKeyboard(t *testing.T) {
	// Shell-focused they page the shell's scrollback; pane-focused they are
	// the pane's to interpret, so the shell viewport must not move.
	routed := newRoutedHost(t)
	routed.shellChild.run("for i in $(seq 1 40); do echo line-$i; done")
	routed.shellChild.await(t, "scrollback to accumulate", func(string) bool {
		return routed.shellChild.view().history > 5
	})

	pageUp := tea.KeyPressMsg(uv.KeyPressEvent{Code: uv.KeyPgUp, Mod: uv.ModShift})

	routed.host.Update(pageUp)
	if routed.host.state.scroll == 0 {
		t.Fatal("shift+pgup must scroll the shell while the shell has the keyboard")
	}

	routed.host.state = routed.host.state.followTail()
	routed.focusPane(t)
	routed.host.Update(pageUp)
	if routed.host.state.scroll != 0 {
		t.Fatal("shift+pgup must not move the shell viewport while a pane has the keyboard")
	}
}

// sentMessages collects what the host emits. Panes report from their own pump
// goroutines, so the slice needs a lock even though the test reads it later.
type sentMessages struct {
	mu       sync.Mutex
	messages []tea.Msg
}

func (s *sentMessages) add(message tea.Msg) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, message)
}

func (s *sentMessages) drain() []tea.Msg {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.messages
	s.messages = nil
	return out
}

// liveHost builds a host that starts real panes through reconcilePane, with
// every message the panes emit captured for replay.
func liveHost(t *testing.T, cols, rows int) (*host, *sentMessages) {
	t.Helper()
	shellChild := startTestShell(t, cols, 6)

	h := newHost(shellChild.shell, newState(cols, rows, 6), nil, "/bin/sh")
	t.Cleanup(h.closePane)

	sent := &sentMessages{}
	h.attach(func() {}, sent.add)
	return h, sent
}

func TestShrinkingPastThePaneAndGrowingBackKeepsTheDocsPin(t *testing.T) {
	// Retiring a pane because it no longer fits is the host's decision, not
	// the program quitting. Treating the resulting exit as a quit would unpin
	// Docs, and growing the terminal back would come up empty.
	h, sent := liveHost(t, 80, 24)

	h.adopt(h.state.setPin(pinDocs))
	if h.pane == nil {
		t.Fatal("test setup: the docs pane never started")
	}

	// Three rows leave a header-only pin: nowhere for a child to live.
	h.adopt(h.state.resize(80, 3))
	if h.pane != nil {
		t.Fatal("the pane must be retired when it no longer fits")
	}
	if h.state.pin != pinDocs {
		t.Fatalf("pin = %q, want docs: shrinking is not unpinning", h.state.pin.label())
	}

	// Replay whatever the retired pane emitted on its way out.
	for _, message := range sent.drain() {
		h.Update(message)
	}
	if h.state.pin != pinDocs {
		t.Fatalf("pin = %q, want docs: the retired pane's exit unpinned it", h.state.pin.label())
	}

	h.adopt(h.state.resize(80, 24))
	if h.pane == nil {
		t.Fatal("the pane must come back once there is room for it again")
	}
}

func TestAStalePaneExitCannotCloseANewerPane(t *testing.T) {
	// The message from a pane the host already retired must not tear down the
	// one that replaced it.
	h, sent := liveHost(t, 80, 24)

	h.adopt(h.state.setPin(pinDocs))
	first := h.pane
	if first == nil {
		t.Fatal("test setup: the first pane never started")
	}

	// Retire it and open a fresh one in its place.
	h.closePane()
	h.reconcilePane()
	second := h.pane
	if second == nil {
		t.Fatal("test setup: the replacement pane never started")
	}
	if second == first {
		t.Fatal("test setup: the pane was not actually replaced")
	}

	// Everything the first pane emitted arrives late.
	for _, message := range sent.drain() {
		h.Update(message)
	}
	// And explicitly, in case its pump had not reported yet.
	h.Update(paneGoneMsg{child: first})

	if h.pane != second {
		t.Fatal("a stale pane exit closed the pane that replaced it")
	}
	if h.state.pin != pinDocs {
		t.Fatalf("pin = %q, want docs", h.state.pin.label())
	}
}

func TestAPaneThatQuitsOnItsOwnStillUnpins(t *testing.T) {
	// The identity check must not make genuine exits inert.
	h, _ := liveHost(t, 80, 24)

	h.adopt(h.state.setPin(pinDocs))
	child := h.pane
	if child == nil {
		t.Fatal("test setup: the docs pane never started")
	}

	h.Update(paneGoneMsg{child: child})

	if h.pane != nil {
		t.Fatal("a pane whose program quit must be closed")
	}
	if h.state.pin != pinOff {
		t.Fatalf("pin = %q, want off once the pane's program quit", h.state.pin.label())
	}
}
