package menu

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEnterOnReadyCommandQuitsToFinish(t *testing.T) {
	cfg := testMenuConfig(Task{Name: "dev", Run: "nix develop -c $SHELL"})
	cfg.Execute = true
	m := newModel(cfg, newStyles(cfg), nil)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if cmd == nil {
		t.Fatal("enter on a ready command returned a nil command")
	}
	if !got.hasExecCmd {
		t.Fatal("enter kept the menu alive instead of handing the command to finish")
	}
	if got.execCmd != "nix develop -c $SHELL" {
		t.Fatalf("execCmd = %q, want assembled run script", got.execCmd)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("enter must quit the TUI so finish replaces this process")
	}
}

func TestArgSubmitQuitsToFinish(t *testing.T) {
	task := Task{
		Name: "deploy",
		Run:  "just deploy",
		Args: []Arg{{Token: "ENV", Required: true}},
	}
	cfg := testMenuConfig(task)
	cfg.Execute = true
	m := newModel(cfg, newStyles(cfg), &task)
	m.prompt = m.prompt.WithValue("prod")

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if cmd == nil {
		t.Fatal("arg submit returned a nil command")
	}
	if !got.hasExecCmd {
		t.Fatal("arg submit kept the menu alive instead of handing the command to finish")
	}
	if got.execCmd != "just deploy prod" {
		t.Fatalf("execCmd = %q, want assembled run script", got.execCmd)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("arg submit must quit the TUI so finish replaces this process")
	}
}

func TestEnterOnMultilineCommandPreservesScriptForFinish(t *testing.T) {
	const run = "sync-docs\nrecord-docs"
	cfg := testMenuConfig(Task{Name: "gen", Run: run})
	cfg.Execute = true
	m := newModel(cfg, newStyles(cfg), nil)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if got.execCmd != run {
		t.Fatalf("execCmd = %q, want original multiline script", got.execCmd)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("enter must quit the TUI so finish replaces this process")
	}
}

func TestEnterOnArgTaskStaysInMenu(t *testing.T) {
	cfg := testMenuConfig(Task{
		Name: "deploy",
		Run:  "just deploy",
		Args: []Arg{{Token: "ENV", Required: true}},
	})
	cfg.Execute = true
	m := newModel(cfg, newStyles(cfg), nil)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if cmd != nil {
		t.Fatal("enter on an arg task must not quit or exec")
	}
	if got.hasExecCmd {
		t.Fatal("enter on an arg task must not mark a command for finish")
	}
	if got.mode != modeArgs {
		t.Fatalf("mode = %d, want argument-entry", got.mode)
	}
}

func TestEnterOnModuleParentOpensSubmenu(t *testing.T) {
	cfg := testMenuConfig(Task{
		Name:  "e2e",
		Label: "e2e",
		Run:   "just e2e",
		Children: []Task{
			{Name: "e2e::coder", Label: "coder", Run: "just e2e::coder"},
		},
	})
	m := newModel(cfg, newStyles(cfg), nil)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if cmd != nil {
		t.Fatal("enter on a module parent must not quit or exec")
	}
	if got.hasExecCmd {
		t.Fatal("enter on a module parent must not mark a command for finish")
	}
	if got.sub == nil || got.sub.Name != "e2e" {
		t.Fatalf("sub = %#v, want the e2e parent", got.sub)
	}
	if len(got.flat) != 1 || got.flat[0].Name != "e2e::coder" {
		t.Fatalf("submenu list = %#v", got.flat)
	}
	if got.promptCtx != "e2e" {
		t.Fatalf("promptCtx = %q, want the module label", got.promptCtx)
	}
}

func TestEscFromSubmenuRestoresRootList(t *testing.T) {
	cfg := testMenuConfig(Task{
		Name:  "e2e",
		Label: "e2e",
		Run:   "just e2e",
		Children: []Task{
			{Name: "e2e::coder", Label: "coder", Run: "just e2e::coder"},
		},
	})
	m := newModel(cfg, newStyles(cfg), nil)

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	next, cmd := next.(model).Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if cmd != nil {
		t.Fatal("esc from a submenu must not quit the menu")
	}
	if got.sub != nil {
		t.Fatal("esc must close the submenu")
	}
	if len(got.flat) != 1 || got.flat[0].Name != "e2e" {
		t.Fatalf("root list = %#v", got.flat)
	}
	if len(got.matches) != 1 || got.flat[got.matches[0]].Name != "e2e" {
		t.Fatalf("root matches = %#v", got.matches)
	}
	if got.promptCtx != "~/test" {
		t.Fatalf("promptCtx = %q, want the project context restored", got.promptCtx)
	}
}

func TestEnterOnSubmenuChildOpensArgMode(t *testing.T) {
	cfg := testMenuConfig(Task{
		Name:  "e2e",
		Label: "e2e",
		Run:   "just e2e",
		Children: []Task{{
			Name:  "e2e::coder",
			Label: "coder",
			Run:   "just e2e::coder",
			Args:  []Arg{{Token: "file", Required: true}},
		}},
	})
	m := newModel(cfg, newStyles(cfg), nil)

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	next, cmd := next.(model).Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if cmd != nil {
		t.Fatal("enter on an arg subcommand must not quit or exec")
	}
	if got.mode != modeArgs {
		t.Fatalf("mode = %d, want argument-entry", got.mode)
	}
	if got.args.Task() == nil || got.args.Task().Name != "e2e::coder" {
		t.Fatalf("arg task = %#v, want the subcommand", got.args.Task())
	}
}
