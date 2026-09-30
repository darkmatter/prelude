package menu

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"

	"prelude/pkg/shared"

	tea "charm.land/bubbletea/v2"
)

// debugLog is enabled via PRELUDE_MENU_DEBUG=<path> for TUI diagnostics.
var debugLog bool

// Run is the binary entry point. The Nix wrappers pass the config with
// --config; PRELUDE_MENU_CONFIG supplies it when the binary runs unwrapped.
func Run() {
	cfgPath := flag.String("config", os.Getenv("PRELUDE_MENU_CONFIG"), "path to the menu config JSON")
	xMode := flag.Bool("x", false, "dispatch using x command names")
	xList := flag.Bool("list", false, "list x commands")
	showHelp := flag.Bool("help", false, "print usage and exit")
	flag.Parse()
	if *showHelp {
		usage()
	}

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "menu:", err)
		os.Exit(1)
	}
	importJust(cfg)
	if path := os.Getenv("PRELUDE_MENU_DEBUG"); path != "" {
		if f, err := tea.LogToFile(path, "menu"); err == nil {
			defer f.Close()
			debugLog = true
			log.Println("debug log enabled")
		}
	}
	st := newStyles(cfg)

	args := flag.Args()
	switch {
	case *xMode && *xList:
		printList(cfg, st)

	case *xMode && len(args) > 0:
		xFastPath(cfg, st, args)

	case *xMode:
		// Bare `x` opens the same picker as bare `menu`.
		runTUI(cfg, st, nil, nil)

	case len(args) > 0:
		// `menu` only opens the interactive picker. Execution and listing
		// belong to the public `x` dispatcher.
		w := shared.ColorWriter(os.Stderr, os.Environ(), cfg.ColorProfile)
		fmt.Fprintln(w, st.errText.Render("menu: opens the interactive picker only"))
		fmt.Fprintln(w, st.dim.Render("hint: run commands with `x <key>`; list with `x --list`"))
		os.Exit(1)

	default:
		runTUI(cfg, st, nil, nil)
	}
}

func xFastPath(cfg *Config, st styles, args []string) {
	decision, err := resolveXInvocation(cfg, args)
	finishDecision(cfg, st, "x", decision, err)
}

func finishDecision(cfg *Config, st styles, command string, decision invocationDecision, err error) {
	if err != nil {
		w := shared.ColorWriter(os.Stderr, os.Environ(), cfg.ColorProfile)
		fmt.Fprintln(w, st.errText.Render(command+": "+err.Error()))
		os.Exit(1)
	}
	switch decision.kind {
	case commandInvocation:
		finish(cfg, st, decision.command)
	case collectArgumentsInvocation:
		runTUI(cfg, st, &decision.task, nil)
	case collectSubcommandInvocation:
		runTUI(cfg, st, nil, &decision.task)
	}
}

func runTUI(cfg *Config, st styles, argTask *Task, subTask *Task) {
	runProgram(cfg, st, newPicker(cfg, st, argTask, subTask))
}

// newPicker opens the TUI where a decision still needs input: argument entry
// for argTask, the subcommand picker for subTask, else the root list.
func newPicker(cfg *Config, st styles, argTask *Task, subTask *Task) model {
	m := newModel(cfg, st, argTask)
	if subTask != nil {
		m.enterSubMode(*subTask)
	}
	return m
}

// usage prints a short command synopsis to stderr and exits 0 without
// entering the TUI.
func usage() {
	fmt.Fprintln(os.Stderr, "usage: menu [--config path]")
	fmt.Fprintln(os.Stderr, "       x [--config path] [--list | <command-key> [args…]]")
	fmt.Fprintln(os.Stderr, "shortcuts: motd|?  x|m  docs|d")
	os.Exit(0)
}

func runProgram(cfg *Config, st styles, m model) {
	chosen, err := runPicker(cfg, m)
	if err != nil {
		w := shared.ColorWriter(os.Stderr, os.Environ(), cfg.ColorProfile)
		fmt.Fprintln(w, "menu:", err)
		fmt.Fprintln(w, st.dim.Render("hint: `x --list` prints the tasks non-interactively"))
		os.Exit(1)
	}
	if chosen != nil {
		finish(cfg, st, chosen.Command)
	}
}

// runPicker runs the TUI to completion and returns what it resolved, or nil
// when the user left without choosing. It never exits or execs, so library
// hosts share it with the CLI.
func runPicker(cfg *Config, m model) (*Selection, error) {
	options := []tea.ProgramOption{}
	if profile, ok := shared.ConfiguredColorProfile(cfg.ColorProfile); ok {
		options = append(options, tea.WithColorProfile(profile))
	}
	final, err := tea.NewProgram(m, options...).Run()
	if err != nil {
		return nil, err
	}
	if fm, ok := final.(model); ok {
		return fm.chosen, nil
	}
	return nil, nil
}

// finish either execs the assembled command (replacing this process) or
// prints it, per the execute option. The TUI quits before this is reached,
// so syscall.Exec replaces the menu process outright; there is nothing to
// return to.
func finish(cfg *Config, st styles, cmd string) {
	if !cfg.Execute {
		fmt.Println(cmd)
		return
	}
	w := shared.ColorWriter(os.Stdout, os.Environ(), cfg.ColorProfile)
	fmt.Fprintln(w)
	fmt.Fprintln(w, st.accent.Render("$ ")+st.fg.Render(cmd))
	fmt.Fprintln(w)

	sh, err := shellPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "menu:", err)
		os.Exit(1)
	}
	if err := syscall.Exec(sh, []string{sh, "-c", cmd}, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "menu: exec:", err)
		os.Exit(1)
	}
}

// shellPath returns the path to bash (preferred) or sh for command execution.
func shellPath() (string, error) {
	if sh, err := exec.LookPath("bash"); err == nil {
		return sh, nil
	}
	return exec.LookPath("sh")
}
