package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

const panelTimeout = 2 * time.Second

// panelSnapshot is an immutable cell image of a panel command's terminal.
// Keeping cells instead of text preserves colors, attributes, hyperlinks,
// cursor addressing, erases, and all other VT rendering decisions.
type panelSnapshot struct {
	cols  int
	rows  int
	cells []*uv.Cell
}

func (snapshot *panelSnapshot) cellAt(x, y int) *uv.Cell {
	if snapshot == nil || x < 0 || x >= snapshot.cols || y < 0 || y >= snapshot.rows {
		return nil
	}
	return snapshot.cells[y*snapshot.cols+x]
}

func snapshotTerminal(emulator *vt.Emulator, cols, rows int) *panelSnapshot {
	snapshot := &panelSnapshot{
		cols:  cols,
		rows:  rows,
		cells: make([]*uv.Cell, cols*rows),
	}
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			if cell := emulator.CellAt(x, y); cell != nil {
				snapshot.cells[y*cols+x] = cell.Clone()
			}
		}
	}
	return snapshot
}

type panelResultMsg struct {
	generation uint64
	mode       pinMode
	snapshot   *panelSnapshot
	errText    string
}

func loadPanelCmd(request panelRequest, env []string) tea.Cmd {
	return func() tea.Msg {
		snapshot, err := loadPanel(request.mode, request.cols, request.rows, env)
		result := panelResultMsg{
			generation: request.generation,
			mode:       request.mode,
			snapshot:   snapshot,
		}
		if err != nil {
			result.errText = err.Error()
		}
		return result
	}
}

func loadPanel(mode pinMode, cols, rows int, env []string) (*panelSnapshot, error) {
	cols = max(cols, 1)
	rows = max(rows, 1)
	ctx, cancel := context.WithTimeout(context.Background(), panelTimeout)
	defer cancel()

	var (
		snapshot *panelSnapshot
		err      error
	)
	switch mode {
	case pinMotd:
		snapshot, err = capturePanelCommand(ctx, "motd", nil, cols, rows, env)
	default:
		return nil, fmt.Errorf("no static panel for %s", mode.label())
	}
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func capturePanelCommand(ctx context.Context, name string, args []string, cols, rows int, env []string) (*panelSnapshot, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("%s not on PATH", name)
	}

	cols = max(cols, 1)
	rows = max(rows, 1)
	cmd := exec.CommandContext(ctx, path, args...)
	// PRELUDE_MOTD_PURE is a mode switch, not a boolean option. Remove it so
	// the child follows its interactive terminal path. Normalize an empty TERM
	// because several terminal-aware programs treat TERM= as a dumb terminal.
	cmd.Env = filterEnv(env, "PRELUDE_MOTD_PURE")
	cmd.Env = replaceEnv(cmd.Env, "TERM", envValue(cmd.Env, "TERM", "xterm-256color"))
	cmd.Env = replaceEnv(cmd.Env, "COLUMNS", fmt.Sprintf("%d", cols))
	cmd.Env = replaceEnv(cmd.Env, "LINES", fmt.Sprintf("%d", rows))

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = ptmx.Close() }()

	emulator := vt.NewEmulator(cols, rows)
	emulator.SetDefaultForegroundColor(pinForeground)
	emulator.SetDefaultBackgroundColor(pinBackground)

	// Terminal queries emitted by the child (for example OSC 11 background
	// detection) are parsed by the emulator and returned through Read. Without
	// this reverse pump, a PTY exists but no terminal actually answers it.
	replies := startTerminalReplyPump(ptmx, emulator)

	stopCancelWatch := make(chan struct{})
	cancelWatchDone := make(chan struct{})
	go func() {
		defer close(cancelWatchDone)
		select {
		case <-ctx.Done():
			_ = ptmx.Close()
		case <-stopCancelWatch:
		}
	}()

	written, readErr := io.Copy(emulator, ptmx)
	close(stopCancelWatch)
	<-cancelWatchDone
	waitErr := cmd.Wait()
	snapshot := snapshotTerminal(emulator, cols, rows)
	_ = ptmx.Close()
	replies.Stop()
	_ = emulator.Close()

	if ctx.Err() != nil {
		return snapshot, fmt.Errorf("%s: %w", name, ctx.Err())
	}
	if waitErr != nil {
		return snapshot, fmt.Errorf("%s: %w", name, waitErr)
	}
	if readErr != nil && written == 0 && !errors.Is(readErr, os.ErrClosed) {
		return nil, fmt.Errorf("%s output: %w", name, readErr)
	}
	return snapshot, nil
}

func envValue(env []string, key, fallback string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			if value := strings.TrimPrefix(entry, prefix); value != "" {
				return value
			}
			break
		}
	}
	return fallback
}

func filterEnv(env []string, key string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env))
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			out = append(out, entry)
		}
	}
	return out
}
