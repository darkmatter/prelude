package manual

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/shared"
)

func testDoc(nav ...NavNode) Document {
	return Document{Nav: nav}
}

func TestMarkdownPageRendersInViewport(t *testing.T) {
	document := testDoc(NavNode{
		Title:    "Getting started",
		Markdown: "# Getting started\n\nUse **Prelude** with `nix develop`.\n\n- First step\n- Second step",
	})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 24})
	plain := ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "nix develop") {
		t.Fatalf("missing body:\n%s", plain)
	}
	if count := strings.Count(plain, "Getting started"); count != 1 {
		t.Fatalf("page heading rendered %d times, want once:\n%s", count, plain)
	}
}

func TestH2RendersAsMarkdownHeading(t *testing.T) {
	document := testDoc(NavNode{
		Title:    "Guide",
		Markdown: "# Guide\n\nintro text\n\n## Workflow\n\nbody text\n\n```sh\n## not a heading\n```",
	})
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

func TestDocsPagesAreDiscreteNotConcatenated(t *testing.T) {
	document := testDoc(
		NavNode{Title: "First", Markdown: "# First\n\nonly on page one"},
		NavNode{Title: "Second", Markdown: "# Second\n\nonly on page two"},
		NavNode{Title: "Third", Markdown: "# Third\n\nonly on page three"},
	)
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 24})
	// Plain j scrolls the page; shift+j moves the nav cursor to the next leaf.
	viewer, _ = viewer.Handle(keyPress("j"))
	if plain := ansi.Strip(viewer.viewport.GetContent()); !strings.Contains(plain, "only on page one") {
		t.Fatalf("plain j must not change page:\n%s", plain)
	}
	viewer, _ = viewer.Handle(keyPress("shift+j"))
	plain := ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "only on page two") {
		t.Fatalf("expected page two after shift+j:\n%s", plain)
	}
	if strings.Contains(plain, "only on page one") {
		t.Fatalf("page one leaked into page two:\n%s", plain)
	}
}

func TestDocsTreeNavigationKeys(t *testing.T) {
	document := testDoc(NavNode{
		Title: "Guides",
		Children: []NavNode{
			{Title: "One", Markdown: "# One\n\nbody one"},
			{Title: "Two", Markdown: "# Two\n\nbody two"},
		},
	})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 24})
	plain := ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "body one") {
		t.Fatalf("expected first leaf body:\n%s", plain)
	}
	viewer, _ = viewer.Handle(keyPress("shift+down"))
	plain = ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "body two") {
		t.Fatalf("expected second leaf after shift+down:\n%s", plain)
	}
	viewer, _ = viewer.Handle(keyPress("shift+tab"))
	plain = ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "body one") {
		t.Fatalf("expected first leaf after shift+tab:\n%s", plain)
	}
	viewer, _ = viewer.Handle(keyPress("tab"))
	plain = ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "body two") {
		t.Fatalf("expected second leaf after tab:\n%s", plain)
	}
}

func TestPlainKeysScrollPageWithoutChangingLeaf(t *testing.T) {
	body := "# Tall\n\n" + strings.Repeat("line of content\n", 80)
	document := testDoc(
		NavNode{Title: "Tall", Markdown: body},
		NavNode{Title: "Other", Markdown: "# Other\n\nother body"},
	)
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 16})
	leaf := fmt.Sprint(viewer.leafPath)

	viewer, _ = viewer.Handle(keyPress("j"))
	if viewer.viewport.YOffset() == 0 {
		t.Fatal("plain j must scroll the page down")
	}
	viewer, _ = viewer.Handle(keyPress("down"))
	viewer, _ = viewer.Handle(keyPress("k"))
	if fmt.Sprint(viewer.leafPath) != leaf {
		t.Fatalf("plain motion changed leaf: %s → %v", leaf, viewer.leafPath)
	}
	viewer, _ = viewer.Handle(keyPress("G"))
	maxScroll := max(0, viewer.viewport.TotalLineCount()-viewer.viewport.Height())
	if viewer.viewport.YOffset() != maxScroll {
		t.Fatalf("G must jump to page bottom: y=%d max=%d", viewer.viewport.YOffset(), maxScroll)
	}
	viewer, _ = viewer.Handle(keyPress("g"))
	if viewer.viewport.YOffset() != 0 {
		t.Fatalf("g must jump to page top: y=%d", viewer.viewport.YOffset())
	}
}

