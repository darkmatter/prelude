package wizard

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func testWizard() wizardModel {
	cfg := Config{
		DefaultFont: "thin",
		Fonts: []Font{
			{Name: "standard", Path: "/standard"},
			{Name: "thin", Path: "/thin"},
		},
		DefaultTheme: "phosphor",
		Themes: []Theme{
			{Name: "nord", Palette: map[string]string{"accent": "#88c0d0", "bg": "#2e3440"}},
			{Name: "phosphor", Palette: map[string]string{"accent": "#68e371", "bg": "#0c110e"}},
		},
	}
	return newWizard(cfg, Recipe{Text: "acme", Font: "thin"}, func(font Font, text string) (string, error) {
		return font.Name + ":" + text, nil
	})
}

func pressKey(t *testing.T, m wizardModel, msg tea.KeyPressMsg) wizardModel {
	t.Helper()
	next, _ := m.Update(msg)
	model, ok := next.(wizardModel)
	if !ok {
		t.Fatalf("Update returned %T, want wizardModel", next)
	}
	return model
}

func enter(t *testing.T, m wizardModel) wizardModel {
	t.Helper()
	return pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
}

func letter(t *testing.T, m wizardModel, r rune) wizardModel {
	t.Helper()
	return pressKey(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
}

func TestWizardWalksEveryStepAndCollectsSelections(t *testing.T) {
	m := testWizard()

	// Title: prefilled from the recipe, defaults the theme from the config.
	if m.themeIndex != 1 {
		t.Fatalf("themeIndex = %d, want default phosphor at 1", m.themeIndex)
	}
	m = enter(t, m)
	if m.step != stepFont || m.preview != "thin:acme" {
		t.Fatalf("after title: step=%d preview=%q", m.step, m.preview)
	}

	// Font: page forward wraps to standard.
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if m.fontIndex != 0 || m.preview != "standard:acme" {
		t.Fatalf("after paging: fontIndex=%d preview=%q", m.fontIndex, m.preview)
	}
	m = enter(t, m)

	// Project: auto-filled from the title text.
	if m.step != stepProject || m.projectInput.Value() != "acme" {
		t.Fatalf("after font: step=%d project=%q", m.step, m.projectInput.Value())
	}
	m = enter(t, m)

	// Theme: move up from phosphor to nord.
	if m.step != stepTheme {
		t.Fatalf("step = %d, want theme", m.step)
	}
	m = letter(t, m, 'k')
	m = enter(t, m)

	// Color profile: move down once to truecolor.
	if m.step != stepProfile {
		t.Fatalf("step = %d, want profile", m.step)
	}
	m = letter(t, m, 'j')
	m = enter(t, m)

	// Components: toggle docs on (motd, menu, prompt stay on).
	if m.step != stepComponents {
		t.Fatalf("step = %d, want components", m.step)
	}
	m = letter(t, m, 'j')
	m = letter(t, m, 'j')
	m = letter(t, m, 'j')
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = enter(t, m)

	// Commands: add one via the three-field entry sub-flow.
	if m.step != stepCommands || m.commandPhase != commandList {
		t.Fatalf("step = %d phase = %d, want commands list", m.step, m.commandPhase)
	}
	m = letter(t, m, 'a')
	if m.commandPhase != commandName {
		t.Fatalf("phase = %d, want name entry", m.commandPhase)
	}
	for _, r := range "dev" {
		m = letter(t, m, r)
	}
	m = enter(t, m)
	if m.commandPhase != commandExec {
		t.Fatalf("phase = %d, want exec entry (err=%q)", m.commandPhase, m.err)
	}
	for _, r := range "make" {
		m = letter(t, m, r)
	}
	m = enter(t, m)
	for _, r := range "run" {
		m = letter(t, m, r)
	}
	m = enter(t, m)
	if m.commandPhase != commandList || len(m.commands) != 1 {
		t.Fatalf("after entry: phase=%d commands=%d", m.commandPhase, len(m.commands))
	}
	m = enter(t, m)

	// Integration is no longer a separate step after moving to JSON emission.
	// Confirm.
	if m.step != stepConfirm {
		t.Fatalf("step = %d, want confirm", m.step)
	}
	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = next.(wizardModel)
	if !m.done || cmd == nil {
		t.Fatalf("confirm: done=%v cmd nil=%v", m.done, cmd == nil)
	}

	got := m.result()
	want := wizardResult{
		Recipe:       Recipe{Text: "acme", Font: "standard"},
		Project:      "acme",
		Theme:        "nord",
		ColorProfile: "truecolor",
		Motd:         true,
		Menu:         true,
		Prompt:       true,
		Docs:         true,
		Commands:     []wizardCommand{{Name: "dev", Exec: "make", Description: "run"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result = %#v, want %#v", got, want)
	}
}

func TestWizardProjectStopsFollowingTitleAfterEdit(t *testing.T) {
	m := testWizard()
	m = enter(t, m) // title -> font
	m = enter(t, m) // font -> project

	m = letter(t, m, 'x') // project becomes "acmex", detaching auto-sync
	if m.projectAuto {
		t.Fatal("projectAuto still set after editing the project field")
	}

	// Walk back to the title and change it; the project must keep the edit.
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyEscape}) // project -> font
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyEscape}) // font -> title
	m = letter(t, m, 'z')                                    // title becomes "acmez"
	m = enter(t, m)                                          // title -> font
	m = enter(t, m)                                          // font -> project
	if got := m.projectInput.Value(); got != "acmex" {
		t.Fatalf("project = %q, want the manual edit to survive", got)
	}
}

