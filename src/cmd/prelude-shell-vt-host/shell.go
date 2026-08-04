package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// shellCloseGrace is how long a hung child gets between SIGHUP and SIGKILL.
const shellCloseGrace = 300 * time.Millisecond

// replyPumpGrace bounds how long shutdown waits for an emulator reply pump to
// notice that its PTY is gone.
const replyPumpGrace = 300 * time.Millisecond

// readChunk is the PTY read size. The buffer is reused for the life of the
// pump, so a chatty child costs no allocations on the read path.
const readChunk = 32 * 1024

type cursorState struct {
	visible  bool
	style    vt.CursorStyle
	blink    bool
	position uv.Position
}

// shellView is an immutable snapshot of everything the renderer needs to know
// about the child, taken under one lock so a frame cannot mix states.
type shellView struct {
	title         string
	altScreen     bool
	cursor        cursorState
	mouseTracking bool
	focused       bool
	history       int
	cols          int
	rows          int
}

type shellSpec struct {
	command    string
	args       []string
	cols       int
	rows       int
	scrollback int
	env        []string
}

// shell owns the child process, its PTY, and the virtual terminal that absorbs
// everything the child writes. The child's escape sequences never reach the
// outer terminal; only cells leave this type.
type shell struct {
	cmd  *exec.Cmd
	ptmx *os.File

	// mu guards the emulator and every field mirrored out of it. The emulator
	// invokes its callbacks from inside Write and the Send* helpers, all of
	// which are only reached while mu is held, so the callbacks below mutate
	// these fields directly and must never take the lock themselves.
	mu         sync.Mutex
	emulator   *vt.Emulator
	cols       int
	rows       int
	cursor     cursorState
	title      string
	altScreen  bool
	focused    bool
	mouseModes [len(mouseTrackingModes)]bool

	exited   chan struct{}
	exitErr  error
	stopOnce sync.Once

	// repliesDone closes when the reverse pump goroutine has returned, so
	// shutdown can prove nothing is inside Emulator.Read before closing it.
	repliesDone chan struct{}
}

func startShell(spec shellSpec) (*shell, error) {
	cols := max(spec.cols, 1)
	rows := max(spec.rows, 1)

	command := spec.command
	if command == "" {
		command = envValue(spec.env, "SHELL", "/bin/sh")
	}

	cmd := exec.Command(command, spec.args...)
	cmd.Env = withEnv(spec.env, "PRELUDE_SHELL_VT_HOST", "1")
	cmd.Env = withEnv(cmd.Env, "TERM", envValue(cmd.Env, "TERM", "xterm-256color"))

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, fmt.Errorf("start %s on a PTY: %w", command, err)
	}

	result := &shell{
		cmd:         cmd,
		ptmx:        ptmx,
		emulator:    vt.NewEmulator(cols, rows),
		cols:        cols,
		rows:        rows,
		cursor:      cursorState{visible: true, style: vt.CursorBlock, blink: true},
		title:       command,
		exited:      make(chan struct{}),
		repliesDone: make(chan struct{}),
	}
	if spec.scrollback > 0 {
		result.emulator.SetScrollbackSize(spec.scrollback)
	}
	result.emulator.SetDefaultBackgroundColor(shellBackground)
	result.emulator.SetCallbacks(vt.Callbacks{
		Title: func(title string) {
			if title != "" {
				result.title = title
			}
		},
		AltScreen:        func(active bool) { result.altScreen = active },
		CursorVisibility: func(visible bool) { result.cursor.visible = visible },
		CursorStyle: func(style vt.CursorStyle, blink bool) {
			result.cursor.style = style
			result.cursor.blink = blink
		},
		EnableMode:  func(mode ansi.Mode) { result.setMouseMode(mode, true) },
		DisableMode: func(mode ansi.Mode) { result.setMouseMode(mode, false) },
	})

	// The emulator answers the child's terminal queries (cursor position
	// reports, OSC colour probes, mouse and key encodings). Without this
	// reverse pump the child would hold a PTY that nothing ever replies on.
	//
	// Emulator.Read blocks until a reply exists, so this goroutine is normally
	// parked inside it. stop unparks it by closing the PTY and then poking a
	// reply through, which turns the next write into an error and ends the
	// copy. Joining on repliesDone before Emulator.Close is what keeps the
	// close off the unsynchronized fast path inside Read.
	go func() {
		defer close(result.repliesDone)
		_, _ = io.Copy(ptmx, result.emulator)
	}()

	go func() {
		result.exitErr = cmd.Wait()
		close(result.exited)
	}()

	return result, nil
}

