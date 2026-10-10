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
	h.dismissCompletion()
	if h.pendingCommand != "" {
		h.notice = "A command is already waiting for Bash; MOTD not queued"
		return nil
	}
	h.paneVisible, h.focusPane, h.notice = false, false, ""
	if err := h.resizeWorkspace(); err != nil {
		return h.fail(err)
	}
	if cmd := h.syncFocus(); cmd != nil {
		return cmd
	}
	h.queueMenuCommand("command motd\n")
	return nil
}

func (h *host) queueMenuCommand(command string) {
	if command == "" {
		return // cancellation does not touch readline or the shell's last status
	}
	if h.pendingCommand != "" {
		h.notice = "A menu command is already waiting for Bash; selection not run"
		return
	}
	h.pendingCommand = command
	h.dispatchMenuCommand()
}

// Input can leave readline before its output markers are consumed. Quoted
// control characters and bracketed-paste newlines do not accept the line.
func (h *host) noteShellInput(data []byte, quoted bool) {
	if !quoted && !strings.HasPrefix(string(data), "\x1b[200~") &&
		strings.ContainsAny(string(data), "\r\n\x0f\x03") {
		h.shellAcceptPending, h.shellCommandFinished = true, false
	}
}

// A single waiting selection is enough for this spike. Never send the private
// readline key while a foreground program or secondary PS2 prompt owns input.
func (h *host) dispatchMenuCommand() {
	if h.pendingCommand == "" || !h.prompt.ready || h.shellAcceptPending || h.restoreInput || h.child == nil {
		return
	}
	command := h.pendingCommand
	h.pendingCommand = ""
	if err := os.WriteFile(h.child.commandFile, []byte(command), 0o600); err != nil {
		h.notice = "Menu command not run: " + err.Error()
		return
	}
	if err := h.child.send([]byte(menuSubmitKey)); err != nil {
		h.notice = "Menu command not run: " + err.Error()
		return
	}
	h.restoreInput, h.menuCommandAcknowledged, h.menuCommandFinished = true, false, false
	h.shellAcceptPending, h.shellCommandFinished = true, false
}

func (h *host) menuPromptMarker(marker promptMarker) {
	switch marker.kind {
	case 'L':
		if h.restoreInput {
			h.menuCommandAcknowledged = true
		}
	case 'D':
		if h.shellAcceptPending {
			h.shellCommandFinished = true
		}
		if h.restoreInput && h.menuCommandAcknowledged {
			h.menuCommandFinished = true
		}
	}
}

func (h *host) menuPromptReady() error {
	if !h.prompt.ready || (h.shellAcceptPending && !h.shellCommandFinished) {
		return nil
	}
	if h.restoreInput {
		if !h.menuCommandAcknowledged {
			// PTY delivery alone does not prove that readline saved a buffer.
			h.notice = "Menu handoff was not acknowledged; parked input not replaced"
		} else if !h.menuCommandFinished {
			return nil
		} else if err := h.child.send([]byte(menuRestoreKey)); err != nil {
			return err
		}
		h.restoreInput, h.menuCommandAcknowledged, h.menuCommandFinished = false, false, false
	}
	h.shellAcceptPending, h.shellCommandFinished = false, false
	h.dispatchMenuCommand()
	return nil
}
