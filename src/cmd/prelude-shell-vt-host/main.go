// Command prelude-shell-vt-host keeps a real interactive shell in a short pane
// while Prelude owns pinned rows above it and a status row below.
//
// The child shell talks to an in-process virtual terminal, never to the outer
// terminal. Nothing rewrites its escape sequences into outer coordinates:
// CUP, ED, DECSTBM, saved cursors, and the alternate screen all resolve inside
// a private screen, and only the resulting cells are composed into the frame.
// That is what makes the pinned rows structurally safe rather than defended.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

type options struct {
	shellRows  int
	command    string
	docs       string
	scrollback int
	frameRate  int
}

func main() {
	opts := options{}
	flag.IntVar(&opts.shellRows, "shell-rows", 10, "shell rows while a pinned surface is active (try 10 or 1)")
	flag.StringVar(&opts.command, "shell", "", "shell to run (defaults to $SHELL)")
	flag.StringVar(&opts.docs, "docs", "docs", "program a live docs pin runs in the pinned band")
	flag.IntVar(&opts.scrollback, "scrollback", 5000, "lines of child scrollback to retain")
	flag.IntVar(&opts.frameRate, "fps", 60, "maximum frames composed per second while the child is chatty")
	flag.Parse()

	code, err := run(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, "prelude-shell-vt-host:", err)
		os.Exit(1)
	}
	os.Exit(code)
}

// run returns the child shell's exit status. A shell that exits non-zero is
// not a host failure, so it is reported as a status rather than an error.
func run(opts options) (int, error) {
	if opts.shellRows < 1 {
		return 1, fmt.Errorf("-shell-rows must be at least 1")
	}
	if opts.frameRate < 1 {
		return 1, fmt.Errorf("-fps must be at least 1")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return 1, fmt.Errorf("needs a real TTY on stdin and stdout")
	}

	// Start the child at the size it will actually be given. Booting it at a
	// guessed 80x24 and correcting on the first resize makes every shell
	// redraw its prompt before the user has typed anything.
	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		cols, rows = 80, 24
	}
	initial := newState(cols, rows, opts.shellRows)

	child, err := startShell(shellSpec{
		command:    opts.command,
		cols:       initial.layout.cols,
		rows:       initial.layout.shell.height,
		scrollback: opts.scrollback,
		env:        os.Environ(),
	})
	if err != nil {
		return 1, err
	}
	defer child.stop()

	model := newHost(child, initial, os.Environ(), opts.docs)
	program := tea.NewProgram(model)

	frames := newThrottle(time.Second/time.Duration(opts.frameRate), func() {
		program.Send(frameMsg{})
	})
	defer frames.stop()

	// A live pinned pane drives frames and reports its exit the same way the
	// shell does, so it needs the same handles.
	model.attach(frames.request, func(message tea.Msg) { program.Send(message) })
	defer model.closePane()

	child.pump(frames.request, func(exitErr error) {
		frames.stop()
		program.Send(childGoneMsg{err: exitErr})
	})

	if _, err := program.Run(); err != nil {
		return 1, fmt.Errorf("host terminal: %w", err)
	}
	// A virtual terminal write failure is a host bug; a child exit status is
	// news for the caller, not a failure of this program.
	var broken *hostError
	if errors.As(model.fatal, &broken) {
		return 1, broken
	}
	return child.exitCode(), nil
}