// mouseTrackingModes are the DEC private modes that mean "the child wants the
// mouse". The host mirrors them instead of grabbing the mouse unconditionally,
// so native selection keeps working whenever the child is not asking for it.
var mouseTrackingModes = [...]ansi.DECMode{
	ansi.ModeMouseX10,
	ansi.ModeMouseNormal,
	ansi.ModeMouseHighlight,
	ansi.ModeMouseButtonEvent,
	ansi.ModeMouseAnyEvent,
}

// setMouseMode runs as an emulator callback, i.e. already under mu.
func (s *shell) setMouseMode(mode ansi.Mode, enabled bool) {
	dec, ok := mode.(ansi.DECMode)
	if !ok {
		return
	}
	for index, tracked := range mouseTrackingModes {
		if tracked == dec {
			s.mouseModes[index] = enabled
			return
		}
	}
}

// hostError marks a failure in the host itself rather than in the child. It is
// the difference between "your shell exited 1", which is news for the caller,
// and "the host broke", which is a bug.
type hostError struct{ err error }

func (e *hostError) Error() string { return e.err.Error() }

func (e *hostError) Unwrap() error { return e.err }

// pump drains the PTY into the virtual terminal for the life of the child.
// onOutput is called after each absorbed chunk — the caller throttles it — and
// onExit exactly once, with the child's wait error or a [hostError].
func (s *shell) pump(onOutput func(), onExit func(error)) {
	go func() {
		buffer := make([]byte, readChunk)
		for {
			count, readErr := s.ptmx.Read(buffer)
			if count > 0 {
				s.mu.Lock()
				_, writeErr := s.emulator.Write(buffer[:count])
				s.mu.Unlock()
				if writeErr != nil {
					onExit(&hostError{fmt.Errorf("virtual terminal: %w", writeErr)})
					return
				}
				onOutput()
			}
			if readErr != nil {
				// EIO on the master side is the normal way a PTY reports that
				// the child closed the slave; the child's own status decides
				// whether anything actually went wrong.
				onExit(s.wait())
				return
			}
		}
	}()
}

// resize applies a new geometry to both the virtual screen and the PTY. It
// reports whether anything actually changed so callers can avoid handing the
// child a SIGWINCH storm while a window edge is being dragged.
func (s *shell) resize(cols, rows int) (bool, error) {
	cols = max(cols, 1)
	rows = max(rows, 1)

	s.mu.Lock()
	if cols == s.cols && rows == s.rows {
		s.mu.Unlock()
		return false, nil
	}
	s.emulator.Resize(cols, rows)
	s.cols, s.rows = cols, rows
	s.mu.Unlock()

	if err := pty.Setsize(s.ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}); err != nil {
		return true, fmt.Errorf("resize shell PTY: %w", err)
	}
	return true, nil
}

func (s *shell) sendKey(key uv.KeyEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emulator.SendKey(key)
}

func (s *shell) sendText(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emulator.SendText(text)
}

func (s *shell) paste(text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emulator.Paste(text)
}

func (s *shell) sendMouse(event vt.Mouse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emulator.SendMouse(event)
}

// setFocus tells the child whether it is the active program. The flag is
// mirrored onto the shell so callers can ask who believes it has focus without
// inferring it from the host's own bookkeeping.
func (s *shell) setFocus(focused bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.focused = focused
	if focused {
		s.emulator.Focus()
		return
	}
	s.emulator.Blur()
}

// view snapshots the render-relevant child state.
func (s *shell) view() shellView {
	s.mu.Lock()
	defer s.mu.Unlock()
	cursor := s.cursor
	cursor.position = s.emulator.CursorPosition()
	tracking := false
	for _, enabled := range s.mouseModes {
		tracking = tracking || enabled
	}
	return shellView{
		title:         s.title,
		altScreen:     s.altScreen,
		cursor:        cursor,
		mouseTracking: tracking,
		focused:       s.focused,
		history:       s.emulator.ScrollbackLen(),
		cols:          s.cols,
		rows:          s.rows,
	}
}

