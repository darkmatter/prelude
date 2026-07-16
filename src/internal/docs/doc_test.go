package docs

import "testing"

func TestManualDocumentUsesFirstH1AsPageTitle(t *testing.T) {
	document := manualDocument(&Config{Pages: []Page{
		{Text: "intro\n\n# Getting *started*\n\nHello."},
	}})

	if got, want := document.Sections[0].Title, "Getting started"; got != want {
		t.Fatalf("page title = %q, want %q", got, want)
	}
	if got, want := document.Sections[0].Markdown, "intro\n\n# Getting *started*\n\nHello."; got != want {
		t.Fatalf("Markdown changed during adaptation: got %q, want %q", got, want)
	}
}

func TestManualDocumentFallsBackToPageNumberWithoutH1(t *testing.T) {
	document := manualDocument(&Config{Pages: []Page{
		{Text: "No heading here."},
		{Text: "Still no heading."},
	}})

	if got, want := document.Sections[1].Title, "page 2"; got != want {
		t.Fatalf("fallback title = %q, want %q", got, want)
	}
}
