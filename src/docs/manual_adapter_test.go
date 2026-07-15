package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/manual"
	"prelude/shared"
)

func TestDocsContentAdaptsToSharedManualViewer(t *testing.T) {
	cfg := &Config{
		Project: "prelude",
		Palette: shared.Palette{
			Fg: "#ffffff", Muted: "#aaaaaa", Dim: "#777777", Border: "#555555",
			Accent: "#00ff99", Accent2: "#ffaa00", SelectionFg: "#000000",
			Bg: "#111111", Surface: "#222222", Secondary: "#333333",
		},
		Sections: []Section{{
			Title: "guide",
			Blocks: []Block{
				{Type: "lead", Term: "prelude", Text: "devshell UI"},
				{Type: "option", Term: "--config", Text: "select configuration"},
				{Type: "shell", Command: "menu list", Note: "print tasks"},
				{Type: "unknown", Text: "visible fallback"},
			},
		}},
	}

	viewer := manual.New(manualDocument(cfg), cfg.Palette)
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 80, Height: 24})
	plain := ansi.Strip(viewer.View().Content)
	for _, want := range []string{"GUIDE", "prelude — devshell UI", "--config", "select configuration", "$ menu list", "print tasks", "visible fallback"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("shared viewer output missing %q:\n%s", want, plain)
		}
	}
}
