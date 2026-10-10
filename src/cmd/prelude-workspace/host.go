package main

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"

	tea "charm.land/bubbletea/v2"
)

// host is the UI-loop owner of native terminal state and its derived snapshot.
// Background workers publish bytes/exit results; Update mutates the engines and
// View composes Go-owned cells without calling back into Ghostty.
type host struct {
	child                 *shellProcess
	terminal              *terminal
	cols                  int
	rows                  int
	frame                 terminalFrame
	parser                promptParser
	prompt                promptState
	completion            *completionState
	completionWanted      bool
	lastCompletionQueryID int
	nextCompletionApplyID int

	pendingCommand          string
	restoreInput            bool
	menuCommandAcknowledged bool
	menuCommandFinished     bool
	shellAcceptPending      bool
	shellCommandFinished    bool

	quoted bool
	closed bool
	code   int
	err    error

	env     []string
	palette chromePalette

	surface         surfaceKind
	pane            *pane
	paneVisible     bool
	placement       placement
	focusPane       bool
	leader          bool
	paneError       error
	notice          string
	terminalFocused bool
}

// The child cannot address the footer: its PTY and emulator are both one row
// shorter than the outer frame. At one row, View omits the footer instead.
func shellRows(rows int) int { return max(rows-1, 1) }

func newHost(cols, rows int, env []string, starship bool) (*host, error) {
	cols, rows = max(1, min(cols, 65535)), max(1, min(rows, 65535))
	palette, err := loadChromePalette(env)
	if err != nil {
		return nil, err
	}
	engine, err := newTerminal(cols, shellRows(rows))
	if err != nil {
		return nil, fmt.Errorf("libghostty-vt: %w", err)
	}
	frame, err := engine.Snapshot()
	if err != nil {
		engine.Close()
		return nil, fmt.Errorf("libghostty-vt initial snapshot: %w", err)
	}
	child, err := startBash(cols, shellRows(rows), env, starship)
	if err != nil {
		engine.Close()
		return nil, err
	}

	return &host{
		child: child, terminal: engine, cols: cols, rows: rows, frame: frame,
		env: append([]string(nil), env...), palette: palette, terminalFocused: true,
	}, nil
}

func (h *host) close() error {
	if h.closed {
		return nil
	}
	h.closed = true
	var err error
	if h.pane != nil {
		err = h.pane.close()
	}
	if h.child != nil {
		err = errors.Join(err, h.child.close())
	}
	// Initialization and teardown bracket the UI loop; no reader, waiter or
	// tea.Cmd ever calls an engine, including Close.
	h.terminal.Close()
	return err
}

func (h *host) Init() tea.Cmd {
	if h.child == nil {
		return nil
	}
	return h.child.nextOutput
}

func (h *host) fail(err error) tea.Cmd {
	h.err = err
	return tea.Quit // run prints the error after restoring the outer terminal
}

func (h *host) forward(data []byte, err error) tea.Cmd {
	if err != nil {
		return h.fail(fmt.Errorf("libghostty-vt input: %w", err))
	}
	if h.child != nil {

		if err := h.child.send(data); err != nil {
			return h.fail(err)
		}
	}
	return nil
}

func (h *host) snapshot() error {
	frame, err := h.terminal.Snapshot()
	if err != nil {
		return fmt.Errorf("libghostty-vt snapshot: %w", err)
	}
	h.frame = frame
	h.prompt.observe(frame)
	if h.completionVisible() && h.layout().Completion.Empty() {
		if h.prompt.anchor < 0 {
			h.completion.phase = completionRedraw // Readline callbacks can temporarily clear PS1.
		} else {
			h.dismissCompletion() // Real geometry changes cannot leave an invisible choice active.
		}
	}
	h.completionRedrawReady()
	return nil
}

// Feed through each OSC133 marker before interpreting it. Snapshots at A
// (prompt starts), B (input starts), or C (input submitted) must describe that
// point in the stream, not later output from the same PTY read. The parser only
// observes; no bytes are removed from the stream sent to Ghostty.
func (h *host) absorb(data []byte) error {
	from := 0
	completionReply, completionApplyReply := 0, 0

	write := func(to int) error {
		if from == to {
			return nil
		}
		h.prompt.promptOutput(data[from:to])
		replies, err := h.terminal.Write(data[from:to])
		from = to
		if err != nil {
			return fmt.Errorf("libghostty-vt output: %w", err)
		}
		return h.child.send(replies)
	}
	for _, marker := range h.parser.scan(data) {
		if err := write(marker.end); err != nil {
			return err
		}
		frame := h.frame
		if marker.kind == 'A' || marker.kind == 'B' || marker.kind == 'C' {
			var err error
			frame, err = h.terminal.Snapshot()
			if err != nil {
				return fmt.Errorf("libghostty-vt prompt snapshot: %w", err)
			}
		}
		h.prompt.mark(marker, frame)

		h.menuPromptMarker(marker)
		if marker.kind == 'C' && h.completion != nil {

			h.completion = nil // no readline reply can belong to a foreground job
		}
		if marker.kind == 'Q' {
			completionReply = max(completionReply, marker.sequence)
		} else if marker.kind == 'R' {
			completionApplyReply = max(completionApplyReply, marker.sequence)
		}
	}
	if err := write(len(data)); err != nil {
		return err
	}
	if err := h.snapshot(); err != nil {
		return err
	}
	// A later C or continuation A in this same batch can invalidate an earlier
	// B. Only the final stream state may dispatch or restore readline input.

	if completionReply > 0 {
		if err := h.completionReply(completionReply); err != nil {
			return err
		}
	}
	if completionApplyReply > 0 {
		if err := h.completionApplyReply(completionApplyReply); err != nil {
			return err
		}
	}

	return h.menuPromptReady()
}

