package docs

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderPageFromInlineMarkdown(t *testing.T) {
	cfg, err := ParseConfig([]byte(`{
		"project": "acme",
		"nav": [
			{"title": "Guide", "markdown": "# Guide\n\nInline body text."},
			{"kind": "group", "title": "More", "children": [
				{"title": "Deploy", "markdown": "Ship it with care."}
			]}
		]
	}`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	lines, pages, err := RenderPage(cfg, 2, 60)
	if err != nil {
		t.Fatalf("RenderPage: %v", err)
	}
	if pages != 2 {
		t.Fatalf("pages = %d, want 2 leaves", pages)
	}
	if text := ansi.Strip(strings.Join(lines, "\n")); !strings.Contains(text, "Ship it with care.") {
		t.Fatalf("page 2 should be the nested leaf:\n%s", text)
	}

	if _, _, err := RenderPage(cfg, 3, 60); err == nil {
		t.Fatal("RenderPage accepted a page past the end")
	}
}
