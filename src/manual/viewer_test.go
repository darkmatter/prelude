package manual

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/shared"
)

func testPalette() shared.Palette {
	return shared.Palette{
		Fg:          "#ffffff",
		Muted:       "#aaaaaa",
		Dim:         "#777777",
		Border:      "#555555",
		Accent:      "#00ff99",
		Accent2:     "#ffaa00",
		SelectionFg: "#000000",
		Bg:          "#111111",
		Surface:     "#222222",
		Secondary:   "#333333",
	}
}

func testDocument() Document {
	paragraphs := func(prefix string, count int) []Block {
		blocks := make([]Block, count)
		for i := range blocks {
			blocks[i] = Block{
				Indent:     4,
				Wrap:       true,
				BlankAfter: true,
				Spans:      []Span{{Role: Muted, Text: prefix + " content that occupies a viewer row"}},
			}
		}
		return blocks
	}
	return Document{Sections: []Section{
		{Title: "first", Blocks: paragraphs("first", 8)},
		{Title: "second", Blocks: paragraphs("second", 8)},
		{Title: "third", Blocks: paragraphs("third", 8)},
	}}
}

func sizedViewer(t *testing.T) Viewer {
	t.Helper()
	v := New(testDocument(), testPalette())
	next, _ := v.Handle(tea.WindowSizeMsg{Width: 72, Height: 18})
	return next
}

func TestViewerOwnsFullBleedGeometry(t *testing.T) {
	v := sizedViewer(t)
	view := v.View()

	if !view.AltScreen {
		t.Fatal("manual viewer must use the alternate screen")
	}
	if view.MouseMode != tea.MouseModeCellMotion {
		t.Fatal("manual viewer must enable mouse navigation")
	}
	if got := lipgloss.Height(view.Content); got != 18 {
		t.Fatalf("height = %d, want 18", got)
	}
	for _, line := range strings.Split(view.Content, "\n") {
		if got := lipgloss.Width(line); got != 72 {
			t.Fatalf("row width = %d, want 72", got)
		}
	}
}

func TestViewerNavigationUsesOneInterface(t *testing.T) {
	v := sizedViewer(t)

	jumped, _ := v.Handle(tea.KeyPressMsg{Code: '2'})
	if plain := ansi.Strip(jumped.View().Content); !strings.Contains(plain, "NORMAL :second") {
		t.Fatalf("digit jump did not activate second section:\n%s", plain)
	}

	clicked, _ := v.Handle(tea.MouseClickMsg{X: 2, Y: SidebarItemsTop + 2, Button: tea.MouseLeft})
	if plain := ansi.Strip(clicked.View().Content); !strings.Contains(plain, "NORMAL :third") {
		t.Fatalf("sidebar click did not activate third section:\n%s", plain)
	}

	ended, _ := v.Handle(tea.KeyPressMsg{Code: 'G'})
	if plain := ansi.Strip(ended.View().Content); !strings.Contains(plain, "NORMAL :third") || !strings.Contains(plain, "bot") {
		t.Fatalf("end navigation did not clamp at the document tail:\n%s", plain)
	}

	top, _ := ended.Handle(tea.KeyPressMsg{Code: 'g'})
	if plain := ansi.Strip(top.View().Content); !strings.Contains(plain, "NORMAL :first") || !strings.Contains(plain, "top") {
		t.Fatalf("home navigation did not return to the document start:\n%s", plain)
	}
}

func TestViewerWheelAndQuitAreHandledByViewer(t *testing.T) {
	v := sizedViewer(t)

	scrolled, _ := v.Handle(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if scrolled.View().Content == v.View().Content {
		t.Fatal("mouse wheel must change the rendered viewport")
	}

	_, cmd := v.Handle(tea.KeyPressMsg{Code: 'q'})
	if cmd == nil {
		t.Fatal("q must quit the viewer")
	}
}
