package manual

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestParseRootReadmeExtractsTaglineAndChips(t *testing.T) {
	src := strings.TrimSpace(`
<div align="center">
  <img src="x.png" />
  <br/><strong>Make your devshell easy to use and nice to look at</strong><br/>
  <sub>Conventional commands • Documentation TUI • Powerful command menu</sub><br /><br />
</div>

Prelude is a DX-focused utility.
`)
	got := parseRootReadme(src)
	if got.Tagline != "Make your devshell easy to use and nice to look at" {
		t.Fatalf("tagline = %q", got.Tagline)
	}
	if !strings.Contains(got.Chips, "Documentation TUI") {
		t.Fatalf("chips = %q", got.Chips)
	}
	if !strings.Contains(got.Body, "Prelude is a DX-focused") {
		t.Fatalf("body missing prose: %q", got.Body)
	}
	if strings.Contains(got.Body, "<div") {
		t.Fatalf("body still has HTML hero: %q", got.Body)
	}
}

func TestRenderRootReadmeShowsProjectTitleAndIndentedIntro(t *testing.T) {
	doc := Document{
			Project: "prelude",
		Nav: []NavNode{{
			Title:      "README",
			RootReadme: true,
			Markdown: `<div align="center">
  <strong>Make your devshell easy to use and nice to look at</strong>
  <sub>Conventional commands • Documentation TUI • Powerful command menu • Highly customizable</sub>
</div>

Body paragraph here.
`,
		}},
	}
	viewer := New(doc, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 40})
	plain := ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "prelude") {
		t.Fatalf("missing project title:\n%s", plain)
	}
	if !strings.Contains(plain, "Make your devshell easy") {
		t.Fatalf("missing tagline:\n%s", plain)
	}
	// Chip wrap lines should not start at column 0.
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "Documentation TUI") || strings.Contains(line, "Highly customizable") {
			trimmed := strings.TrimLeft(line, " ")
			if trimmed == line {
				t.Fatalf("chip/tagline line not indented: %q", line)
			}
		}
	}
}

// TestRenderRootReadmeShowsFigletHero asserts that a baked FIGlet hero is
// rendered on the root-README page, and that the plain bold project name is
// NOT shown when the hero is present (the hero replaces it, not augments it).
func TestRenderRootReadmeShowsFigletHero(t *testing.T) {
	hero := "  _  _ \n / \\/ \\ \n/_/\\_/\\_\n"
	doc := Document{
			Project: "prelude",
		Hero:    hero,
		Nav: []NavNode{{
			Title:      "README",
			RootReadme: true,
			Markdown:   "Body paragraph.\n",
		}},
	}
	viewer := New(doc, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 40})
	plain := ansi.Strip(viewer.viewport.GetContent())
	// The hero's distinct glyph row must be present.
	if !strings.Contains(plain, "/ \\/ \\ ") {
		t.Fatalf("missing FIGlet hero art:\n%s", plain)
	}
	// The plain project name must NOT also be rendered as a heading when
	// the hero is showing — it would duplicate the wordmark.
	for _, line := range strings.Split(plain, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "prelude" {
			t.Fatalf("plain project name rendered alongside hero:\n%s", plain)
		}
	}
}

// TestRenderRootReadmeFallsBackToBoldNameWhenNoHero asserts that when no
// FIGlet hero is baked (older config bundle, empty project name), the viewer
// falls back to rendering the bold project name — the pre-hero behavior.
func TestRenderRootReadmeFallsBackToBoldNameWhenNoHero(t *testing.T) {
	doc := Document{
			Project: "prelude",
		Hero:    "",
		Nav: []NavNode{{
			Title:      "README",
			RootReadme: true,
			Markdown:   "Body paragraph.\n",
		}},
	}
	viewer := New(doc, testPalette())
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 40})
	plain := ansi.Strip(viewer.viewport.GetContent())
	if !strings.Contains(plain, "prelude") {
		t.Fatalf("missing bold project-name fallback:\n%s", plain)
	}
}
