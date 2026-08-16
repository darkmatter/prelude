package wizard

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
			{Name: "nord", Palette: map[string]string{"accent": "#88c0d0", "bg": "#2e3440", "surface": "#3b4252", "fg": "#eceff4", "muted": "#d8dee9"}},
			{Name: "phosphor", Palette: map[string]string{"accent": "#68e371", "bg": "#0c110e", "surface": "#172119", "fg": "#d8f3dc", "muted": "#9fc8a5"}},
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

func ctrlD(t *testing.T, m wizardModel) wizardModel {
	t.Helper()
	return pressKey(t, m, tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
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

	// Commands come before every component/MOTD preview so previews can use them.
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

	// Components: the MOTD preview already contains dev; toggle docs on.
	if m.step != stepComponents {
		t.Fatalf("step = %d, want components", m.step)
	}
	if preview := m.componentPreview(); !strings.Contains(preview, "dev") {
		t.Fatalf("component preview does not use configured commands:\n%s", preview)
	}
	m = letter(t, m, 'j')
	m = letter(t, m, 'j')
	m = letter(t, m, 'j')
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = enter(t, m)

	if m.step != stepMotdContent || m.motdContentPhase != motdContentTagline {
		t.Fatalf("step=%d content phase=%d, want MOTD tagline", m.step, m.motdContentPhase)
	}
	m.motdContentInput.SetValue("Ship confidently")
	m = enter(t, m)
	m.motdDescriptionInput.SetValue("Everything needed to build, test,\nand ship.")
	m = ctrlD(t, m)
	if m.motdContentPhase != motdContentStatus || !m.motdContent.NixFlakeCheck {
		t.Fatalf("status phase=%d flake=%v, want default flake toggle", m.motdContentPhase, m.motdContent.NixFlakeCheck)
	}
	m = letter(t, m, 'j')
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = enter(t, m)
	if m.motdContentPhase != motdContentDevServerURL {
		t.Fatalf("content phase = %d, want dev server URL", m.motdContentPhase)
	}
	m.motdContentInput.SetValue("http://127.0.0.1:8080/health")
	m = enter(t, m)
	if m.step != stepMotdLayout {
		t.Fatalf("step = %d, want MOTD layout after content", m.step)
	}
	renderedPreview := m.motdPreview()
	for _, fragment := range []string{"dev", "Ship confidently", "Everything needed", "flake", "dev server"} {
		if !strings.Contains(renderedPreview, fragment) {
			t.Fatalf("MOTD preview does not use %q from commands/content:\n%s", fragment, renderedPreview)
		}
	}
	m = enter(t, m)
	if m.step != stepMotdSpacing {
		t.Fatalf("step = %d, want MOTD spacing", m.step)
	}
	m = enter(t, m)
	if m.step != stepMotdSurface {
		t.Fatalf("step = %d, want MOTD surface", m.step)
	}
	m = enter(t, m)
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
		Envrc:        true,
		Commands:     []wizardCommand{{Name: "dev", Exec: "make", Description: "run"}},
		MotdContent: wizardMotdContent{
			Tagline:            "Ship confidently",
			Description:        "Everything needed to build, test,\nand ship.",
			NixFlakeCheck:      true,
			DevServerStatus:    true,
			DevServerHealthURL: "http://127.0.0.1:8080/health",
		},
		MotdContentSet: true,
		MotdStyle:      defaultMotdStyle(),
		MotdStyleSet:   true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result = %#v, want %#v", got, want)
	}
}

func TestWizardEnvrcEvalsPreflight(t *testing.T) {
	if !strings.Contains(wizardEnvrcContents, "\nif has prelude-preflight; then\n  eval \"$(prelude-preflight)\"\nfi\n") {
		t.Fatalf("generated .envrc must eval prelude-preflight after use flake: %q", wizardEnvrcContents)
	}
}

func TestWizardEnvrcToggleDefaultsOnAndCanBeDisabled(t *testing.T) {
	m := testWizard()
	m.step = stepComponents
	m.componentIndex = componentEnvrc

	if !m.components[componentEnvrc] || !m.result().Envrc {
		t.Fatal(".envrc toggle should default on")
	}
	view := m.View().Content
	for _, fragment := range []string{"[x] .envrc", "use flake"} {
		if !strings.Contains(view, fragment) {
			t.Fatalf(".envrc toggle view missing %q:\n%s", fragment, view)
		}
	}

	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if m.components[componentEnvrc] || m.result().Envrc {
		t.Fatal(".envrc toggle did not turn off")
	}
}

var ansiBackgroundParam = regexp.MustCompile(`\x1b\[[0-9;]*48;(?:2;\d+;\d+;\d+|5;\d+)m`)

