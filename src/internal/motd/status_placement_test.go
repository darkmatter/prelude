package motd

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestGeneratedTitleStatusRendersInHeaderNotFooter(t *testing.T) {
	model := Resolve(Config{
		Title: "ACME",
		Header: Header{
			Tagline: "Fancy devshells for your nix flake",
			Status: []HeaderStatus{{
				Label:  "flake",
				Status: "pending",
			}},
		},
		Description: StyledText{Text: "Welcome to the dev environment."},
		Links:       []Link{{Label: "docs", URL: "https://example.com/docs"}},
		Width:       64,
	}, Cache{}, 64, 24, time.Now())
	r := newRenderer(model)

	footer := ansi.Strip((FooterView{r: r}).Render())
	if strings.Contains(footer, "flake") {
		t.Fatalf("generated-title footer still contains status:\n%s", footer)
	}
	if !strings.Contains(footer, "docs") {
		t.Fatalf("footer lost configured links:\n%s", footer)
	}

	output := ansi.Strip((MOTDView{r: r}).Render())
	titleIndex := strings.Index(output, "ACME")
	dividerIndex := strings.Index(output, "━")
	statusIndex := strings.Index(output, "flake")
	taglineIndex := strings.Index(output, "Fancy devshells")
	descriptionIndex := strings.Index(output, "Welcome to the dev environment.")
	if titleIndex < 0 || dividerIndex < 0 || statusIndex < 0 || taglineIndex < 0 || descriptionIndex < 0 {
		t.Fatalf("render is missing expected content:\n%s", output)
	}
	if !(titleIndex < dividerIndex && dividerIndex < statusIndex && statusIndex < taglineIndex && taglineIndex < descriptionIndex) {
		t.Fatalf(
			"unexpected generated-title order: title=%d divider=%d status=%d tagline=%d description=%d\n%s",
			titleIndex,
			dividerIndex,
			statusIndex,
			taglineIndex,
			descriptionIndex,
			output,
		)
	}
}
