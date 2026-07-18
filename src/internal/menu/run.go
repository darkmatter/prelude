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
	xHelp := flag.Bool("help", false, "show x command help")
	flag.Parse()

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "menu:", err)
		os.Exit(1)
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

	case *xMode && *xHelp:
		runHelp(cfg, st)

	case *xMode && len(args) > 0:
		xFastPath(cfg, st, args)

	case !*xMode && len(args) > 0 && args[0] == "list":
		printList(cfg, st)

	case !*xMode && len(args) > 0 && args[0] == "help":
		runHelp(cfg, st)

	case len(args) > 0:
		fastPath(cfg, st, args[0], args[1:])

	default:
		runTUI(cfg, st, nil)
	}
}

// fastPath resolves direct CLI task invocations. Tasks with declared args
// and no explicit extras open the TUI in argument-entry mode.
func fastPath(cfg *Config, st styles, selector string, extra []string) {
	decision, err := resolveInvocation(cfg, selector, extra)
	finishDecision(cfg, st, "menu", decision, err)
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

// runHelp opens the TUI directly in the manual viewer.
func runHelp(cfg *Config, st styles) {
	m := newModel(cfg, st, nil)
	m.mode = modeHelp
	runProgram(cfg, st, m)
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
		fmt.Fprintln(w, st.dim.Render("hint: `menu list` prints the tasks non-interactively"))
		os.Exit(1)
	}
	if fm, ok := final.(model); ok && fm.hasExecCmd {
		finish(cfg, st, fm.execCmd)
	}
}

// finish either execs the assembled command (replacing this process) or
// prints it, per the execute option.
func finish(cfg *Config, st styles, cmd string) {
	if !cfg.Execute {
		fmt.Println(cmd)
		return
	}
	w := shared.ColorWriter(os.Stdout, os.Environ(), cfg.ColorProfile)
	fmt.Fprintln(w)
	fmt.Fprintln(w, st.accent.Render("$ ")+st.fg.Render(cmd))
	fmt.Fprintln(w)

	sh, err := exec.LookPath("bash")
	if err != nil {
		if sh, err = exec.LookPath("sh"); err != nil {
			fmt.Fprintln(os.Stderr, "menu: no shell found on PATH")
			os.Exit(1)
		}
	}
	if err := syscall.Exec(sh, []string{sh, "-c", cmd}, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "menu: exec:", err)
		os.Exit(1)
	}
}
