package menu

import (
	"os"
	"path/filepath"
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
		{Name: "db", Run: "just db", Source: sourceJust, Children: []Task{{Name: "db::migrate"}}, group: "just", justModule: true},
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
	// `t/s` is the route `t s` (a `/` nests a script name), so it is hidden too.
	for _, key := range []string{"tools/status", "tools status", "t status", "tools s", "t s", "t/s"} {
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
	for key, command := range map[string]string{"s": "exit 3"} {
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
		{Name: "e2e", Description: "2 subcommands", Source: sourceJust, group: "just", justModule: true, Children: []Task{
			{Name: "e2e::coder", Label: "coder", Description: "run coder e2e", Source: sourceJust},
			{Name: "e2e::sub", Label: "sub", Description: "1 subcommand", Source: sourceJust, justModule: true, Children: []Task{
				{Name: "e2e::sub::x", Label: "x", Description: "deep", Source: sourceJust},
			}},
		}},
		{Name: "e2e::smoke", Label: "smoke", Description: "smoke it", Source: sourceJust, group: "ops", justPath: []string{"e2e", "smoke"}},
		{Name: "dev", Source: sourceJust, group: "just"},
	})
	mergeTasks(cfg, []Task{
		{Name: "lint", Description: "eslint .", Source: sourceScripts, group: "scripts"},
		{Name: "lint fix", Description: "eslint --fix .", Source: sourceScripts, group: "scripts"},
		{Name: "db", Label: "db", Description: "1 subcommand", Source: sourceScripts, group: "scripts", Children: []Task{
			{Name: "db/migrate", Label: "migrate", Description: "migrate up", Source: sourceScripts},
		}},
	})

	var out strings.Builder
	writeImports(&out, cfg)
	// Every imported node prints its route of public words, at every depth.
	// Declared and hidden tasks are already complete, a grouped module recipe
	// is reached through its module's words, and a word that needs quoting
	// cannot be inserted as one.
	want := strings.Join([]string{
		"build\tbuild it all",
		"e2e\t2 subcommands",
		"e2e coder\trun coder e2e",
		"e2e sub\t1 subcommand",
		"e2e sub x\tdeep",
		"e2e smoke\tsmoke it",
		"db\t1 subcommand",
		"db migrate\tmigrate up",
		"lint\teslint .",
	}, "\n") + "\n"
	if out.String() != want {
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

func TestRootIsWhereImportsAreReadAndCommandsRun(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writePackageJSON(t, root, `{"lint": "eslint ."}`)
	bin := t.TempDir()
	// The stand-in just records the directory it was run in.
	if err := os.WriteFile(filepath.Join(bin, "just"), []byte(`#!/bin/sh
pwd -P > "$JUST_RAN_IN"
printf '%s\n' '{"recipes":{"build":{"name":"build","namepath":"build","private":false}}}'
`), 0o755); err != nil {
		t.Fatal(err)
	}
	ranIn := filepath.Join(t.TempDir(), "ran-in")
	t.Setenv("JUST_RAN_IN", ranIn)
	t.Setenv("PATH", bin)
	t.Chdir(t.TempDir())

	rooted := func(root string) *Config {
		cfg := testMenuConfig(Task{Name: "dev", Run: "bun run dev"})
		cfg.Just = JustConfig{Enable: true, Group: "just"}
		cfg.Scripts = ScriptsConfig{Enable: true, Group: "scripts"}
		cfg.Root = root
		cfg.PathPrefix = []string{"/menu/own/bin"}
		return parseTestConfig(t, cfg)
	}

	cfg := rooted(root)
	for _, key := range []string{"dev", "build", "lint"} {
		got, err := Select(cfg, []string{key})
		if err != nil || got.Dir != root {
			t.Fatalf("x %s = %+v (%v), want it to run in the root %s", key, got, err, root)
		}
		// The menu's own directories come after a task's own.
		if prefix := got.PathPrefix; len(prefix) == 0 || prefix[len(prefix)-1] != "/menu/own/bin" {
			t.Fatalf("x %s PATH prefix = %q, want the menu's own last", key, prefix)
		}
	}
	if data, err := os.ReadFile(ranIn); err != nil || strings.TrimSpace(string(data)) != root {
		t.Fatalf("just ran in %q (%v), want the root %s", data, err, root)
	}

	other := t.TempDir()
	t.Setenv("PRELUDE_ROOT", other)
	if got, err := Select(rooted(root), []string{"dev"}); err != nil || got.Dir != other {
		t.Fatalf("x dev = %+v (%v), want PRELUDE_ROOT %s to win", got, err, other)
	}
	// A root another bound menu handed on stays with that menu.
	t.Setenv("PRELUDE_ROOT_MENU", "/another/menu.json")
	if got, err := Select(rooted(root), []string{"dev"}); err != nil || got.Dir != root {
		t.Fatalf("x dev = %+v (%v), want another menu's root ignored", got, err)
	}
	t.Setenv("PRELUDE_ROOT_MENU", "")
	// A menu without a root, such as a devshell's, ignores an inherited one.
	unbound := parseTestConfig(t, testMenuConfig(Task{Name: "dev", Run: "bun run dev"}))
	if got, err := Select(unbound, []string{"dev"}); err != nil || got.Dir != "" {
		t.Fatalf("x dev = %+v (%v), want an unbound menu to stay in the caller's directory", got, err)
	}
}
