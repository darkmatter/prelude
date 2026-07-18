package ui

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"
)

func TestFadingRuleRendersUnicodeLabelByGrapheme(t *testing.T) {
	surface := lipgloss.Color("#101010")
	frame := lipgloss.Color("#808080")
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	rule := FadingRule{
		Width:   16,
		Surface: surface,
		Frame:   frame,
		Label:   &label,
		Fade:    true,
	}

	got := rule.Render("e\u0301 界 👋🏽")
	if width := lipgloss.Width(got); width != rule.Width {
		t.Fatalf("rendered width = %d, want %d", width, rule.Width)
	}
	if plain := ansi.Strip(got); !strings.Contains(plain, " e\u0301 界 👋🏽 ") {
		t.Fatalf("rendered rule does not preserve label graphemes: %q", plain)
	}
}

func TestCodeBlockPaintsSurfaceBehindRuleAndTitle(t *testing.T) {
	surface := lipgloss.Color("#102030")
	block := CodeBlock{
		Context: NewContext(Palette{
			Accent: "#ffffff",
			Border: "#808080",
		}, surface, false),
		Title: "hi",
		Width: 8,
		Rule:  FadingRule{Fade: true},
	}

	rows := block.Render()
	canvas := lipgloss.NewCanvas(block.Width, 1).
		Compose(lipgloss.NewLayer(rows[0]))
	wantR, wantG, wantB, wantA := surface.RGBA()
	for column := 0; column < block.Width; column++ {
		cell := canvas.CellAt(column, 0)
		if cell == nil || cell.Style.Bg == nil {
			t.Fatalf("column %d has no background: %#v", column, cell)
		}
		gotR, gotG, gotB, gotA := cell.Style.Bg.RGBA()
		if gotR != wantR || gotG != wantG || gotB != wantB || gotA != wantA {
			t.Fatalf("column %d background = %v, want %v", column, cell.Style.Bg, surface)
		}
	}
}

func TestCodeBlockHeaderTransparentSkipsBackground(t *testing.T) {
	surface := lipgloss.Color("#102030")
	block := CodeBlock{
		Context: NewContext(Palette{
			Accent: "#ffffff",
			Border: "#808080",
		}, surface, false),
		Title:             "hi",
		Width:             8,
		Rule:              FadingRule{Fade: true},
		HeaderTransparent: true,
	}

	rows := block.Render()
	// Top rule (header) must not paint surface backgrounds.
	if strings.Contains(rows[0], ";48;2;") {
		t.Fatalf("transparent header paints backgrounds: %q", rows[0])
	}
	// Bottom rule must still paint the surface background.
	if !strings.Contains(rows[len(rows)-1], ";48;2;") {
		t.Fatalf("bottom rule lost its surface background: %q", rows[len(rows)-1])
	}
}

func TestFadingRuleClipsWideLabelToRuleWidth(t *testing.T) {
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	rule := FadingRule{
		Width:   6,
		Surface: lipgloss.Color("#101010"),
		Frame:   lipgloss.Color("#808080"),
		Label:   &label,
	}

	got := rule.Render("界界界界")
	if width := lipgloss.Width(got); width != rule.Width {
		t.Fatalf("rendered width = %d, want %d", width, rule.Width)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("rendered rule contains an unexpected newline: %q", got)
	}
}

func TestFadingRuleFadesAcrossFullWidthWithoutLabel(t *testing.T) {
	rule := FadingRule{
		Width:   8,
		Surface: lipgloss.Color("#101010"),
		Frame:   lipgloss.Color("#808080"),
		Fade:    true,
	}

	got := rule.Render("")
	if width := lipgloss.Width(got); width != rule.Width {
		t.Fatalf("rendered width = %d, want %d", width, rule.Width)
	}
	if plain := ansi.Strip(got); plain != strings.Repeat("─", rule.Width) {
		t.Fatalf("rendered rule = %q, want a full-width rule", plain)
	}

	colors := regexp.MustCompile(`38;2;(\d+);(\d+);(\d+)`).FindAllStringSubmatch(got, -1)
	if len(colors) != rule.Width {
		t.Fatalf("foreground color count = %d, want %d: %q", len(colors), rule.Width, got)
	}
	if first := colors[0][1:]; strings.Join(first, ",") != "128,128,128" {
		t.Fatalf("first foreground = %v, want frame color [128 128 128]", first)
	}
	if last := colors[len(colors)-1][1:]; strings.Join(last, ",") != "16,16,16" {
		t.Fatalf("last foreground = %v, want surface color [16 16 16]", last)
	}
}

func TestFadingRuleWithNonPositiveWidthIsEmpty(t *testing.T) {
	rule := FadingRule{Width: 0}
	if got := rule.Render("title"); got != "" {
		t.Fatalf("rendered rule = %q, want empty output", got)
	}
}
