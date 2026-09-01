package menu

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseJustDumpProjectsPublicRecipes(t *testing.T) {
	data := []byte(`{
		"recipes": {
			"build": {
				"doc": "build the project",
				"name": "build",
				"namepath": "build",
				"parameters": [],
				"private": false
			},
			"deploy": {
				"doc": "deploy to an environment",
				"name": "deploy",
				"namepath": "deploy",
				"parameters": [
					{"default": null, "flag": false, "help": "target environment", "kind": "singular", "name": "environment"},
					{"default": "production", "flag": false, "help": null, "kind": "singular", "name": "stage"},
					{"default": null, "flag": true, "help": "skip confirmation", "kind": "singular", "name": "confirm", "long": "confirm"}
				],
				"private": false
			}
		},
		"modules": {
			"database": {
				"recipes": {
					"migrate": {
						"doc": "run migrations",
						"name": "migrate",
						"namepath": "database::migrate",
						"parameters": [],
						"private": false
					}
				},
				"modules": {}
			}
		}
	}`)

	tasks, err := parseJustDump(data, JustConfig{Enable: true, Group: "just"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Fatalf("got %d tasks, want 3", len(tasks))
	}

	build := findTask(tasks, "build")
	if build == nil {
		t.Fatal("build recipe was not imported")
	}
	if build.Run != "just build" || build.Description != "build the project" || build.group != "just" {
		t.Fatalf("build task = %#v", *build)
	}

	deploy := findTask(tasks, "deploy")
	if deploy == nil {
		t.Fatal("deploy recipe was not imported")
	}
	if len(deploy.Args) != 3 {
		t.Fatalf("deploy has %d args, want 3", len(deploy.Args))
	}
	if !deploy.Args[0].Required || deploy.Args[1].Required {
		t.Fatalf("required args = %#v", deploy.Args)
	}
	if !deploy.Args[2].Boolean || deploy.Args[2].Token != "--confirm" {
		t.Fatalf("flag arg = %#v", deploy.Args[2])
	}

	migrate := findTask(tasks, "database::migrate")
	if migrate == nil {
		t.Fatal("module recipe was not imported")
	}
	if migrate.group != "database" || migrate.Label != "migrate" || migrate.Run != "just database::migrate" {
		t.Fatalf("module task = %#v", *migrate)
	}
}

func TestJustfilePathIsUsedForImportedInvocations(t *testing.T) {
	justfile := "/workspace/project.just"
	tasks, err := parseJustDump([]byte(`{
		"recipes": {"check": {"name": "check", "namepath": "check", "private": false}}
	}`), JustConfig{Enable: true, Justfile: &justfile})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Run != "just --justfile '/workspace/project.just' check" {
		t.Fatalf("tasks = %#v", tasks)
	}
}

func TestParseJustDumpSkipsPrivateRecipes(t *testing.T) {
	data := []byte(`{
		"recipes": {
			"_internal": {"name": "_internal", "namepath": "_internal", "private": true},
			"hidden": {"name": "hidden", "namepath": "hidden", "private": true},
			"public": {"name": "public", "namepath": "public", "private": false}
		}
	}`)

	tasks, err := parseJustDump(data, JustConfig{Enable: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Name != "public" {
		t.Fatalf("tasks = %#v", tasks)
	}
}

func TestMergeJustTasksPreservesExplicitTasks(t *testing.T) {
	cfg := &Config{Groups: []Group{{Title: "just", Tasks: []Task{{Name: "build", Run: "nix build"}}}}}
	tasks := []Task{
		{Name: "build", Run: "just build", group: "just"},
		{Name: "test", Label: "test", Run: "just test", group: "just"},
	}

	mergeJustTasks(cfg, tasks)
	if len(cfg.Groups) != 1 || len(cfg.Groups[0].Tasks) != 2 {
		t.Fatalf("groups = %#v", cfg.Groups)
	}
	if cfg.Groups[0].Tasks[0].Name != "build" || cfg.Groups[0].Tasks[0].Run != "nix build" {
		t.Fatalf("explicit task was not preserved: %#v", cfg.Groups[0].Tasks)
	}
	if cfg.Groups[0].Tasks[1].Name != "test" {
		t.Fatalf("imported task missing: %#v", cfg.Groups[0].Tasks)
	}
}

func TestLoadJustTasksRunsJustDump(t *testing.T) {
	dir := t.TempDir()
	just := filepath.Join(dir, "just")
	if err := os.WriteFile(just, []byte(`#!/bin/sh
printf '%s\n' '{"recipes":{"check":{"name":"check","namepath":"check","private":false}}}'
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	tasks, err := loadJustTasks(JustConfig{Enable: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || tasks[0].Name != "check" || tasks[0].Run != "just check" {
		t.Fatalf("tasks = %#v", tasks)
	}
}

func TestLoadJustTasksFailsWithoutJust(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := loadJustTasks(JustConfig{Enable: true})
	if err == nil {
		t.Fatal("loadJustTasks unexpectedly succeeded without just")
	}
}

func TestParseJustDumpAppliesGroupAttributes(t *testing.T) {
	data := []byte(`{
		"recipes": {
			"cleanup": {
				"attributes": [{"group": "ops"}],
				"doc": "clean up",
				"name": "cleanup",
				"namepath": "cleanup",
				"private": false
			},
			"admin-wipe": {
				"attributes": [{"group": ["ops", "admin"]}],
				"name": "admin-wipe",
				"namepath": "admin-wipe",
				"private": false
			},
			"multi": {
				"attributes": [{"group": "admin"}, {"group": "ops"}],
				"name": "multi",
				"namepath": "multi",
				"private": false
			},
			"mixed-attrs": {
				"attributes": ["unix", {"group": "ship"}],
				"name": "mixed-attrs",
				"namepath": "mixed-attrs",
				"private": false
			},
			"build": {"name": "build", "namepath": "build", "private": false}
		}
	}`)

	tasks, err := parseJustDump(data, JustConfig{Enable: true, Group: "just"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 5 {
		t.Fatalf("got %d tasks, want 5", len(tasks))
	}
	if task := findTask(tasks, "cleanup"); task == nil || task.group != "ops" {
		t.Fatalf("cleanup task = %#v", task)
	}
	// A list-valued group attribute keeps its first member for placement.
	if task := findTask(tasks, "admin-wipe"); task == nil || task.group != "ops" {
		t.Fatalf("admin-wipe task = %#v", task)
	}
	// Several group attributes: the first in dump order owns placement.
	if task := findTask(tasks, "multi"); task == nil || task.group != "admin" {
		t.Fatalf("multi task = %#v", task)
	}
	// Plain-string attributes ([private], [unix]) must not break parsing.
	if task := findTask(tasks, "mixed-attrs"); task == nil || task.group != "ship" {
		t.Fatalf("mixed-attrs task = %#v", task)
	}
	// Recipes without attributes stay in the configured default group.
	if task := findTask(tasks, "build"); task == nil || task.group != "just" {
		t.Fatalf("build task = %#v", task)
	}
}

func TestParseJustDumpGroupAttributeOverridesModuleNamepath(t *testing.T) {
	data := []byte(`{
		"recipes": {},
		"modules": {
			"deploy": {
				"recipes": {
					"staging": {
						"attributes": [{"group": "ship"}],
						"doc": "deploy to staging",
						"name": "staging",
						"namepath": "deploy::staging",
						"private": false
					}
				},
				"modules": {}
			}
		}
	}`)

	tasks, err := parseJustDump(data, JustConfig{Enable: true, Group: "just"})
	if err != nil {
		t.Fatal(err)
	}
	task := findTask(tasks, "deploy::staging")
	if task == nil {
		t.Fatal("module recipe was not imported")
	}
	if task.group != "ship" || task.Label != "staging" || task.Run != "just deploy::staging" {
		t.Fatalf("module task = %#v", *task)
	}
}

func TestParseJustDumpImportsAliases(t *testing.T) {
	data := []byte(`{
		"recipes": {
			"build": {
				"doc": "build the project",
				"name": "build",
				"namepath": "build",
				"private": false
			},
			"container-config": {
				"attributes": [{"group": "ops"}],
				"doc": "validate the compose stack",
				"name": "container-config",
				"namepath": "container-config",
				"private": false
			},
			"secret": {"name": "secret", "namepath": "secret", "private": true}
		},
		"aliases": {
			"b": {"name": "b", "target": "build"},
			"cc": {"name": "cc", "target": "container-config"},
			"s": {"name": "s", "target": "secret"},
			"dangling": {"name": "dangling", "target": "missing"}
		},
		"modules": {
			"deploy": {
				"recipes": {
					"staging": {
						"doc": "deploy to staging",
						"name": "staging",
						"namepath": "deploy::staging",
						"private": false
					}
				},
				"aliases": {"st": {"name": "st", "target": "staging"}},
				"modules": {}
			}
		}
	}`)

	tasks, err := parseJustDump(data, JustConfig{Enable: true, Group: "just"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 6 {
		t.Fatalf("got %d tasks, want 6", len(tasks))
	}

	b := findTask(tasks, "b")
	if b == nil {
		t.Fatal("root alias was not imported")
	}
	if b.Run != "just b" || b.group != "just" || b.Description != "build the project" || b.Details != "Alias of build" {
		t.Fatalf("alias task = %#v", *b)
	}

	cc := findTask(tasks, "cc")
	if cc == nil {
		t.Fatal("alias to a grouped recipe was not imported")
	}
	if cc.group != "ops" {
		t.Fatalf("alias must inherit the target group, got %#v", *cc)
	}

	st := findTask(tasks, "deploy::st")
	if st == nil {
		t.Fatal("module alias was not imported")
	}
	if st.Run != "just deploy::st" || st.group != "deploy" || st.Label != "st" {
		t.Fatalf("module alias task = %#v", *st)
	}
	if st.Description != "deploy to staging" {
		t.Fatalf("module alias description = %#v", st.Description)
	}

	if findTask(tasks, "s") != nil {
		t.Fatal("alias to a private recipe must not be imported")
	}
	if findTask(tasks, "dangling") != nil {
		t.Fatal("alias with a dangling target must not be imported")
	}
}

func TestParseJustDumpAliasWithoutTargetDoc(t *testing.T) {
	tasks, err := parseJustDump([]byte(`{
		"recipes": {"build": {"name": "build", "namepath": "build", "private": false}},
		"aliases": {"b": {"name": "b", "target": "build"}}
	}`), JustConfig{Enable: true})
	if err != nil {
		t.Fatal(err)
	}
	b := findTask(tasks, "b")
	if b == nil {
		t.Fatal("alias was not imported")
	}
	if b.Description != "alias of build" {
		t.Fatalf("description = %#v", b.Description)
	}
}

func TestParseJustDumpMapsTypedArgAttributes(t *testing.T) {
	data := []byte(`{
		"recipes": {
			"compound": {
				"doc": "compound fees",
				"name": "compound",
				"namepath": "compound",
				"private": false,
				"parameters": [
					{"default": "base", "flag": false, "help": "comma list of chains", "kind": "singular", "long": "chains", "name": "chains"},
					{"default": "", "flag": false, "help": "target Safe (addr or ENS)", "kind": "singular", "long": "safe", "name": "safe"},
					{"default": null, "flag": true, "help": "send transactions", "kind": "singular", "long": "broadcast", "name": "broadcast"},
					{"default": null, "flag": false, "help": "extra flags after --", "kind": "star", "name": "extra"}
				]
			}
		}
	}`)

	tasks, err := parseJustDump(data, JustConfig{Enable: true, Group: "just"})
	if err != nil {
		t.Fatal(err)
	}
	task := findTask(tasks, "compound")
	if task == nil {
		t.Fatal("compound recipe was not imported")
	}
	if task.Usage != "just compound [--chains=base] [--safe] [--broadcast] [extra...]" {
		t.Fatalf("usage = %#v", task.Usage)
	}
	if len(task.Args) != 4 {
		t.Fatalf("got %d args, want 4", len(task.Args))
	}

	chains := task.Args[0]
	if chains.Token != "--chains" || chains.Description != "comma list of chains" ||
		chains.Required || chains.Default == nil || *chains.Default != "base" {
		t.Fatalf("chains arg = %#v", chains)
	}
	safe := task.Args[1]
	if safe.Token != "--safe" || safe.Required || safe.Default == nil || *safe.Default != "" {
		t.Fatalf("safe arg = %#v", safe)
	}
	broadcast := task.Args[2]
	if broadcast.Token != "--broadcast" || !broadcast.Boolean || broadcast.Required {
		t.Fatalf("broadcast arg = %#v", broadcast)
	}
	extra := task.Args[3]
	if extra.Token != "extra" || extra.Required {
		t.Fatalf("extra arg = %#v", extra)
	}
}

func TestParseJustDumpPatternAlternationBecomesOptions(t *testing.T) {
	data := []byte(`{
		"recipes": {
			"probe": {
				"name": "probe",
				"namepath": "probe",
				"private": false,
				"parameters": [
					{"default": null, "flag": false, "help": null, "kind": "star", "name": "flag", "pattern": ["--help|--version"]}
				]
			}
		}
	}`)

	tasks, err := parseJustDump(data, JustConfig{Enable: true, Group: "just"})
	if err != nil {
		t.Fatal(err)
	}
	task := findTask(tasks, "probe")
	if task == nil {
		t.Fatal("probe recipe was not imported")
	}
	options := task.Args[0].Options
	if len(options) != 2 || options[0] != "--help" || options[1] != "--version" {
		t.Fatalf("options = %#v", options)
	}
}

func TestParseJustDumpRegexPatternStaysOptionless(t *testing.T) {
	data := []byte(`{
		"recipes": {
			"probe": {
				"name": "probe",
				"namepath": "probe",
				"private": false,
				"parameters": [
					{"default": null, "flag": false, "help": null, "kind": "singular", "name": "id", "pattern": ["^[a-z]+$"]}
				]
			}
		}
	}`)

	tasks, err := parseJustDump(data, JustConfig{Enable: true, Group: "just"})
	if err != nil {
		t.Fatal(err)
	}
	task := findTask(tasks, "probe")
	if task == nil {
		t.Fatal("probe recipe was not imported")
	}
	if len(task.Args[0].Options) != 0 {
		t.Fatalf("regex pattern must not become options: %#v", task.Args[0].Options)
	}
}

func findTask(tasks []Task, name string) *Task {
	for index := range tasks {
		if tasks[index].Name == name {
			return &tasks[index]
		}
	}
	return nil
}
