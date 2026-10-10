package menu

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"

	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestEmbeddedMenuView(t *testing.T) {
	cfg := testMenuConfig(
		Task{
			Name: "dev", Run: "bun run dev", Key: "d", Description: "Start the service",
			Details: "Start the development service.", Usage: "x dev", Examples: []string{"x dev --watch"},
		},
		Task{Name: "check", Run: "go test ./...", Key: "c", Description: "Check the service"},
	)
	cfg.MaxWidth = 60
	cfg.Palette.SelectionFg = "#112233"
	menus := []model{
		newModel(cfg, newStyles(cfg, false), nil),
		newModel(cfg, newStyles(cfg, true), nil),
	}
	for i := range menus {
		menus[i].applyLayout(90, 34)
		menus[i].syncList()
	}
	accent := lipgloss.Color(string(cfg.Palette.Accent))
	selectionFg := lipgloss.Color(string(cfg.Palette.SelectionFg))
	chrome := lipgloss.Color(string(cfg.Palette.Surface))
	muted := lipgloss.Color(string(cfg.Palette.Muted))

	for _, stage := range []struct {
		name  string
		keys  []tea.KeyPressMsg
		texts []string
	}{
		{name: "list", texts: []string{"command menu", "~/test", "check", "navigate", "details", "ready"}},
		{name: "details", keys: []tea.KeyPressMsg{{Code: tea.KeyTab}}, texts: []string{
			"Start the development service.", "$ x dev", "example ❯ x dev --watch",
		}},
		{name: "empty-filter", keys: []tea.KeyPressMsg{
			{Code: 'z', Text: "z"}, {Code: 'z', Text: "z"}, {Code: 'z', Text: "z"}, {Code: 'z', Text: "z"},
		}, texts: []string{"no commands match", "zzzz", "to reset"}},
	} {
		t.Run(stage.name, func(t *testing.T) {
			for _, key := range stage.keys {
				for i := range menus {
					next, _ := menus[i].Update(key)
					menus[i] = next.(model)
				}
			}
			plain, embedded := menus[0].View(), menus[1].View()
			cells := assertEmbeddedMenuPanel(t, plain, embedded)
			assertMenuTextColors(t, cells, "command menu", chrome, muted)
			assertMenuTextColors(t, cells, "navigate", chrome, muted)
			if stage.name != "empty-filter" {
				assertMenuTextColors(t, cells, "dev", accent, selectionFg)
				assertLastRowScript(t, menus[1], "bun run dev")
			}
			if stage.name == "details" {
				assertMenuTextColors(t, cells, "Start the development service.", lipgloss.Color(string(cfg.Palette.Bg)), muted)
			}
			for _, text := range stage.texts {
				if !strings.Contains(ansi.Strip(embedded.Content), text) {
					t.Fatalf("menu lost %q", text)
				}
			}
		})
	}
}

