package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// The outer terminal only forwards wheel ticks when the view claims the mouse.
// Without this, Ghostty translates them into Up/Down, which Bash reads as
// history navigation.
func TestViewClaimsTheWheelWithoutChildTracking(t *testing.T) {
	h := &host{cols: 40, rows: 10}
	h.frame = spikeFrame(40, 9)
	if got := h.View().MouseMode; got != tea.MouseModeCellMotion {
		t.Fatalf("mouse mode = %v, want cell motion so the host receives the wheel", got)
	}
	h.frame.MouseTracking = true
	if got := h.View().MouseMode; got != tea.MouseModeAllMotion {
		t.Fatalf("mouse mode = %v, want all motion for a tracking child", got)
	}
}

// A wheel over the shell band must move the viewport through retained history,
// not forward an event the child never asked for.
func TestWheelScrollsShellHistory(t *testing.T) {
	h := &host{cols: 40, rows: 10}
	layout := h.layout()
	term, err := newTerminal(layout.Shell.Dx(), layout.Shell.Dy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(term.Close)
	h.terminal = term

	ghosttyWrite(t, term, "\x1b[2J\x1b[H")
	for i := 1; i <= 20; i++ {
		ghosttyWrite(t, term, fmt.Sprintf("line-%02d\r\n", i))
	}
	if err := h.snapshot(); err != nil {
		t.Fatal(err)
	}
	if live := spikeFrameText(h.frame); strings.Contains(live, "line-01") {
		t.Fatalf("setup did not push line-01 into history: %q", live)
	}

	wheel := tea.MouseWheelMsg{X: 2, Y: layout.Shell.Min.Y + 1, Button: tea.MouseWheelUp}
	for range 10 {
		if cmd := h.mouseInput(wheel); cmd != nil {
			t.Fatalf("wheel returned a command instead of scrolling: %#v", cmd())
		}
	}
	if got := spikeFrameText(h.frame); !strings.Contains(got, "line-01") {
		t.Fatalf("wheel did not expose retained history: %q", got)
	}

	wheel.Button = tea.MouseWheelDown
	for range 10 {
		if cmd := h.mouseInput(wheel); cmd != nil {
			t.Fatalf("wheel down returned a command instead of scrolling: %#v", cmd())
		}
	}
	if got := spikeFrameText(h.frame); strings.Contains(got, "line-01") {
		t.Fatalf("wheel down did not return to the live screen: %q", got)
	}
}

// Full-screen children without mouse tracking have no scrollback here. The
// wheel must still reach them as the arrow key the outer terminal would send,
// or `less`, `man`, and the menu pane would stop responding to it.
func TestWheelOnAlternateScreenForwardsArrowKeys(t *testing.T) {
	h := &host{cols: 40, rows: 10}
	layout := h.layout()
	term, err := newTerminal(layout.Shell.Dx(), layout.Shell.Dy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(term.Close)
	h.terminal = term
	ghosttyWrite(t, term, "\x1b[?1049h\x1b[Hfull-screen")
	if err := h.snapshot(); err != nil {
		t.Fatal(err)
	}
	if !h.frame.AltScreen || h.frame.MouseTracking || h.frame.Scrolled {
		t.Fatalf("frame = %+v, want an unscrolled alternate screen without tracking", h.frame)
	}

	child, err := startPTY("cat", nil, layout.Shell.Dx(), layout.Shell.Dy(), os.Environ(), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.close() })
	h.child = child

	wheel := tea.MouseWheelMsg{X: 2, Y: layout.Shell.Min.Y, Button: tea.MouseWheelUp}
	if cmd := h.mouseInput(wheel); cmd != nil {
		t.Fatalf("wheel returned a command instead of forwarding: %#v", cmd())
	}
	select {
	case message := <-child.output:
		// The child's PTY echoes ESC as the visible ^[ notation.
		if got := string(message.data); got != "\x1b[A" && got != "^[[A" {
			t.Fatalf("child received %q, want the Up key", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("alternate-screen child never received the wheel as a key")
	}
}