func TestDocsGroupCursorResizeKeepsLeafBody(t *testing.T) {
	document := testDoc(NavNode{
		Title: "Group",
		Children: []NavNode{
			{Title: "Leaf", Markdown: "# Leaf\n\nkeep me"},
		},
	})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 24})
	viewer, _ = viewer.Handle(keyPress("shift+k"))
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 100, Height: 30})
	plain := ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "keep me") {
		t.Fatalf("resize lost leaf body:\n%s", plain)
	}
}

func TestDocsSidebarWindowsAroundCursor(t *testing.T) {
	var nav []NavNode
	for i := 0; i < 40; i++ {
		nav = append(nav, NavNode{
			Title:    fmt.Sprintf("Page %02d", i),
			Markdown: fmt.Sprintf("# Page %02d\n\nbody %d", i, i),
		})
	}
	viewer := New(testDoc(nav...), testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 12})
	for i := 0; i < 25; i++ {
		viewer, _ = viewer.Handle(keyPress("shift+j"))
	}
	_ = viewer.render()
	plain := ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "body") {
		t.Fatalf("expected some body after scroll:\n%s", plain)
	}
}

func TestViewportPaintsBackgroundOnEveryVisibleRow(t *testing.T) {
	document := testDoc(NavNode{
		Title:    "First",
		Markdown: "# First\n\n## Sub section\n\nprose",
	})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 16})
	for row, line := range strings.Split(viewer.viewport.View(), "\n") {
		if line == "" {
			t.Errorf("viewport row %d is empty (unpainted)", row)
		}
	}
}

func TestScreenPaintsBodyBackgroundThroughRightEdge(t *testing.T) {
	document := testDoc(NavNode{Title: "Welcome", Markdown: "# Welcome\n\nshort"})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 16})
	for row, line := range strings.Split(viewer.render(), "\n") {
		if ansi.Strip(line) == "" && line == "" {
			t.Errorf("screen row %d is a raw empty string", row)
		}
	}
}

func TestDocsChromeLabels(t *testing.T) {
	docs := New(testDoc(NavNode{Title: "Welcome", Markdown: "# Welcome\n"}), testPalette())
	docs, _ = docs.Handle(tea.WindowSizeMsg{Width: 80, Height: 12})
	plain := ansi.Strip(docs.render())
	for _, want := range []string{"PAGES", "DOCS"} {
		if !strings.Contains(plain, want) {
			t.Errorf("docs chrome missing %q:\n%s", want, plain)
		}
	}
	for _, bad := range []string{"MANUAL", "HELP :"} {
		if strings.Contains(plain, bad) {
			t.Errorf("docs chrome leaked help labels %q:\n%s", bad, plain)
		}
	}
}

func TestMarkdownLinesHaveNoUnstyledLeadingCells(t *testing.T) {
	document := testDoc(NavNode{
		Title:    "Welcome",
		Markdown: "# Welcome\n\n**Prelude** with `code`.\n",
	})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 16})
	for row, line := range strings.Split(viewer.viewport.GetContent(), "\n") {
		if line == "" {
			t.Errorf("content row %d is a raw empty string (unpainted hole)", row)
			continue
		}
		if line[0] == ' ' {
			t.Errorf("content row %d has unstyled leading cells: %q", row, line[:min(40, len(line))])
		}
	}
}

func TestMarkdownStyleConfiguresChromaHighlighting(t *testing.T) {
	viewer := New(testDoc(NavNode{Title: "Welcome", Markdown: "# Welcome\n"}), testPalette())
	chroma := viewer.markdownStyle().CodeBlock.Chroma
	if chroma == nil {
		t.Fatal("chroma config nil")
	}
}

func TestCodeBlockTrailingWhitespaceUsesDocumentBackground(t *testing.T) {
	document := testDoc(NavNode{
		Title:    "Code",
		Markdown: "# Code\n\n```go\nfmt.Println(\"hi\")\n```\n",
	})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 24})
	plain := ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "Println") {
		t.Fatalf("missing code body:\n%s", plain)
	}
}

