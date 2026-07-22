package menu

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/shared"
)

func TestListViewUsesViewportNotCustomScroll(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "one", Description: "first"},
		Task{Name: "two", Description: "second"},
		Task{Name: "three", Description: "third"},
		Task{Name: "four", Description: "fourth"},
		Task{Name: "five", Description: "fifth"},
	)
	st := newStyles(cfg)
	list := newListView(st, 40).WithSize(40)
	frame := Frame{st: st}.WithSize(40)
	flat := cfg.flatten()
	matches := []int{0, 1, 2, 3, 4}

	list = list.Sync(flat, matches, 4, false, 3, "", frame)
	if list.Height() != 3 {
		t.Fatalf("Height = %d, want configured 3", list.Height())
	}
	view := ansi.Strip(list.View())
	if !strings.Contains(view, "five") {
		t.Fatalf("selected row not kept visible:\n%s", view)
	}
	// Viewport should not expose the old helpers — content is windowed.
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("viewport window height = %d, want 3:\n%s", len(lines), view)
	}
}

func TestListViewMouseWheelUpdatesViewport(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "one", Description: "first"},
		Task{Name: "two", Description: "second"},
		Task{Name: "three", Description: "third"},
		Task{Name: "four", Description: "fourth"},
		Task{Name: "five", Description: "fifth"},
		Task{Name: "six", Description: "sixth"},
	)
	st := newStyles(cfg)
	list := newListView(st, 40).WithSize(40)
	frame := Frame{st: st}.WithSize(40)
	flat := cfg.flatten()
	matches := make([]int, len(flat))
	for i := range matches {
		matches[i] = i
	}
	list = list.Sync(flat, matches, 0, false, 3, "", frame)
	before := list.viewport.YOffset()

	list, _ = list.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	list, _ = list.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if list.viewport.YOffset() <= before {
		t.Fatalf("mouse wheel did not scroll viewport: before=%d after=%d", before, list.viewport.YOffset())
	}
}

func TestListViewEmptyFilterMessage(t *testing.T) {
	cfg := &Config{
		Project: "test",
		Height:  8,
		Palette: shared.Palette{Fg: "#fff", Muted: "#888", Accent: "#0f0", Accent2: "#ff0", Bg: "#000", Surface: "#111"},
	}
	st := newStyles(cfg)
	list := newListView(st, 40).WithSize(40)
	frame := Frame{st: st}.WithSize(40)
	list = list.Sync(nil, nil, 0, false, 6, "zzz", frame)
	if !strings.Contains(ansi.Strip(list.View()), "no commands match") {
		t.Fatalf("empty match message missing:\n%s", list.View())
	}
}
