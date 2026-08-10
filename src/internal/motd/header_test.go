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

func leadingSpaces(value string) int {
	return len(value) - len(strings.TrimLeft(value, " "))
}