func TestWizardRequiresNonEmptyProject(t *testing.T) {
	m := testWizard()
	m = enter(t, m) // title -> font
	m = enter(t, m) // font -> project
	m.projectInput.SetValue("   ")
	m = enter(t, m)
	if m.step != stepProject || m.err == "" {
		t.Fatalf("empty project accepted: step=%d err=%q", m.step, m.err)
	}
}

func TestWizardEscapeBacktracksWithoutCanceling(t *testing.T) {
	m := testWizard()
	m = enter(t, m) // title -> font
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.step != stepTitle || m.canceled {
		t.Fatalf("esc from font: step=%d canceled=%v", m.step, m.canceled)
	}
}

func TestRenderWizardConfigEmitsOptionsTemplate(t *testing.T) {
	got := renderWizardConfig(wizardResult{
		Recipe:       Recipe{Text: "acme", Font: "thin"},
		Project:      "acme-web",
		Theme:        "nord",
		ColorProfile: "auto",
		Motd:         true,
		Menu:         true,
		Prompt:       true,
		Docs:         false,
		Commands: []wizardCommand{
			{Name: "dev", Exec: "pnpm dev", Description: "start the dev server"},
			{Name: "db:migrate", Description: "apply pending migrations"},
		},
	}, "title.txt")

	// Wizard choices stay active; every other option is present as a commented
	// default with a trailing note on the same line (defaults.nix surface).
	for _, fragment := range []string{
		"# Generated by prelude setup.",
		"intentionally separate from flake.nix",
		"inputs.prelude.flakeModules.default",
		"./prelude.nix",
		"Every supported option is listed below.",
		`theme = "nord";`,
		`default "prelude"`,
		"# palette.fg = null;",
		"# palette.accent = null;",
		`colorProfile = "auto";`,
		`project = "acme-web";`,
		`default "acme"`,
		"dev = {",
		`exec = "pnpm dev";`,
		`description = "start the dev server";`,
		"motd = 1;",
		`"db:migrate" = {`,
		"# exec = \"migrate\";",
		`description = "apply pending migrations";`,
		"motd = 2;",
		"# key = null;",
		"# usage =",
		"# details = null;",
		"# args = [ ];",
		"title = {",
		"text = ./title.txt;",
		`# align = "center";`,
		"# background = false;",
		"# windowBackground = false;",
		"# maxWidth = 120;",
		"# margin = {",
		"#   x = 4;",
		"#   bottom = 4;",
		"#   minHeight = 40;",
		`#     text = "everything you need to build, test & ship";`,
		"#   statusHint = {",
		`#     layout = "inline";`,
		"menu = {",
		"enable = true;",
		"# height = 20;",
		"# maxWidth = 80;",
		"prompt = {",
		"docs.pages = [ ];",
		`# sort.groups = [ "develop" "database" "ops" ];`,
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("options template missing fragment %q:\n%s", fragment, got)
		}
	}
	if strings.Contains(got, "prelude.json") || strings.Contains(got, "fromJSON") {
		t.Fatalf("template must not use JSON sidecar:\n%s", got)
	}
}

