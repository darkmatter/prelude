package menu

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestEnterOnReadyCommandQuitsToFinish(t *testing.T) {
	cfg := testMenuConfig(Task{Name: "dev", Run: "nix develop -c $SHELL"})
	cfg.Execute = true
	m := newModel(cfg, newStyles(cfg, false), nil)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if cmd == nil {
		t.Fatal("enter on a ready command returned a nil command")
	}
	if got.chosen == nil {
		t.Fatal("enter kept the menu alive instead of handing the command to finish")
	}
	if got.chosen.Command != "nix develop -c $SHELL" {
		t.Fatalf("chosen.Command = %q, want assembled run script", got.chosen.Command)
	}
	if got.chosen.Name != "dev" || got.chosen.Line != "" {
		t.Fatalf("chosen = %+v, want the dev task with no argument line", *got.chosen)
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
	m := newModel(cfg, newStyles(cfg, false), &task)
	m.prompt = m.prompt.WithValue("prod")

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if cmd == nil {
		t.Fatal("arg submit returned a nil command")
	}
	if got.chosen == nil {
		t.Fatal("arg submit kept the menu alive instead of handing the command to finish")
	}
	if got.chosen.Command != "just deploy prod" {
		t.Fatalf("chosen.Command = %q, want assembled run script", got.chosen.Command)
	}
	// Library hosts parse the argument text themselves, so it must survive
	// separately from the assembled command.
	if got.chosen.Name != "deploy" || got.chosen.Line != "prod" {
		t.Fatalf("chosen = %+v, want the deploy task with line %q", *got.chosen, "prod")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("arg submit must quit the TUI so finish replaces this process")
	}
}

func TestEnterOnMultilineCommandPreservesScriptForFinish(t *testing.T) {
	const run = "sync-docs\nrecord-docs"
	cfg := testMenuConfig(Task{Name: "gen", Run: run})
	cfg.Execute = true
	m := newModel(cfg, newStyles(cfg, false), nil)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if got.chosen == nil || got.chosen.Command != run {
		t.Fatalf("chosen = %+v, want original multiline script", got.chosen)
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
	m := newModel(cfg, newStyles(cfg, false), nil)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if cmd != nil {
		t.Fatal("enter on an arg task must not quit or exec")
	}
	if got.chosen != nil {
		t.Fatal("enter on an arg task must not mark a command for finish")
	}
	if got.mode != modeArgs {
		t.Fatalf("mode = %d, want argument-entry", got.mode)
	}
}

func TestEnterOnModuleParentOpensSubmenu(t *testing.T) {
	cfg := testMenuConfig(Task{
		Name:       "e2e",
		Label:      "e2e",
		Run:        "just e2e",
		justModule: true,
		Children: []Task{
			{Name: "e2e::coder", Label: "coder", Run: "just e2e::coder"},
		},
	})
	m := newModel(cfg, newStyles(cfg, false), nil)

	next, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if cmd != nil {
		t.Fatal("enter on a module parent must not quit or exec")
	}
	if got.chosen != nil {
		t.Fatal("enter on a module parent must not mark a command for finish")
	}
	if got.sub() == nil || got.sub().Name != "e2e" {
		t.Fatalf("sub = %#v, want the e2e parent", got.sub())
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
		Name:       "e2e",
		Label:      "e2e",
		Run:        "just e2e",
		justModule: true,
		Children: []Task{
			{Name: "e2e::coder", Label: "coder", Run: "just e2e::coder"},
		},
	})
	m := newModel(cfg, newStyles(cfg, false), nil)

	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	next, cmd := next.(model).Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	if cmd != nil {
		t.Fatal("esc from a submenu must not quit the menu")
	}
	if got.sub() != nil {
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
		Name:       "e2e",
		Label:      "e2e",
		Run:        "just e2e",
		justModule: true,
		Children: []Task{{
			Name:  "e2e::coder",
			Label: "coder",
			Run:   "just e2e::coder",
			Args:  []Arg{{Token: "file", Required: true}},
		}},
	})
	m := newModel(cfg, newStyles(cfg, false), nil)

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

func TestStandaloneCommandRunsWhereTheSelectionSays(t *testing.T) {
	if got := standaloneCommand(&Config{}, &Selection{Command: "go test ./..."}); got != "go test ./..." {
		t.Fatalf("a selection with nowhere to go should print bare, got %q", got)
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "greet"), []byte("#!/bin/sh\necho \"hi from $(pwd)\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A trailing comment must not swallow the subshell's closing parenthesis.
	sel := &Selection{Command: "greet # say hello", Dir: dir, PathPrefix: []string{bin}}
	sh, err := shellPath()
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(sh, "-c", standaloneCommand(&Config{}, sel)).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", standaloneCommand(&Config{}, sel), err, out)
	}
	if string(out) != "hi from "+dir+"\n" {
		t.Fatalf("output = %q, want greet run from %s", out, dir)
	}

	// A bound menu's printed command carries what exec mode would hand it,
	// even into a shell that has none of it.
	bound := &Config{Root: dir, path: "/menu/config.json"}
	printed := standaloneCommand(bound, &Selection{Command: `echo "$PRELUDE_ROOT $PRELUDE_MENU_CONFIG"`, Dir: dir})
	run := exec.Command(sh, "-c", printed)
	run.Env = []string{}
	out, err = run.CombinedOutput()
	if err != nil || string(out) != dir+" /menu/config.json\n" {
		t.Fatalf("%s printed %q (%v), want the root and the Config", printed, out, err)
	}
}

// press sends one key to m and returns the next model and command.
func press(t *testing.T, m model, key tea.KeyPressMsg) (model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(key)
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	return got, cmd
}

// pickerPath names the parents of the open pickers, outermost first.
func pickerPath(m model) []string {
	path := make([]string, len(m.pickers))
	for index, frame := range m.pickers {
		path[index] = frame.parent.Name
	}
	return path
}

func selectedName(m model) string {
	return m.flat[m.matches[m.sel]].Name
}

func TestNestedPickersPushAndPopOneLevel(t *testing.T) {
	cfg := testMenuConfig(subcommandTestTasks()...)
	m := newModel(cfg, newStyles(cfg, false), nil)

	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyEnter}) // db
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyDown})  // seed
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyEnter}) // into seed
	if path := pickerPath(m); !slices.Equal(path, []string{"db", "db seed"}) {
		t.Fatalf("pickers = %v, want db then db seed", path)
	}
	if !slices.Equal(taskNames(m.flat), []string{"db seed orders", "db seed users"}) || m.promptCtx != "seed" {
		t.Fatalf("seed picker lists %v under %q", taskNames(m.flat), m.promptCtx)
	}

	// Backspace on an empty query closes one level, back on the row it left.
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if path := pickerPath(m); !slices.Equal(path, []string{"db"}) || selectedName(m) != "db seed" || m.promptCtx != "db" {
		t.Fatalf("after backspace: pickers %v, selected %q, context %q", path, selectedName(m), m.promptCtx)
	}

	// The right arrow opens a container's subcommands as Enter does.
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if path := pickerPath(m); !slices.Equal(path, []string{"db", "db seed"}) {
		t.Fatalf("right on seed: pickers = %v", path)
	}

	// Esc clears a query first, then closes one level at a time, then quits.
	m.prompt = m.prompt.WithValue("ord")
	m.filter()
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.prompt.Value() != "" || len(m.pickers) != 2 {
		t.Fatalf("esc with a query: query %q, pickers %v", m.prompt.Value(), pickerPath(m))
	}
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	m, cmd := press(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if len(m.pickers) != 0 || cmd != nil || m.promptCtx != "~/test" || selectedName(m) != "db" {
		t.Fatalf("esc twice: pickers %v, context %q, selected %q", pickerPath(m), m.promptCtx, selectedName(m))
	}
	if _, cmd = press(t, m, tea.KeyPressMsg{Code: tea.KeyEsc}); cmd == nil {
		t.Fatal("esc at the root must quit")
	}
}

