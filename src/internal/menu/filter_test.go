package menu

import (
	"testing"

	"prelude/pkg/shared"
)

func testMenuConfig(tasks ...Task) *Config {
	return &Config{
		Project:     "test",
		Placeholder: "filter",
		Height:      12,
		Palette:     shared.Palette{Fg: "#fff", Muted: "#888", Accent: "#0f0", Accent2: "#ff0", Bg: "#000", Surface: "#111"},
		Groups:      []Group{{Title: "develop", Tasks: tasks}},
	}
}

func TestFilterEmptyQueryReturnsAll(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "dev", Description: "start server"},
		Task{Name: "test:unit", Description: "unit tests"},
	)
	m := newModel(cfg, newStyles(cfg, false), nil)
	if len(m.matches) != 2 {
		t.Fatalf("empty filter matches = %d, want 2", len(m.matches))
	}
}

func TestFilterFuzzySubsequence(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "check", Description: "open picker"},
		Task{Name: "motd", Description: "banner"},
		Task{Name: "docs", Description: "manual"},
	)
	m := newModel(cfg, newStyles(cfg, false), nil)
	// Non-contiguous subsequence unique to "check".
	m.prompt = m.prompt.WithValue("chk")
	m.filter()

	if len(m.matches) == 0 {
		t.Fatal("fuzzy filter matched nothing for \"chk\"")
	}
	found := false
	for _, idx := range m.matches {
		name := m.flat[idx].Name
		if name == "check" {
			found = true
		}
		if name == "docs" || name == "motd" {
			t.Fatalf("%s should not fuzzy-match \"chk\"", name)
		}
	}
	if !found {
		t.Fatalf("check did not fuzzy-match \"chk\": %#v", m.matches)
	}
}

func TestFilterPreservesCatalogueOrder(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "alpha", Description: "first"},
		Task{Name: "beta", Description: "second"},
		Task{Name: "gamma", Description: "third"},
	)
	m := newModel(cfg, newStyles(cfg, false), nil)
	m.prompt = m.prompt.WithValue("a")
	m.filter()

	var names []string
	for _, idx := range m.matches {
		names = append(names, m.flat[idx].Name)
	}
	// UnsortedFilter keeps input order: alpha before gamma (both match "a").
	if len(names) < 2 || names[0] != "alpha" {
		t.Fatalf("catalogue order lost: %v", names)
	}
}

func TestFilterNoMatchClampsSelection(t *testing.T) {
	cfg := testMenuConfig(Task{Name: "dev", Description: "start"})
	m := newModel(cfg, newStyles(cfg, false), nil)
	m.sel = 0
	m.prompt = m.prompt.WithValue("zzzz-nope")
	m.filter()
	if len(m.matches) != 0 {
		t.Fatalf("expected no matches, got %d", len(m.matches))
	}
	if m.sel != 0 {
		t.Fatalf("sel = %d, want 0 when empty", m.sel)
	}
}

func TestFilterSelectsCommandTypedInFull(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "record-docs", Description: "record showcases"},
		Task{Name: "sync-docs", Description: "regenerate docs"},
		Task{Name: "docs", Description: "browse project documentation"},
	)
	m := newModel(cfg, newStyles(cfg, false), nil)
	m.prompt = m.prompt.WithValue("docs")
	m.filter()
	if len(m.matches) != 3 {
		t.Fatalf("matches = %d, want all three, in catalogue order", len(m.matches))
	}
	if got := m.flat[m.matches[m.sel]].Name; got != "docs" {
		t.Fatalf("selected %q, want the command typed in full", got)
	}
}
