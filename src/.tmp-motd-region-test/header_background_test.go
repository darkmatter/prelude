package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestHeaderAndContainerRenderAsSiblingRegions(t *testing.T) {
	cfg := Config{
		Project:          "demo",
		Palette:          testPalette(),
		Background:       "#101010",
		WindowBackground: "#080808",
		Padding:          Spacing{Top: 1},
		Header: Header{
			TitleStyle: titleStyleSpine,
			Background: "#332244",
			Tagline:    "container tagline",
		},
		Width: 24,
	}

	lines := strings.Split(newRenderer(cfg, 24, nil).renderBody(), "\n")
	if !hasBackground(lines[0], "51;34;68") {
		t.Fatalf("top padding is not owned by header: %q", lines[0])
	}

	title := lineContaining(t, lines, "demo")
	if !hasBackground(title, "51;34;68") || hasBackground(title, "16;16;16") {
		t.Fatalf("title row does not belong exclusively to header: %q", title)
	}

	divider := lineContaining(t, lines, "━")
	if !hasBackground(divider, "8;8;8") {
		t.Fatalf("divider is not on window/default surface: %q", divider)
	}
	if hasBackground(divider, "51;34;68") || hasBackground(divider, "16;16;16") {
		t.Fatalf("divider leaks header or container background: %q", divider)
	}

	tagline := lineContaining(t, lines, "container tagline")
	if !hasBackground(tagline, "16;16;16") || hasBackground(tagline, "51;34;68") {
		t.Fatalf("tagline is not owned exclusively by container: %q", tagline)
	}
}

func lineContaining(t *testing.T, lines []string, needle string) string {
	t.Helper()
	for _, line := range lines {
		if strings.Contains(ansi.Strip(line), needle) {
			return line
		}
	}
	t.Fatalf("no rendered line contains %q", needle)
	return ""
}

func hasBackground(line, rgb string) bool {
	return strings.Contains(line, "48;2;"+rgb)
}
