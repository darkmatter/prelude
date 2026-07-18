package manual

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func TestH2RendersAsMarkdownHeading(t *testing.T) {
	document := Document{Sections: []Section{{
		Title:    "Guide",
		Markdown: "# Guide\n\nintro text\n\n## Workflow\n\nbody text\n\n```sh\n## not a heading\n```",
	}}}
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 40})

	var headingLine string
	for _, line := range strings.Split(viewer.viewport.GetContent(), "\n") {
		plain := ansi.Strip(line)
		if strings.Contains(plain, "Workflow") {
			headingLine = plain
			break
		}
	}
	if headingLine == "" {
		t.Fatal("H2 title missing from rendered page")
	}
	if !strings.Contains(headingLine, "## Workflow") {
		t.Fatalf("H2 did not render with its Markdown heading marker: %q", headingLine)
	}
	if strings.Contains(headingLine, "─") {
		t.Fatalf("H2 rendered as a labeled rule instead of a Markdown heading: %q", headingLine)
	}

	plain := ansi.Strip(viewer.viewport.GetContent())
	for _, want := range []string{"intro text", "body text", "## not a heading"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("rendered page missing %q:\n%s", want, plain)
		}
	}
}

func TestTabStepsThroughSectionsAndWraps(t *testing.T) {
	document := Document{Sections: []Section{
		{Title: "First"},
		{Title: "Second"},
		{Title: "Third"},
	}}
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 6})

	viewer, _ = viewer.Handle(tea.KeyPressMsg{Code: tea.KeyTab})
	if viewer.active != 1 {
		t.Fatalf("tab: active = %d, want 1", viewer.active)
	}
	// The viewport clamps to its max scroll, so assert movement toward the
	// section rather than an exact offset (digit jumps clamp identically).
	if viewer.viewport.YOffset() == 0 {
		t.Fatal("tab did not scroll the viewport toward the section")
	}

	viewer, _ = viewer.Handle(tea.KeyPressMsg{Code: tea.KeyTab})
	viewer, _ = viewer.Handle(tea.KeyPressMsg{Code: tea.KeyTab})
	if viewer.active != 0 {
		t.Fatalf("tab past the last section did not wrap: active = %d", viewer.active)
	}

	viewer, _ = viewer.Handle(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if viewer.active != 2 {
		t.Fatalf("shift+tab from the first section did not wrap back: active = %d", viewer.active)
	}
}

func TestViewportPaintsBackgroundOnEveryVisibleRow(t *testing.T) {
	document := Document{Sections: []Section{
		{Title: "First", Markdown: "# First\n\n## Sub section\n\nprose"},
		{Title: "Second"},
		{Title: "Third"},
	}}
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 8})

	hasBG := func(s string) bool {
		return strings.Contains(s, ";48;") || strings.Contains(s, "[48;")
	}
	for row, line := range strings.Split(viewer.viewport.View(), "\n") {
		if !hasBG(line) {
			t.Errorf("viewport row %d has no background color: %q", row, line)
		}
	}
	// Full screen rows (sidebar + body + status) must also carry a bg SGR so
	// nothing punches through to the terminal default.
	for row, line := range strings.Split(viewer.render(), "\n") {
		if !hasBG(line) {
			t.Errorf("screen row %d has no background color: %q", row, line)
		}
	}
}

func TestScreenPaintsBodyBackgroundThroughRightEdge(t *testing.T) {
	const width, height = 120, 8
	document := Document{Kind: KindDocs, Sections: []Section{{
		Title:    "Welcome",
		Markdown: "# Welcome\n\nShort content",
	}}}
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: width, Height: height})

	canvas := lipgloss.NewCanvas(width, height).
		Compose(lipgloss.NewLayer(viewer.render()))
	want := lipgloss.Color(string(testPalette().Bg))
	wantR, wantG, wantB, wantA := want.RGBA()
	for row := 1; row < height-1; row++ {
		cell := canvas.CellAt(width-1, row)
		if cell == nil || cell.Style.Bg == nil {
			t.Fatalf("rightmost cell on body row %d has no background: %#v", row, cell)
		}
		gotR, gotG, gotB, gotA := cell.Style.Bg.RGBA()
		if gotR != wantR || gotG != wantG || gotB != wantB || gotA != wantA {
			t.Fatalf("rightmost cell on body row %d background = %v, want %v", row, cell.Style.Bg, want)
		}
	}
}

