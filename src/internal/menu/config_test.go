package menu

import (
	"strings"
	"testing"
)

func TestTaskDisplayNameUsesParsedLabel(t *testing.T) {
	task := Task{Name: "demos:menu", Label: "menu"}
	if got := task.displayName(); got != "menu" {
		t.Fatalf("displayName() = %q, want %q", got, "menu")
	}

	ungrouped := Task{Name: "dev"}
	if got := ungrouped.displayName(); got != "dev" {
		t.Fatalf("displayName() fallback = %q, want %q", got, "dev")
	}
}

func TestFlattenSearchesSelectorLabelAndGroup(t *testing.T) {
	cfg := Config{Groups: []Group{{
		Title: "demos",
		Tasks: []Task{{Name: "demos:menu", Label: "menu"}},
	}}}

	flat := cfg.flatten()
	if len(flat) != 1 {
		t.Fatalf("flatten() returned %d tasks, want 1", len(flat))
	}
	for _, want := range []string{"demos:menu", "menu", "demos"} {
		if !strings.Contains(flat[0].haystack, want) {
			t.Errorf("haystack %q does not contain %q", flat[0].haystack, want)
		}
	}
}
