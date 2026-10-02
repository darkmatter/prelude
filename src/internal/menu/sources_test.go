package menu

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestListNotesImportsHiddenByDeclaredCommands(t *testing.T) {
	cfg := testMenuConfig(
		Task{Name: "test", Run: "go test ./...", Description: "run tests"},
		Task{Name: "db", Run: "docker compose up db"},
	)
	mergeTasks(cfg, []Task{
		{Name: "test", Run: "just test", Source: sourceJust, group: "just"},
		{Name: "db", Run: "just db", Source: sourceJust, Children: []Task{{Name: "db::migrate"}}, group: "just"},
		{Name: "build", Run: "just build", Source: sourceJust, group: "just"},
	})

	lines := listLines(cfg)
	for _, note := range []string{
		"test: just recipe hidden by declared command",
		"db: just module hidden by declared command",
	} {
		if !slices.Contains(lines, note) {
			t.Fatalf("--list should say %q:\n%s", note, strings.Join(lines, "\n"))
		}
	}
	if !slices.Contains(lines, "JUST") {
		t.Fatalf("the unclaimed recipe should still be listed:\n%s", strings.Join(lines, "\n"))
	}
}

func TestDetailsNameWhatATaskHides(t *testing.T) {
	cfg := testMenuConfig(Task{Name: "test", Run: "go test ./...", Details: "runs the suite"})
	mergeTasks(cfg, []Task{{Name: "test", Run: "just test", Source: sourceJust, group: "just"}})
	st := newStyles(cfg)
	list := newListView(st, 60).WithSize(60)
	frame := Frame{st: st}.WithSize(60)

	list = list.Sync(cfg.flatten(), []int{0}, 0, true, 12, "", frame)
	if view := ansi.Strip(list.View()); !strings.Contains(view, "hides just recipe test") {
		t.Fatalf("expanded view should name the hidden recipe:\n%s", view)
	}
}

func TestWriteImportsListsTheImportsXDispatches(t *testing.T) {
	cfg := testMenuConfig(Task{Name: "dev", Run: "bun run dev"})
	mergeTasks(cfg, []Task{
		{Name: "build", Description: "build it\nall", Source: sourceJust, group: "just"},
		{Name: "db", Description: "1 subcommand", Source: sourceJust, Children: []Task{{Name: "db::migrate"}}, group: "just"},
		{Name: "db::reset", Source: sourceJust, group: "ops", justPath: []string{"db", "reset"}},
		{Name: "dev", Source: sourceJust, group: "just"},
	})
	mergeTasks(cfg, []Task{
		{Name: "lint", Description: "eslint .", Source: sourceScripts, group: "scripts"},
		{Name: "lint fix", Description: "eslint --fix .", Source: sourceScripts, group: "scripts"},
		{Name: "web/dev", Description: "vite", Source: sourceScripts, group: "web"},
	})

	var out strings.Builder
	writeImports(&out, cfg)
	// Declared and hidden tasks are already complete, a grouped module recipe
	// is reached through its module, and a key that needs quoting cannot be
	// inserted as one word.
	if want := "build\tbuild it all\ndb\t1 subcommand\nlint\teslint .\nweb/dev\tvite\n"; out.String() != want {
		t.Fatalf("writeImports = %q, want %q", out.String(), want)
	}
}
