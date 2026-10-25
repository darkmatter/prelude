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

func TestListViewTruncatesDescriptionsByDefault(t *testing.T) {
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
			var row string
			for _, line := range strings.Split(view, "\n") {
				if strings.Contains(line, "alpha") {
					row = line
				}
				if strings.Contains(line, "delta") || strings.Contains(line, "epsilon") {
					t.Fatalf("collapsed rows must not wrap the description:\n%s", view)
				}
			}
			if row == "" || !strings.Contains(row, "…") || !strings.Contains(row, "alpha beta") {
				t.Fatalf("overflowing description should be clipped with an ellipsis:\n%s", view)
			}
			if ansi.StringWidth(row) != 34 {
				t.Fatalf("clipped row width = %d, want 34:\n%s", ansi.StringWidth(row), view)
			}
		})
	}
}

func TestListViewFolderCueIncludesRunnableParents(t *testing.T) {
	cfg := testMenuConfig(
		Task{
			Name:        "demos",
			Run:         "nix run .#examples",
			Description: "tour every feature demo",
			Children:    []Task{{Name: "defaults"}},
		},
		Task{Name: "build", Description: "build an output"},
	)
	st := newStyles(cfg, false)
	list := newListView(st, 60).WithSize(60)
	frame := Frame{st: st}.WithSize(60)
	list = list.Sync(cfg.flatten(), []int{0, 1}, 0, false, 8, "", frame)
	view := ansi.Strip(list.View())

	if !strings.Contains(view, "demos/") {
		t.Fatalf("runnable parent lost its folder cue:\n%s", view)
	}
	if strings.Contains(view, "build/") {
		t.Fatalf("leaf command gained a folder cue:\n%s", view)
	}
}

func TestListViewCollapsedDescriptionFlattensControlWhitespace(t *testing.T) {
	cfg := testMenuConfig(Task{Name: "run", Description: "alpha\tbeta\r\ngamma"})
	st := newStyles(cfg, false)
	list := newListView(st, 40)
	frame := Frame{st: st}.WithSize(40)
	list.Sync(cfg.flatten(), []int{0}, 0, false, 8, "", frame)

	view := ansi.Strip(list.View())
	for _, line := range strings.Split(view, "\n") {
		if !strings.Contains(line, "alpha") {
			continue
		}
		if strings.ContainsAny(line, "\r\t") {
			t.Fatalf("collapsed row kept control whitespace: %q", line)
		}
		if !strings.Contains(line, "alpha beta") || !strings.Contains(line, "gamma") {
			t.Fatalf("description words should survive flattening:\n%s", view)
		}
		return
	}
	t.Fatalf("description missing:\n%s", view)
}

func TestListViewExpandsFullDescription(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "run", Key: "r", Description: "alpha beta gamma delta epsilon"},
		Task{Name: "next", Description: "next command"},
	)
	st := newStyles(cfg, false)
	list := newListView(st, 32)
	frame := Frame{st: st}.WithSize(32)
	list.Sync(cfg.flatten(), []int{0, 1}, 0, true, 8, "", frame)

	view := ansi.Strip(list.View())
	lines := strings.Split(view, "\n")
	if len(lines) != 8 {
		t.Fatalf("expanded view height = %d, want the configured 8:\n%s", len(lines), view)
	}
	if !strings.Contains(lines[0], "▼ run") || strings.Contains(lines[0], "…") {
		t.Fatalf("expanded header should be a clean downward-caret row:\n%s", view)
	}
	if strings.Contains(lines[0], "alpha") || strings.Contains(lines[0], "❯") || strings.Contains(lines[0], "│r│") {
		t.Fatalf("expanded header must drop the description, selection caret, and keycap:\n%s", view)
	}
	if !strings.HasPrefix(lines[2], "│ alpha") || !strings.Contains(lines[2], "alpha beta gamma") {
		t.Fatalf("full description should start in the content area:\n%s", view)
	}
	body := strings.Join(lines[1:], " ")
	if !strings.Contains(body, "delta") || !strings.Contains(body, "epsilon") {
		t.Fatalf("full description should wrap inside the content area:\n%s", view)
	}
	if strings.Count(view, "▼") != 1 {
		t.Fatalf("expanded view should show one downward caret:\n%s", view)
	}
	if strings.Contains(view, "next command") {
		t.Fatalf("expanded view should hide sibling commands:\n%s", view)
	}
}

