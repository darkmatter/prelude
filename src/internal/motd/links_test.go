package motd

import (
	"strings"
	"testing"

	"prelude/pkg/shared"
)

func TestLinksPreserveOSC8HyperlinksThroughMOTDLayout(t *testing.T) {
	const url = "https://github.com/darkmatter/prelude"
	cfg := Config{
		Palette: shared.Palette{
			Bg:     "#101010",
			Accent: "#00aaff",
		},
		Links: []Link{{
			Label: "github.com/darkmatter/prelude",
			URL:   url,
		}},
		Width: 40,
	}
	r := newRenderer(cfg, 40, 20)

	component := strings.Join((Links{r: r}).Render(), "\n")
	assertOSC8Link(t, "links component", component, url)

	output := (MOTDView{r: r}).Render()
	assertOSC8Link(t, "complete MOTD", output, url)
}

func assertOSC8Link(t *testing.T, name, output, url string) {
	t.Helper()
	if !strings.Contains(output, "\x1b]8;") {
		t.Fatalf("%s has no OSC 8 sequence: %q", name, output)
	}
	if !strings.Contains(output, url) {
		t.Fatalf("%s has no target URL: %q", name, output)
	}
}
