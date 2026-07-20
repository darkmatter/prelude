package motd

import (
	"testing"

	"charm.land/lipgloss/v2"

	"prelude/pkg/shared"
)

func TestCodeblockTitleRuleUsesBottomRuleSurface(t *testing.T) {
	cfg := Config{
		Background: "#000000",
		Palette: shared.Palette{
			Accent:  "#ffffff",
			Border:  "#808080",
			Bg:      "#000000",
			Surface: "#203040",
		},
	}
	r := newRenderer(cfg, 12, 10)
	rows := (Codeblock{r: r}).Render(Recipe{Title: "build"})
	top := lipgloss.NewCanvas(r.contentWidth, 1).
		Compose(lipgloss.NewLayer(rows[0]))
	bottom := lipgloss.NewCanvas(r.contentWidth, 1).
		Compose(lipgloss.NewLayer(rows[len(rows)-1]))

	for column := 0; column < r.contentWidth; column++ {
		topCell := top.CellAt(column, 0)
		bottomCell := bottom.CellAt(column, 0)
		if topCell == nil || bottomCell == nil {
			t.Fatalf("column %d is missing a rendered rule cell", column)
		}
		if !colorsEqual(topCell.Style.Bg, bottomCell.Style.Bg) {
			t.Fatalf("column %d title-rule background = %v, want bottom-rule background %v", column, topCell.Style.Bg, bottomCell.Style.Bg)
		}
	}
}
