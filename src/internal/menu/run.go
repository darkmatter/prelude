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

// Run is the binary entry point. defaultConfigPath is injected by Nix at link
// time via ldflags; it acts as the fallback when PRELUDE_MENU_CONFIG is unset.
func Run(defaultConfigPath string) {
	configPathDefault := os.Getenv("PRELUDE_MENU_CONFIG")
	if configPathDefault == "" {
		configPathDefault = defaultConfigPath
	}
	cfgPath := flag.String("config", configPathDefault, "path to the menu config JSON")
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
	if cfg.Just.Enable {
		if tasks, err := loadJustTasks(cfg.Just); err == nil {
			mergeJustTasks(cfg, tasks)
		} else {
			cfg.justImportWarning = "just recipes unavailable; check that just and a Justfile are available"
		}
	}
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
		runTUI(cfg, st, nil)

	case len(args) > 0:
		// `menu` only opens the interactive picker. Execution and listing
		// belong to the public `x` dispatcher.
		w := shared.ColorWriter(os.Stderr, os.Environ(), cfg.ColorProfile)
		fmt.Fprintln(w, st.errText.Render("menu: opens the interactive picker only"))
		fmt.Fprintln(w, st.dim.Render("hint: run commands with `x <key>`; list with `x --list`"))
		os.Exit(1)

	default:
		runTUI(cfg, st, nil)
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
		runTUI(cfg, st, &decision.task)
	}
}

func runTUI(cfg *Config, st styles, argTask *Task) {
	runProgram(cfg, st, newModel(cfg, st, argTask))
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
	options := []tea.ProgramOption{}
	if profile, ok := shared.ConfiguredColorProfile(cfg.ColorProfile); ok {
		options = append(options, tea.WithColorProfile(profile))
	}
	p := tea.NewProgram(m, options...)
	final, err := p.Run()
	if err != nil {
		w := shared.ColorWriter(os.Stderr, os.Environ(), cfg.ColorProfile)
		fmt.Fprintln(w, "menu:", err)
		fmt.Fprintln(w, st.dim.Render("hint: `x --list` prints the tasks non-interactively"))
		os.Exit(1)
	}
	if fm, ok := final.(model); ok && fm.hasExecCmd {
		finish(cfg, st, fm.execCmd)
	}
}

// finish either execs the assembled command (replacing this process) or
// prints it, per the execute option. This is the non-TUI (x fast path)
// exit: syscall.Exec replaces the menu process so there is nothing to
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

// execCommandCmd builds a tea.Cmd that runs the assembled command as a child
// process via the Program's ExecProcess facility. The Program pauses, releases
// the terminal to the child shell, and resumes on completion — so the menu
// survives the execution and returns to its prior filter/selection state.
// The callback maps the child's exit error (if any) to execFinishedMsg.
func execCommandCmd(cmd string) tea.Cmd {
	sh, err := shellPath()
	if err != nil {
		return func() tea.Msg { return execFinishedMsg{err: err} }
	}
	c := exec.Command(sh, "-c", cmd)
	c.Env = os.Environ()
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return execFinishedMsg{err: err}
	})
}
