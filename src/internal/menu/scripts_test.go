package menu

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func writePackageJSON(t *testing.T, dir string, scripts string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "package.json")
	if err := os.WriteFile(path, []byte(`{"name": "web", "scripts": `+scripts+`}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// parseTestConfig sends cfg through ParseConfig as Nix or a host would, so
// the runtime imports run as they do for the CLI and libprelude.
func parseTestConfig(t *testing.T, cfg *Config) *Config {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func listLines(cfg *Config) []string {
	var out strings.Builder
	List(&out, nil, nil, cfg, 100)
	lines := strings.Split(ansi.Strip(out.String()), "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return lines
}

func TestScriptsRunAsWrittenWhereTheyLive(t *testing.T) {
	dir := t.TempDir()
	path := writePackageJSON(t, dir, `{
		"build": "tsc && vite build",
		"test:unit": "vitest run",
		"web/dev": "vite",
		"//": ["comments are not scripts"]
	}`)
	cfg := testMenuConfig(Task{Name: "dev", Run: "bun run dev"})
	cfg.Scripts = ScriptsConfig{Enable: true, PackageJSON: &path, Group: "npm"}
	cfg = parseTestConfig(t, cfg)

	got, err := Select(cfg, []string{"build", "--watch"})
	if err != nil {
		t.Fatalf("Select(build --watch): %v", err)
	}
	// npm puts node_modules/.bin of the package and of every directory above
	// it ahead of PATH, nearest first.
	var bins []string
	for d := dir; ; d = filepath.Dir(d) {
		bins = append(bins, filepath.Join(d, "node_modules", ".bin"))
		if d == filepath.Dir(d) {
			break
		}
	}
	want := Selection{
		Name:       "build",
		Line:       "--watch",
		Command:    "tsc && vite build --watch",
		Source:     "scripts",
		Dir:        dir,
		PathPrefix: bins,
	}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("Select(build --watch) = %+v\nwant %+v", *got, want)
	}

	// Every script lists in the configured group; names are never parsed for
	// one, and a `/` nests the script under its prefix instead.
	lines := listLines(cfg)
	for _, title := range []string{"TEST", "WEB"} {
		if slices.Contains(lines, title) {
			t.Fatalf("--list should not infer a %s group:\n%s", title, strings.Join(lines, "\n"))
		}
	}
	for _, line := range []string{"NPM", "test:unit vitest run", "dev vite"} {
		if !slices.Contains(lines, line) {
			t.Fatalf("--list should show %q:\n%s", line, strings.Join(lines, "\n"))
		}
	}
	if got, err := Select(cfg, []string{"web", "dev"}); err != nil || got.Command != "vite" || got.Dir != dir {
		t.Fatalf("Select(web dev) = %+v (%v), want vite run in %s", got, err, dir)
	}
	if text := strings.Join(lines, "\n"); strings.Contains(text, "comments are not scripts") {
		t.Fatalf("a non-string entry was imported as a script:\n%s", text)
	}
}

func TestScriptsNestOnSlash(t *testing.T) {
	path := writePackageJSON(t, t.TempDir(), `{
		"db": "echo db",
		"db/migrate": "migrate up",
		"db/seed/users": "seed users",
		"lint:fix": "eslint --fix .",
		"/odd": "echo odd"
	}`)
	cfg := testMenuConfig(Task{Name: "dev", Run: "bun run dev"})
	cfg.Scripts = ScriptsConfig{Enable: true, PackageJSON: &path, Group: "scripts"}
	cfg = parseTestConfig(t, cfg)

	for _, tc := range []struct {
		args    []string
		command string
	}{
		// db is a script and a prefix: it runs like any script.
		{args: []string{"db"}, command: "echo db"},
		{args: []string{"db", "--dry-run"}, command: "echo db --dry-run"},
		{args: []string{"db", "migrate"}, command: "migrate up"},
		{args: []string{"db", "seed", "users"}, command: "seed users"},
		{args: []string{"lint:fix"}, command: "eslint --fix ."},
		// An empty word has nothing to nest under, so the name stays whole.
		{args: []string{"/odd"}, command: "echo odd"},
	} {
		got, err := Select(cfg, tc.args)
		if err != nil || got.Command != tc.command {
			t.Fatalf("x %v = %+v (%v), want %q", tc.args, got, err, tc.command)
		}
	}
	// seed is only a prefix: it opens its scripts and takes no arguments.
	if decision, err := resolveXInvocation(cfg, []string{"db", "seed"}); err != nil || decision.kind != collectSubcommandInvocation {
		t.Fatalf("x db seed = %#v (%v), want the subcommand picker", decision, err)
	}
	if _, err := resolveXInvocation(cfg, []string{"db", "seed", "nope"}); err == nil || err.Error() != `unknown command "db seed nope"` {
		t.Fatalf("x db seed nope error = %v", err)
	}

	var out strings.Builder
	List(&out, nil, nil, cfg, 100)
	rendered := ansi.Strip(out.String())
	indents := []int{
		rowIndent(rendered, "db echo db"),
		rowIndent(rendered, "seed 1 subcommand"),
		rowIndent(rendered, "users seed users"),
	}
	if indents[0] < 0 || indents[0] >= indents[1] || indents[1] >= indents[2] {
		t.Fatalf("rows should indent one level per depth, got %v:\n%s", indents, rendered)
	}
	lines := listLines(cfg)
	if !slices.Contains(lines, "SCRIPTS") || slices.Contains(lines, "DB") || slices.Contains(lines, "LINT") {
		t.Fatalf("every script should list under SCRIPTS:\n%s", strings.Join(lines, "\n"))
	}
}

func TestScriptsComeFromTheNearestPackageJSONOrTheProjectRoot(t *testing.T) {
	// The cases below build their own trees under the temp directory, so a
	// flake.nix or package.json above it would change what they resolve to.
	for _, name := range []string{"flake.nix", "package.json"} {
		if dir, found := nearestWith(os.TempDir(), name); found {
			t.Skipf("%s in %s, above the temp directory, would change path resolution", name, dir)
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "flake.nix"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	writePackageJSON(t, root, `{"root-task": "echo root"}`)
	web := filepath.Dir(writePackageJSON(t, filepath.Join(root, "web"), `{"web-task": "echo web"}`))
	src := filepath.Join(web, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	scriptDir := func(configured *string, task string) string {
		t.Helper()
		cfg := testMenuConfig(Task{Name: "dev", Run: "bun run dev"})
		cfg.Scripts = ScriptsConfig{Enable: true, PackageJSON: configured, Group: "scripts"}
		got, err := Select(parseTestConfig(t, cfg), []string{task})
		if err != nil {
			t.Fatalf("Select(%s): %v", task, err)
		}
		return got.Dir
	}

	t.Chdir(src)
	if got := scriptDir(nil, "web-task"); got != web {
		t.Fatalf("the nearest package.json should win, ran in %s", got)
	}
	relative := "package.json"
	if got := scriptDir(&relative, "root-task"); got != root {
		t.Fatalf("a configured relative path should resolve from the flake root, ran in %s", got)
	}

	// Without a flake.nix above, a relative path resolves from here.
	plain := t.TempDir()
	writePackageJSON(t, filepath.Join(plain, "tools"), `{"tool": "echo tool"}`)
	t.Chdir(plain)
	nested := "tools/package.json"
	if got := scriptDir(&nested, "tool"); got != filepath.Join(plain, "tools") {
		t.Fatalf("a relative path without a flake should resolve from the working directory, ran in %s", got)
	}

	t.Chdir(t.TempDir())
	cfg := testMenuConfig(Task{Name: "dev", Run: "bun run dev"})
	cfg.Scripts = ScriptsConfig{Enable: true, Group: "scripts"}
	if text := strings.Join(listLines(parseTestConfig(t, cfg)), "\n"); !strings.Contains(text, "package.json scripts unavailable: no package.json in") {
		t.Fatalf("--list should say why scripts are missing:\n%s", text)
	}
}

func TestRecipesAndDeclaredShortcutsKeepKeysScriptsAlsoUse(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "just"), []byte(`#!/bin/sh
printf '%s\n' '{"recipes":{"test":{"name":"test","namepath":"test","private":false}}}'
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	path := writePackageJSON(t, t.TempDir(), `{"test": "vitest", "lint": "eslint .", "d": "echo d"}`)
	cfg := testMenuConfig(Task{Name: "dev", Run: "bun run dev", Key: "d"})
	cfg.Just = JustConfig{Enable: true, Group: "just"}
	cfg.Scripts = ScriptsConfig{Enable: true, PackageJSON: &path, Group: "scripts"}
	cfg = parseTestConfig(t, cfg)

	for word, want := range map[string]string{"test": "just", "lint": "scripts", "d": "declared"} {
		got, err := Select(cfg, []string{word})
		if err != nil || got.Source != want {
			t.Fatalf("x %s chose %+v (%v), want the %s entry", word, got, err, want)
		}
	}
	lines := listLines(cfg)
	for _, note := range []string{
		"test: package.json script hidden by just recipe",
		"d: package.json script hidden by declared command dev",
	} {
		if !slices.Contains(lines, note) {
			t.Fatalf("--list should say %q:\n%s", note, strings.Join(lines, "\n"))
		}
	}
}
