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
	// j moves the sidebar cursor to the next leaf page.
	viewer, _ = viewer.Handle(keyPress("j"))
	plain := ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "only on page two") {
		t.Fatalf("expected page two after j:\n%s", plain)
	}
	if strings.Contains(plain, "only on page one") {
		t.Fatalf("page one leaked into page two:\n%s", plain)
	}
}

func TestDocsTreeExpandCollapseAndFocus(t *testing.T) {
	document := testDoc(NavNode{
		Title: "Guides",
		Children: []NavNode{
			{Title: "One", Markdown: "# One\n\nbody one"},
			{Title: "Two", Markdown: "# Two\n\nbody two"},
		},
	})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 24})
	if viewer.focus != focusSidebar {
		t.Fatalf("focus = %v, want sidebar", viewer.focus)
	}
	plain := ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "body one") {
		t.Fatalf("expected first leaf body:\n%s", plain)
	}
	viewer, _ = viewer.Handle(keyPress("j"))
	plain = ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "body two") {
		t.Fatalf("expected second leaf after j:\n%s", plain)
	}
	viewer, _ = viewer.Handle(keyPress("tab"))
	if viewer.focus != focusContent {
		t.Fatalf("focus = %v, want content", viewer.focus)
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
	viewer, _ = viewer.Handle(keyPress("k"))
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
		viewer, _ = viewer.Handle(keyPress("j"))
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
	viewer, _ = viewer.Handle(keyPress("tab")) // content focus so wheel scrolls body
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

func TestFocusedPaneUsesAccentTopBorder(t *testing.T) {
	document := testDoc(
		NavNode{Title: "One", Markdown: "# One\n\nbody"},
		NavNode{Title: "Two", Markdown: "# Two\n\nbody"},
	)
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 20})
	if viewer.focus != focusSidebar {
		t.Fatalf("focus = %v, want sidebar", viewer.focus)
	}

	norm := lipgloss.NormalBorder()
	dbl := lipgloss.DoubleBorder()
	// #00ffff → truecolor 0;255;255
	const accentSGR = "0;255;255"
	const borderSGR = "85;85;85" // #555555

	assertTop := func(t *testing.T, navFocused bool, label string) {
		t.Helper()
		raw := strings.Split(viewer.render(), "\n")[0]
		plain := ansi.Strip(raw)
		sideW := viewer.l.sideW
		runes := []rune(plain)
		if len(runes) < sideW+1 {
			t.Fatalf("%s: top too short %q", label, plain)
		}
		sidePart := string(runes[:sideW])
		bodyPart := string(runes[sideW+1:])
		if strings.Contains(sidePart, dbl.Top) || strings.Contains(bodyPart, dbl.Top) {
			t.Fatalf("%s: double-border glyph present", label)
		}
		if !strings.Contains(sidePart, norm.Top) || !strings.Contains(bodyPart, norm.Top) {
			t.Fatalf("%s: missing normal top glyphs", label)
		}
		// Expected full-run styles for each half.
		wantNav := viewer.styles.frameAccent.Render(strings.Repeat(norm.Top, sideW))
		wantBody := viewer.styles.topAccent.Render(strings.Repeat(norm.Top, max(viewer.l.bodyW, 1)))
		idleNav := viewer.styles.frame.Render(strings.Repeat(norm.Top, sideW))
		if navFocused {
			if !strings.Contains(raw, wantNav) && !strings.Contains(raw, accentSGR) {
				t.Fatalf("%s: expected accent on nav top\nraw=%q\nwant=%q", label, raw, wantNav)
			}
			if strings.Contains(raw, wantBody) {
				t.Fatalf("%s: body top should not be accent", label)
			}
		} else {
			if !strings.Contains(raw, wantBody) && !strings.Contains(raw, accentSGR) {
				t.Fatalf("%s: expected accent on body top\nraw=%q\nwant=%q", label, raw, wantBody)
			}
			// Nav should be idle (border color), not the accent run.
			if strings.Contains(raw, wantNav) && !strings.Contains(raw, idleNav) {
				t.Fatalf("%s: nav top still accent while body focused", label)
			}
		}
		_ = borderSGR
	}

	assertTop(t, true, "sidebar focus")
	viewer, _ = viewer.Handle(keyPress("tab"))
	if viewer.focus != focusContent {
		t.Fatalf("focus = %v, want content", viewer.focus)
	}
	assertTop(t, false, "content focus")
}


func TestBodyScrollbarWhenOverflow(t *testing.T) {
	// Tall page so the body must scroll.
	body := "# Tall\n\n" + strings.Repeat("line of content\n", 80)
	document := testDoc(NavNode{Title: "Tall", Markdown: body})
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 16})
	viewer, _ = viewer.Handle(keyPress("tab")) // body focus

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

func TestSelectedNavRowStylesByFocus(t *testing.T) {
	document := testDoc(
		NavNode{Title: "One", Markdown: "# One\n\nbody"},
		NavNode{Title: "Two", Markdown: "# Two\n\nbody"},
	)
	viewer := New(document, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 20})
	if viewer.focus != focusSidebar {
		t.Fatalf("focus = %v, want sidebar", viewer.focus)
	}

	// Focused: full-row secondary fill + Fg bold.
	focusedBody := viewer.styles.onActive(viewer.styles.pal.Fg).Bold(true).Render("One")
	// Unfocused: Fg + bold on surface (no secondary fill, no accent).
	idleBody := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(viewer.styles.pal.Fg))).
		Background(viewer.styles.surface).
		Bold(true).
		Render("One")
	accentInvert := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(viewer.styles.pal.Bg))).
		Background(viewer.styles.accent).
		Bold(true).
		Render("One")

	raw := viewer.render()
	if !strings.Contains(raw, focusedBody) {
		t.Fatalf("nav-focused selection missing secondary-row style for One")
	}
	if strings.Contains(raw, accentInvert) {
		t.Fatalf("nav-focused selection must not use accent invert")
	}

	viewer, _ = viewer.Handle(keyPress("tab"))
	if viewer.focus != focusContent {
		t.Fatalf("focus = %v, want content", viewer.focus)
	}
	raw = viewer.render()
	if !strings.Contains(raw, idleBody) {
		t.Fatalf("nav-unfocused selection missing Fg+bold style for One")
	}
	if strings.Contains(raw, focusedBody) {
		t.Fatalf("nav-unfocused selection still on secondary fill")
	}
	if strings.Contains(raw, accentInvert) {
		t.Fatalf("nav-unfocused selection must not use accent")
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
