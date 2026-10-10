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
	st := newStyles(cfg, false)
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
	st := newStyles(cfg, false)
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

func TestListViewWrapsDescriptions(t *testing.T) {
	for _, selected := range []int{0, 1} {
		t.Run([]string{"selected", "unselected"}[selected], func(t *testing.T) {
			cfg := testMenuConfig(
				Task{Name: "run", Key: "r", Description: "alpha beta gamma delta epsilon"},
				Task{Name: "next", Description: "next command"},
			)
			st := newStyles(cfg, false)
			list := newListView(st, 32)
			frame := Frame{st: st}.WithSize(32)
			list.Sync(cfg.flatten(), []int{0, 1}, selected, false, 8, "", frame)

			view := ansi.Strip(list.View())
			lines := strings.Split(view, "\n")
			for i, line := range lines {
				if !strings.Contains(line, "alpha") {
					continue
				}
				if !strings.Contains(line, "alpha beta gamma") {
					t.Fatalf("description should wrap between words:\n%s", view)
				}
				if i+1 >= len(lines) || !strings.HasPrefix(lines[i+1], "│"+strings.Repeat(" ", 10)+"delta epsilon") {
					t.Fatalf("continuation should align with the description column:\n%s", view)
				}
				if !strings.HasSuffix(lines[i+1], "  │") || ansi.StringWidth(lines[i+1]) != 34 {
					t.Fatalf("continuation should keep padding and frame rails:\n%s", view)
				}
				if strings.Count(view, "│r│") != 1 {
					t.Fatalf("hotkey should only appear on the first row:\n%s", view)
				}
				return
			}
			t.Fatalf("description missing:\n%s", view)
		})
	}
}

func TestListViewKeepsWrappedSelectionVisible(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "one", Description: "first command"},
		Task{Name: "run", Key: "r", Description: "alpha beta gamma delta epsilon"},
	)
	st := newStyles(cfg, false)
	list := newListView(st, 32)
	frame := Frame{st: st}.WithSize(32)
	list.Sync(cfg.flatten(), []int{0, 1}, 1, false, 3, "", frame)

	view := ansi.Strip(list.View())
	if !strings.Contains(view, "❯ run") || !strings.Contains(view, "delta epsilon") {
		t.Fatalf("the selected command and its continuation should stay visible:\n%s", view)
	}
}

func TestListViewWrapsLongDetailWords(t *testing.T) {
	cfg := testMenuConfig(Task{Name: "run", Details: strings.Repeat("x", 52)})
	st := newStyles(cfg, false)
	list := newListView(st, 24)
	frame := Frame{st: st}.WithSize(24)
	list.Sync(cfg.flatten(), []int{0}, 0, true, 8, "", frame)

	view := ansi.Strip(list.View())
	var description strings.Builder
	for _, line := range strings.Split(view, "\n") {
		if !strings.Contains(line, "x") {
			continue
		}
		if !strings.HasPrefix(line, "│ x") || !strings.HasSuffix(line, "  │") || ansi.StringWidth(line) != 26 {
			t.Fatalf("wrapped details should retain padding and frame rails:\n%s", view)
		}
		description.WriteString(strings.Trim(line, "│ "))
	}
	if description.String() != cfg.Groups[0].Tasks[0].Details {
		t.Fatalf("wrapped details lost text:\n%s", view)
	}
}

func TestListViewEmptyFilterMessage(t *testing.T) {
	cfg := &Config{
		Project: "test",
		Height:  8,
		Palette: shared.Palette{Fg: "#fff", Muted: "#888", Accent: "#0f0", Accent2: "#ff0", Bg: "#000", Surface: "#111"},
	}
	st := newStyles(cfg, false)
	list := newListView(st, 40).WithSize(40)
	frame := Frame{st: st}.WithSize(40)
	list = list.Sync(nil, nil, 0, false, 6, "zzz", frame)
	if !strings.Contains(ansi.Strip(list.View()), "no commands match") {
		t.Fatalf("empty match message missing:\n%s", list.View())
	}
}