func TestWizardThemeBackgroundToggleAffectsOnlyViewer(t *testing.T) {
	m := testWizard()
	m.step = stepTheme
	wantIndex := m.themeIndex
	wantResult := m.result()

	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	viewerWithoutBackgrounds := strings.Join(append(m.themeRows(), m.themePreview()), "\n")
	if ansiBackgroundParam.MatchString(viewerWithoutBackgrounds) {
		t.Fatalf("theme viewer still emits ANSI background parameters when disabled:\n%q", viewerWithoutBackgrounds)
	}
	if m.themeIndex != wantIndex {
		t.Fatalf("themeIndex = %d, want %d after toggling backgrounds", m.themeIndex, wantIndex)
	}
	if got := m.result(); !reflect.DeepEqual(got, wantResult) {
		t.Fatalf("result changed after toggling theme viewer backgrounds:\n got: %#v\nwant: %#v", got, wantResult)
	}
	if footer := ansi.Strip(m.View().Content); !strings.Contains(footer, "space toggle backgrounds") {
		t.Fatalf("theme viewer footer does not advertise the background toggle:\n%s", footer)
	}

	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	viewerWithBackgrounds := strings.Join(append(m.themeRows(), m.themePreview()), "\n")
	if !ansiBackgroundParam.MatchString(viewerWithBackgrounds) {
		t.Fatalf("theme viewer did not restore ANSI background parameters:\n%q", viewerWithBackgrounds)
	}
}

func TestWizardMOTDStyleStepsCollectSelectionsAndPreview(t *testing.T) {
	m := testWizard()
	for range 5 {
		m = enter(t, m)
	}
	if m.step != stepCommands {
		t.Fatalf("step = %d, want commands before components", m.step)
	}
	m = enter(t, m)
	if m.step != stepComponents {
		t.Fatalf("step = %d, want components after commands", m.step)
	}
	m.themeIndex = 0 // use the complete nord palette for explicit background emission

	m = enter(t, m) // components -> MOTD content
	m = enter(t, m) // tagline
	m = ctrlD(t, m) // description
	m = enter(t, m) // default flake status on; dev server off
	if m.step != stepMotdLayout {
		t.Fatalf("step = %d, want MOTD layout", m.step)
	}
	m = letter(t, m, 'l') // block center -> right
	m = letter(t, m, 'j') // vertical field
	m = letter(t, m, 'h') // bottom -> center
	m = letter(t, m, 'j') // title field
	m = letter(t, m, 'l') // title center -> right
	if !strings.Contains(m.motdPreview(), "right block · center vertical") {
		t.Fatalf("preview does not reflect layout choices: %s", m.motdPreview())
	}
	m = enter(t, m)

	m = letter(t, m, 'l') // margin balanced -> spacious
	m = letter(t, m, 'j') // padding field
	m = letter(t, m, 'l') // padding roomy -> generous
	m = letter(t, m, 'j') // width field
	m = letter(t, m, 'h') // width wide -> compact
	m = enter(t, m)

	m = letter(t, m, 'l')                                              // card theme background -> surface color
	m = letter(t, m, 'j')                                              // border field
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}) // border on
	m = letter(t, m, 'j')                                              // clear-screen field
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}) // clear off
	m = enter(t, m)
	if m.step != stepConfirm {
		t.Fatalf("step = %d, want confirmation after MOTD pages", m.step)
	}

	got := m.result()
	wantStyle := motdStyle{
		Align:         "right",
		VerticalAlign: "center",
		TitleAlign:    "right",
		Border:        true,
		Background:    `"#3b4252"`,
		ClearScreen:   false,
		MarginX:       6,
		MarginY:       3,
		PaddingX:      6,
		PaddingY:      3,
		Width:         "80",
		MaxWidth:      "80",
	}
	if !reflect.DeepEqual(got.MotdStyle, wantStyle) || !got.MotdStyleSet {
		t.Fatalf("MOTD style = %#v (set=%v), want %#v", got.MotdStyle, got.MotdStyleSet, wantStyle)
	}
}

func TestWizardSkipsMOTDStepsWhenDisabled(t *testing.T) {
	m := testWizard()
	m.components[0] = false
	m.step = stepCommands
	m = enter(t, m)
	if m.step != stepComponents {
		t.Fatalf("step = %d, want components after commands", m.step)
	}
	current, total := m.stepProgress()
	if current != 7 || total != 8 {
		t.Fatalf("progress = %d/%d, want 7/8", current, total)
	}
	m = enter(t, m)
	if m.step != stepConfirm {
		t.Fatalf("step = %d, want confirm when MOTD is disabled", m.step)
	}
}

