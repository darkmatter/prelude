package motd

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func TestActivationAddsBlankLineBeforeFullWidthSubtitle(t *testing.T) {
	const subtitle = "This subtitle stays on one full-width line after the activation heading."

	r := newRenderer(Resolve(Config{
		Header: Header{TaglineAlign: "left"},
		Width:  80,
	}, Cache{}, 80, 20, time.Now()))
	lines := sections{r: r}.activation("Dev Shell Activated", subtitle, "")

	if len(lines) < 3 {
		t.Fatalf("subtitle layout is missing the blank line: %q", lines)
	}

	blank := strings.TrimSpace(ansi.Strip(lines[len(lines)-2]))
	if blank != "" {
		t.Fatalf("line before subtitle is not blank: %q", blank)
	}

	gotSubtitle := strings.TrimSpace(ansi.Strip(lines[len(lines)-1]))
	if gotSubtitle != subtitle {
		t.Fatalf("subtitle was wrapped or changed: got %q, want %q", gotSubtitle, subtitle)
	}
}
