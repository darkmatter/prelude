package title

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

	// Integration: keep the flake-parts default after a round trip.
	if m.step != stepIntegration {
		t.Fatalf("step = %d, want integration", m.step)
	}
	m = letter(t, m, 'j')
	m = letter(t, m, 'j')
	m = enter(t, m)

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
		FlakeParts:   true,
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

func TestRenderWizardConfigEmitsReadyToUseModule(t *testing.T) {
	config := renderWizardConfig(wizardResult{
		Recipe:       Recipe{Text: "acme", Font: "thin"},
		Project:      "acme-web",
		Theme:        "nord",
		ColorProfile: "auto",
		Motd:         true,
		Menu:         true,
		Prompt:       true,
		Docs:         false,
		FlakeParts:   true,
	}, "title.txt")

	want := `# Generated by prelude setup.
#
# A flake-parts module: import it next to Prelude's module, e.g.
#
#   imports = [ prelude.flakeModules.default ./prelude.nix ];
#
{ ... }:
{
  prelude = {
    theme = "nord";
    colorProfile = "auto";
    project = "acme-web";

    # Declare project commands for the menu and MOTD:
    # commands.dev = { exec = "pnpm dev"; description = "start the dev server"; };

    motd = {
      enable = true;
      title = {
        text = ./title.txt;
        align = "center";
      };
    };
    menu.enable = true;
    prompt.enable = true;

    # Markdown pages enable the docs viewer:
    # docs.pages = [ { text = ./docs/getting-started.md; } ];
  };
}
`
	if config != want {
		t.Fatalf("config mismatch:\n--- got ---\n%s\n--- want ---\n%s", config, want)
	}
}

func TestRenderWizardConfigEmitsCommandsAndMotdNextSteps(t *testing.T) {
	config := renderWizardConfig(wizardResult{
		Recipe:       Recipe{Text: "acme", Font: "thin"},
		Project:      "acme",
		Theme:        "nord",
		ColorProfile: "auto",
		Motd:         true,
		Menu:         true,
		Prompt:       true,
		FlakeParts:   true,
		Commands: []wizardCommand{
			{Name: "dev", Exec: "pnpm dev", Description: "start the dev server"},
			{Name: "db:migrate", Description: "apply pending migrations"},
			{Name: "lint", Exec: "pnpm lint"},
			{Name: "deploy", Exec: "pnpm deploy"},
		},
	}, "title.txt")

	for _, fragment := range []string{
		"    commands = {\n      dev = {\n        exec = \"pnpm dev\";\n        description = \"start the dev server\";\n      };\n",
		// Public keys containing colons are not plain Nix identifiers, so quote them.
		"      \"db:migrate\" = {\n        description = \"apply pending migrations\";\n      };\n",
		// The MOTD advertises at most the first three commands.
		"      commands = [ \"dev\" \"db:migrate\" \"lint\" ];\n",
	} {
		if !strings.Contains(config, fragment) {
			t.Fatalf("config missing fragment %q:\n%s", fragment, config)
		}
	}
	if strings.Contains(config, "\"deploy\" ]") || strings.Contains(config, "exec = \"db:migrate\"") {
		t.Fatalf("config leaked capped or inferred values:\n%s", config)
	}
}

