package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	completionApplyKey = "\x1b[995~"

	completionLimit = 1 << 20
)

type completionCandidate struct {
	name, description, line string
	point, mark             int
}

type completionSnapshot struct {
	id          int
	line        string
	point, mark int
	candidates  []completionCandidate
}

type completionPhase uint8

const (
	completionQuery completionPhase = iota
	completionRedraw
	completionPopup
	completionApply
)

// This is a snapshot/choice, never a second editable buffer. Only Bash reads or
// edits READLINE_LINE. One in-flight query/apply keeps the private file ordered;
// edits invalidate a delayed query rather than completing an obsolete prefix.
type completionState struct {
	phase     completionPhase
	snapshot  completionSnapshot
	selected  int
	cancelled bool
	refresh   bool

	fallback []byte
	queued   [][]byte
	applyID  int
}

func (h *host) completionSafe() bool {
	// A/B can split during redraw. Bash's own primary-prompt guard decides
	// eligibility; the host only forwards the real Tab, never a query escape.
	return h.child != nil && h.prompt.phase == "prompt" &&
		!h.shellAcceptPending && !h.restoreInput && h.pendingCommand == "" &&
		!h.paneFocused() && h.terminalFocused && !h.frame.AltScreen
}

func (h *host) completionVisible() bool {
	return h.completion != nil && h.completion.phase == completionPopup && !h.completion.cancelled
}

// Completion changes only the displayed viewport, never native/PTY geometry.
// Crop earlier output when necessary, but keep the entire visible prompt and
// editable tail. If those leave no safe space, suppress the chooser.
func (h *host) completionLayout(layout workspaceLayout) workspaceLayout {
	if !h.completionVisible() || layout.Shell.Empty() || h.frame.Cols < 1 ||
		h.prompt.anchor < 0 || h.prompt.anchor >= len(h.frame.Cells) {
		return layout
	}
	endRow := min(max(0, h.frame.CursorY), layout.Shell.Dy()-1)
	startRow := 0
	if h.prompt.anchor >= 0 && h.prompt.anchor < len(h.frame.Cells) {
		anchorRow := h.prompt.anchor / h.frame.Cols
		promptRows := h.frame.Rows
		if h.prompt.promptRowsCols == h.frame.Cols {
			promptRows = max(1, h.prompt.promptRows)
		}
		startRow = max(0, anchorRow-promptRows+1)
		// A mid-line caret can have wrapped input below it.
		for i := h.prompt.anchor; i < len(h.frame.Cells); i++ {
			cell := h.frame.Cells[i]
			if cell.Width > 0 && cell.Content != "" && cell.Content != " " {
				endRow = max(endRow, i/h.frame.Cols)
			}
		}
	}
	endRow = min(endRow, layout.Shell.Dy()-1)
	below := max(0, layout.Shell.Dy()-endRow-1)
	height := min(len(h.completion.snapshot.candidates), 8, below+startRow)
	for ; height >= 1; height-- {
		scroll := max(0, height-below)
		promptBand := uv.Rect(layout.Shell.Min.X, layout.Shell.Min.Y+startRow-scroll, layout.Shell.Dx(), endRow-startRow+1)
		if scroll > 0 && promptBand.Overlaps(layout.Panel) {
			continue // Never move a visible prompt underneath a floating pane.
		}
		layout.ShellScroll = scroll
		layout.Completion = uv.Rect(layout.Shell.Min.X, layout.Shell.Min.Y+endRow+1-scroll, layout.Shell.Dx(), height)
		break
	}
	return layout
}

func (h *host) completionArea(layout workspaceLayout) uv.Rectangle {
	return layout.Completion
}

// Keep a cancelled request until its acknowledgement arrives. Clearing it early
// would let an old response masquerade as a subsequent Tab's fresh snapshot.
func (h *host) dismissCompletion() {
	h.completionWanted = false
	if h.completion != nil {
		if h.completion.phase == completionPopup || h.completion.phase == completionRedraw {
			h.completion = nil
		} else {
			h.completion.cancelled, h.completion.refresh = true, false
			h.completion.queued = nil
		}
	}
}

func (h *host) closeCompletion() error {
	h.dismissCompletion()
	return nil
}

func (h *host) requestCompletion(fallback []byte) error {
	h.completionWanted = true
	h.completion = &completionState{phase: completionQuery, fallback: bytes.Clone(fallback)}
	// Readline expands its Tab macro internally. A foreground program can
	// receive only the user's actual Tab, never a host query escape sequence.
	return h.child.send(fallback)
}