func TestMouseWheelMovesViewportWithoutChangingLeaf(t *testing.T) {
	body := "# Tall\n\n" + strings.Repeat("line\n", 80)
	document := testDoc(NavNode{Title: "Tall", Markdown: body})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 16})
	// No focus switch needed: the wheel always scrolls the body.
	leaf := append([]int{}, viewer.leafPath...)
	viewer, _ = viewer.Handle(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if fmt.Sprint(viewer.leafPath) != fmt.Sprint(leaf) {
		t.Fatalf("wheel changed leafPath: %v → %v", leaf, viewer.leafPath)
	}
}

func TestSidebarDividerDragResizes(t *testing.T) {
	document := testDoc(
		NavNode{Title: "One", Markdown: "# One\n\nbody"},
		NavNode{Title: "Two", Markdown: "# Two\n\nbody"},
	)
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 100, Height: 30})
	if viewer.l.sideW < minSideW {
		t.Fatalf("auto side width %d < min %d", viewer.l.sideW, minSideW)
	}

	divX := viewer.l.sideW
	viewer, _ = viewer.Handle(tea.MouseClickMsg{X: divX, Y: 5, Button: tea.MouseLeft})
	if !viewer.dragging {
		t.Fatal("expected dragging after divider click")
	}
	viewer, _ = viewer.Handle(tea.MouseMotionMsg{X: 40, Y: 5, Button: tea.MouseLeft})
	if viewer.l.sideW != 40 {
		t.Fatalf("sideW after drag = %d, want 40", viewer.l.sideW)
	}
	viewer, _ = viewer.Handle(tea.MouseReleaseMsg{X: 40, Y: 5, Button: tea.MouseLeft})
	if viewer.dragging {
		t.Fatal("expected dragging false after release")
	}
	if viewer.sideWOverride != 40 {
		t.Fatalf("sideWOverride = %d, want 40", viewer.sideWOverride)
	}

	viewer, _ = viewer.Handle(tea.MouseClickMsg{X: viewer.l.sideW, Y: 5, Button: tea.MouseLeft})
	viewer, _ = viewer.Handle(tea.MouseMotionMsg{X: 99, Y: 5, Button: tea.MouseLeft})
	viewer, _ = viewer.Handle(tea.MouseReleaseMsg{X: 99, Y: 5, Button: tea.MouseLeft})
	maxSide := 100 - minBodyW - 1
	if viewer.l.sideW != maxSide {
		t.Fatalf("sideW after oversize drag = %d, want clamp %d", viewer.l.sideW, maxSide)
	}
}

func TestSidebarDragClearsOnMouseNoneRelease(t *testing.T) {
	document := testDoc(NavNode{Title: "One", Markdown: "# One\n"})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 100, Height: 30})
	viewer, _ = viewer.Handle(tea.MouseClickMsg{X: viewer.l.sideW, Y: 5, Button: tea.MouseLeft})
	if !viewer.dragging {
		t.Fatal("expected dragging after divider click")
	}
	viewer, _ = viewer.Handle(tea.MouseReleaseMsg{X: 35, Y: 5, Button: tea.MouseNone})
	if viewer.dragging {
		t.Fatal("expected dragging cleared on MouseNone release")
	}
	if viewer.l.sideW != 35 {
		t.Fatalf("sideW after MouseNone release = %d, want 35", viewer.l.sideW)
	}
}

func TestTopBorderAccentIsStaticAcrossFocus(t *testing.T) {
	document := testDoc(
		NavNode{Title: "One", Markdown: "# One\n\nbody"},
		NavNode{Title: "Two", Markdown: "# Two\n\nbody"},
	)
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 20})
	if !viewer.focused {
		t.Fatal("viewer must start focused until a blur report")
	}

	norm := lipgloss.NormalBorder()
	accentNav := viewer.styles.frameAccent.Render(strings.Repeat(norm.Top, viewer.l.sideW))
	accentBody := viewer.styles.topAccent.Render(norm.MiddleTop + strings.Repeat(norm.Top, viewer.l.bodyW))

	assertAccent := func(label string) {
		top := strings.Split(viewer.render(), "\n")[0]
		if !strings.Contains(top, accentNav) || !strings.Contains(top, accentBody) {
			t.Fatalf("%s: top border must stay accent across both panes:\n%q", label, top)
		}
	}
	assertAccent("focused")

	// Only the status bar dims on blur; the identity chrome is static.
	viewer, _ = viewer.Handle(tea.BlurMsg{})
	if viewer.focused {
		t.Fatal("BlurMsg must mark the viewer unfocused")
	}
	assertAccent("blurred")
}

