package menu

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestListViewPreviewsSelectedScriptOnLastRow(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "dev", Run: "nix develop -c $SHELL"},
		Task{Name: "check", Run: "nix flake check"},
	)
	m := newModel(cfg, newStyles(cfg), nil)
	m.applyLayout(80, 24)
	m.syncList()

	assertLastRowScript(t, m, "nix develop -c $SHELL")

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	assertLastRowScript(t, got, "nix flake check")
}

func TestArgsViewPreviewsAssembledScriptOnLastRow(t *testing.T) {
	task := Task{
		Name: "deploy",
		Run:  "just deploy",
		Args: []Arg{{Token: "ENV"}},
	}
	cfg := testMenuConfig(task)
	m := newModel(cfg, newStyles(cfg), &task)
	m.applyLayout(80, 24)
	m.prompt = m.prompt.WithValue("prod")

	assertLastRowScript(t, m, "just deploy prod")
}

func TestListViewCollapsesMultilineScriptOnLastRow(t *testing.T) {
	// prelude.commands.gen.exec is authored as two shell lines
	// (nix/internal/prelude.nix). Display must stay on the last terminal
	// row; finish() still receives the original newlines.
	const run = "sync-docs\nrecord-docs"
	cfg := testMenuConfig(Task{Name: "gen", Run: run})
	m := newModel(cfg, newStyles(cfg), nil)
	m.applyLayout(80, 24)
	m.syncList()

	assertLastRowScript(t, m, "sync-docs record-docs")
}

func TestScriptPreviewLeavesStatusFooterIntact(t *testing.T) {
	// Shipped default menu height is 16. On a 24-row terminal the panel
	// used to fill the window, so overwriting row 23 destroyed the
	// status footer's lower half-cell (▀).
	cfg := testMenuConfig(Task{Name: "dev", Run: "nix develop -c $SHELL"})
	cfg.Height = 16
	m := newModel(cfg, newStyles(cfg), nil)
	m.applyLayout(80, 24)
	m.syncList()

	lines := strippedView(t, m)
	last := strings.TrimSpace(lines[len(lines)-1])
	above := strings.TrimSpace(lines[len(lines)-2])
	if strings.Contains(last, "navigate") || strings.Contains(last, "run") {
		t.Fatal("last row is the keymap; overlay did not reserve a preview row")
	}
	if !strings.Contains(above, "▀") {
		t.Fatalf("row above preview %q is not the status footer half-pad", above)
	}
	assertLastRowScript(t, m, "nix develop -c $SHELL")
}

func TestListViewMarksPendingArgumentsOnLastRow(t *testing.T) {
	cfg := testMenuConfig(Task{
		Name: "deploy",
		Run:  "just deploy",
		Args: []Arg{{Token: "ENV", Required: true}},
	})
	m := newModel(cfg, newStyles(cfg), nil)
	m.applyLayout(80, 24)
	m.syncList()

	assertLastRowScript(t, m, "just deploy")
	last := strings.TrimSpace(strippedView(t, m)[m.layout.height-1])
	if !strings.Contains(last, "…") {
		t.Fatalf("last row %q should mark that Enter collects arguments", last)
	}
}

func TestArgsViewRendersScriptPreviewOnce(t *testing.T) {
	task := Task{
		Name: "deploy",
		Run:  "just deploy",
		Args: []Arg{{Token: "ENV"}},
	}
	cfg := testMenuConfig(task)
	m := newModel(cfg, newStyles(cfg), &task)
	m.applyLayout(80, 24)
	m.prompt = m.prompt.WithValue("prod")

	content := ansi.Strip(m.View().Content)
	if n := strings.Count(content, "just deploy prod"); n != 1 {
		t.Fatalf("assembled script appears %d times, want once on the last row", n)
	}
	assertLastRowScript(t, m, "just deploy prod")
}

func strippedView(t *testing.T, m model) []string {
	t.Helper()
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	if len(lines) != m.layout.height {
		t.Fatalf("view has %d rows, want terminal height %d", len(lines), m.layout.height)
	}
	return lines
}

func assertLastRowScript(t *testing.T, m model, script string) {
	t.Helper()
	last := strings.TrimSpace(strippedView(t, m)[m.layout.height-1])
	if !strings.Contains(last, script) {
		t.Fatalf("last row %q does not preview %q", last, script)
	}
}
