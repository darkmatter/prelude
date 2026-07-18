package title

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestInitialRecipeDoesNotAssumeARecipePath(t *testing.T) {
	cfg := Config{DefaultFont: "thin", Fonts: []Font{{Name: "thin", Path: "/thin"}}}
	recipe, err := initialRecipe(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	if recipe.Text == "" || recipe.Font != "thin" {
		t.Fatalf("initialRecipe() = %#v", recipe)
	}
}

func TestChooserStartsFromExistingRecipe(t *testing.T) {
	model := testChooser()
	if model.input.Value() != "acme" {
		t.Fatalf("input value = %q, want acme", model.input.Value())
	}
	if model.selected != 1 {
		t.Fatalf("selected = %d, want thin at index 1", model.selected)
	}
}

func TestStylePreviewPreservesMultilineOffsetsBesideDivider(t *testing.T) {
	cfg := Config{DefaultFont: "thin", Fonts: []Font{{Name: "thin", Path: "/thin"}}}
	model := newChooser(cfg, Recipe{Text: "acme", Font: "thin"}, func(Font, string) (string, error) {
		return "  top\nbottom", nil
	})
	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = next.(chooserModel)

	var topColumn, bottomColumn = -1, -1
	for _, line := range strings.Split(ansi.Strip(model.View().Content), "\n") {
		if column := strings.Index(line, "top"); column >= 0 {
			topColumn = column
		}
		if column := strings.Index(line, "bottom"); column >= 0 {
			bottomColumn = column
		}
	}
	if topColumn < 0 || bottomColumn < 0 {
		t.Fatalf("preview lines not found: top=%d bottom=%d", topColumn, bottomColumn)
	}
	if topColumn-bottomColumn != 2 {
		t.Fatalf("relative text offset = %d, want 2", topColumn-bottomColumn)
	}
}

func TestChooserSubmitsTitlePagesStyleAndConfirms(t *testing.T) {
	model := testChooser()

	next, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = next.(chooserModel)
	if model.stage != stageStyle {
		t.Fatalf("stage = %d, want style stage", model.stage)
	}
	if model.preview != "thin:acme" {
		t.Fatalf("preview = %q, want thin:acme", model.preview)
	}
	if plain := ansi.Strip(model.View().Content); !strings.Contains(plain, strings.Repeat("━", 66)) {
		t.Fatal("style preview does not contain the full-width title divider")
	}

	next, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	model = next.(chooserModel)
	if model.selected != 0 || model.preview != "standard:acme" {
		t.Fatalf("paged selection = %d, preview = %q", model.selected, model.preview)
	}

	next, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model = next.(chooserModel)
	if !model.done || cmd == nil {
		t.Fatalf("confirmed chooser = done %v, command nil %v", model.done, cmd == nil)
	}
	if got := model.selectedRecipe(); got != (Recipe{Text: "acme", Font: "standard"}) {
		t.Fatalf("selected recipe = %#v", got)
	}
}

func testChooser() chooserModel {
	cfg := Config{
		DefaultFont: "thin",
		Fonts: []Font{
			{Name: "standard", Path: "/standard"},
			{Name: "thin", Path: "/thin"},
		},
	}
	return newChooser(cfg, Recipe{Text: "acme", Font: "thin"}, func(font Font, text string) (string, error) {
		return font.Name + ":" + text, nil
	})
}
