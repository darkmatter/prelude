package motd

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"prelude/pkg/shared"
)

var ansiCSI = regexp.MustCompile(`\x1b\[[0-9;]*[[:alpha:]]`)

func TestGeneratedTitleAlignsWholeFIGletBlock(t *testing.T) {
	const title = "AA\nAAAA"
	for _, test := range []struct {
		align  string
		offset int
	}{
		{align: "left", offset: 0},
		{align: "center", offset: 8},
		{align: "right", offset: 16},
	} {
		t.Run(test.align, func(t *testing.T) {
			model := Resolve(Config{
				Title:      title,
				TitleAlign: test.align,
				Width:      20,
				Palette: shared.Palette{
					Fg:           "#ffffff",
					Muted:        "#aaaaaa",
					Dim:          "#777777",
					Border:       "#555555",
					AccentBorder: "#666666",
					Accent:       "#00aaff",
					Bg:           "#112233",
					Surface:      "#223344",
				},
			}, Cache{}, 22, 24, time.Now())
			lines := strings.Split((HeaderView{r: newRenderer(model)}).Render(), "\n")
			if len(lines) != 2 {
				t.Fatalf("rendered %d lines, want 2: %q", len(lines), lines)
			}

			for index, line := range lines {
				plain := ansiCSI.ReplaceAllString(line, "")
				if got := leadingSpaces(plain); got != test.offset {
					t.Fatalf("line %d leading spaces = %d, want %d: %q", index, got, test.offset, plain)
				}
				if got := strings.TrimSpace(plain); index == 0 && got != "AA" {
					t.Fatalf("line %d content = %q, want AA", index, got)
				} else if index == 1 && got != "AAAA" {
					t.Fatalf("line %d content = %q, want AAAA", index, got)
				}
			}
		})
	}
}

func TestHeaderGradientRuleUsesHorizontalPadding(t *testing.T) {
	model := Resolve(Config{
		Title:   "title",
		Width:   20,
		Padding: Spacing{Left: 3, Right: 4},
		Palette: shared.Palette{
			Fg:      "#ffffff",
			Accent:  "#00aaff",
			Bg:      "#112233",
			Surface: "#223344",
		},
	}, Cache{}, 20, 24, time.Now())

	rule := HeaderView{r: newRenderer(model)}.Divider()
	plain := ansiCSI.ReplaceAllString(rule, "")
	want := strings.Repeat(" ", 3) + strings.Repeat("━", 13) + strings.Repeat(" ", 4)
	if plain != want {
		t.Fatalf("gradient rule = %q, want %q", plain, want)
	}
}

func TestInlineTitleGradientRuleUsesHorizontalPadding(t *testing.T) {
	model := Resolve(Config{
		Project: "go",
		Header:  Header{TitleStyle: titleStyleInline},
		Width:   20,
		Padding: Spacing{Left: 3, Right: 4},
		Palette: shared.Palette{
			Fg:      "#ffffff",
			Accent:  "#00aaff",
			Bg:      "#112233",
			Surface: "#223344",
		},
	}, Cache{}, 20, 24, time.Now())

	plain := ansiCSI.ReplaceAllString(HeaderView{r: newRenderer(model)}.Divider(), "")
	label := " go "
	ruleWidth := 20 - 3 - 4
	start := (ruleWidth - len(label)) / 2
	want := strings.Repeat(" ", 3) + strings.Repeat("━", start) + label +
		strings.Repeat("━", ruleWidth-start-len(label)) + strings.Repeat(" ", 4)
	if plain != want {
		t.Fatalf("inline gradient rule = %q, want %q", plain, want)
	}
}

func leadingSpaces(value string) int {
	return len(value) - len(strings.TrimLeft(value, " "))
}
