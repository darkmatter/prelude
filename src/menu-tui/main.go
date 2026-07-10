// prelude-menu: an interactive devshell command menu, ported from the
// cli-menu-design demo's command palette.
//
//	prelude-menu --config cfg.json               interactive picker
//	prelude-menu --config cfg.json list          print the task table
//	prelude-menu --config cfg.json <name|key> …  run a task directly
//
// Tasks with declared args open argument-entry mode (option chips, boolean
// flags, required validation, live preview) unless extra CLI args are given.
// The selected command is exec'd via bash -c (or printed when execute=false).
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

// debugLog is enabled via PRELUDE_MENU_DEBUG=<path> for TUI diagnostics.
var debugLog bool

func main() {
	cfgPath := flag.String("config", os.Getenv("PRELUDE_MENU_CONFIG"), "path to the menu config JSON")
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
	case len(args) > 0 && args[0] == "list":
		printList(cfg, st)

	case len(args) > 0:
		fastPath(cfg, st, args[0], args[1:])

	default:
		runTUI(cfg, st, nil)
	}
}

// fastPath resolves a task by name or key. Extra CLI args skip argument
// entry; tasks with declared args and no extras open the TUI in arg mode.
func fastPath(cfg *Config, st styles, sel string, extra []string) {
	t := cfg.find(sel)
	if t == nil {
		w := colorWriter(os.Stderr, os.Environ(), cfg.ColorProfile)
		fmt.Fprintln(w, st.errText.Render(fmt.Sprintf("menu: unknown task %q", sel)))
		os.Exit(1)
	}
	if len(extra) > 0 {
		finish(cfg, st, t.Run+" "+strings.Join(extra, " "))
		return
	}
	if len(t.Args) > 0 {
		runTUI(cfg, st, t)
		return
	}
	finish(cfg, st, t.Run)
}

func runTUI(cfg *Config, st styles, argTask *Task) {
	m := newModel(cfg, st, argTask)
	options := []tea.ProgramOption{}
	if profile, ok := configuredColorProfile(cfg.ColorProfile); ok {
		options = append(options, tea.WithColorProfile(profile))
	}
	p := tea.NewProgram(m, options...)
	final, err := p.Run()
	if err != nil {
		w := colorWriter(os.Stderr, os.Environ(), cfg.ColorProfile)
		fmt.Fprintln(w, "menu:", err)
		fmt.Fprintln(w, st.dim.Render("hint: `menu list` prints the tasks non-interactively"))
		os.Exit(1)
	}
	if fm, ok := final.(model); ok && fm.execCmd != "" {
		finish(cfg, st, fm.execCmd)
	}
}

func colorWriter(output io.Writer, environ []string, profileName string) *colorprofile.Writer {
	w := colorprofile.NewWriter(output, environ)
	if profile, ok := configuredColorProfile(profileName); ok && w.Profile != colorprofile.NoTTY {
		w.Profile = profile
	}
	return w
}

func configuredColorProfile(name string) (colorprofile.Profile, bool) {
	switch name {
	case "truecolor":
		return colorprofile.TrueColor, true
	case "ansi256":
		return colorprofile.ANSI256, true
	default:
		return colorprofile.Unknown, false
	}
}

// finish either execs the assembled command (replacing this process) or
// prints it, per the execute option.
func finish(cfg *Config, st styles, cmd string) {
	cmd = strings.TrimSpace(cmd)
	if !cfg.Execute {
		fmt.Println(cmd)
		return
	}
	w := colorWriter(os.Stdout, os.Environ(), cfg.ColorProfile)
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
