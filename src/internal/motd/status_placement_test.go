package motd

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestGeneratedTitleStatusRendersInFooter(t *testing.T) {
	model := Resolve(Config{
		Title: "ACME",
		Header: Header{
			Tagline: "Fancy devshells for your nix flake",
			Status: []HeaderStatus{{
				Label:  "build",
				Status: "pending",
			}},
		},
		Description: StyledText{Text: "Welcome to the dev environment."},
		Links:       []Link{{Label: "docs", URL: "https://example.com/docs"}},
		Width:       64,
	}, Cache{}, 64, 24, time.Now())
	r := newRenderer(model)

	footer := ansi.Strip((FooterView{r: r}).Render())
	statusIndex := strings.Index(footer, "build")
	linkIndex := strings.Index(footer, "docs")
	if statusIndex < 0 || linkIndex < 0 {
		t.Fatalf("generated-title footer is missing status or links:\n%s", footer)
	}
	if statusIndex >= linkIndex {
		t.Fatalf("footer status must precede links:\n%s", footer)
	}

	output := ansi.Strip((MOTDView{r: r}).Render())
	titleIndex := strings.Index(output, "ACME")
	dividerIndex := strings.Index(output, "━")
	taglineIndex := strings.Index(output, "Fancy devshells")
	descriptionIndex := strings.Index(output, "Welcome to the dev environment.")
	statusIndex = strings.Index(output, "build")
	linkIndex = strings.Index(output, "docs")
	if titleIndex < 0 || dividerIndex < 0 || taglineIndex < 0 ||
		descriptionIndex < 0 || statusIndex < 0 || linkIndex < 0 {
		t.Fatalf("render is missing expected content:\n%s", output)
	}
	if !(titleIndex < dividerIndex &&
		dividerIndex < taglineIndex &&
		taglineIndex < descriptionIndex &&
		descriptionIndex < statusIndex &&
		statusIndex < linkIndex) {
		t.Fatalf(
			"unexpected generated-title order: title=%d divider=%d tagline=%d description=%d status=%d links=%d\n%s",
			titleIndex,
			dividerIndex,
			taglineIndex,
			descriptionIndex,
			statusIndex,
			linkIndex,
			output,
		)
	}
}
