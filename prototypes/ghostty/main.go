// Ghostty spike: an isolated interactive Bash, a protected footer, and a real
// borderless Prelude docs/menu window that can be shown or hidden. --noprofile
// --rcfile skips user shell rc files; INPUTRC and HISTFILE are also isolated.
// Devshell tools and exported functions are inherited. The engine is VT-only.
//
// Output: children -> PTYs -> Ghostty -> Go-owned cells -> workspace frame.
// Input: outer terminal -> Bubble Tea -> focused Ghostty encoder -> child PTY.
// Child escape sequences never reach the outer terminal directly: Ghostty
// interprets them inside the child's screen, and the host draws the result.
//
// Reading order: run, host.Update, chords.go/render.go, prompt.go, terminal.go.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

func main() {
	starship := flag.Bool("starship", false, "use real Starship in private Bash (Bash >=4.4; fixed spike prompt is the default)")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Ghostty real-shell spike (no user rc/inputrc/history file).")
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(), "Alt+m prints MOTD in the main shell; Alt+x toggles the menu; Alt+d toggles docs. Starship keeps its bracketed keymap with one Alt + label.")
		fmt.Fprintln(flag.CommandLine.Output(), "Ctrl+] next / Ctrl+[ previous (legacy fallback Ctrl+\\); floating -> left -> right -> top -> bottom -> floating.")
		fmt.Fprintln(flag.CommandLine.Output(), "Ctrl+P unlocks one command: m MOTD | x menu | d docs | t window | Tab focus | v layout | c close. The footer highlights UNLOCKED until the action; Esc or Ctrl+P again locks.")
		fmt.Fprintln(flag.CommandLine.Output(), "Ctrl+P then t shows/hides the current window without losing state (opens menu if none). Ctrl+G belongs to the child, not Prelude. Ctrl+V quotes reserved keys; exit: quit the main shell.")
		fmt.Fprintln(flag.CommandLine.Output(), "Ctrl+P then x/d toggles menu/docs without restarting a live pane; c closes it. Menu selection closes the picker and runs in the main Bash, preserving parked readline input.")
		fmt.Fprintln(flag.CommandLine.Output(), "With normal Bash prompt markers, a busy/secondary-prompt selection waits for the next primary prompt; see the spike README for the OSC133 trust limitation.")
		fmt.Fprintln(flag.CommandLine.Output(), "Normal motd prints once before the first prompt; Alt+m (or unlocked m) reprints it in the main shell, preserving parked input and waiting if Bash is busy or at PS2. MOTD never uses a pane.")
		fmt.Fprintln(flag.CommandLine.Output(), "The Nix launcher supplies completion-only Bash init without ble.sh. x<Tab> opens a chooser below the prompt; Tab/Shift+Tab cycle, Enter inserts without running, Esc dismisses. Single matches insert directly.")
		fmt.Fprintln(flag.CommandLine.Output(), "File/argument completion stays in Readline; a later Enter runs the completed command in the main shell. Key hints match the command menu: bold accent keycaps, muted labels, and right-aligned status; only keycaps have a background.")
		fmt.Fprintln(flag.CommandLine.Output(), "Generated Starship retains its original right-side styling: Alt + [m] motd [x] menu [d] docs. The separate footer shows LOCKED / UNLOCKED; custom prompt configs and normal shell shortcuts are preserved.")
		fmt.Fprintln(flag.CommandLine.Output(), "Completion is a rendering-only overlay: no Bash PTY resize; the displayed viewport shifts only when needed to preserve the full prompt above the chooser.")
		fmt.Fprintln(flag.CommandLine.Output(), "Menu panes use x --embedded: transparent outer canvas, with themed menu panel backgrounds and highlights retained.")
	}
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "ghostty spike: no positional arguments supported")
		os.Exit(1)
	}
	code, err := run(*starship)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ghostty spike:", err)
		code = 1
	}
	os.Exit(code)
}

// run owns both lifecycles: Bubble Tea restores the outer terminal, then the
// deferred host cleanup closes all child PTYs and frees their native engines.
func run(starship bool) (code int, err error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return 1, fmt.Errorf("needs a real TTY on stdin and stdout")
	}
	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 1, fmt.Errorf("outer terminal size: %w", err)
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)

	h, err := newHost(cols, rows, os.Environ(), starship)
	if err != nil {
		return 1, err
	}
	// Keep cleanup in run: main's os.Exit does not execute deferred functions.
	defer func() {
		if closeErr := h.close(); closeErr != nil {
			err = errors.Join(err, closeErr)
			code = 1
		}
	}()
	program := tea.NewProgram(h, tea.WithFPS(60), tea.WithoutSignalHandler(),
		tea.WithFilter(func(_ tea.Model, msg tea.Msg) tea.Msg {
			if sig, ok := msg.(syscall.Signal); ok {
				h.code = 128 + int(sig) // The filter runs on the UI loop, before QuitMsg.
				return tea.Quit()
			}
			return msg
		}))
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case sig := <-signals:
			program.Send(sig)
		case <-done:
		}
	}()
	if _, err := program.Run(); err != nil {
		return 1, fmt.Errorf("host terminal: %w", err)
	}
	if h.err != nil {
		return 1, h.err
	}
	return h.code, nil
}