func (h *host) completionKey(message tea.KeyPressMsg) (bool, tea.Cmd) {
	if !h.completionSafe() {
		if err := h.closeCompletion(); err != nil {
			return true, h.fail(err)
		}
		return false, nil
	}
	key := message.String()
	c := h.completion
	if c != nil && c.phase == completionPopup && h.layout().Completion.Empty() {
		h.dismissCompletion()
		return false, nil
	}
	if c == nil {
		if key != "tab" {
			h.completionWanted = false
			return false, nil
		}
		fallback, err := h.terminal.Key(message.Key())
		if err == nil {
			err = h.requestCompletion(fallback)
		}
		if err != nil {
			return true, h.fail(err)
		}
		return true, nil
	}
	if key == "esc" {
		if err := h.closeCompletion(); err != nil {
			return true, h.fail(err)
		}
		return true, nil
	}
	if c.cancelled {
		if key == "tab" {
			c.refresh, h.completionWanted = true, true
			c.queued = append(c.queued, []byte{'\t'})
			return true, nil
		}
		return false, nil
	}
	switch key {
	case "tab", "shift+tab":
		if c.phase != completionPopup {
			data, err := h.terminal.Key(message.Key())
			if err != nil {
				return true, h.fail(err)
			}
			c.queued = append(c.queued, bytes.Clone(data))
		}
		delta := 1
		if key == "shift+tab" {
			delta = -1
		}
		if c.phase == completionApply {
			c.refresh, h.completionWanted = true, true
		} else {
			c.selected += delta
			if c.phase == completionPopup {
				c.selected = completionIndex(c.selected, len(c.snapshot.candidates))
			}
		}
		return true, nil
	case "enter", "ctrl+m", "ctrl+j":
		if c.phase == completionQuery || c.phase == completionRedraw {
			h.dismissCompletion()
			return false, nil // no visible chooser yet: ordinary shell Enter
		} else if c.phase == completionPopup {
			if err := h.acceptCompletion(); err != nil {
				return true, h.fail(err)
			}
		}
		// Enter during apply also waits; no completion response may execute.
		return true, nil
	default:
		if err := h.closeCompletion(); err != nil {
			return true, h.fail(err)
		}
		return false, nil
	}
}

func completionIndex(index, count int) int { return (index%count + count) % count }

func (h *host) acceptCompletion() error {
	c := h.completion
	choice := c.snapshot.candidates[completionIndex(c.selected, len(c.snapshot.candidates))]
	h.nextCompletionApplyID++
	c.applyID = h.nextCompletionApplyID
	h.completionWanted = false
	fields := []string{"apply", strconv.Itoa(c.applyID), strconv.Itoa(c.snapshot.id), c.snapshot.line, strconv.Itoa(c.snapshot.point), strconv.Itoa(c.snapshot.mark), choice.line, strconv.Itoa(choice.point), strconv.Itoa(choice.mark)}
	if err := os.WriteFile(h.child.completionApplyFile, []byte(strings.Join(fields, "\x00")+"\x00"), 0o600); err != nil {
		h.completion = nil
		h.notice = "Completion not inserted: " + err.Error()
		return nil
	}
	c.phase = completionApply
	return h.child.send([]byte(completionApplyKey))
}

// Called after the complete output batch, not at an intermediate prompt marker.
func (h *host) completionReply(sequence int) error {
	if h.child == nil {
		return nil
	}
	fields, err := readCompletionFields(h.child.completionFile)
	if err != nil {
		h.completion = nil
		h.notice = "Completion unavailable: " + err.Error()
		return nil
	}
	id, err := completionID(fields)
	if err != nil || id < sequence || id <= h.lastCompletionQueryID {
		return nil
	}
	h.lastCompletionQueryID = id
	c := h.completion
	if c != nil && c.phase == completionApply {
		return nil // Query generations can never acknowledge an apply operation.
	}
	if c == nil {
		if !h.completionWanted || !h.completionSafe() {
			return nil
		}
		c = &completionState{phase: completionQuery, fallback: []byte{'\t'}}
		h.completion = c
	}
	refresh := c.refresh
	if c.cancelled || !h.completionSafe() {
		h.completion = nil
		if refresh && h.completionSafe() {
			if err := h.requestCompletion(c.fallback); err != nil {
				return err
			}
			if len(c.queued) > 1 {
				h.completion.queued = c.queued[1:]
				h.completion.selected = len(c.queued) - 1
			}
		}
		return nil
	}

	switch fields[0] {
	case "fallback", "applied":

		// Readline's internal macro already performed native completion, or
		// inserted its single x match. Replay only the EXTRA gestures consumed
		// while waiting; the first Tab has already happened in Readline.
		h.completion = nil
		// These are server snapshots, not acknowledgements of a client query.
		// An old fallback must not disarm a newer Tab already in Readline's queue.
		h.completionWanted = h.completionWanted || len(c.queued) > 0

		return h.child.send(bytes.Join(c.queued, nil))
	case "candidates":
		snapshot, err := decodeCompletionSnapshot(fields)
		if err != nil {
			h.completion = nil
			h.notice = "Completion unavailable: " + err.Error()
			return nil
		}
		if len(snapshot.candidates) == 0 {
			h.completion = nil
			return nil
		}
		c.snapshot, c.phase = snapshot, completionRedraw
		c.selected = completionIndex(c.selected, len(snapshot.candidates))
		if len(snapshot.candidates) == 1 {
			return h.acceptCompletion()
		}
		c.queued = nil // pending gestures are now represented by selected
		h.completionRedrawReady()
		return nil
	default:
		h.completion = nil
		h.notice = "Completion unavailable"
		if len(fields) > 1 {
			h.notice += ": " + displayText(fields[len(fields)-1])
		}
		return nil
	}
}

