package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	tea "charm.land/bubbletea/v2"
)

type surfaceKind uint8

const (
	surfaceNone surfaceKind = iota
	surfaceMenu
	surfaceDocs
)

func (kind surfaceKind) String() string {
	switch kind {
	case surfaceMenu:
		return "menu"
	case surfaceDocs:
		return "docs"

	default:
		return "none"
	}
}

// A pane runs the existing public wrapper in its own PTY. The UI loop owns its
// native terminal and frame; moving the pane changes geometry, not its process.
type pane struct {
	kind          surfaceKind
	process       *shellProcess
	terminal      *terminal
	frame         terminalFrame
	done          bool
	exitCode      int
	err           error
	closed        bool
	selectionFile string // menu-only result, separate from its PTY output
}

type paneOutputMsg struct {
	pane   *pane
	output outputMsg
}
type paneExitMsg struct {
	pane *pane
	err  error
}
type paneFailureMsg struct {
	pane *pane
	err  error
}

func startPane(kind surfaceKind, cols, rows int, env []string) (*pane, error) {
	var command string
	var args []string
	switch kind {
	case surfaceMenu:
		command, args = "x", []string{"--embedded"}
	case surfaceDocs:
		command = "docs"

	default:
		return nil, fmt.Errorf("cannot start pane for surface kind %d", kind)
	}
	path, err := exec.LookPath(command)
	if err != nil {
		return nil, fmt.Errorf("find %s pane command %q on PATH: %w", kind, command, err)
	}
	engine, err := newTerminal(cols, rows)
	if err != nil {
		return nil, fmt.Errorf("libghostty-vt %s pane: %w", kind, err)
	}
	p := &pane{kind: kind, terminal: engine}
	if err := p.snapshot(); err != nil {
		engine.Close()
		return nil, err
	}
	if kind == surfaceMenu {
		result, err := os.CreateTemp("", "prelude-menu-selection-*")
		if err != nil {
			engine.Close()
			return nil, fmt.Errorf("private menu selection file: %w", err)
		}
		p.selectionFile = result.Name()
		if err := result.Close(); err != nil {
			_ = p.close()
			return nil, err
		}
		args = append(args, "--select-output", p.selectionFile)
	}
	p.process, err = startPTY(path, args, cols, rows, env, "")
	if err != nil {
		_ = p.close()
		return nil, fmt.Errorf("start %s pane command %q: %w", kind, command, err)
	}
	return p, nil
}

// This is a blocking output command, not an engine operation. Do not read UI
// state here: close/finish can run while a command is pending. Pointer tags let
// the host discard messages from a pane it has since replaced or closed.
func (p *pane) nextOutput() tea.Msg {
	switch message := p.process.nextOutput().(type) {
	case outputMsg:
		return paneOutputMsg{pane: p, output: message}
	case childExitMsg:
		return paneExitMsg{pane: p, err: message.err}
	case hostFailureMsg:
		return paneFailureMsg{pane: p, err: message.err}
	default:
		return nil
	}
}

func (p *pane) absorb(data []byte) error {
	replies, err := p.terminal.Write(data)
	if err != nil {
		return fmt.Errorf("libghostty-vt %s pane output: %w", p.kind, err)
	}
	if err := p.forward(replies, nil); err != nil {
		return err
	}
	return p.snapshot()
}

func (p *pane) resize(cols, rows int) error {
	if err := p.terminal.Resize(cols, rows); err != nil {
		return fmt.Errorf("libghostty-vt %s pane resize: %w", p.kind, err)
	}
	if !p.done && !p.closed {
		select {
		case <-p.process.exited:
		default:
			// This delivers SIGWINCH without replacing the child or its PTY.
			if err := p.process.resize(cols, rows); err != nil {
				return fmt.Errorf("resize %s pane PTY: %w", p.kind, err)
			}
		}
	}
	// Mode-2048 reports are queued by Resize, even without fresh child output.
	replies, err := p.terminal.Write(nil)
	if err != nil {
		return fmt.Errorf("libghostty-vt %s pane resize replies: %w", p.kind, err)
	}
	if err := p.forward(replies, nil); err != nil {
		return err
	}
	return p.snapshot()
}

func (p *pane) snapshot() error {
	frame, err := p.terminal.Snapshot()
	if err != nil {
		return fmt.Errorf("libghostty-vt %s pane snapshot: %w", p.kind, err)
	}
	p.frame = frame
	return nil
}

func (p *pane) forward(data []byte, err error) error {
	if err != nil {
		return fmt.Errorf("libghostty-vt %s pane input: %w", p.kind, err)
	}
	if p.done || p.closed {
		return nil
	}
	return p.process.send(data)
}

// Exit status is pane state, not a host failure. Keep the terminal alive so the
// final owned frame can still be viewed and reflowed after its child is reaped.
func (p *pane) finish(err error) error {
	if p.done {
		return p.err
	}
	p.done, p.exitCode = true, exitStatus(err)
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		p.err = err
	}
	if closeErr := p.process.close(); closeErr != nil {
		p.err = errors.Join(p.err, closeErr)
	}
	return p.err
}

func (p *pane) close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	var err error
	if p.process != nil {
		err = p.process.close()
	}
	// Teardown, like all native operations, belongs to the UI-loop owner.
	p.terminal.Close()
	if p.selectionFile != "" {
		if removeErr := os.Remove(p.selectionFile); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}
	return err
}
