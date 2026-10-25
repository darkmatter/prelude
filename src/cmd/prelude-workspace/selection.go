package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const (
	maxMenuCommand = 1 << 20
	menuSubmitKey  = "\x1b[992~"
	menuRestoreKey = "\x1b[993~"
)

// Readline owns the exact parked line, point, and mark. Load shell source from a
// private file instead of pasting arbitrary command bytes into terminal input.
// Accept-line keeps Bash's normal job control, OSC133, and Starship timing.
const bashSelectionRC = `__prelude_ghostty_load_command() {
    local status=$?
    __prelude_ghostty_saved_line=$READLINE_LINE
    __prelude_ghostty_saved_point=$READLINE_POINT
    __prelude_ghostty_saved_mark=$READLINE_MARK
    builtin printf -v READLINE_LINE 'builtin source %q' "$__prelude_ghostty_command_file"
    READLINE_POINT=${#READLINE_LINE}
    READLINE_MARK=0
    # L is a private acknowledgement, not a standard OSC133 lifecycle marker.
    builtin printf '\e]133;L\a'
    return "$status"
}
__prelude_ghostty_restore_input() {
    local status=$?
    READLINE_LINE=$__prelude_ghostty_saved_line
    READLINE_POINT=$__prelude_ghostty_saved_point
    READLINE_MARK=$__prelude_ghostty_saved_mark
    builtin unset __prelude_ghostty_saved_line __prelude_ghostty_saved_point __prelude_ghostty_saved_mark
    return "$status"
}
builtin bind -x '"\e[991~":__prelude_ghostty_load_command'
builtin bind '"\e[992~": "\e[991~\C-m"'
builtin bind -x '"\e[993~":__prelude_ghostty_restore_input'
`

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

// Only a successful picker exit publishes a result. Terminal output is never
// interpreted as a command; the CLI owns cwd/PATH quoting in this shell source.
func (p *pane) selectedCommand() (string, error) {
	f, err := os.Open(p.selectionFile)
	if err != nil {
		return "", fmt.Errorf("read menu selection: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("menu selection is not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxMenuCommand+1))
	if err != nil {
		return "", fmt.Errorf("read menu selection: %w", err)
	}
	if len(data) > maxMenuCommand || strings.IndexByte(string(data), 0) >= 0 {
		return "", fmt.Errorf("menu selection exceeds 1 MiB or contains NUL")
	}
	return string(data), nil
}

// MOTD remains main-shell output. Reuse the private Readline handoff so a
// shortcut cannot overwrite parked input or leak a command into a foreground job.
func (h *host) showMotd() tea.Cmd {

	h.paneVisible, h.focusPane, h.notice = false, false, ""
	if err := h.resizeWorkspace(); err != nil {
		return h.fail(err)
	}
	if cmd := h.syncFocus(); cmd != nil {
		return cmd
	}
	h.dispatchMenuCommand("command motd\n")
	return nil
}

// Dispatch now or drop: there is no waiting selection. Only a ready primary
// prompt (OSC133 B, never PS2 or a foreground job) may receive the private
// readline key; otherwise the footer says so and nothing runs later.
func (h *host) dispatchMenuCommand(command string) {
	if command == "" {
		return // cancellation does not touch readline or the shell's last status
	}
	if !h.prompt.ready || h.restoreInput || h.child == nil {
		h.notice = "Shell busy; command not run"
		return
	}
	if err := os.WriteFile(h.child.commandFile, []byte(command), 0o600); err != nil {
		h.notice = "Menu command not run: " + err.Error()
		return
	}
	if err := h.child.send([]byte(menuSubmitKey)); err != nil {
		h.notice = "Menu command not run: " + err.Error()
		return
	}
	h.restoreInput, h.menuCommandAcknowledged, h.menuCommandFinished = true, false, false
}

func (h *host) menuPromptMarker(marker promptMarker) {
	switch marker.kind {
	case 'L':
		if h.restoreInput {
			h.menuCommandAcknowledged = true
		}
	case 'D':
		if h.restoreInput {
			h.menuCommandFinished = true
		}
	}
}

func (h *host) menuPromptReady() error {
	if !h.prompt.ready || !h.restoreInput {
		return nil
	}
	// A prompt redraw can repeat B before the load acknowledgement (L) and
	// the dispatched command's D arrive; only D proves it has finished.
	if !h.menuCommandFinished {
		return nil
	}
	if !h.menuCommandAcknowledged {
		// PTY delivery alone does not prove that readline saved a buffer.
		h.notice = "Menu handoff was not acknowledged; parked input not replaced"
	} else if err := h.child.send([]byte(menuRestoreKey)); err != nil {
		return err
	}
	h.restoreInput, h.menuCommandAcknowledged, h.menuCommandFinished = false, false, false
	return nil
}