func TestRenderStandaloneConfigUsesLibBuilders(t *testing.T) {
	config := renderWizardConfig(wizardResult{
		Recipe:       Recipe{Text: "acme", Font: "thin"},
		Project:      "acme",
		Theme:        "nord",
		ColorProfile: "auto",
		Motd:         true,
		Menu:         true,
		Prompt:       true,
		Docs:         true,
		FlakeParts:   false,
		Commands: []wizardCommand{
			{Name: "dev", Exec: "pnpm dev", Description: "start the dev server"},
		},
	}, "docs/title.txt")

	for _, fragment := range []string{
		"{ pkgs, prelude }:",
		"  motd = prelude.lib.mkMotd\n    { inherit (pkgs) lib writeText buildGoModule; }",
		"        text = ./docs/title.txt;",
		"      commandCatalog = commands;",
		"      commands = [ \"dev\" ];",
		"  menu = prelude.lib.mkMenu\n    { inherit (pkgs) lib writeShellApplication writeText buildGoModule; }",
		"    (shared // { inherit commands; });",
		"  docs = prelude.lib.mkDocs",
		"no standalone builder yet",
	} {
		if !strings.Contains(config, fragment) {
			t.Fatalf("standalone config missing %q:\n%s", fragment, config)
		}
	}
	if strings.Contains(config, "prelude = {") || strings.Contains(config, "flakeModules") {
		t.Fatalf("standalone config leaked flake-parts shape:\n%s", config)
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

func TestRenderStandaloneConfigWithoutCommandsCommentsOutMenu(t *testing.T) {
	config := renderWizardConfig(wizardResult{
		Recipe:       Recipe{Text: "acme", Font: "thin"},
		Project:      "acme",
		Theme:        "nord",
		ColorProfile: "auto",
		Motd:         true,
		Menu:         true,
		FlakeParts:   false,
	}, "docs/title.txt")

	// mkMenu asserts a non-empty catalogue, so an active call would fail the
	// first build; it must ship commented until commands exist.
	if strings.Contains(config, "\n  menu = prelude.lib.mkMenu") {
		t.Fatalf("empty catalogue emitted an active mkMenu call:\n%s", config)
	}
	if !strings.Contains(config, "# menu = prelude.lib.mkMenu") {
		t.Fatalf("commented mkMenu template missing:\n%s", config)
	}
}

func TestRenderWizardConfigDisabledMotdKeepsTitleReference(t *testing.T) {
	config := renderWizardConfig(wizardResult{
		Recipe:       Recipe{Text: "acme", Font: "thin"},
		Project:      "acme",
		Theme:        "phosphor",
		ColorProfile: "auto",
		Motd:         false,
		Menu:         false,
		Prompt:       false,
		Docs:         true,
		FlakeParts:   true,
	}, "assets/title.txt")

	if strings.Contains(config, "motd = {\n") {
		t.Fatal("disabled MOTD still emits an active motd block")
	}
	if !strings.Contains(config, "# motd = { enable = true; title.text = ./assets/title.txt; };") {
		t.Fatalf("disabled MOTD lost the title reference:\n%s", config)
	}
	if !strings.Contains(config, "menu.enable = false;") || !strings.Contains(config, "prompt.enable = false;") {
		t.Fatalf("component toggles not emitted:\n%s", config)
	}
	if !strings.Contains(config, "\n    docs.pages = [ { text = ./docs/getting-started.md; } ];") {
		t.Fatalf("enabled docs stayed commented:\n%s", config)
	}
}

func TestNixStringEscapesInterpolationAndQuotes(t *testing.T) {
	got := nixString(`a"b${c}\d`)
	want := `"a\"b\${c}\\d"`
	if got != want {
		t.Fatalf("nixString = %s, want %s", got, want)
	}
}

func TestNixPathNormalizesRelativeAndAbsolute(t *testing.T) {
	cases := map[string]string{
		"title.txt":        "./title.txt",
		"assets/title.txt": "./assets/title.txt",
		"./title.txt":      "./title.txt",
		"/tmp/title.txt":   "/tmp/title.txt",
		"../title.txt":     "../title.txt",
	}
	for input, want := range cases {
		if got := nixPath(input); got != want {
			t.Fatalf("nixPath(%q) = %q, want %q", input, got, want)
		}
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
		FlakeParts:   true,
	}
	var stdout, stderr bytes.Buffer
	code := finishWizard(m.cfg, m.render, result, "title.txt", &stdout, &stderr)
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
	if !strings.Contains(stdout.String(), "docs.pages = [ { text = ./docs/getting-started.md; } ];") {
		t.Fatalf("config missing docs pages:\n%s", stdout.String())
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
		Motd: true, Menu: true, Prompt: true, Docs: true, FlakeParts: true,
	}
	var stdout, stderr bytes.Buffer
	if code := finishWizard(m.cfg, m.render, result, "title.txt", &stdout, &stderr); code != 0 {
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
