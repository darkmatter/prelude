// PROTOTYPE — in-process virtual-terminal shell host.
//
// Question: can a child shell remain fully interactive in a short pane while
// Prelude owns pinned MOTD/Docs rows and a status row, without remapping
// the child's escape sequences into outer-terminal coordinates?
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

func main() {
	shellRows := flag.Int("shell-rows", 10, "shell rows while a pinned surface is active (try 10 or 1)")
	flag.Parse()

	if err := run(*shellRows); err != nil {
		fmt.Fprintln(os.Stderr, "prelude-shell-vt-spike:", err)
		os.Exit(1)
	}
}

func run(shellRows int) error {
	if shellRows < 1 {
		return fmt.Errorf("-shell-rows must be >= 1")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("needs a real TTY (stdin+stdout)")
	}

	state := initialHostState(shellRows)
	session, err := startShell(state.layout.cols, state.layout.shellRows)
	if err != nil {
		return err
	}
	defer session.Close()

	model := newHostModel(session, shellRows, os.Environ())
	defer model.Close()
	program := tea.NewProgram(model)
	final, err := program.Run()
	if err != nil {
		return fmt.Errorf("host terminal: %w", err)
	}
	if finalModel, ok := final.(*hostModel); ok && finalModel.exitErr != nil {
		return fmt.Errorf("child shell: %w", finalModel.exitErr)
	}
	return nil
}
