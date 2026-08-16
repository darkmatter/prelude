package manual

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderLeafLinesUsesFirstContentLine(t *testing.T) {
	doc := testDoc(NavNode{
		Title:    "Getting started",
		Markdown: "# Getting started\n\nUse **Prelude** with `nix develop`.",
	})

	lines, err := RenderLeafLines(doc, testPalette(), 1, 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) == 0 {
		t.Fatal("no lines")
	}
	if ansi.Strip(lines[0]) == "" {
		t.Fatalf("print offset 0 is a blank seed row:\n%q", lines[0])
	}
	joined := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "nix develop") {
		t.Fatalf("missing body:\n%s", joined)
	}
}

func TestRenderLeafLinesIndexesDepthFirstLeaves(t *testing.T) {
	doc := testDoc(NavNode{
		Title: "Guides",
		Children: []NavNode{
			{Title: "One", Markdown: "# One\n\nbody one"},
			{Title: "Two", Markdown: "# Two\n\nbody two"},
		},
	})

	lines, err := RenderLeafLines(doc, testPalette(), 2, 40)
	if err != nil {
		t.Fatal(err)
	}
	joined := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "body two") {
		t.Fatalf("page 2 should be the second leaf:\n%s", joined)
	}
	if strings.Contains(joined, "body one") {
		t.Fatalf("page 1 leaked into page 2:\n%s", joined)
	}
}

func TestRenderLeafLinesRejectsOutOfRangePage(t *testing.T) {
	doc := testDoc(NavNode{Title: "Only", Markdown: "# Only\n\nbody"})
	if _, err := RenderLeafLines(doc, testPalette(), 2, 40); err == nil {
		t.Fatal("expected out-of-range error")
	}
}

func TestRenderLeafLinesWrapsToWidth(t *testing.T) {
	doc := testDoc(NavNode{
		Title:    "Wrap",
		Markdown: "alpha bravo charlie delta echo foxtrot golf hotel india juliet",
	})
	wide, err := RenderLeafLines(doc, testPalette(), 1, 80)
	if err != nil {
		t.Fatal(err)
	}
	narrow, err := RenderLeafLines(doc, testPalette(), 1, 24)
	if err != nil {
		t.Fatal(err)
	}
	if len(narrow) <= len(wide) {
		t.Fatalf("narrow wrap produced %d lines, wide produced %d", len(narrow), len(wide))
	}
}

func TestRenderLeafLinesHasNoBodyFillPad(t *testing.T) {
	doc := testDoc(NavNode{Title: "Pad", Markdown: "short"})
	lines, err := RenderLeafLines(doc, testPalette(), 1, 40)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, line := range lines {
		plain := strings.TrimRight(ansi.Strip(line), " ")
		if strings.Contains(plain, "short") {
			found = true
			if ansi.StringWidth(line) >= 40 {
				t.Fatalf("print line is padded to wrap width: %q (width %d)", line, ansi.StringWidth(line))
			}
		}
	}
	if !found {
		t.Fatalf("missing body:\n%s", strings.Join(lines, "\n"))
	}
}
