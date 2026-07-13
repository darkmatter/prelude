package main

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func helpModel() model {
	cfg := testConfig()
	m := newModel(cfg, newStyles(cfg), nil)
	m.mode = modeHelp
	m.width, m.height = 100, 30
	return m
}

func TestHelpDocHasEverySection(t *testing.T) {
	m := helpModel()
	lines, starts := m.helpDoc(80)

	if len(starts) != len(helpSectionTitles) {
		t.Fatalf("section starts = %d, want %d", len(starts), len(helpSectionTitles))
	}
	for i, s := range starts {
		if s < 0 || s >= len(lines) {
			t.Fatalf("section %d starts at %d, outside doc of %d lines", i, s, len(lines))
		}
		if i > 0 && s <= starts[i-1] {
			t.Fatalf("section starts must increase: %v", starts)
		}
		header := strings.ToUpper(helpSectionTitles[i])
		if got := strings.TrimSpace(ansi.Strip(lines[s])); got != header {
			t.Fatalf("section %d start line = %q, want header %q", i, got, header)
		}
	}
}

func TestHelpViewEnablesMouseAndAltScreen(t *testing.T) {
	m := helpModel()

	view := m.View()

	if view.MouseMode != tea.MouseModeCellMotion {
		t.Fatal("help view must enable cell-motion mouse tracking for click/wheel navigation")
	}
	if !view.AltScreen {
		t.Fatal("help view must use the alternate screen")
	}
}

func TestHelpViewFillsTerminal(t *testing.T) {
	m := helpModel()

	content := m.View().Content
	if got := lipgloss.Height(content); got != m.height {
		t.Fatalf("view height = %d, want %d", got, m.height)
	}
	for _, line := range strings.Split(content, "\n") {
		if w := lipgloss.Width(line); w != m.width {
			t.Fatalf("row width = %d, want full-bleed %d", w, m.width)
		}
	}
}

func TestHelpScrollTracksActiveSection(t *testing.T) {
	m := helpModel()

	got, _ := m.updateHelp(tea.KeyPressMsg{Code: 'G'})
	if got.helpActive == 0 {
		t.Fatal("scrolling to the end must move the active section off NAME")
	}

	top, _ := got.updateHelp(tea.KeyPressMsg{Code: 'g'})
	if top.helpScroll != 0 || top.helpActive != 0 {
		t.Fatalf("g must return to top: scroll=%d active=%d", top.helpScroll, top.helpActive)
	}
}

func TestHelpDigitJumpsToSection(t *testing.T) {
	m := helpModel()

	got, _ := m.updateHelp(tea.KeyPressMsg{Code: '5'})

	if got.helpActive != 4 {
		t.Fatalf("active section = %d, want 4 (commands)", got.helpActive)
	}
	_, starts := m.helpDoc(m.helpLayout().textW)
	if got.helpScroll > starts[4] {
		t.Fatalf("scroll = %d, must not pass section start %d", got.helpScroll, starts[4])
	}
}

func TestHelpSidebarClickNavigates(t *testing.T) {
	m := helpModel()

	next, _ := m.Update(tea.MouseClickMsg{X: 2, Y: helpSidebarItemsTop + 2, Button: tea.MouseLeft})
	got := next.(model)

	if got.helpActive != 2 {
		t.Fatalf("active section = %d, want 2 (description)", got.helpActive)
	}
}

func TestHelpClickOutsideSidebarIsIgnored(t *testing.T) {
	m := helpModel()

	next, _ := m.Update(tea.MouseClickMsg{X: m.width - 2, Y: helpSidebarItemsTop, Button: tea.MouseLeft})
	got := next.(model)

	if got.helpActive != 0 || got.helpScroll != 0 {
		t.Fatal("clicks in the content pane must not navigate")
	}
}

func TestHelpWheelScrolls(t *testing.T) {
	m := helpModel()

	next, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	got := next.(model)

	if got.helpScroll != 3 {
		t.Fatalf("wheel scroll = %d, want 3", got.helpScroll)
	}
}

func TestHelpQuitKeys(t *testing.T) {
	m := helpModel()

	for _, key := range []rune{'q'} {
		_, cmd := m.updateHelp(tea.KeyPressMsg{Code: key})
		if cmd == nil {
			t.Fatalf("%q must quit the help viewer", key)
		}
	}
}