// A layout change reflows both engines and delivers SIGWINCH to the existing
// processes. A hidden pane keeps its last geometry/frame, never a zero-sized PTY.
func (h *host) resizeWorkspace() error {
	layout := h.layout()
	cols, rows := max(1, layout.Shell.Dx()), max(1, layout.Shell.Dy())
	if h.terminal != nil && (h.frame.Cols != cols || h.frame.Rows != rows) {
		if err := h.terminal.Resize(cols, rows); err != nil {
			return fmt.Errorf("libghostty-vt resize: %w", err)
		}
		if h.child != nil {
			if err := h.child.resize(cols, rows); err != nil {
				return fmt.Errorf("resize Bash PTY: %w", err)
			}
		}
		// Resize can queue native mode-2048 replies even without output.
		replies, err := h.terminal.Write(nil)
		if err != nil {
			return fmt.Errorf("libghostty-vt resize replies: %w", err)
		}
		if cmd := h.forward(replies, nil); cmd != nil {
			return h.err
		}
		if err := h.snapshot(); err != nil {
			return err
		}
	}
	if layout.Body.Empty() {
		h.focusPane = false
		return nil
	}
	if p := h.pane; p != nil && !p.closed && p.terminal != nil &&
		(p.frame.Cols != layout.Body.Dx() || p.frame.Rows != layout.Body.Dy()) {
		if err := p.resize(layout.Body.Dx(), layout.Body.Dy()); err != nil {
			h.failPane(err)
		}
	}
	return h.err
}

func (h *host) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	if h.err != nil || h.closed {
		return h, nil
	}
	switch message := message.(type) {
	case outputMsg:
		if len(message.data) > 0 {
			if err := h.absorb(message.data); err != nil {
				return h, h.fail(err)
			}
		}
		if message.err != nil {
			if !errors.Is(message.err, io.EOF) && !errors.Is(message.err, syscall.EIO) {
				return h, h.fail(fmt.Errorf("read Bash PTY: %w", message.err))
			}
			return h, h.child.waitExit
		}
		return h, h.child.nextOutput
	case childExitMsg:
		h.code = exitStatus(message.err)
		h.prompt.phase, h.prompt.ready = "exited", false

		var exitErr *exec.ExitError
		if message.err != nil && !errors.As(message.err, &exitErr) {
			return h, h.fail(fmt.Errorf("wait Bash: %w", message.err))
		}
		return h, tea.Quit
	case hostFailureMsg:
		return h, h.fail(message.err)
	case paneOutputMsg:
		if !h.currentPane(message.pane) {
			return h, nil
		}
		p := message.pane
		if len(message.output.data) > 0 {
			if err := p.absorb(message.output.data); err != nil {
				return h, h.failPane(err)
			}
		}
		if err := message.output.err; err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EIO) {
				return h, h.failPane(fmt.Errorf("read %s PTY: %w", p.kind, err))
			}
			// Reuse the process's bounded exit wait, tagging even failures with
			// this pane. Commands only read process events, never native state.
			process := p.process
			return h, func() tea.Msg {
				switch msg := process.waitExit().(type) {
				case childExitMsg:
					return paneExitMsg{pane: p, err: msg.err}
				case hostFailureMsg:
					return paneFailureMsg{pane: p, err: msg.err}
				}
				return nil
			}
		}
		return h, p.nextOutput
	case paneExitMsg:
		if !h.currentPane(message.pane) {
			return h, nil
		}
		p := message.pane
		if err := p.finish(message.err); err != nil {
			return h, h.failPane(err)
		}
		if p.kind == surfaceMenu && p.exitCode == 0 {
			command, err := p.selectedCommand()
			if err != nil {
				return h, h.failPane(err)
			}
			cmd := h.closePane()
			if h.err == nil {
				h.queueMenuCommand(command)
			}
			return h, cmd
		}
		h.focusPane = false
		return h, h.syncFocus()
	case paneFailureMsg:
		if h.currentPane(message.pane) {
			return h, h.failPane(message.err)
		}
	case tea.WindowSizeMsg:
		cols, rows := max(1, min(message.Width, 65535)), max(1, min(message.Height, 65535))
		if cols == h.cols && rows == h.rows {
			return h, nil
		}
		h.cols, h.rows = cols, rows
		if h.completionVisible() && h.layout().Completion.Empty() {
			h.dismissCompletion()
		}
		wasPane := h.focusPane
		if err := h.resizeWorkspace(); err != nil {
			return h, h.fail(err)
		}
		if wasPane != h.focusPane {
			return h, h.syncFocus()
		}
	case tea.KeyPressMsg:
		return h, h.keyInput(message)
	case tea.PasteMsg:
		if err := h.closeCompletion(); err != nil {
			return h, h.fail(err)
		}
		h.leader, h.quoted, h.notice = false, false, ""
		if h.paneFocused() {
			return h, h.forwardPane(h.pane.terminal.Paste(message.Content))
		}
		data, err := h.terminal.Paste(message.Content)
		if err == nil {
			h.noteShellInput(data, false)
		}
		return h, h.forward(data, err)
	case tea.FocusMsg:
		h.terminalFocused = true
		return h, h.syncFocus()
	case tea.BlurMsg:
		if err := h.closeCompletion(); err != nil {
			return h, h.fail(err)
		}
		h.terminalFocused, h.leader, h.quoted = false, false, false
		return h, h.syncFocus()
	case tea.MouseMsg:
		return h, h.mouseInput(message)
	}
	return h, nil
}

func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		return exitErr.ExitCode()
	}
	return 1
}
