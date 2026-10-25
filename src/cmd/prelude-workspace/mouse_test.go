package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// The outer terminal owns the mouse until a visible child asks for tracking;
// then the view mirrors the child's mode (1000/1002 as cell motion, 1003 as
// all motion) so clicks can still reach and focus that child.
func TestViewMirrorsVisibleChildMouseMode(t *testing.T) {
	h := &host{cols: 40, rows: 10}
	h.frame = spikeFrame(40, 9)
	if got := h.View().MouseMode; got != tea.MouseModeNone {
		t.Fatalf("mouse mode = %v, want none with no tracking child", got)
	}
	for _, mode := range []int{1000, 1002} {
		h.frame.MouseMode = mode
		if got := h.View().MouseMode; got != tea.MouseModeCellMotion {
			t.Fatalf("child mode %d: mouse mode = %v, want cell motion", mode, got)
		}
	}
	h.frame.MouseMode = 1003
	if got := h.View().MouseMode; got != tea.MouseModeAllMotion {
		t.Fatalf("child mode 1003: mouse mode = %v, want all motion", got)
	}
	h.frame.MouseMode = 0
	if got := h.View().MouseMode; got != tea.MouseModeNone {
		t.Fatalf("mouse mode = %v, want none after the child disables tracking", got)
	}
}
