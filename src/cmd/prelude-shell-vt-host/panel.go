package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// panelTimeout bounds a capture. A pinned surface is chrome: if it cannot be
// produced quickly it is not worth blocking the pin on.
const panelTimeout = 3 * time.Second

// loadPanel produces the surface for one captured pin mode at the requested
// size. Live modes never reach here: they are running children the host owns,
// not commands photographed once.
//
// Commands are captured through a virtual terminal of exactly the pin body's
// geometry rather than by stripping escape sequences from their output. That
// keeps colour, attributes, hyperlinks, cursor addressing, and erases intact,
// and it means a command that repaints itself lands as the picture it meant to
// draw instead of the transcript of how it got there.
func loadPanel(request panelRequest, env []string) (*surface, error) {
	cols := max(request.cols, 1)
	rows := max(request.rows, 1)

	switch request.mode {
	case pinMotd:
		return capturePanel(panelCapture{name: "motd", cols: cols, rows: rows, env: env})

	case pinMenu:
		surface, err := capturePanel(panelCapture{
			name: "x", args: []string{"--list"}, cols: cols, rows: rows, env: env,
		})
		if err == nil {
			return surface, nil
		}
		// Prefer public `x --list`; fall back to the menu binary's
		// `--x --list` path when only that wrapper is on PATH.
		return capturePanel(panelCapture{
			name: "menu", args: []string{"-x", "--list"}, cols: cols, rows: rows, env: env,
		})
	}
	return nil, fmt.Errorf("no panel for pin mode %q", request.mode.label())
}

type panelCapture struct {
	name string
	args []string
	cols int
	rows int
	env  []string
}

// capturePanel runs one command on its own PTY and returns the terminal image
// it painted. A partial image is returned alongside an error whenever the
// command produced output before failing, so a noisy exit status still shows
// the operator something real.
func capturePanel(spec panelCapture) (*surface, error) {
	path, err := exec.LookPath(spec.name)
	if err != nil {
		return nil, fmt.Errorf("%s is not on PATH", spec.name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), panelTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, spec.args...)
	// PRELUDE_MOTD_PURE selects a rendering mode rather than toggling a
	// boolean, so unsetting it is the only way to ask for the interactive
	// path. TERM is normalised because several terminal-aware programs treat
	// an empty TERM as a dumb terminal and give up on colour.
	cmd.Env = withoutEnv(spec.env, "PRELUDE_MOTD_PURE")
	cmd.Env = withEnv(cmd.Env, "TERM", envValue(cmd.Env, "TERM", "xterm-256color"))
	cmd.Env = withEnv(cmd.Env, "COLUMNS", fmt.Sprint(spec.cols))
	cmd.Env = withEnv(cmd.Env, "LINES", fmt.Sprint(spec.rows))

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(spec.rows),
		Cols: uint16(spec.cols),
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", spec.name, err)
	}
	defer func() { _ = ptmx.Close() }()

	emulator := vt.NewEmulator(spec.cols, spec.rows)
	emulator.SetDefaultForegroundColor(pinForeground)
	emulator.SetDefaultBackgroundColor(pinBackground)

	// Terminal queries from the child (cursor position reports, OSC colour
	// probes) are parsed by the emulator and answered through its reader.
	replies := make(chan struct{})
	go func() {
		_, _ = io.Copy(ptmx, emulator)
		close(replies)
	}()

	// Closing the master is what unblocks the read below when the deadline
	// expires; killing the process alone can leave a grandchild holding the
	// slave open.
	watchDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = ptmx.Close()
		case <-watchDone:
		}
	}()

	_, _ = io.Copy(emulator, ptmx)
	close(watchDone)
	waitErr := cmd.Wait()

	image := captureSurface(emulator, spec.cols, spec.rows, pinPaint)

	// The reply pump is parked in Emulator.Read, whose closed-check is not
	// synchronised against Close. Closing the PTY makes its next write fail
	// and a poked reply wakes it, so the join happens before the close. An
	// unjoined pump leaves the emulator open rather than racing it.
	_ = ptmx.Close()
	if retireReplyPump(replies, func() { emulator.SendText("\x00") }) {
		_ = emulator.Close()
	}

	switch {
	case ctx.Err() != nil:
		return image, fmt.Errorf("%s: %w", spec.name, ctx.Err())
	case waitErr != nil:
		return image, fmt.Errorf("%s: %w", spec.name, waitErr)
	}
	return image, nil
}
