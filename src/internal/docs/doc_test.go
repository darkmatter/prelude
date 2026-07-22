package docs

import "testing"

func TestManualDocumentUsesFirstH1AsPageTitle(t *testing.T) {
	document := manualDocument(&Config{Nav: []NavNode{
		{Kind: "leaf", Markdown: "intro\n\n# Getting *started*\n\nHello."},
	}})

	if got, want := document.Nav[0].Title, "Getting started"; got != want {
		t.Fatalf("page title = %q, want %q", got, want)
	}
	if got, want := document.Nav[0].Markdown, "intro\n\n# Getting *started*\n\nHello."; got != want {
		t.Fatalf("Markdown changed during adaptation: got %q, want %q", got, want)
	}
}

func TestManualDocumentFallsBackToPageNumberWithoutH1(t *testing.T) {
	document := manualDocument(&Config{Nav: []NavNode{
		{Kind: "leaf", Markdown: "No heading here."},
		{Kind: "leaf", Markdown: "Still no heading."},
	}})

	if got, want := document.Nav[1].Title, "page 2"; got != want {
		t.Fatalf("fallback title = %q, want %q", got, want)
	}
}

func TestManualDocumentKeepsExplicitTitle(t *testing.T) {
	document := manualDocument(&Config{Nav: []NavNode{
		{Kind: "leaf", Title: "Options", Markdown: "# prelude\n\nbody"},
	}})
	if got, want := document.Nav[0].Title, "Options"; got != want {
		t.Fatalf("explicit title = %q, want %q", got, want)
	}
}

func TestManualDocumentPreservesGroups(t *testing.T) {
	document := manualDocument(&Config{Nav: []NavNode{
		{
			Kind:  "group",
			Title: "Options",
			Children: []NavNode{
				{Kind: "leaf", Title: "prelude", Markdown: "# prelude\n"},
			},
		},
	}})
	if !document.Nav[0].IsGroup() {
		t.Fatal("expected group node")
	}
	if got, want := document.Nav[0].Children[0].Title, "prelude"; got != want {
		t.Fatalf("child title = %q, want %q", got, want)
	}
}