func TestWizardMOTDDefaultsUseProjectAndRequestedCopy(t *testing.T) {
	m := testWizard()
	if got := m.motdContent.Tagline; got != motdDefaultTagline {
		t.Fatalf("tagline = %q, want %q", got, motdDefaultTagline)
	}
	if got := m.motdContent.Description; got != defaultWizardMotdDescription("acme") {
		t.Fatalf("description = %q, want project-aware default", got)
	}
	if !m.motdContent.NixFlakeCheck || m.motdContent.DevServerStatus {
		t.Fatalf("default statuses: flake=%v server=%v", m.motdContent.NixFlakeCheck, m.motdContent.DevServerStatus)
	}
	if got := m.motdContent.DevServerHealthURL; got != motdDevServerHealthURLPlaceholder {
		t.Fatalf("health URL = %q, want %q", got, motdDevServerHealthURLPlaceholder)
	}

	m.step = stepProject
	m.projectInput.SetValue("prelude")
	m = enter(t, m)
	if got := m.motdContent.Description; got != defaultWizardMotdDescription("prelude") {
		t.Fatalf("updated description = %q, want project-aware default", got)
	}

	m.step = stepMotdContent
	m.focusMotdContentField(motdContentTagline)
	if got := m.motdContentInput.Placeholder; got != motdDefaultTagline {
		t.Fatalf("tagline placeholder = %q, want %q", got, motdDefaultTagline)
	}
	if view := m.View().Content; !strings.Contains(view, "Write a one-line description of your project") {
		t.Fatalf("tagline guidance missing from view:\n%s", view)
	}

	m.focusMotdContentField(motdContentDescription)
	if view := m.View().Content; !strings.Contains(view, "Write your welcome message (multiline okay)") {
		t.Fatalf("welcome heading missing from view:\n%s", view)
	}
}

func TestWizardMOTDDescriptionAcceptsMultipleLines(t *testing.T) {
	m := testWizard()
	m.step = stepMotdContent
	m.focusMotdContentField(motdContentDescription)
	m.motdDescriptionInput.SetValue("First line")
	m.motdDescriptionInput.MoveToEnd()

	m = enter(t, m)
	if m.motdContentPhase != motdContentDescription || !strings.Contains(m.motdDescriptionInput.Value(), "\n") {
		t.Fatalf("enter did not add a description line: phase=%d value=%q", m.motdContentPhase, m.motdDescriptionInput.Value())
	}
	m.motdDescriptionInput.SetValue("First line\nSecond line")
	m = ctrlD(t, m)
	if m.motdContentPhase != motdContentStatus || m.motdContent.Description != "First line\nSecond line" {
		t.Fatalf("multiline description not committed: phase=%d value=%q", m.motdContentPhase, m.motdContent.Description)
	}
}

func TestWizardMOTDStatusTogglesAndHealthURLDefault(t *testing.T) {
	m := testWizard()
	m.step = stepMotdContent
	m.focusMotdContentField(motdContentStatus)

	if !m.motdContent.NixFlakeCheck || m.motdContent.DevServerStatus {
		t.Fatalf("unexpected defaults: flake=%v server=%v", m.motdContent.NixFlakeCheck, m.motdContent.DevServerStatus)
	}
	m = letter(t, m, 'j')
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if !m.motdContent.DevServerStatus {
		t.Fatal("dev server status did not toggle on")
	}
	m = enter(t, m)
	if m.motdContentPhase != motdContentDevServerURL {
		t.Fatalf("phase = %d, want health URL", m.motdContentPhase)
	}
	if got := m.motdContentInput.Placeholder; got != motdDevServerHealthURLPlaceholder {
		t.Fatalf("URL placeholder = %q, want %q", got, motdDevServerHealthURLPlaceholder)
	}
	if got := m.motdContentInput.Value(); got != motdDevServerHealthURLPlaceholder {
		t.Fatalf("URL default = %q, want placeholder value", got)
	}

	m.motdContentInput.SetValue("")
	m = enter(t, m)
	if m.step != stepMotdLayout || m.motdContent.DevServerHealthURL != motdDevServerHealthURLPlaceholder {
		t.Fatalf("empty URL did not retain default: step=%d url=%q", m.step, m.motdContent.DevServerHealthURL)
	}
}

