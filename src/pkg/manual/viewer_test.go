package manual

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/shared"
)

func TestScrollingMovesViewportWithoutChangingSelectedSection(t *testing.T) {
	document := Document{Sections: []Section{
		{Title: "First"},
		{Title: "Second"},
		{Title: "Third"},
	}}
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 4})
	viewer, _ = viewer.Handle(keyPress("1"))
	before := viewer.render()

	viewer, _ = viewer.Handle(keyPress("j"))
	viewer, _ = viewer.Handle(keyPress("j"))

	if viewer.active != 0 {
		t.Fatalf("scroll changed selected section: got %d, want 0", viewer.active)
	}
	if after := viewer.render(); after == before {
		t.Fatal("scroll did not move the document viewport")
	}
}

func TestMarkdownPageRendersInViewport(t *testing.T) {
	document := Document{Sections: []Section{{
		Title:    "Getting started",
		Markdown: "# Getting started\n\nUse **Prelude** with `nix develop`.\n\n- First step\n- Second step",
	}}}
	viewer := New(document, testPalette())
	plain := ansi.Strip(viewer.viewport.GetContent())

	for _, want := range []string{"Getting started", "Prelude", "nix develop", "First step", "Second step"} {
		if !strings.Contains(plain, want) {
			t.Errorf("rendered Markdown does not contain %q:\n%s", want, plain)
		}
	}
	if count := strings.Count(plain, "Getting started"); count != 1 {
		t.Fatalf("page heading rendered %d times, want once:\n%s", count, plain)
	}
}

func TestViewportPaintsBackgroundOnEveryVisibleRow(t *testing.T) {
	document := Document{Sections: []Section{
		{Title: "First"},
		{Title: "Second"},
		{Title: "Third"},
	}}
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 8})

	for row, line := range strings.Split(viewer.viewport.View(), "\n") {
		if !strings.Contains(line, "\x1b[48;") {
			t.Errorf("viewport row %d has no background color: %q", row, line)
		}
	}
}

func TestMouseWheelMovesViewportWithoutChangingSelectedSection(t *testing.T) {
	document := Document{Sections: []Section{
		{Title: "First"},
		{Title: "Second"},
		{Title: "Third"},
	}}
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 4})
	viewer, _ = viewer.Handle(keyPress("1"))
	before := viewer.viewport.YOffset()

	viewer, _ = viewer.Handle(tea.MouseWheelMsg{Button: tea.MouseWheelDown})

	if viewer.active != 0 {
		t.Fatalf("mouse wheel changed selected section: got %d, want 0", viewer.active)
	}
	if after := viewer.viewport.YOffset(); after <= before {
		t.Fatalf("mouse wheel did not move the document viewport: offset stayed at %d", after)
	}
}

func keyPress(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: key, Code: []rune(key)[0]}
}

func testPalette() shared.Palette {
	return shared.Palette{
		Bg:      "#000000",
		Fg:      "#ffffff",
		Muted:   "#aaaaaa",
		Dim:     "#777777",
		Accent:  "#00ffff",
		Accent2: "#ff00ff",
		Border:  "#555555",
		Surface: "#111111",
	}
}