func TestRunnableParentRunsOnEnterAndOpensOnRight(t *testing.T) {
	cfg := testMenuConfig(subcommandTestTasks()...)
	m := newModel(cfg, newStyles(cfg, false), nil)
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyDown}) // deploy

	ran, cmd := press(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if ran.chosen == nil || ran.chosen.Command != "deploy.sh" || cmd == nil {
		t.Fatalf("enter on a runnable parent chose %+v, want it run", ran.chosen)
	}

	// Mid-query the right arrow moves the cursor; at the end it opens.
	m.prompt = m.prompt.WithValue("deploy")
	m.filter()
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyLeft})
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if len(m.pickers) != 0 {
		t.Fatalf("right mid-query opened %v", pickerPath(m))
	}
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if path := pickerPath(m); !slices.Equal(path, []string{"deploy"}) || !slices.Equal(taskNames(m.flat), []string{"deploy staging"}) {
		t.Fatalf("right on deploy: pickers %v listing %v", path, taskNames(m.flat))
	}

	// A leaf has nothing to open.
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if len(m.pickers) != 1 {
		t.Fatalf("right on a leaf changed pickers to %v", pickerPath(m))
	}
}

func TestPickerOpensUnderTheDispatchedParents(t *testing.T) {
	cfg := testMenuConfig(subcommandTestTasks()...)
	st := newStyles(cfg, false)

	decision, err := resolveXInvocation(cfg, []string{"db", "seed"})
	if err != nil {
		t.Fatal(err)
	}
	m := newPicker(cfg, st, decision)
	if path := pickerPath(m); !slices.Equal(path, []string{"db", "db seed"}) {
		t.Fatalf("x db seed opened pickers %v", path)
	}
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if selectedName(m) != "db seed" || m.promptCtx != "db" {
		t.Fatalf("backspace from x db seed: selected %q under %q", selectedName(m), m.promptCtx)
	}

	// Leaving argument entry returns to the picker the task was listed in.
	decision, err = resolveXInvocation(cfg, []string{"db", "seed", "users"})
	if err != nil {
		t.Fatal(err)
	}
	m = newPicker(cfg, st, decision)
	if m.mode != modeArgs || m.args.Task().Name != "db seed users" {
		t.Fatalf("x db seed users: mode %d task %#v", m.mode, m.args.Task())
	}
	m, _ = press(t, m, tea.KeyPressMsg{Code: tea.KeyEsc})
	if m.mode != modeList || m.promptCtx != "seed" || selectedName(m) != "db seed users" {
		t.Fatalf("esc from arguments: mode %d, context %q, selected %q", m.mode, m.promptCtx, selectedName(m))
	}
}