func TestEmbeddedMenuFormView(t *testing.T) {
	task := Task{
		Name: "deploy", Run: "just deploy",
		Args: []Arg{{Token: "ENV", Required: true, Options: []string{"prod", "stage"}}},
	}
	cfg := testMenuConfig(task)
	cfg.Execute = true
	cfg.MaxWidth = 60
	cfg.Palette.SelectionFg = "#112233"
	cfg.Palette.Secondary = "#243546"
	cfg.Palette.Error = "#ff6677"
	menus := []model{
		newModel(cfg, newStyles(cfg, false), nil),
		newModel(cfg, newStyles(cfg, true), nil),
	}
	for i := range menus {
		menus[i].applyLayout(90, 34)
		menus[i].syncList()
	}
	secondary := lipgloss.Color(string(cfg.Palette.Secondary))
	accent := lipgloss.Color(string(cfg.Palette.Accent))
	fg := lipgloss.Color(string(cfg.Palette.Fg))
	selectionFg := lipgloss.Color(string(cfg.Palette.SelectionFg))

	for _, stage := range []struct {
		name string
		key  rune
	}{
		{name: "form", key: tea.KeyEnter},
		{name: "error", key: tea.KeyEnter},
		{name: "focused-chip", key: tea.KeyTab},
	} {
		t.Run(stage.name, func(t *testing.T) {
			for i := range menus {
				next, _ := menus[i].Update(tea.KeyPressMsg{Code: stage.key})
				menus[i] = next.(model)
			}
			plain, embedded := menus[0].View(), menus[1].View()
			cells := assertEmbeddedMenuPanel(t, plain, embedded)
			prodBg, prodFg := secondary, fg
			if stage.name == "focused-chip" {
				prodBg, prodFg = accent, selectionFg
			}
			assertMenuTextColors(t, cells, "prod", prodBg, prodFg)
			assertMenuTextColors(t, cells, "stage", secondary, fg)
			assertMenuTextColors(t, cells, "chips", lipgloss.Color(string(cfg.Palette.Surface)), lipgloss.Color(string(cfg.Palette.Muted)))
			assertLastRowScript(t, menus[1], "just deploy")
			for _, text := range []string{"enter arguments", "REQUIRED", "ENV"} {
				if !strings.Contains(ansi.Strip(embedded.Content), text) {
					t.Fatalf("form lost %q", text)
				}
			}
			if stage.name != "form" && !strings.Contains(ansi.Strip(embedded.Content), "missing required argument ENV") {
				t.Fatal("form lost its validation error")
			}
		})
	}

	for i := range menus {
		next, _ := menus[i].Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		menus[i] = next.(model)
	}
	assertEmbeddedMenuPanel(t, menus[0].View(), menus[1].View())
	assertLastRowScript(t, menus[1], "just deploy prod")
	next, cmd := menus[1].Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m := next.(model)
	if m.chosen == nil || m.chosen.Name != "deploy" || m.chosen.Line != "prod" || m.chosen.Command != "just deploy prod" {
		t.Fatalf("embedded form selection = %+v", m.chosen)
	}
	if cmd == nil {
		t.Fatal("embedded form did not quit for command execution")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("embedded form must quit before finish executes the selection")
	}
}

func TestMenuCLIEmbedded(t *testing.T) {
	const helperEnv = "PRELUDE_MENU_TEST_CLI"
	if os.Getenv(helperEnv) == "1" {
		// Run owns flag parsing, exit, and exec, so exercise it in a process
		// isolated from the test runner's flags and lifecycle.
		args := flag.Args()
		flag.CommandLine = flag.NewFlagSet("menu", flag.ExitOnError)
		os.Args = append([]string{os.Args[0]}, args...)
		Run()
		os.Exit(0)
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var environ []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "BASH_ENV", "ENV", "SSH_CLIENT", "SSH_CONNECTION", "PRELUDE_MENU_CONFIG", "PRELUDE_MENU_DEBUG", helperEnv:
			continue
		}
		environ = append(environ, entry)
	}
	environ = append(environ, helperEnv+"=1")
	run := func(args ...string) (string, string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, executable, append([]string{"-test.run=^TestMenuCLIEmbedded$", "--"}, args...)...)
		cmd.Env = environ
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("menu %v: %v (timeout: %v)\n%s", args, err, ctx.Err(), stderr.String())
		}
		return stdout.String(), stderr.String()
	}

	stdout, help := run("--x", "--embedded", "--help")
	if stdout != "" || !strings.Contains(help, "[--embedded]") || !strings.Contains(help, "canvas") {
		t.Fatalf("embedded help = stdout %q, stderr %q", stdout, help)
	}

	cfg := testMenuConfig(Task{Name: "echo", Run: "printf 'ran:%s\\n'", Description: "Echo a value"})
	path := filepath.Join(t.TempDir(), "menu.json")
	writeConfig := func() {
		t.Helper()
		raw, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig()
	plain, _ := run("--x", "--config", path, "--list")
	embedded, _ := run("--x", "--config", path, "--embedded", "--list")
	if embedded != plain || !strings.Contains(embedded, "Echo a value") {
		t.Fatalf("--embedded changed --list output: plain %q, embedded %q", plain, embedded)
	}

	plain, _ = run("--x", "--config", path, "echo", "--", "--embedded")
	embedded, _ = run("--x", "--config", path, "--embedded", "echo", "--", "--embedded")
	if embedded != plain || embedded != cfg.Groups[0].Tasks[0].Run+" --embedded\n" {
		t.Fatalf("--embedded changed dispatch or consumed a task argument: %q", embedded)
	}

	cfg.Execute = true
	writeConfig()
	plain, _ = run("--x", "--config", path, "echo", "prod")
	embedded, _ = run("--x", "--config", path, "--embedded", "echo", "prod")
	if embedded != plain || !strings.HasSuffix(embedded, "ran:prod\n") {
		t.Fatalf("--embedded changed command execution: plain %q, embedded %q", plain, embedded)
	}
}