func TestWizardCommandNameValidation(t *testing.T) {
	m := testWizard()
	m.step = stepCommands
	m.commandPhase = commandList

	m = letter(t, m, 'a')
	m.commandInput.SetValue("bad name")
	m = enter(t, m)
	if m.commandPhase != commandName || m.err == "" {
		t.Fatalf("invalid name accepted: phase=%d err=%q", m.commandPhase, m.err)
	}

	m.commandInput.SetValue("scripts:test:unit")
	m = enter(t, m) // name -> exec
	m = enter(t, m) // exec (empty ok) -> description
	m = enter(t, m) // description (empty ok) -> appended
	if len(m.commands) != 1 || m.commands[0].Name != "scripts:test:unit" {
		t.Fatalf("commands = %#v", m.commands)
	}

	// Duplicate names are rejected at the name field.
	m = letter(t, m, 'a')
	m.commandInput.SetValue("scripts:test:unit")
	m = enter(t, m)
	if m.commandPhase != commandName || !strings.Contains(m.err, "already exists") {
		t.Fatalf("duplicate accepted: phase=%d err=%q", m.commandPhase, m.err)
	}
}

func TestFinishWizardWritesTitleStarterDocsAndConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	m := testWizard()
	result := wizardResult{
		Recipe:       Recipe{Text: "acme", Font: "thin"},
		Project:      "acme",
		Theme:        "nord",
		ColorProfile: "auto",
		Motd:         true,
		Menu:         true,
		Prompt:       true,
		Docs:         true,
	}
	var stderr bytes.Buffer
	code := finishWizard(m.cfg, m.render, result, "prelude.nix", &stderr)
	if code != 0 {
		t.Fatalf("finishWizard = %d, stderr: %s", code, stderr.String())
	}
	title, err := os.ReadFile("title.txt")
	if err != nil || string(title) != "thin:acme\n" {
		t.Fatalf("title.txt = %q, err %v", title, err)
	}
	page, err := os.ReadFile(starterDocsPath)
	if err != nil || !strings.Contains(string(page), "# Getting started") {
		t.Fatalf("starter docs page = %q, err %v", page, err)
	}
	if _, err := os.Stat("prelude.json"); !os.IsNotExist(err) {
		t.Fatalf("prelude.json should not be written, err=%v", err)
	}
	nixData, err := os.ReadFile("prelude.nix")
	if err != nil {
		t.Fatalf("read prelude.nix: %v", err)
	}
	for _, fragment := range []string{
		`theme = "nord";`,
		`project = "acme";`,
		"text = ./title.txt;",
		"docs.pages = [ { text = ./docs/getting-started.md; } ];",
		"# background = false;",
		"# maxWidth = 120;",
	} {
		if !strings.Contains(string(nixData), fragment) {
			t.Fatalf("prelude.nix missing %q:\n%s", fragment, nixData)
		}
	}
	if !strings.Contains(stderr.String(), "wrote title.txt\n") || !strings.Contains(stderr.String(), "wrote prelude.nix\n") {
		t.Fatalf("stderr missing write notices: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "import this module from your flake.nix") {
		t.Fatalf("stderr missing import next steps: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "./prelude.nix") {
		t.Fatalf("stderr missing import path: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "direnv log lines") || !strings.Contains(stderr.String(), "log_format = \"-\"") {
		t.Fatalf("stderr missing direnv tip (MOTD enabled): %s", stderr.String())
	}
}

func TestFinishWizardOmitsDirenvTipWhenMotdDisabled(t *testing.T) {
	t.Chdir(t.TempDir())
	m := testWizard()
	result := wizardResult{
		Recipe: Recipe{Text: "acme", Font: "thin"},
		Project: "acme", Theme: "nord", ColorProfile: "auto",
		Motd: false, Menu: true, Prompt: true,
	}
	var stderr bytes.Buffer
	if code := finishWizard(m.cfg, m.render, result, "prelude.nix", &stderr); code != 0 {
		t.Fatalf("finishWizard = %d, stderr: %s", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "direnv log lines") {
		t.Fatalf("stderr should not mention direnv (MOTD disabled): %s", stderr.String())
	}
}

func TestFinishWizardRefusesFlakeNix(t *testing.T) {
	t.Chdir(t.TempDir())
	m := testWizard()
	result := wizardResult{
		Recipe:  Recipe{Text: "acme", Font: "thin"},
		Project: "acme", Theme: "nord", ColorProfile: "auto",
		Motd: true, Menu: true, Prompt: true,
	}
	var stderr bytes.Buffer
	if code := finishWizard(m.cfg, m.render, result, "flake.nix", &stderr); code == 0 {
		t.Fatal("finishWizard accepted flake.nix")
	}
	if _, err := os.Stat("flake.nix"); !os.IsNotExist(err) {
		t.Fatal("flake.nix was written")
	}
}

func TestFinishWizardWritesTitleBesideNestedConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	m := testWizard()
	result := wizardResult{
		Recipe:  Recipe{Text: "acme", Font: "thin"},
		Project: "acme", Theme: "nord", ColorProfile: "auto",
		Motd: true, Menu: true, Prompt: true,
	}
	var stderr bytes.Buffer
	if code := finishWizard(m.cfg, m.render, result, "nix/prelude.nix", &stderr); code != 0 {
		t.Fatalf("finishWizard = %d, stderr: %s", code, stderr.String())
	}
	title, err := os.ReadFile("nix/title.txt")
	if err != nil || string(title) != "thin:acme\n" {
		t.Fatalf("nix/title.txt = %q, err %v", title, err)
	}
	if _, err := os.Stat("nix/prelude.json"); !os.IsNotExist(err) {
		t.Fatalf("nested JSON should not be written, err=%v", err)
	}
	nixData, err := os.ReadFile("nix/prelude.nix")
	if err != nil {
		t.Fatalf("read nix/prelude.nix: %v", err)
	}
	// Title path is relative to the config file, not the repo root.
	if !strings.Contains(string(nixData), "text = ./title.txt;") {
		t.Fatalf("nested config missing sibling title reference:\n%s", nixData)
	}
	if !strings.Contains(string(nixData), `project = "acme";`) {
		t.Fatalf("nested config missing project:\n%s", nixData)
	}
}

func TestFinishWizardKeepsExistingDocsPage(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("docs", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(starterDocsPath, []byte("# Mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := testWizard()
	result := wizardResult{
		Recipe:  Recipe{Text: "acme", Font: "thin"},
		Project: "acme", Theme: "nord", ColorProfile: "auto",
		Motd: true, Menu: true, Prompt: true, Docs: true,
	}
	var stderr bytes.Buffer
	if code := finishWizard(m.cfg, m.render, result, "prelude.nix", &stderr); code != 0 {
		t.Fatalf("finishWizard = %d", code)
	}
	page, err := os.ReadFile(starterDocsPath)
	if err != nil || string(page) != "# Mine\n" {
		t.Fatalf("existing docs page was clobbered: %q, err %v", page, err)
	}
	if !strings.Contains(stderr.String(), "kept existing "+starterDocsPath) {
		t.Fatalf("stderr missing keep notice: %s", stderr.String())
	}
}

func TestNixPathLiteralPatternRejectsUnrepresentablePaths(t *testing.T) {
	if nixPathLiteralPattern.MatchString("my dir/title.txt") {
		t.Fatal("space accepted in Nix path literal")
	}
	if !nixPathLiteralPattern.MatchString("assets/title-v2.txt") {
		t.Fatal("plain relative path rejected")
	}
}

func TestTitlePathBesideConfig(t *testing.T) {
	cases := map[string]string{
		"prelude.nix":     "title.txt",
		"./prelude.nix":   "title.txt",
		"nix/prelude.nix": "nix/title.txt",
		"/tmp/ui.nix":     "/tmp/title.txt",
	}
	for config, want := range cases {
		if got := titlePathBesideConfig(config); got != want {
			t.Fatalf("titlePathBesideConfig(%q) = %q, want %q", config, got, want)
		}
	}
}

// TestWriteExampleFixture regenerates nix/internal/example.nix from the stock
// setup wizard presets (keep in sync with src/prelude/wizard-presets.nix).
func TestWriteExampleFixture(t *testing.T) {
	if os.Getenv("WRITE_EXAMPLE") == "" {
		t.Skip("set WRITE_EXAMPLE=1 to regenerate nix/internal/example.nix")
	}
	got := renderWizardConfig(wizardResult{
		Recipe:       Recipe{Text: "acme", Font: "kompaktblk"},
		Project:      "acme",
		Theme:        "prelude",
		ColorProfile: "auto",
		Motd:         true,
		Menu:         true,
		Prompt:       true,
		Docs:         false,
		Commands: []wizardCommand{
			{Name: "dev", Exec: "pnpm dev", Description: "start the dev server with hot reload"},
			{Name: "test", Exec: "pnpm test", Description: "run the unit test suite"},
			{Name: "build", Exec: "pnpm build", Description: "compile an optimized production bundle"},
		},
	}, "title.txt")
	path := "../../../nix/internal/example.nix"
	if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
		t.Fatal(err)
	}
}
