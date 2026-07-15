package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/manual"
	"prelude/shared"
)

func manualAdapterConfig() *Config {
	return &Config{
		Project: "prelude",
		Execute: true,
		Palette: shared.Palette{
			Fg: "#ffffff", Muted: "#aaaaaa", Dim: "#777777", Border: "#555555",
			Accent: "#00ff99", Accent2: "#ffaa00", SelectionFg: "#000000",
			Bg: "#111111", Surface: "#222222", Secondary: "#333333",
		},
		Groups: []Group{{Title: "develop", Tasks: []Task{{
			Name: "dev", Key: "d", Description: "start development", Usage: "menu dev", Examples: []string{"menu dev --port 3000"},
		}}}},
	}
}

func TestTaskHelpAdaptsToSharedManualViewer(t *testing.T) {
	cfg := manualAdapterConfig()
	viewer := manual.New(helpDocument(cfg), cfg.Palette)
	viewer, _ = viewer.Handle(tea.WindowSizeMsg{Width: 96, Height: 30})
	plain := ansi.Strip(viewer.View().Content)
	viewer, _ = viewer.Handle(tea.KeyPressMsg{Code: '5'})
	plain += "\n" + ansi.Strip(viewer.View().Content)
	for _, want := range []string{"NAME", "SYNOPSIS", "COMMANDS", "prelude — devshell UI", "DEVELOP", "dev  (d)", "start development", "$ menu dev", "$ menu dev --port 3000"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("shared viewer output missing %q:\n%s", want, plain)
		}
	}
}

func TestMenuHelpModeDelegatesInteractionAndViewToSharedViewer(t *testing.T) {
	cfg := manualAdapterConfig()
	m := newModel(cfg, newStyles(cfg), nil)
	m.mode = modeHelp

	next, _ := m.Update(tea.WindowSizeMsg{Width: 96, Height: 30})
	m = next.(model)
	next, _ = m.Update(tea.KeyPressMsg{Code: '5'})
	m = next.(model)

	view := m.View()
	if view.MouseMode != tea.MouseModeCellMotion || !view.AltScreen {
		t.Fatal("menu help must use the shared viewer screen and mouse settings")
	}
	if plain := ansi.Strip(view.Content); !strings.Contains(plain, "NORMAL :commands") {
		t.Fatalf("menu help did not delegate navigation to the shared viewer:\n%s", plain)
	}
}
