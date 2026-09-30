package menu

import "testing"

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
				Name:     "e2e",
				Label:    "e2e",
				Run:      "just e2e",
				Children: []Task{{Name: "e2e::coder", Label: "coder", Run: "just e2e::coder"}},
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