// bind -x can clear the prompt and acknowledge Q before Readline redraws B.
// Keep the acknowledged choice pending until that redraw supplies safe geometry;
// Enter still belongs to Bash while no chooser is visible.
func (h *host) completionRedrawReady() {
	c := h.completion
	if c == nil || c.phase != completionRedraw {
		return
	}
	if !h.completionSafe() {
		h.completion = nil
		return
	}
	if h.prompt.anchor < 0 {
		return
	}
	c.phase = completionPopup
	c.selected = completionIndex(c.selected, len(c.snapshot.candidates))
	c.queued = nil
	if h.layout().Completion.Empty() {
		h.completion = nil
		h.notice = "Resize to show command completions"
	}
}

func (h *host) completionApplyReply(sequence int) error {
	c := h.completion
	if c == nil || c.phase != completionApply || c.applyID != sequence {
		return nil
	}
	fields, err := readCompletionFields(h.child.completionApplyFile)
	if err != nil {
		return err
	}
	id, err := completionID(fields)
	if err != nil || id != c.applyID {
		return nil
	}
	h.completion = nil
	if len(fields) != 2 || fields[0] != "applied" {
		h.notice = "Completion expired; press Tab again"
	}
	if c.refresh && h.completionSafe() {
		h.completionWanted = true
		if len(c.queued) == 0 {
			return h.requestCompletion([]byte{'\t'})
		}
		return h.child.send(bytes.Join(c.queued, nil))
	}
	return nil
}

func completionID(fields []string) (int, error) {
	if len(fields) < 2 {
		return 0, fmt.Errorf("missing completion generation")
	}
	id, err := strconv.Atoi(fields[1])
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid completion generation")
	}
	return id, nil
}

func readCompletionFields(path string) ([]string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("completion response is not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, completionLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > completionLimit || data[len(data)-1] != 0 || !utf8.Valid(data) || bytes.Count(data, []byte{0}) > 5125 {
		return nil, fmt.Errorf("invalid or oversized response")
	}
	return strings.Split(string(data[:len(data)-1]), "\x00"), nil
}

func completionOffset(value, line string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 || n > len(line) || !utf8.ValidString(line[:n]) {
		return 0, fmt.Errorf("invalid readline offset")
	}
	return n, nil
}

func decodeCompletionSnapshot(fields []string) (completionSnapshot, error) {
	var snapshot completionSnapshot
	if len(fields) < 5 || (len(fields)-5)%5 != 0 || (len(fields)-5)/5 > 1024 {
		return snapshot, fmt.Errorf("invalid candidate framing")
	}
	snapshot.line = fields[2]
	var err error
	if snapshot.id, err = completionID(fields); err != nil {
		return snapshot, err
	}
	if snapshot.point, err = completionOffset(fields[3], snapshot.line); err != nil {
		return snapshot, err
	}
	if snapshot.mark, err = completionOffset(fields[4], snapshot.line); err != nil {
		return snapshot, err
	}
	for i := 5; i < len(fields); i += 5 {
		choice := completionCandidate{name: fields[i], description: fields[i+1], line: fields[i+2]}
		if choice.name == "" {
			return snapshot, fmt.Errorf("empty completion key")
		}
		if choice.point, err = completionOffset(fields[i+3], choice.line); err != nil {
			return snapshot, err
		}
		if choice.mark, err = completionOffset(fields[i+4], choice.line); err != nil {
			return snapshot, err
		}
		snapshot.candidates = append(snapshot.candidates, choice)
	}
	return snapshot, nil
}