func assertEmbeddedMenuPanel(t *testing.T, standalone, embedded tea.View) []uv.Line {
	t.Helper()
	if standalone.BackgroundColor == nil || embedded.BackgroundColor != nil {
		t.Fatalf("renderer canvas backgrounds: standalone %v, embedded %v", standalone.BackgroundColor, embedded.BackgroundColor)
	}
	if !standalone.AltScreen || !embedded.AltScreen {
		t.Fatal("embedded render mode changed the menu's alt-screen lifecycle")
	}
	if standalone.Cursor == nil || embedded.Cursor == nil || standalone.Cursor.Position != embedded.Cursor.Position ||
		standalone.Cursor.Shape != embedded.Cursor.Shape || standalone.Cursor.Blink != embedded.Cursor.Blink {
		t.Fatal("embedded render mode changed the prompt cursor")
	}
	width, height := lipgloss.Width(standalone.Content), lipgloss.Height(standalone.Content)
	if lipgloss.Width(embedded.Content) != width || lipgloss.Height(embedded.Content) != height {
		t.Fatal("embedded render mode changed the terminal canvas dimensions")
	}
	plain := uv.NewScreenBuffer(width, height)
	uv.NewStyledString(standalone.Content).Draw(&plain, plain.Bounds())
	actual := uv.NewScreenBuffer(width, height)
	uv.NewStyledString(embedded.Content).Draw(&actual, actual.Bounds())

	// Locate the visible standalone panel, excluding the terminal-wide preview.
	// Its half-cell chrome spans the full panel width, including padded cells.
	left, top, right, bottom := width, height, -1, -1
	for y, row := range plain.Lines[:height-1] {
		for x, cell := range row {
			if strings.TrimSpace(cell.Content) != "" {
				left, top = min(left, x), min(top, y)
				right, bottom = max(right, x), max(bottom, y)
			}
		}
	}
	if left <= 0 || top <= 0 || right >= width-1 || bottom >= height-2 {
		t.Fatal("render fixture must expose margins on every side of the panel")
	}
	if !strings.Contains(standalone.Content, "▄") || !strings.Contains(standalone.Content, "▀") {
		t.Fatal("standalone menu lost its half-cell chrome")
	}
	for y, row := range plain.Lines {
		for x, want := range row {
			region := "panel"
			if y == height-1 {
				region = "script preview"
			} else if x < left || x > right || y < top || y > bottom {
				region = "canvas margin"
			}
			if region != "panel" {
				if !sameMenuColor(want.Style.Bg, standalone.BackgroundColor) {
					t.Fatalf("standalone %s lost its canvas background at (%d,%d)", region, x, y)
				}
				want.Style.Bg = nil
			}
			got := actual.Lines[y][x]
			if got.Content != want.Content || got.Width != want.Width || !got.Style.Equal(&want.Style) {
				t.Fatalf("embedded %s differs at (%d,%d): got %q %v, want %q %v", region, x, y, got.Content, got.Style, want.Content, want.Style)
			}
		}
	}
	return actual.Lines
}

func assertMenuTextColors(t *testing.T, cells []uv.Line, text string, background, foreground color.Color) {
	t.Helper()
	for _, row := range cells {
		var content strings.Builder
		for _, cell := range row {
			content.WriteString(cell.Content)
		}
		line := content.String()
		index := strings.Index(line, text)
		if index < 0 {
			continue
		}
		start := ansi.StringWidth(line[:index])
		for x := start; x < start+ansi.StringWidth(text); x++ {
			if x >= len(row) || !sameMenuColor(row[x].Style.Bg, background) || !sameMenuColor(row[x].Style.Fg, foreground) {
				t.Fatalf("%q lost its background/foreground at column %d", text, x)
			}
		}
		return
	}
	t.Fatalf("menu text %q not rendered", text)
}

func sameMenuColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return color.NRGBAModel.Convert(a) == color.NRGBAModel.Convert(b)
}