func TestListViewExpandedViewHidesSiblings(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "one", Description: "first command"},
		Task{Name: "run", Key: "r", Description: "alpha beta gamma delta epsilon"},
	)
	st := newStyles(cfg, false)
	list := newListView(st, 32)
	frame := Frame{st: st}.WithSize(32)
	list.Sync(cfg.flatten(), []int{0, 1}, 1, true, 8, "", frame)

	view := ansi.Strip(list.View())
	if !strings.Contains(view, "▼ run") || !strings.Contains(view, "delta") || !strings.Contains(view, "epsilon") {
		t.Fatalf("the expanded header and full description should stay visible:\n%s", view)
	}
	if strings.Contains(view, "first command") {
		t.Fatalf("expanded view should hide sibling commands:\n%s", view)
	}
	if got := strings.Count(view, "▼"); got != 1 {
		t.Fatalf("expanded view should show one downward caret, got %d:\n%s", got, view)
	}
	if strings.Contains(view, "❯") {
		t.Fatalf("expanded view should drop the selection caret:\n%s", view)
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

func TestMenuTabTogglesClippedDescription(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "run", Description: "alpha beta gamma delta epsilon"},
		Task{Name: "next", Description: "next command"},
	)
	m := newModel(cfg, newStyles(cfg, false), nil)
	m.applyLayout(30, 20)
	m.syncList()

	collapsed := ansi.Strip(m.View().Content)
	if !strings.Contains(collapsed, "…") || strings.Contains(collapsed, "delta") {
		t.Fatalf("collapsed menu should clip the description:\n%s", collapsed)
	}
	if !strings.Contains(collapsed, "next command") {
		t.Fatalf("collapsed menu should list sibling commands:\n%s", collapsed)
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = next.(model)
	expanded := ansi.Strip(m.View().Content)
	if !strings.Contains(expanded, "delta") || !strings.Contains(expanded, "epsilon") {
		t.Fatalf("Tab should reveal the full description:\n%s", expanded)
	}
	if !strings.Contains(expanded, "scroll") || !strings.Contains(expanded, "⇧⇥") || strings.Contains(expanded, "next command") {
		t.Fatalf("expanded menu should advertise scroll/toggle and hide siblings:\n%s", expanded)
	}
	if !strings.Contains(expanded, "▼ run") || strings.Contains(expanded, "❯ run") {
		t.Fatalf("expanded menu should show a downward-caret header:\n%s", expanded)
	}
	next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	m = next.(model)
	again := ansi.Strip(m.View().Content)
	if strings.Contains(again, "delta") || !strings.Contains(again, "next command") || !strings.Contains(again, "navigate") {
		t.Fatalf("second Tab should restore the list:\n%s", again)
	}
}

func TestMenuExpandedArrowsScrollDetailWithoutMovingHeader(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "run", Description: "summary", Details: strings.Repeat("detail ", 120)},
		Task{Name: "next", Description: "next command"},
	)
	m := newModel(cfg, newStyles(cfg, false), nil)
	m.applyLayout(40, 24)
	m.syncList()
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if !m.expanded {
		t.Fatal("Tab did not expand the selected command")
	}
	header := strings.Split(ansi.Strip(m.list.View()), "\n")[0]
	if !strings.Contains(header, "▼ run") {
		t.Fatalf("expanded header missing downward caret:\n%s", header)
	}
	if m.list.viewport.YOffset() != 0 {
		t.Fatalf("expanded content should start at the top, offset=%d", m.list.viewport.YOffset())
	}

	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
	if !m.expanded || m.sel != 0 {
		t.Fatalf("Down should scroll, not leave the view or move selection: expanded=%v sel=%d", m.expanded, m.sel)
	}
	down := m.list.viewport.YOffset()
	if down == 0 {
		t.Fatal("Down did not scroll the detail content")
	}
	if got := strings.Split(ansi.Strip(m.list.View()), "\n")[0]; got != header {
		t.Fatalf("scrolling moved the pinned header: got %q want %q", got, header)
	}

	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
	if up := m.list.viewport.YOffset(); up >= down {
		t.Fatalf("Up did not scroll back toward the top: down=%d up=%d", down, up)
	}
}

func TestMenuTabAndShiftTabToggleDetailView(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{name: "tab", key: tea.KeyPressMsg{Code: tea.KeyTab}},
		{name: "shift-tab", key: tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testMenuConfig(
				Task{Name: "run", Description: "alpha beta gamma delta epsilon"},
				Task{Name: "next", Description: "next command"},
			)
			m := newModel(cfg, newStyles(cfg, false), nil)
			m.applyLayout(60, 20)
			m.syncList()

			m, _ = press(t, m, tc.key)
			if !m.expanded || m.list.header == "" {
				t.Fatalf("%s did not open the detail view", tc.name)
			}
			expanded := ansi.Strip(m.View().Content)
			if !strings.Contains(expanded, "scroll") || !strings.Contains(expanded, "collapse") {
				t.Fatalf("%s detail view should advertise scroll/collapse:\n%s", tc.name, expanded)
			}

			m, _ = press(t, m, tc.key)
			if m.expanded {
				t.Fatalf("%s did not return to the list", tc.name)
			}
			if m.list.header != "" {
				t.Fatalf("%s left a pinned detail header: %q", tc.name, m.list.header)
			}
			view := ansi.Strip(m.View().Content)
			if !strings.Contains(view, "next command") || !strings.Contains(view, "expand") {
				t.Fatalf("%s did not restore the list view:\n%s", tc.name, view)
			}
		})
	}
}

func TestMenuDetailToggleRequiresDetails(t *testing.T) {
	cfg := testMenuConfig(Task{Name: "run", Run: "run"})
	m := newModel(cfg, newStyles(cfg, false), nil)
	m.applyLayout(60, 20)
	m.syncList()

	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if m.expanded {
		t.Fatal("Tab should not open an empty detail page")
	}
	if view := ansi.Strip(m.View().Content); strings.Contains(view, "expand") {
		t.Fatalf("footer advertises an unavailable detail page:\n%s", view)
	}
}

func TestMenuLeftKeepsRootDetailViewOpen(t *testing.T) {
	cfg := testMenuConfig(Task{Name: "run", Description: "alpha beta gamma"})
	m := newModel(cfg, newStyles(cfg, false), nil)
	m.applyLayout(30, 20)
	m.syncList()
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyTab})
	if !m.expanded {
		t.Fatal("Tab did not open the detail view")
	}

	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	if !m.expanded {
		t.Fatal("Left must belong to group navigation, not close the detail view")
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