// draw copies the child's viewport into dst. scroll is how many lines above
// the live screen to show; zero follows the child. Rows are stitched from
// scrollback first and the live grid second, which is exactly how a real
// terminal viewport spans the boundary.
//
// band decides both the backdrop for columns the child does not cover and
// whether cells that left their colours unset adopt it. A child rendered as
// the shell keeps them unset, so it looks the way it would in any terminal; a
// child rendered as a pinned pane is chrome and must be opaque.
//
// The whole band is written under one lock, so a frame never mixes cells from
// before and after a child write.
func (s *shell) draw(dst *uv.ScreenBuffer, area band, scroll int, style paint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	history := s.emulator.ScrollbackLen()
	first := history - clamp(scroll, 0, history)

	for y := range area.height {
		row := first + y
		for x := 0; x < dst.Width(); {
			var source *uv.Cell
			if x < s.cols {
				if row < history {
					source = s.emulator.ScrollbackCellAt(x, row)
				} else {
					source = s.emulator.CellAt(x, row-history)
				}
			}
			cell := style.fill
			if source != nil {
				cell = style.apply(*source)
			}
			x += setComposedCell(dst, x, area.top+y, &cell)
		}
	}
}

// text returns the visible screen as plain text. Test-facing: it is the
// cheapest way to assert what the child actually painted.
func (s *shell) text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.emulator.String()
}

func (s *shell) wait() error {
	<-s.exited
	return s.exitErr
}

// exitCode reports the child's status once it has exited. A child killed by a
// signal reports the conventional 128+signal.
func (s *shell) exitCode() int {
	select {
	case <-s.exited:
	default:
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(s.exitErr, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		return exitErr.ExitCode()
	}
	if s.exitErr != nil {
		return 1
	}
	return 0
}

// stop tears the child down: close the PTY, retire the reverse pump, close the
// virtual terminal, then SIGHUP and finally SIGKILL if the child will not go.
func (s *shell) stop() {
	s.stopOnce.Do(func() {
		// Closing the PTY first means the reverse pump's next write fails,
		// which is what ends its copy loop. The poke needs no lock: SendText
		// only writes to the emulator's reply pipe, which is safe against a
		// concurrent Close and touches no other emulator state.
		_ = s.ptmx.Close()
		joined := retireReplyPump(s.repliesDone, func() { s.emulator.SendText("\x00") })

		if joined {
			// Nothing is inside Emulator.Read, so closing cannot race its
			// closed-flag fast path. This also releases a poke still parked
			// in the reply pipe.
			s.mu.Lock()
			_ = s.emulator.Close()
			s.mu.Unlock()
		}

		select {
		case <-s.exited:
			return
		default:
		}
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Signal(syscall.SIGHUP)
		}
		select {
		case <-s.exited:
		case <-time.After(shellCloseGrace):
			if s.cmd.Process != nil {
				_ = s.cmd.Process.Kill()
			}
			<-s.exited
		}
	})
}

// retireReplyPump unparks a goroutine copying an emulator's reply stream to a
// PTY and reports whether it returned.
//
// Emulator.Read parks until a reply exists, and its closed-flag fast path is
// not synchronised against Emulator.Close, so the pump must be joined before
// the emulator is closed. Callers close the PTY first; the poke below then
// wakes the pump, whose write to the dead PTY fails and ends the copy.
//
// The poke runs on its own goroutine on purpose. An emulator's reply channel
// is an unbuffered pipe, so once the pump has returned — which it may have
// done before this call, on an earlier failed write — nothing is reading and
// the poke blocks indefinitely. Doing that inline would hang shutdown instead
// of bounding it. A caller that sees true closes the emulator, which fails the
// pending pipe write and releases the goroutine.
//
// False means the pump never came back. The emulator must then be left open:
// leaking a goroutine is strictly better than writing the closed flag out from
// under a live reader.
func retireReplyPump(done <-chan struct{}, poke func()) bool {
	go poke()

	select {
	case <-done:
		return true
	case <-time.After(replyPumpGrace):
		return false
	}
}
