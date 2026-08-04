// PROTOTYPE — in-process virtual-terminal shell host with status-only chrome.
//
// Question: can a real interactive shell plus a useful two-row status surface
// stand on its own, without a pinned Docs panel?
package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

func main() {
	scrollback := flag.Int("scrollback", 5_000, "child scrollback lines to retain")
	flag.Parse()

	if err := run(*scrollback); err != nil {
		fmt.Fprintln(os.Stderr, "prelude-shell-vt-status-spike:", err)
		os.Exit(1)
	}
}

func run(scrollback int) error {
	if scrollback < 1 {
		return fmt.Errorf("-scrollback must be >= 1")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("needs a real TTY (stdin+stdout)")
	}

	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		cols, rows = 80, 24
	}
	state := initialHostState(cols, rows, scrollback)
	catalog, err := loadMenuCatalog(os.Getenv("PRELUDE_MENU_CONFIG"))
	if err != nil {
		return err
	}
	session, err := startShell(state.layout.cols, state.layout.shellRows, scrollback, catalog)
	if err != nil {
		return err
	}
	defer session.Close()

	model := newHostModel(session, state, catalog)
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
