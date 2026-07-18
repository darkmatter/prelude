package menu

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestModelViewPlacesBarCursorAtPromptInput(t *testing.T) {
	cfg := &Config{
		Project:     "prelude",
		Placeholder: "filter commands",
		Height:      8,
		MaxWidth:    60,
	}
	m := newModel(cfg, newStyles(cfg), nil)

	for _, size := range []struct {
		width  int
		height int
	}{
		{width: 80, height: 24},
		{width: 81, height: 25},
	} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			m.width = size.width
			m.height = size.height
			m.resizeChrome()
			m.syncList()

			view := m.View()
			if view.Cursor == nil {
				t.Fatal("view cursor is nil; prompt must use the real terminal cursor")
			}
			if view.Cursor.Shape != tea.CursorBar {
				t.Fatalf("cursor shape = %v, want bar", view.Cursor.Shape)
			}
			if !view.Cursor.Blink {
				t.Fatal("cursor blink = false, want terminal-controlled blinking")
			}

			const promptPrefix = "~/prelude ❯ "
			lines := strings.Split(ansi.Strip(view.Content), "\n")
			wantY := -1
			wantX := -1
			for y, line := range lines {
				if x := strings.Index(line, promptPrefix); x >= 0 {
					wantY = y
					wantX = ansi.StringWidth(line[:x] + promptPrefix)
					break
				}
			}
			if wantY < 0 {
				t.Fatalf("rendered prompt prefix %q not found", promptPrefix)
			}
			if view.Cursor.Position.X != wantX || view.Cursor.Position.Y != wantY {
				t.Fatalf(
					"cursor position = (%d,%d), want input start at (%d,%d)",
					view.Cursor.Position.X,
					view.Cursor.Position.Y,
					wantX,
					wantY,
				)
			}
		})
	}
}

func TestPromptKeepsLongInputCursorInsideRow(t *testing.T) {
	const inner = 40
	prompt := newPrompt(styles{}, "prelude", "", inner).
		WithValue(strings.Repeat("x", 200)).
		WithCursorEnd()

	cursor := prompt.Cursor()
	if cursor == nil {
		t.Fatal("prompt cursor is nil")
	}
	if cursor.Position.X >= inner+2 {
		t.Fatalf("cursor x = %d, want inside %d-cell prompt row", cursor.Position.X, inner+2)
	}
}