func TestMOTDDevServerHealthCommandQuotesURL(t *testing.T) {
	if got := motdDevServerHealthCommand(motdDevServerHealthURLPlaceholder); got != motdDevServerHealthCommandDefault {
		t.Fatalf("default health command = %q, want %q", got, motdDevServerHealthCommandDefault)
	}
	editedAppHostURL := `${APP_HOST:-http://127.0.0.1:3000}/ready`
	if err := validateMotdDevServerHealthURL(editedAppHostURL); err != nil {
		t.Fatalf("edited APP_HOST URL rejected: %v", err)
	}
	if got := motdDevServerHealthCommand(editedAppHostURL); got != `curl -fsS "${APP_HOST:-http://127.0.0.1:3000}/ready"` {
		t.Fatalf("edited APP_HOST command = %q", got)
	}
	if got := motdDevServerHealthCommand("http://localhost:3000/it's-ready"); got != `curl -fsS 'http://localhost:3000/it'"'"'s-ready'` {
		t.Fatalf("quoted health command = %q", got)
	}
}

func TestWizardMOTDRejectsInvalidHealthURL(t *testing.T) {
	m := testWizard()
	m.step = stepMotdContent
	m.motdContent.DevServerStatus = true
	m.focusMotdContentField(motdContentDevServerURL)
	m.motdContentInput.SetValue("curl example.com/health")

	m = enter(t, m)
	if m.step != stepMotdContent || m.motdContentPhase != motdContentDevServerURL || m.err == "" {
		t.Fatalf("invalid health URL accepted: step=%d phase=%d err=%q", m.step, m.motdContentPhase, m.err)
	}
}

func TestWizardMOTDStatusViewsShowTogglesAndURLWithoutOverflow(t *testing.T) {
	m := testWizard()
	m.step = stepMotdContent
	m.width = 80
	m.focusMotdContentField(motdContentStatus)

	view := m.View().Content
	for _, fragment := range []string{"[x] nix flake check", "[ ] dev server"} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("status toggle view missing %q:\n%s", fragment, view)
		}
	}

	m.motdContent.DevServerStatus = true
	m.focusMotdContentField(motdContentDevServerURL)
	view = m.View().Content
	for _, fragment := range []string{"APP_HOST", "127.0.0.1:3000"} {
		if !strings.Contains(view, fragment) {
			t.Fatalf("health URL view missing %q:\n%s", fragment, view)
		}
	}
	for index, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > m.width {
			t.Fatalf("health URL row %d width = %d, want <= %d:\n%s", index, got, m.width, view)
		}
	}
}

