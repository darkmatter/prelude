package menu

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestParseConfigIsStrictAndDefaulted(t *testing.T) {
	cfg, err := ParseConfig([]byte(`{"project":"acme","groups":[]}`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if cfg.Height != 12 {
		t.Fatalf("Height = %d, want the CLI default 12", cfg.Height)
	}

	// A TypeScript host is a second author of this boundary; unknown fields
	// must fail loudly exactly as they do for Nix-generated files.
	if _, err := ParseConfig([]byte(`{"project":"acme","bogus":true}`)); err == nil {
		t.Fatal("ParseConfig accepted an unknown field")
	}
}

func TestSelectResolvesReadyCommandsWithoutThePicker(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "dev", Run: "bun run dev"},
		Task{Name: "deploy", Run: "just deploy", Args: []Arg{{Token: "--alias"}}},
	)

	got, err := Select(cfg, []string{"dev"})
	if err != nil {
		t.Fatalf("Select(dev): %v", err)
	}
	if *got != (Selection{Name: "dev", Command: "bun run dev"}) {
		t.Fatalf("Select(dev) = %+v", *got)
	}

	// Explicit arguments skip argument entry; the host receives them as the
	// same line the picker would have produced.
	got, err = Select(cfg, []string{"deploy", "--alias", "staging"})
	if err != nil {
		t.Fatalf("Select(deploy …): %v", err)
	}
	want := Selection{Name: "deploy", Line: "--alias staging", Command: "just deploy --alias staging"}
	if *got != want {
		t.Fatalf("Select(deploy …) = %+v, want %+v", *got, want)
	}
}

func TestSelectRejectsUnknownCommands(t *testing.T) {
	cfg := testMenuConfig(Task{Name: "dev", Run: "bun run dev"})
	if _, err := Select(cfg, []string{"nope"}); err == nil || !strings.Contains(err.Error(), `unknown command "nope"`) {
		t.Fatalf("Select(nope) error = %v", err)
	}
}

func TestListNamesTheConfiguredDispatcher(t *testing.T) {
	cfg := testMenuConfig(Task{Name: "dev", Run: "bun run dev", Description: "start"})

	var out strings.Builder
	List(&out, nil, nil, cfg, 60)
	if !strings.Contains(ansi.Strip(out.String()), "run x to pick a task interactively") {
		t.Fatalf("default footer should name x: %q", out.String())
	}

	cfg.Dispatcher = "acme"
	out.Reset()
	List(&out, nil, nil, cfg, 60)
	if !strings.Contains(ansi.Strip(out.String()), "run acme to pick a task interactively") {
		t.Fatalf("footer should name the configured dispatcher: %q", out.String())
	}
}
