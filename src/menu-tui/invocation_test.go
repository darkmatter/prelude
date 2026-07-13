package main

import (
	"reflect"
	"strings"
	"testing"
)

func invocationTestConfig() *Config {
	return &Config{Groups: []Group{{
		Title: "develop",
		Tasks: []Task{
			{Name: "d", Run: "just named"},
			{Name: "deploy", Key: "d", Run: "just deploy", Args: []Arg{{Token: "<environment>", Required: true}}},
			{Name: "check", Key: "c", Run: "just check"},
		},
	}}}
}

func TestResolveInvocationPrefersExactNameOverKey(t *testing.T) {
	cfg := &Config{Groups: []Group{
		{Title: "first", Tasks: []Task{{Name: "deploy", Key: "d", Run: "just deploy"}}},
		{Title: "second", Tasks: []Task{{Name: "d", Run: "just named"}}},
	}}
	decision, err := resolveInvocation(cfg, "d", nil)
	if err != nil {
		t.Fatal(err)
	}
	if decision.kind != commandInvocation || decision.command != "just named" {
		t.Fatalf("decision = %#v, want command %q", decision, "just named")
	}
}

func TestResolveInvocationFindsTaskByKey(t *testing.T) {
	decision, err := resolveInvocation(invocationTestConfig(), "c", nil)
	if err != nil {
		t.Fatal(err)
	}
	if decision.kind != commandInvocation || decision.command != "just check" {
		t.Fatalf("decision = %#v, want command %q", decision, "just check")
	}
}

func TestResolveInvocationRejectsUnknownTask(t *testing.T) {
	decision, err := resolveInvocation(invocationTestConfig(), "missing", nil)
	if err == nil {
		t.Fatal("unknown selection must return an error")
	}
	if decision.kind != invalidInvocation {
		t.Fatalf("decision kind = %v, want invalid", decision.kind)
	}
	if got := err.Error(); got != `unknown task "missing"` {
		t.Fatalf("error = %q", got)
	}
}

func TestResolveInvocationMatchesBeginDecisionWithoutExtras(t *testing.T) {
	cfg := invocationTestConfig()
	tests := []struct {
		selector string
		task     Task
	}{
		{selector: "d", task: cfg.Groups[0].Tasks[0]},
		{selector: "deploy", task: cfg.Groups[0].Tasks[1]},
	}
	for _, tt := range tests {
		t.Run(tt.selector, func(t *testing.T) {
			direct, err := resolveInvocation(cfg, tt.selector, nil)
			if err != nil {
				t.Fatal(err)
			}
			interactive := beginInvocation(tt.task)
			if !reflect.DeepEqual(direct, interactive) {
				t.Fatalf("direct decision = %#v, TUI decision = %#v", direct, interactive)
			}
		})
	}
}

func TestResolveInvocationTreatsExplicitEmptyArgumentAsProvided(t *testing.T) {
	decision, err := resolveInvocation(invocationTestConfig(), "deploy", []string{""})
	if err != nil {
		t.Fatal(err)
	}
	if decision.kind != commandInvocation || decision.command != "just deploy" {
		t.Fatalf("decision = %#v, want immediate command", decision)
	}
}

func TestResolveInvocationJoinsDirectArgumentsWithoutQuoting(t *testing.T) {
	decision, err := resolveInvocation(
		invocationTestConfig(),
		"deploy",
		[]string{"staging west", "&&", "notify"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "just deploy staging west && notify"
	if decision.kind != commandInvocation || decision.command != want {
		t.Fatalf("decision = %#v, want command %q", decision, want)
	}
}

func TestResolveInvocationPreservesSpacesInsideDirectArguments(t *testing.T) {
	decision, err := resolveInvocation(
		invocationTestConfig(),
		"deploy",
		[]string{" staging ", " prod "},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "just deploy  staging   prod"
	if decision.kind != commandInvocation || decision.command != want {
		t.Fatalf("decision = %#v, want command %q", decision, want)
	}
}

func TestBeginInvocationCollectsDeclaredArguments(t *testing.T) {
	task := Task{Name: "deploy", Run: "just deploy", Args: []Arg{{Token: "<environment>"}}}
	decision := beginInvocation(task)
	if decision.kind != collectArgumentsInvocation || !reflect.DeepEqual(decision.task, task) {
		t.Fatalf("decision = %#v, want argument collection", decision)
	}
}

func TestCompleteInvocationReportsFirstRequiredArgument(t *testing.T) {
	task := Task{
		Name: "deploy",
		Run:  "just deploy",
		Args: []Arg{
			{Token: "[optional]"},
			{Token: "<environment>", Required: true},
			{Token: "<region>", Required: true},
		},
	}
	decision, err := completeInvocation(task, " \t ")
	if err == nil {
		t.Fatal("blank required arguments must return an error")
	}
	if decision.kind != invalidInvocation {
		t.Fatalf("decision kind = %v, want invalid", decision.kind)
	}
	if got := err.Error(); got != "deploy: missing required argument <environment>" {
		t.Fatalf("error = %q", got)
	}
}

func TestCompleteInvocationAcceptsBlankOptionalArguments(t *testing.T) {
	task := Task{Name: "check", Run: " just check ", Args: []Arg{{Token: "[package]"}}}
	decision, err := completeInvocation(task, "  ")
	if err != nil {
		t.Fatal(err)
	}
	if decision.kind != commandInvocation || decision.command != "just check" {
		t.Fatalf("decision = %#v, want normalized command", decision)
	}
}

func TestCompleteInvocationPreservesOpaqueShellText(t *testing.T) {
	task := Task{Name: "deploy", Run: "just deploy", Args: []Arg{{Token: "<environment>", Required: true}}}
	decision, err := completeInvocation(task, "  staging | tee /tmp/deploy.log  ")
	if err != nil {
		t.Fatal(err)
	}
	want := "just deploy staging | tee /tmp/deploy.log"
	if decision.kind != commandInvocation || decision.command != want {
		t.Fatalf("decision = %#v, want command %q", decision, want)
	}
}

func TestBeginInvocationRepresentsEmptyCommand(t *testing.T) {
	decision := beginInvocation(Task{Name: "empty", Run: " \t "})
	if decision.kind != commandInvocation {
		t.Fatalf("decision kind = %v, want command", decision.kind)
	}
	if decision.command != "" {
		t.Fatalf("command = %q, want empty normalized command", decision.command)
	}
}

func TestOptionTokenConstruction(t *testing.T) {
	tests := []struct {
		name   string
		arg    Arg
		option string
		want   string
	}{
		{name: "boolean", arg: Arg{Token: "--force", Boolean: true}, option: "ignored", want: "--force"},
		{name: "long option", arg: Arg{Token: "--environment"}, option: "staging", want: "--environment staging"},
		{name: "positional", arg: Arg{Token: "<environment>"}, option: "staging", want: "staging"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := invocationToken(tt.arg, tt.option); got != tt.want {
				t.Fatalf("token = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAssembleInvocationTrimsCompleteCommand(t *testing.T) {
	got := assembleInvocation(Task{Run: "  just deploy  "}, "  staging  ")
	if got != "just deploy     staging" {
		t.Fatalf("command = %q", got)
	}
	if strings.HasPrefix(got, " ") || strings.HasSuffix(got, " ") {
		t.Fatalf("command must be trimmed: %q", got)
	}
}
