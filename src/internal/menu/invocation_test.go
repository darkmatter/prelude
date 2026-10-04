package menu

import (
	"slices"
	"testing"
)

func TestResolveXInvocationUsesCompleteCommandKey(t *testing.T) {
	cfg := xTestConfig()

	decision, err := resolveXInvocation(cfg, []string{"go:test"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.kind != commandInvocation || decision.command != "go test -C src ./..." {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestResolveXInvocationPreservesMultiColonKey(t *testing.T) {
	cfg := xTestConfig()

	decision, err := resolveXInvocation(cfg, []string{"test:unit:watch", "--", "--runInBand"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.command != "bun run test:unit:watch --runInBand" {
		t.Fatalf("command = %q", decision.command)
	}
}

func TestResolveXInvocationDoesNotResolveDisplayLabel(t *testing.T) {
	cfg := xTestConfig()

	if _, err := resolveXInvocation(cfg, []string{"test"}); err == nil {
		t.Fatal("display label unexpectedly resolved without the complete command key")
	}
}

func TestResolveXInvocationResolvesAcceleratorKey(t *testing.T) {
	cfg := xTestConfig()

	decision, err := resolveXInvocation(cfg, []string{"t"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.kind != commandInvocation || decision.command != "go test -C src ./..." {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestResolveXInvocationNameOutranksAcceleratorKey(t *testing.T) {
	cfg := &Config{Groups: []Group{
		{Title: "develop", Tasks: []Task{
			{Name: "t", Run: "echo name-wins"},
			{Name: "test", Key: "t", Run: "echo key-loses"},
		}},
	}}

	decision, err := resolveXInvocation(cfg, []string{"t"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.command != "echo name-wins" {
		t.Fatalf("command = %q, want name to outrank accelerator", decision.command)
	}
}

func submoduleTestConfig() *Config {
	return &Config{Groups: []Group{
		{Title: "just", Tasks: []Task{
			{
				Name:       "e2e",
				Label:      "e2e",
				Run:        "just e2e",
				Children:   []Task{{Name: "e2e::coder", Label: "coder", Run: "just e2e::coder"}},
				justModule: true,
			},
		}},
	}}
}

func TestResolveXInvocationRejectsModuleChildCompleteKey(t *testing.T) {
	cfg := submoduleTestConfig()

	if _, err := resolveXInvocation(cfg, []string{"e2e::coder", "extra"}); err == nil {
		t.Fatal("Just namepath unexpectedly resolved as a public selector")
	}
}

func TestResolveXInvocationRoutesParentSubcommandSelector(t *testing.T) {
	cfg := submoduleTestConfig()

	decision, err := resolveXInvocation(cfg, []string{"e2e", "coder"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.kind != commandInvocation || decision.command != "just e2e::coder" {
		t.Fatalf("decision = %#v", decision)
	}
}

func TestResolveXInvocationBareParentOpensSubcommandPicker(t *testing.T) {
	cfg := submoduleTestConfig()

	decision, err := resolveXInvocation(cfg, []string{"e2e"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.kind != collectSubcommandInvocation {
		t.Fatalf("decision = %#v, want the subcommand picker", decision)
	}
}

func TestResolveXInvocationParentFallsBackToModuleDispatch(t *testing.T) {
	cfg := submoduleTestConfig()

	decision, err := resolveXInvocation(cfg, []string{"e2e", "unknown", "flag"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.kind != commandInvocation || decision.command != "just e2e unknown flag" {
		t.Fatalf("decision = %#v", decision)
	}
}

func xTestConfig() *Config {
	return &Config{Groups: []Group{
		{Title: "go", Tasks: []Task{{Name: "go:test", Label: "test", Key: "t", Run: "go test -C src ./..."}}},
		{Title: "test", Tasks: []Task{{Name: "test:unit:watch", Label: "unit:watch", Run: "bun run test:unit:watch"}}},
	}}
}

// subcommandTestTasks is a declared tree as Nix writes it: db is a container
// (no run of its own) whose seed is another, and deploy a runnable parent.
func subcommandTestTasks() []Task {
	return []Task{
		{Name: "db", Label: "db", Description: "2 subcommands", Children: []Task{
			{Name: "db migrate", Label: "migrate", Run: "migrate"},
			{Name: "db seed", Label: "seed", Description: "2 subcommands", Children: []Task{
				{Name: "db seed orders", Label: "orders", Run: "seed orders"},
				{Name: "db seed users", Label: "users", Run: "seed users", Args: []Arg{{Token: "--count"}}},
			}},
		}},
		{Name: "deploy", Label: "deploy", Key: "d", Run: "deploy.sh", Children: []Task{
			{Name: "deploy staging", Label: "staging", Run: "deploy.sh --env staging"},
		}},
	}
}

func taskNames(tasks []Task) []string {
	names := make([]string, len(tasks))
	for index, task := range tasks {
		names[index] = task.Name
	}
	return names
}

func TestResolveXInvocationDescendsOneWordPerLevel(t *testing.T) {
	cfg := testMenuConfig(subcommandTestTasks()...)

	for _, tc := range []struct {
		args    []string
		kind    invocationKind
		task    string
		command string
		trail   []string
	}{
		{args: []string{"db", "seed", "orders"}, kind: commandInvocation, task: "db seed orders", command: "seed orders", trail: []string{"db", "db seed"}},
		// The deepest matching label wins; the words after it are arguments.
		{args: []string{"db", "seed", "users", "--count", "3"}, kind: commandInvocation, task: "db seed users", command: "seed users --count 3", trail: []string{"db", "db seed"}},
		{args: []string{"db", "migrate", "seed"}, kind: commandInvocation, task: "db migrate", command: "migrate seed", trail: []string{"db"}},
		{args: []string{"db", "seed", "users"}, kind: collectArgumentsInvocation, task: "db seed users", trail: []string{"db", "db seed"}},
		{args: []string{"db", "seed"}, kind: collectSubcommandInvocation, task: "db seed", trail: []string{"db"}},
		{args: []string{"db"}, kind: collectSubcommandInvocation, task: "db", trail: []string{}},
	} {
		decision, err := resolveXInvocation(cfg, tc.args)
		if err != nil {
			t.Fatalf("x %v: %v", tc.args, err)
		}
		if decision.kind != tc.kind || decision.task.Name != tc.task || decision.command != tc.command {
			t.Fatalf("x %v = kind %d task %q command %q, want kind %d task %q command %q",
				tc.args, decision.kind, decision.task.Name, decision.command, tc.kind, tc.task, tc.command)
		}
		if trail := taskNames(decision.trail); !slices.Equal(trail, tc.trail) {
			t.Fatalf("x %v trail = %v, want %v", tc.args, trail, tc.trail)
		}
	}
}

func TestResolveXInvocationRejectsUnknownWordsUnderContainers(t *testing.T) {
	cfg := testMenuConfig(subcommandTestTasks()...)

	for _, tc := range []struct {
		args []string
		want string
	}{
		{args: []string{"db", "nope"}, want: `unknown command "db nope"`},
		{args: []string{"db", "seed", "nope", "extra"}, want: `unknown command "db seed nope"`},
		// A container takes no arguments, so `--` has nothing to pass them to.
		{args: []string{"db", "--", "migrate"}, want: `unknown command "db --"`},
	} {
		if _, err := resolveXInvocation(cfg, tc.args); err == nil || err.Error() != tc.want {
			t.Fatalf("x %v error = %v, want %s", tc.args, err, tc.want)
		}
	}
}

func TestResolveXInvocationRunsRunnableParentsLikeLeaves(t *testing.T) {
	cfg := testMenuConfig(subcommandTestTasks()...)

	for _, tc := range []struct {
		args    []string
		command string
	}{
		{args: []string{"deploy"}, command: "deploy.sh"},
		{args: []string{"d"}, command: "deploy.sh"},
		{args: []string{"deploy", "staging"}, command: "deploy.sh --env staging"},
		{args: []string{"deploy", "production"}, command: "deploy.sh production"},
		// `--` ends the subcommand words, so a child's label passes as an argument.
		{args: []string{"deploy", "--", "staging"}, command: "deploy.sh staging"},
	} {
		decision, err := resolveXInvocation(cfg, tc.args)
		if err != nil {
			t.Fatalf("x %v: %v", tc.args, err)
		}
		if decision.kind != commandInvocation || decision.command != tc.command {
			t.Fatalf("x %v = %#v, want command %q", tc.args, decision, tc.command)
		}
	}
}

func TestResolveXInvocationRoutesGroupedModuleRecipeByModuleWords(t *testing.T) {
	cfg := submoduleTestConfig()
	mergeTasks(cfg, []Task{{
		Name:     "e2e::smoke",
		Label:    "smoke",
		Run:      "just e2e::smoke",
		Source:   sourceJust,
		group:    "ci",
		justPath: []string{"e2e", "smoke"},
	}})

	for _, tc := range []struct {
		args    []string
		command string
	}{
		{args: []string{"e2e", "smoke", "--fast"}, command: "just e2e::smoke --fast"},
		{args: []string{"e2e", "coder"}, command: "just e2e::coder"},
	} {
		decision, err := resolveXInvocation(cfg, tc.args)
		if err != nil {
			t.Fatalf("x %v: %v", tc.args, err)
		}
		if decision.kind != commandInvocation || decision.command != tc.command {
			t.Fatalf("x %v = %#v, want command %q", tc.args, decision, tc.command)
		}
	}
}