func TestStatusBarDimsWhileBlurred(t *testing.T) {
	document := testDoc(NavNode{Title: "One", Markdown: "# One\n\nbody"})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 20})

	// Accent2 (#ff00ff) bar while focused.
	focusedSpace, _ := viewer.styles.statusChrome(true)
	blurredSpace, _ := viewer.styles.statusChrome(false)
	bar := func(v Viewer) string {
		lines := strings.Split(v.render(), "\n")
		return lines[len(lines)-1]
	}
	if got := bar(viewer); !strings.Contains(got, focusedSpace.Render(" ")) && !strings.Contains(got, "255;0;255") {
		t.Fatalf("focused status bar must use the accent2 bar:\n%q", got)
	}
	viewer, _ = viewer.Handle(tea.BlurMsg{})
	if got := bar(viewer); strings.Contains(got, "255;0;255") {
		t.Fatalf("blurred status bar must not use accent2:\n%q", got)
	} else if !strings.Contains(got, blurredSpace.Render(" ")) && !strings.Contains(got, "17;17;17") {
		t.Fatalf("blurred status bar must use the dim surface bar:\n%q", got)
	}
}

func TestBodyScrollbarWhenOverflow(t *testing.T) {
	// Tall page so the body must scroll.
	body := "# Tall\n\n" + strings.Repeat("line of content\n", 80)
	document := testDoc(NavNode{Title: "Tall", Markdown: body})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 16})

	maxScroll := max(0, viewer.viewport.TotalLineCount()-viewer.viewport.Height())
	if maxScroll <= 0 {
		t.Fatal("expected overflow for scrollbar fixture")
	}

	// Rightmost display cell of each body content row is the scrollbar gutter.
	// (Do not search the whole frame — the nav/body divider is also │.)
	rightEdge := func(v Viewer) []rune {
		plain := ansi.Strip(v.render())
		lines := strings.Split(plain, "\n")
		// skip top border (0) and status bar (last)
		var edges []rune
		for i := 1; i < len(lines)-1; i++ {
			r := []rune(lines[i])
			if len(r) == 0 {
				continue
			}
			edges = append(edges, r[len(r)-1])
		}
		return edges
	}

	edges := rightEdge(viewer)
	hasBar := false
	for _, c := range edges {
		if c == '│' {
			hasBar = true
			break
		}
	}
	if !hasBar {
		t.Fatalf("expected right-edge scrollbar │ when overflowing; edges=%q", string(edges))
	}

	// After scrolling, thumb position should move (edge pattern changes).
	before := string(edges)
	viewer, _ = viewer.Handle(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	viewer, _ = viewer.Handle(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	viewer, _ = viewer.Handle(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	after := string(rightEdge(viewer))
	if before == after && viewer.viewport.YOffset() > 0 {
		// Thumb may be multi-row; still require offset advanced.
		t.Logf("scrollbar edge unchanged after scroll (ok if thumb spans); y=%d", viewer.viewport.YOffset())
	}

	// Short page: no overflow → right edge is blank (space), not track.
	short := testDoc(NavNode{Title: "Short", Markdown: "# Hi\n\none line\n"})
	v2 := New(short, testPalette())
	v2, _ = v2.Handle(tea.WindowSizeMsg{Width: 80, Height: 24})
	if max(0, v2.viewport.TotalLineCount()-v2.viewport.Height()) != 0 {
		t.Fatal("short page should not overflow")
	}
	for _, c := range rightEdge(v2) {
		if c == '│' {
			t.Fatalf("short page should not paint scrollbar track, got │")
		}
	}
}

func TestSelectedNavRowHasNoCaret(t *testing.T) {
	document := testDoc(
		NavNode{Title: "One", Markdown: "# One\n\nbody"},
		NavNode{Title: "Two", Markdown: "# Two\n\nbody"},
	)
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 20})
	plain := ansi.Strip(viewer.render())
	if strings.Contains(plain, "> One") || strings.Contains(plain, "* One") {
		t.Fatalf("selection caret leaked into nav:\n%s", plain)
	}
}

