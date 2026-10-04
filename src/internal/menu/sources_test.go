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

func TestDeclaredChildRoutesHideImports(t *testing.T) {
	path := writePackageJSON(t, t.TempDir(), `{
		"tools/status": "exit 19",
		"tools status": "exit 19",
		"t status": "exit 19",
		"tools s": "exit 19",
		"t s": "exit 19",
		"s": "exit 3",
		"t/s": "exit 4"
	}`)
	cfg := testMenuConfig(Task{
		Name: "tools", Key: "t", Run: "acme tools",
		Children: []Task{{Name: "tools/status", Label: "status", Key: "s", Run: "exit 7"}},
	})
	cfg.Scripts = ScriptsConfig{Enable: true, PackageJSON: &path, Group: "scripts"}
	cfg = parseTestConfig(t, cfg)

	lines := listLines(cfg)
	for _, key := range []string{"tools/status", "tools status", "t status", "tools s", "t s"} {
		note := key + ": package.json script hidden by declared command tools"
		if !slices.Contains(lines, note) {
			t.Fatalf("--list should say %q:\n%s", note, strings.Join(lines, "\n"))
		}
	}
	if text := strings.Join(lines, "\n"); strings.Contains(text, "exit 19") {
		t.Fatalf("--list still offers a hidden script:\n%s", text)
	}
	for _, parent := range []string{"tools", "t"} {
		got, err := Select(cfg, []string{parent, "status"})
		if err != nil {
			t.Fatalf("Select(%s status): %v", parent, err)
		}
		if got == nil || got.Name != "tools/status" || got.Source != sourceDeclared || got.Command != "exit 7" {
			t.Fatalf("Select(%s status) = %+v, want the declared child", parent, got)
		}
	}
	for key, command := range map[string]string{"s": "exit 3", "t/s": "exit 4"} {
		got, err := Select(cfg, []string{key})
		if err != nil {
			t.Fatalf("Select(%s): %v", key, err)
		}
		if got == nil || got.Source != sourceScripts || got.Command != command {
			t.Fatalf("Select(%s) = %+v, want the unclaimed script", key, got)
		}
	}
}

func TestDetailsNameWhatATaskHides(t *testing.T) {
	cfg := testMenuConfig(Task{Name: "test", Run: "go test ./...", Details: "runs the suite"})
	mergeTasks(cfg, []Task{{Name: "test", Run: "just test", Source: sourceJust, group: "just"}})
	st := newStyles(cfg, false)
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

func TestListShowsUngroupedCommandsFirstWithoutAHeading(t *testing.T) {
	cfg := testMenuConfig()
	cfg.Groups = []Group{
		{Title: "", Tasks: []Task{{Name: "build", Description: "compile the app"}}},
		{Title: "db", Tasks: []Task{{Name: "db/migrate", Label: "migrate", Description: "apply migrations"}}},
	}

	lines := listLines(cfg)
	if !strings.HasPrefix(lines[0], "build") {
		t.Fatalf("an ungrouped command should open the list with no heading above it:\n%s", strings.Join(lines, "\n"))
	}
	if !slices.Contains(lines, "DB") {
		t.Fatalf("a named group keeps its heading:\n%s", strings.Join(lines, "\n"))
	}
}