func TestKindChromeDifferentiatesDocsFromHelp(t *testing.T) {
	help := New(Document{Kind: KindHelp, Sections: []Section{{Title: "synopsis"}}}, testPalette())
	docs := New(Document{Kind: KindDocs, Sections: []Section{{Title: "Welcome"}}}, testPalette())
	help, _ = help.Handle(tea.WindowSizeMsg{Width: 80, Height: 12})
	docs, _ = docs.Handle(tea.WindowSizeMsg{Width: 80, Height: 12})

	helpPlain := ansi.Strip(help.render())
	docsPlain := ansi.Strip(docs.render())
	for _, want := range []string{"MANUAL", "HELP"} {
		if !strings.Contains(helpPlain, want) {
			t.Errorf("help chrome missing %q:\n%s", want, helpPlain)
		}
	}
	for _, want := range []string{"PAGES", "DOCS"} {
		if !strings.Contains(docsPlain, want) {
			t.Errorf("docs chrome missing %q:\n%s", want, docsPlain)
		}
	}
	if strings.Contains(helpPlain, "PAGES") || strings.Contains(helpPlain, "DOCS") {
		t.Errorf("help chrome leaked docs labels:\n%s", helpPlain)
	}
	if strings.Contains(docsPlain, "MANUAL") || strings.Contains(docsPlain, "HELP :") {
		t.Errorf("docs chrome leaked help labels:\n%s", docsPlain)
	}
}

func TestMarkdownLinesHaveNoUnstyledLeadingCells(t *testing.T) {
	document := Document{Kind: KindDocs, Sections: []Section{{
		Title:    "Welcome",
		Markdown: "# Welcome\n\n**Prelude** with `code`.\n",
	}}}
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 16})
	for row, line := range strings.Split(viewer.viewport.GetContent(), "\n") {
		if line == "" {
			t.Errorf("content row %d is a raw empty string (unpainted hole)", row)
			continue
		}
		// A line that starts with a plain space (no leading CSI) has unstyled cells.
		if line[0] == ' ' {
			t.Errorf("content row %d has unstyled leading cells: %q", row, line[:min(40, len(line))])
		}
	}
}

func TestMarkdownStyleConfiguresChromaHighlighting(t *testing.T) {
	viewer := New(Document{Kind: KindDocs, Sections: []Section{{Title: "Welcome"}}}, testPalette())
	chroma := viewer.markdownStyle().CodeBlock.Chroma
	if chroma == nil {
		t.Fatal("code block style does not configure Chroma syntax highlighting")
	}
	if chroma.Keyword.Color == nil || chroma.LiteralString.Color == nil || chroma.Comment.Color == nil {
		t.Fatalf("code block Chroma style is missing token colors: %#v", chroma)
	}
	if *chroma.Keyword.Color == *chroma.LiteralString.Color {
		t.Fatalf("syntax token colors are not differentiated: keyword and string both use %q", *chroma.Keyword.Color)
	}
	if chroma.Text.BackgroundColor == nil || *chroma.Text.BackgroundColor != string(testPalette().Bg) {
		t.Fatalf("syntax token background = %#v, want %q", chroma.Text.BackgroundColor, testPalette().Bg)
	}
}

func TestCodeBlockTrailingWhitespaceUsesDocumentBackground(t *testing.T) {
	document := Document{Kind: KindDocs, Sections: []Section{{
		Title:    "Welcome",
		Markdown: "# Welcome\n\n```go\nconst name = \"prelude\"\n```\n",
	}}}
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 16})

	lines := strings.Split(viewer.viewport.GetContent(), "\n")
	codeRow := -1
	for row, line := range lines {
		if strings.Contains(ansi.Strip(line), "prelude") {
			codeRow = row
			break
		}
	}
	if codeRow == -1 {
		t.Fatalf("rendered code line missing:\n%s", ansi.Strip(viewer.viewport.GetContent()))
	}

	canvas := lipgloss.NewCanvas(viewer.l.textW, len(lines)).
		Compose(lipgloss.NewLayer(viewer.viewport.GetContent()))
	cell := canvas.CellAt(viewer.l.textW-1, codeRow)
	if cell == nil || cell.Style.Bg == nil {
		t.Fatalf("trailing code-block cell has no background: %#v", cell)
	}
	want := lipgloss.Color(string(testPalette().Bg))
	gotR, gotG, gotB, gotA := cell.Style.Bg.RGBA()
	wantR, wantG, wantB, wantA := want.RGBA()
	if gotR != wantR || gotG != wantG || gotB != wantB || gotA != wantA {
		t.Fatalf("trailing code-block background = %v, want document background %v", cell.Style.Bg, want)
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
		Success: "#00ff00",
		Warning: "#ffff00",
		Info:    "#0000ff",
		Error:   "#ff0000",
		Border:  "#555555",
		Surface: "#111111",
	}
}