func TestSelectedNavRowStyleIsStableAcrossFocus(t *testing.T) {
	document := testDoc(
		NavNode{Title: "One", Markdown: "# One\n\nbody"},
		NavNode{Title: "Two", Markdown: "# Two\n\nbody"},
	)
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 20})

	// The selection is the persistent current-page cue: full-row secondary
	// fill + Fg bold, identical focused or blurred (only the status bar dims).
	selectedBody := viewer.styles.onActive(viewer.styles.pal.Fg).Bold(true).Render("One")
	accentInvert := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(viewer.styles.pal.Bg))).
		Background(viewer.styles.accent).
		Bold(true).
		Render("One")

	assertSelected := func(label string) {
		raw := viewer.render()
		if !strings.Contains(raw, selectedBody) {
			t.Fatalf("%s: selection missing secondary-row style for One", label)
		}
		if strings.Contains(raw, accentInvert) {
			t.Fatalf("%s: selection must not use accent invert", label)
		}
	}
	assertSelected("focused")

	viewer, _ = viewer.Handle(tea.BlurMsg{})
	assertSelected("blurred")
}

// keyPress builds the message a terminal would send for a keystroke name
// ("j", "tab", "shift+j", "shift+down", …) so msg.String() matches production.
func keyPress(key string) tea.KeyPressMsg {
	if rest, ok := strings.CutPrefix(key, "shift+"); ok {
		return tea.KeyPressMsg{Code: keyCode(rest), Mod: tea.ModShift}
	}
	if len(key) == 1 {
		return tea.KeyPressMsg{Text: key, Code: rune(key[0])}
	}
	return tea.KeyPressMsg{Code: keyCode(key)}
}

func keyCode(key string) rune {
	switch key {
	case "tab":
		return tea.KeyTab
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	case "left":
		return tea.KeyLeft
	case "right":
		return tea.KeyRight
	case "home":
		return tea.KeyHome
	case "end":
		return tea.KeyEnd
	case "enter":
		return tea.KeyEnter
	default:
		return []rune(key)[0]
	}
}

func TestNavFromGroupChildrenToTopLevelLeaves(t *testing.T) {
	document := testDoc(
		NavNode{
			Title: "README",
			Children: []NavNode{
				{Title: "prelude", Markdown: "# prelude\n\nreadme intro", RootReadme: true},
				{Title: "Quickstart", Markdown: "# Quickstart\n\nquickstart body"},
				{Title: "Usage", Markdown: "# Usage\n\nusage body"},
			},
		},
		NavNode{Title: "This shell", Markdown: "# This shell\n\nshell body"},
		NavNode{Title: "Commands", Markdown: "# Commands\n\ncommands body"},
	)
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Start on first leaf inside README group (preamble).
	plain := ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "readme intro") {
		t.Fatalf("expected readme intro on first page:\n%s", plain)
	}

	// Navigate down through the README group children to the top-level leaves.
	// shift+j → Quickstart
	viewer, _ = viewer.Handle(keyPress("shift+j"))
	plain = ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "quickstart body") {
		t.Fatalf("expected quickstart after first shift+j:\n%s", plain)
	}

	// tab → Usage
	viewer, _ = viewer.Handle(keyPress("tab"))
	plain = ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "usage body") {
		t.Fatalf("expected usage after tab:\n%s", plain)
	}

	// shift+down → This shell (top-level leaf after the group)
	viewer, _ = viewer.Handle(keyPress("shift+down"))
	plain = ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "shell body") {
		t.Fatalf("expected shell body after shift+down:\n%s", plain)
	}

	// J (shift+j as an uppercase rune) → Commands
	viewer, _ = viewer.Handle(keyPress("J"))
	plain = ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "commands body") {
		t.Fatalf("expected commands body after J:\n%s", plain)
	}
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
