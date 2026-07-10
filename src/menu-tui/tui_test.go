package main

import (
	"bytes"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var _ tea.Model = model{}

func testConfig() *Config {
	return &Config{
		Project:     "test-project",
		Placeholder: "filter tasks",
		Height:      8,
		MaxWidth:    80,
		Palette: Palette{
			Fg:          "#ffffff",
			Muted:       "#aaaaaa",
			Dim:         "#777777",
			Border:      "#555555",
			Accent:      "#00ff99",
			Accent2:     "#ffaa00",
			Error:       "#ff5555",
			SelectionFg: "#000000",
			Bg:          "#111111",
			Surface:     "#222222",
			Secondary:   "#333333",
		},
		Groups: []Group{{
			Title: "develop",
			Tasks: []Task{
				{Name: "build", Run: "just build", Description: "build the project"},
				{Name: "dev", Run: "just dev", Description: "start development"},
			},
		}},
	}
}

func TestViewDeclaresAltScreen(t *testing.T) {
	cfg := testConfig()
	m := newModel(cfg, newStyles(cfg), nil)

	view := m.View()

	if !view.AltScreen {
		t.Fatal("View() must declaratively enable the alternate screen")
	}
}

func TestDownKeySelectsNextTask(t *testing.T) {
	cfg := testConfig()
	m := newModel(cfg, newStyles(cfg), nil)

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	got := next.(model)

	if got.sel != 1 {
		t.Fatalf("selected index = %d, want 1", got.sel)
	}
}

func TestRequiredArgumentPreventsSubmission(t *testing.T) {
	cfg := testConfig()
	task := Task{
		Name: "deploy",
		Run:  "just deploy",
		Args: []Arg{{Token: "<environment>", Required: true}},
	}
	m := newModel(cfg, newStyles(cfg), &task)

	got, cmd := m.submitArgs()

	if cmd != nil {
		t.Fatal("invalid arguments must not produce a command")
	}
	if got.execCmd != "" {
		t.Fatalf("exec command = %q, want empty", got.execCmd)
	}
	if got.argErr == "" {
		t.Fatal("required argument must produce a validation message")
	}
}

func TestArgumentSubmissionBuildsCommand(t *testing.T) {
	cfg := testConfig()
	task := Task{Name: "deploy", Run: "just deploy"}
	m := newModel(cfg, newStyles(cfg), &task)
	m.input.SetValue("staging --force")

	got, cmd := m.submitArgs()

	if cmd == nil {
		t.Fatal("valid arguments must quit the program")
	}
	if got.execCmd != "just deploy staging --force" {
		t.Fatalf("exec command = %q", got.execCmd)
	}
}

func TestUpKeyWrapsToLastTask(t *testing.T) {
	cfg := testConfig()
	m := newModel(cfg, newStyles(cfg), nil)

	got, _ := m.updateList(tea.KeyPressMsg{Code: tea.KeyUp})

	if got.sel != 1 {
		t.Fatalf("selected index = %d, want 1", got.sel)
	}
}

func TestEscapeClearsFilterBeforeQuitting(t *testing.T) {
	cfg := testConfig()
	m := newModel(cfg, newStyles(cfg), nil)
	m.input.SetValue("dev")
	m.filter()

	got, cmd := m.updateList(tea.KeyPressMsg{Code: tea.KeyEsc})

	if cmd != nil {
		t.Fatal("clearing a filter must not quit")
	}
	if got.input.Value() != "" {
		t.Fatalf("filter = %q, want empty", got.input.Value())
	}
	if len(got.matches) != 2 {
		t.Fatalf("matches = %d, want 2", len(got.matches))
	}
}

func TestEnterAndEscapeArgMode(t *testing.T) {
	cfg := testConfig()
	cfg.Groups[0].Tasks[0].Args = []Arg{{Token: "<target>"}}
	m := newModel(cfg, newStyles(cfg), nil)

	inArgs, _ := m.updateList(tea.KeyPressMsg{Code: tea.KeyEnter})
	if inArgs.mode != modeArgs || inArgs.argTask == nil {
		t.Fatal("enter on a task with arguments must open argument mode")
	}

	inList, _ := inArgs.updateArgs(tea.KeyPressMsg{Code: tea.KeyEsc})
	if inList.mode != modeList || inList.argTask != nil {
		t.Fatal("escape must return to list mode")
	}
}

func TestTabAndShiftTabMoveChipFocus(t *testing.T) {
	cfg := testConfig()
	task := Task{
		Name: "deploy",
		Run:  "just deploy",
		Args: []Arg{{Token: "<environment>", Options: []string{"staging", "production"}}},
	}
	m := newModel(cfg, newStyles(cfg), &task)

	first, _ := m.updateArgs(tea.KeyPressMsg{Code: tea.KeyTab})
	if first.chipFocus != 0 {
		t.Fatalf("tab focus = %d, want 0", first.chipFocus)
	}
	input, _ := first.updateArgs(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if input.chipFocus != -1 {
		t.Fatalf("shift+tab focus = %d, want input", input.chipFocus)
	}
}

func TestTextInputUsesVirtualCursor(t *testing.T) {
	cfg := testConfig()
	m := newModel(cfg, newStyles(cfg), nil)

	if !m.input.VirtualCursor() {
		t.Fatal("custom framed input must use a virtual cursor")
	}
}

func TestViewFitsConfiguredWidth(t *testing.T) {
	cfg := testConfig()
	cfg.MaxWidth = 48
	m := newModel(cfg, newStyles(cfg), nil)
	m.width = 60
	m.height = 16

	for _, line := range strings.Split(m.View().Content, "\n") {
		if width := lipgloss.Width(line); width > m.width {
			t.Fatalf("rendered line width = %d, terminal width = %d", width, m.width)
		}
	}
}

func TestPrintListStripsANSIForNonTTY(t *testing.T) {
	cfg := testConfig()
	cfg.ColorProfile = "truecolor"
	var out bytes.Buffer

	printListTo(&out, nil, cfg, newStyles(cfg))

	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("non-TTY list output contains ANSI escapes: %q", out.String())
	}
}
