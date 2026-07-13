package main

import (
	"bytes"
	"os"
	"os/exec"
	"prelude/shared"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var _ tea.Model = model{}

func testConfig() *Config {
	return &Config{
		Project:     "test-project",
		Placeholder: "filter tasks",
		Height:      8,
		MaxWidth:    80,
		Palette: shared.Palette{
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

func TestViewSetsWindowBackground(t *testing.T) {
	cfg := testConfig()
	m := newModel(cfg, newStyles(cfg), nil)

	view := m.View()

	if view.BackgroundColor != m.st.bgColor {
		t.Fatalf("window background = %v, want theme bg %v", view.BackgroundColor, m.st.bgColor)
	}
}

func TestViewPaintsBackgroundAcrossEntireWindow(t *testing.T) {
	cfg := testConfig()
	m := newModel(cfg, newStyles(cfg), nil)
	m.width = 72
	m.height = 30

	content := m.View().Content
	if got := lipgloss.Height(content); got != m.height {
		t.Fatalf("view height = %d, want terminal height %d", got, m.height)
	}

	lines := strings.Split(content, "\n")
	bottom := lines[len(lines)-1]
	if got := ansi.Strip(bottom); got != strings.Repeat(" ", m.width) {
		t.Fatalf("bottom row = %q, want a full-width background row", got)
	}
	if bottom == ansi.Strip(bottom) {
		t.Fatal("bottom row is unstyled; background would use the terminal default")
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
	if got.mode != modeArgs {
		t.Fatalf("mode = %v, want argument mode", got.mode)
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
	if !got.hasExecCmd {
		t.Fatal("valid submission must retain a command decision")
	}
	if got.execCmd != "just deploy staging --force" {
		t.Fatalf("exec command = %q", got.execCmd)
	}
}
func TestListSelectionRetainsEmptyCommandDecision(t *testing.T) {
	cfg := testConfig()
	cfg.Groups[0].Tasks = []Task{{Name: "empty", Run: " \t "}}
	m := newModel(cfg, newStyles(cfg), nil)

	got, cmd := m.updateList(tea.KeyPressMsg{Code: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("command selection must quit the program")
	}
	if !got.hasExecCmd {
		t.Fatal("empty command text must still retain a command decision")
	}
	if got.execCmd != "" {
		t.Fatalf("command = %q, want normalized empty command", got.execCmd)
	}
}

func TestArgumentPreviewUsesFinalCommandAssembler(t *testing.T) {
	cfg := testConfig()
	task := Task{
		Name: "deploy",
		Run:  "  just deploy  ",
		Args: []Arg{{Token: "<environment>"}},
	}
	m := newModel(cfg, newStyles(cfg), &task)
	m.input.SetValue("  staging  ")

	decision, err := completeInvocation(task, m.input.Value())
	if err != nil {
		t.Fatal(err)
	}
	content := ansi.Strip(m.View().Content)
	want := "$ " + decision.command
	if !strings.Contains(content, want) {
		t.Fatalf("preview does not contain final command %q:\n%s", want, content)
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

func TestFastPathUnknownTaskExitsOne(t *testing.T) {
	const helperEnv = "PRELUDE_TEST_FAST_PATH_UNKNOWN"
	if os.Getenv(helperEnv) == "1" {
		cfg := testConfig()
		fastPath(cfg, newStyles(cfg), "missing", nil)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestFastPathUnknownTaskExitsOne$")
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	output, err := cmd.CombinedOutput()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("fast path error = %v, want exit error; output:\n%s", err, output)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("exit code = %d, want 1; output:\n%s", exitErr.ExitCode(), output)
	}
	got := ansi.Strip(string(output))
	if !strings.Contains(got, `menu: unknown task "missing"`) {
		t.Fatalf("stderr = %q", got)
	}
}