func TestWizardMOTDContentBackNavigation(t *testing.T) {
	m := testWizard()
	m.step = stepMotdContent
	m.focusMotdContentField(motdContentStatus)
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.motdContentPhase != motdContentDescription {
		t.Fatalf("esc from statuses: phase=%d, want description", m.motdContentPhase)
	}

	m.step = stepMotdLayout
	m.motdContent.DevServerStatus = false
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.step != stepMotdContent || m.motdContentPhase != motdContentStatus {
		t.Fatalf("esc from layout without server: step=%d phase=%d", m.step, m.motdContentPhase)
	}

	m.step = stepMotdLayout
	m.motdContent.DevServerStatus = true
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.step != stepMotdContent || m.motdContentPhase != motdContentDevServerURL {
		t.Fatalf("esc from layout with server: step=%d phase=%d", m.step, m.motdContentPhase)
	}
	m = pressKey(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.motdContentPhase != motdContentStatus {
		t.Fatalf("esc from URL: phase=%d, want statuses", m.motdContentPhase)
	}
}

func TestMOTDPreviewWidthPresetsScaleTo80ColumnCanvas(t *testing.T) {
	wantWidths := []int{48, 64, 72}
	for index, want := range wantWidths {
		if got := motdWidthPresets[index].PreviewWidth; got != want {
			t.Fatalf("%s preview width = %d, want %d", motdWidthPresets[index].Name, got, want)
		}
	}

	cardWidth, contentWidth, _ := previewCardDimensions(
		motdPreviewCanvasWidth,
		motdWidthPresets[1].PreviewWidth,
		motdMarginPresets[1].X,
		motdPaddingPresets[1].X,
		true,
	)
	if cardWidth != 64 || contentWidth != 54 {
		t.Fatalf("default bordered preview dimensions = %d/%d, want card/content 64/54", cardWidth, contentWidth)
	}
}

func TestMOTDPreviewUsesStaticCanvasAndFitsContent(t *testing.T) {
	if motdPreviewCanvasWidth != 80 {
		t.Fatalf("preview canvas width = %d, want 80", motdPreviewCanvasWidth)
	}

	m := testWizard()
	m.preview = strings.Repeat("wide title ", 8)
	var baseline string
	for _, width := range []int{24, 30, 40, 56, 80, 120} {
		m.width = width
		rendered := m.motdPreview()
		if baseline == "" {
			baseline = rendered
		} else if rendered != baseline {
			t.Fatalf("preview changed at window width %d; preview canvas should stay static", width)
		}
		for _, line := range strings.Split(rendered, "\n") {
			if got := lipgloss.Width(line); got > motdPreviewCanvasWidth {
				t.Fatalf("width %d: preview line width = %d, want <= %d:\n%s", width, got, motdPreviewCanvasWidth, rendered)
			}
		}
	}
}

func TestMOTDPreviewBorderDefaultsOffAndToggles(t *testing.T) {
	m := testWizard()
	withoutBorder := m.motdPreview()
	if strings.Contains(withoutBorder, "╭") || strings.Contains(withoutBorder, "╰") {
		t.Fatalf("default MOTD preview rendered a border:\n%s", withoutBorder)
	}

	m.motdBorder = true
	withBorder := m.motdPreview()
	if !strings.Contains(withBorder, "╭") || !strings.Contains(withBorder, "╰") {
		t.Fatalf("enabled MOTD border was not rendered:\n%s", withBorder)
	}
}

func TestPreviewTitleAlignmentPreservesFIGletShape(t *testing.T) {
	const title = "AA\nAAAA"
	for _, test := range []struct {
		align  string
		offset int
	}{
		{align: "left", offset: 0},
		{align: "center", offset: 8},
		{align: "right", offset: 16},
	} {
		t.Run(test.align, func(t *testing.T) {
			lines := strings.Split(alignPreviewTitleBlock(title, 20, test.align), "\n")
			if len(lines) != 2 {
				t.Fatalf("aligned title has %d lines, want 2: %q", len(lines), lines)
			}
			for index, line := range lines {
				if got := len(line) - len(strings.TrimLeft(line, " ")); got != test.offset {
					t.Fatalf("line %d leading spaces = %d, want %d: %q", index, got, test.offset, line)
				}
			}
		})
	}
}

func TestMOTDPreviewOmitsDitherFringe(t *testing.T) {
	rendered := ansi.Strip(testWizard().motdPreview())
	if strings.ContainsAny(rendered, "░▒▓") {
		t.Fatalf("MOTD preview rendered obsolete fringe glyphs:\n%s", rendered)
	}
}

func TestMOTDPreviewRendersFixedCardShape(t *testing.T) {
	m := testWizard()
	m.motdBorder = true
	m.motdWordmark = motdPreviewSampleText
	m.commands = []wizardCommand{{Name: "check"}, {Name: "test"}, {Name: "build"}}
	m.motdContent.NixFlakeCheck = true
	m.motdContent.DevServerStatus = true

	rendered := m.motdPreview()
	lines := strings.Split(rendered, "\n")
	if len(lines) < 15 {
		t.Fatalf("MOTD preview has %d lines, want at least 15:\n%s", len(lines), rendered)
	}
	if got := lipgloss.Width(rendered); got != motdPreviewCanvasWidth {
		t.Fatalf("MOTD preview width = %d, want static canvas width %d:\n%s", got, motdPreviewCanvasWidth, rendered)
	}
	for _, fragment := range []string{
		"live MOTD preview",
		"╭",
		motdPreviewSampleText,
		"flake  pending",
		"dev server  pending",
		"Fancy devshells for your nix",
		"check  ·  test  ·  build",
		"You are now in",
		"center block",
	} {
		if !strings.Contains(rendered, fragment) {
			t.Fatalf("MOTD preview missing %q:\n%s", fragment, rendered)
		}
	}
	titleLine, dividerLine, statusLine, taglineLine := -1, -1, -1, -1
	for index, line := range lines {
		switch {
		case titleLine < 0 && strings.Contains(line, motdPreviewSampleText):
			titleLine = index
		case titleLine >= 0 && statusLine < 0 && strings.Contains(line, "─"):
			dividerLine = index
		case strings.Contains(line, "flake  pending"):
			statusLine = index
		case strings.Contains(line, "Fancy devshells"):
			taglineLine = index
		}
	}
	if !(titleLine < dividerLine && dividerLine < statusLine && statusLine < taglineLine) {
		t.Fatalf(
			"preview status is not beneath the divider: title=%d divider=%d status=%d tagline=%d\n%s",
			titleLine,
			dividerLine,
			statusLine,
			taglineLine,
			rendered,
		)
	}
	// The first and last lines belong to the preview label and metadata. Every
	// viewport row is padded to the static canvas width, including empty rows
	// used for vertical placement.
	viewport := lines[1 : len(lines)-1]
	for index, line := range viewport {
		if got := lipgloss.Width(line); got != motdPreviewCanvasWidth {
			t.Fatalf("viewport row %d width = %d, want %d:\n%s", index, got, motdPreviewCanvasWidth, rendered)
		}
	}

	t.Logf("rendered MOTD preview:\n%s", rendered)
}

func TestMOTDPreviewUsesConfiguredMarginAndPadding(t *testing.T) {
	base := testWizard()
	base.motdWordmark = motdPreviewSampleText
	base.motdAlignIndex = 0         // left
	base.motdVerticalAlignIndex = 0 // top
	base.motdMarginIndex = 0        // compact
	base.motdPaddingIndex = 0       // compact
	baseRendered := base.motdPreview()

	padding := base
	padding.motdPaddingIndex = 2 // generous
	paddingRendered := padding.motdPreview()
	if got, want := lipgloss.Height(paddingRendered) > lipgloss.Height(baseRendered), true; got != want {
		t.Fatalf("generous padding did not increase preview height:\nbase:\n%s\n\ngenerous:\n%s", baseRendered, paddingRendered)
	}

	margin := base
	margin.motdMarginIndex = 2 // spacious
	marginRendered := margin.motdPreview()
	if got, want := lipgloss.Height(marginRendered) > lipgloss.Height(baseRendered), true; got != want {
		t.Fatalf("spacious margin did not increase preview height:\nbase:\n%s\n\nspacious:\n%s", baseRendered, marginRendered)
	}

	for name, rendered := range map[string]string{
		"base":    baseRendered,
		"padding": paddingRendered,
		"margin":  marginRendered,
	} {
		if got := lipgloss.Width(rendered); got != motdPreviewCanvasWidth {
			t.Fatalf("%s preview width = %d, want %d", name, got, motdPreviewCanvasWidth)
		}
	}
	if !strings.Contains(baseRendered, "left block · top vertical") {
		t.Fatalf("base preview does not show the configured non-centered layout:\n%s", baseRendered)
	}

	rightBottom := base
	rightBottom.motdAlignIndex = 2
	rightBottom.motdVerticalAlignIndex = 2
	if rendered := rightBottom.motdPreview(); !strings.Contains(rendered, "right block · bottom vertical") {
		t.Fatalf("preview does not show the alternate layout:\n%s", rendered)
	}
}

func TestRefreshMOTDWordmarkUsesFixedMiniFont(t *testing.T) {
	m := testWizard()
	m.cfg.Fonts = append(m.cfg.Fonts, Font{Name: motdPreviewFontName, Path: "/mini"})

	m.refreshPreview()
	if m.preview != "thin:acme" {
		t.Fatalf("selected-font preview = %q, want thin:acme", m.preview)
	}
	if m.motdWordmark != "mini:acme" {
		t.Fatalf("MOTD wordmark = %q, want mini:acme", m.motdWordmark)
	}
}

func TestRefreshMOTDWordmarkAlwaysUsesAcmeSample(t *testing.T) {
	m := testWizard()
	m.cfg.Fonts = append(m.cfg.Fonts, Font{Name: motdPreviewFontName, Path: "/mini"})
	m.titleInput.SetValue("custom title")
	m.projectInput.SetValue("customer-project")

	m.refreshMotdWordmark()
	if m.motdWordmark != "mini:"+motdPreviewSampleText {
		t.Fatalf("MOTD wordmark = %q, want mini:%s", m.motdWordmark, motdPreviewSampleText)
	}
}

func TestMOTDPreviewIgnoresSelectedFontOutput(t *testing.T) {
	m := testWizard()
	m.cfg.Fonts = append(m.cfg.Fonts, Font{Name: motdPreviewFontName, Path: "/mini"})
	m.refreshMotdWordmark()

	m.preview = strings.Repeat("selected FIGlet output ", 20)
	baseline := m.motdPreview()
	m.preview = "another selected font with different geometry"
	if got := m.motdPreview(); got != baseline {
		t.Fatalf("MOTD preview changed with selected-font output:\nbase:\n%s\n\ngot:\n%s", baseline, got)
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

func TestWizardReviewFitsLongMultilineWelcomeMessage(t *testing.T) {
	m := testWizard()
	m.step = stepConfirm
	m.width = 80
	m.motdContent.Description = strings.Repeat("Long welcome copy ", 12) + "\nSecond paragraph"

	for index, row := range m.summaryRows(newFormStyles()) {
		if strings.Contains(row, "\n") {
			t.Fatalf("summary row %d contains a raw newline: %q", index, row)
		}
	}
	view := m.View().Content
	for index, line := range strings.Split(view, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if got := lipgloss.Width(line); got > m.width {
			t.Fatalf("review row %d width = %d, want <= %d:\n%s", index, got, m.width, view)
		}
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
		"# Generated by prelude wizard.",
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
		`align = "center";`,
		"background = true;",
		"border = false;",
		`verticalAlign = "bottom";`,
		"maxWidth = 100;",
		"margin = {",
		"x = 4;",
		"minHeight = 40;",
		"padding = {",
		`text = "Fancy devshells for your nix flake";`,
		`text = "You are now in the acme-web dev environment, powered by Nix Flakes. All required dependencies, project scripts, and documentation are available in this environment.";`,
		"flake = {",
		`check = "nix flake check";`,
		`# devServer = { label = "dev server";`,
		"# statusHint = {",
		`#   layout = "inline";`,
		"menu = {",
		"enable = true;",
		"# height = 20;",
		"# maxWidth = 80;",
		"prompt = {",
		"docs.pages = [ ];",
		`# sort.groups = [ "develop" "database" "ops" ];`,
		"packages = [",
		"#           config.packages.prelude-motd",
		"#           config.packages.prelude-menu",
	} {
		if !strings.Contains(got, fragment) {
			t.Fatalf("options template missing fragment %q:\n%s", fragment, got)
		}
	}
	if strings.Contains(got, "#           config.packages.prelude-docs") {
		t.Fatalf("disabled docs package leaked into devshell template:\n%s", got)
	}
	if strings.Contains(got, "prelude.json") || strings.Contains(got, "fromJSON") {
		t.Fatalf("template must not use JSON sidecar:\n%s", got)
	}
}

func TestRenderWizardConfigEmitsSelectedMOTDStyle(t *testing.T) {
	result := wizardResult{
		Recipe:       Recipe{Text: "acme", Font: "thin"},
		Project:      "acme",
		Theme:        "nord",
		ColorProfile: "auto",
		Motd:         true,
		Menu:         false,
		Prompt:       false,
		MotdStyle: motdStyle{
			Align:           "right",
			VerticalAlign:   "center",
			TitleAlign:      "left",
			Border:          true,
			Background:      `"#112233"`,
			ClearScreen:     false,
			MarginX:         6,
			MarginY:         3,
			MarginMinHeight: 30,
			PaddingX:        1,
			PaddingY:        0,
			Width:           "80",
			MaxWidth:        "80",
		},
		MotdStyleSet: true,
	}
	for name, rendered := range map[string]string{
		"flake-parts": renderWizardConfig(result, "title.txt"),
		"standalone":  renderStandaloneConfig(result, "title.txt"),
	} {
		for _, fragment := range []string{
			`align = "left";`,
			`background = "#112233";`,
			"border = true;",
			"clearScreen = false;",
			`align = "right";`,
			`verticalAlign = "center";`,
			"width = 80;",
			"maxWidth = 80;",
			"x = 6;",
			"y = 3;",
			"minHeight = 30;",
			"padding = {",
		} {
			if !strings.Contains(rendered, fragment) {
				t.Fatalf("%s template missing selected MOTD fragment %q:\n%s", name, fragment, rendered)
			}
		}
	}
}

func TestRenderWizardConfigEmitsMOTDContentAndStatus(t *testing.T) {
	result := wizardResult{
		Recipe:       Recipe{Text: "acme", Font: "thin"},
		Project:      "acme",
		Theme:        "nord",
		ColorProfile: "auto",
		Motd:         true,
		MotdContent: wizardMotdContent{
			Tagline:            "Ready to ship",
			Description:        "Everything needed for local development.\nWelcome aboard.",
			NixFlakeCheck:      true,
			DevServerStatus:    true,
			DevServerHealthURL: motdDevServerHealthURLPlaceholder,
		},
		MotdContentSet: true,
	}
	for name, rendered := range map[string]string{
		"flake-parts": renderWizardConfig(result, "title.txt"),
		"standalone":  renderStandaloneConfig(result, "title.txt"),
	} {
		for _, fragment := range []string{
			`text = "Ready to ship";`,
			`text = "Everything needed for local development.\nWelcome aboard.";`,
			"flake = {",
			`label = "flake";`,
			`check = "nix flake check";`,
			"devServer = {",
			`label = "dev server";`,
			`check = "curl -fsS \"\${APP_HOST:-http://127.0.0.1:3000}/health\"";`,
			"async = true;",
		} {
			if !strings.Contains(rendered, fragment) {
				t.Fatalf("%s template missing MOTD content fragment %q:\n%s", name, fragment, rendered)
			}
		}
	}
}

func TestRenderWizardConfigHonorsDisabledMOTDStatuses(t *testing.T) {
	result := wizardResult{
		Recipe:       Recipe{Text: "acme", Font: "thin"},
		Project:      "acme",
		Theme:        "nord",
		ColorProfile: "auto",
		Motd:         true,
		MotdContent: wizardMotdContent{
			Tagline:            motdDefaultTagline,
			Description:        defaultWizardMotdDescription("acme"),
			DevServerHealthURL: motdDevServerHealthURLPlaceholder,
		},
		MotdContentSet: true,
	}
	for name, rendered := range map[string]string{
		"flake-parts": renderWizardConfig(result, "title.txt"),
		"standalone":  renderStandaloneConfig(result, "title.txt"),
	} {
		for _, active := range []string{"\n          flake = {", "\n          devServer = {"} {
			if strings.Contains(rendered, active) {
				t.Fatalf("%s template emitted disabled status %q:\n%s", name, active, rendered)
			}
		}
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
		Envrc:        true,
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
	envrc, err := os.ReadFile(wizardEnvrcPath)
	if err != nil || string(envrc) != wizardEnvrcContents {
		t.Fatalf("%s = %q, err %v", wizardEnvrcPath, envrc, err)
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
		"background = true;",
		"border = false;",
		`verticalAlign = "bottom";`,
		"maxWidth = 100;",
	} {
		if !strings.Contains(string(nixData), fragment) {
			t.Fatalf("prelude.nix missing %q:\n%s", fragment, nixData)
		}
	}
	for _, notice := range []string{"wrote title.txt\n", "wrote " + wizardEnvrcPath + "\n", "wrote prelude.nix\n"} {
		if !strings.Contains(stderr.String(), notice) {
			t.Fatalf("stderr missing %q: %s", notice, stderr.String())
		}
	}
	if !strings.Contains(stderr.String(), "import this module from your flake.nix") {
		t.Fatalf("stderr missing import next steps: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "./prelude.nix") {
		t.Fatalf("stderr missing import path: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "config.packages.prelude-shell") ||
		!strings.Contains(stderr.String(), "each enabled prelude-* component package") {
		t.Fatalf("stderr missing shell package next steps: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "direnv log lines") || !strings.Contains(stderr.String(), "log_format = \"-\"") {
		t.Fatalf("stderr missing direnv tip (MOTD enabled): %s", stderr.String())
	}
}

func TestFinishWizardSkipsEnvrcWhenDisabled(t *testing.T) {
	t.Chdir(t.TempDir())
	m := testWizard()
	result := wizardResult{
		Recipe:  Recipe{Text: "acme", Font: "thin"},
		Project: "acme", Theme: "nord", ColorProfile: "auto",
		Motd: true, Menu: true, Prompt: true, Envrc: false,
	}
	var stderr bytes.Buffer
	if code := finishWizard(m.cfg, m.render, result, "prelude.nix", &stderr); code != 0 {
		t.Fatalf("finishWizard = %d, stderr: %s", code, stderr.String())
	}
	if _, err := os.Lstat(wizardEnvrcPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s should not be written when disabled, err=%v", wizardEnvrcPath, err)
	}
	if strings.Contains(stderr.String(), wizardEnvrcPath) {
		t.Fatalf("stderr should not mention disabled %s: %s", wizardEnvrcPath, stderr.String())
	}
}

func TestMaterializeWizardEnvrcKeepsExistingFile(t *testing.T) {
	t.Chdir(t.TempDir())
	const existing = "source_env .envrc.local\n"
	if err := os.WriteFile(wizardEnvrcPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	if err := materializeWizardEnvrc(&stderr); err != nil {
		t.Fatalf("materializeWizardEnvrc: %v", err)
	}
	got, err := os.ReadFile(wizardEnvrcPath)
	if err != nil || string(got) != existing {
		t.Fatalf("existing %s was changed: %q, err %v", wizardEnvrcPath, got, err)
	}
	if !strings.Contains(stderr.String(), "kept existing "+wizardEnvrcPath) {
		t.Fatalf("stderr missing keep notice: %s", stderr.String())
	}
}

func TestFinishWizardOmitsDirenvTipWhenMotdDisabled(t *testing.T) {
	t.Chdir(t.TempDir())
	m := testWizard()
	result := wizardResult{
		Recipe:  Recipe{Text: "acme", Font: "thin"},
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
		Motd: true, Menu: true, Prompt: true, Envrc: true,
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
	envrc, err := os.ReadFile(wizardEnvrcPath)
	if err != nil || string(envrc) != wizardEnvrcContents {
		t.Fatalf("project-root %s = %q, err %v", wizardEnvrcPath, envrc, err)
	}
	if _, err := os.Stat("nix/.envrc"); !os.IsNotExist(err) {
		t.Fatalf("nested .envrc should not be written, err=%v", err)
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
