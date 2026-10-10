package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

const (
	ghosttySmokeCols  = 90
	ghosttySmokeRows  = 24
	smokeWait         = 10 * time.Second
	smokeJoin         = 2 * time.Second
	smokeBaseFooter   = "LOCKED    Ctrl+P  unlock"
	smokeLeaderFooter = "UNLOCKED    m  motd   x  menu   d  docs   t  window   Tab  focus   v  layout   c  close   Esc  lock   Ctrl+P  lock"
)

type smokeCell struct {
	text  string
	width int
}

type smokeFooterStyle struct {
	fg, bg color.Color
	attrs  uint8
}

type smokeKeyHint struct{ key, label string }

var smokeNavigationKeys = []smokeKeyHint{{"LOCKED", ""}, {"Ctrl+P", "unlock"}}
var smokeLeaderKeys = []smokeKeyHint{{"UNLOCKED", ""}, {"m", "motd"}, {"x", "menu"}, {"d", "docs"}, {"t", "window"}, {"Tab", "focus"}, {"v", "layout"}, {"c", "close"}, {"Esc", "lock"}, {"Ctrl+P", "lock"}}
var smokeCompletionKeys = []smokeKeyHint{{"Tab", "next"}, {"Shift+Tab", "prev"}, {"Enter", "accept"}, {"Esc", "dismiss"}}
var smokePaneKeys = smokeNavigationKeys
var smokeHiddenKeys = smokeNavigationKeys

type smokeFooterSpan struct {
	x, width int
	role     string
}

type smokeKeymapPalette struct {
	key, label, bg, success, status color.Color
}

type smokeFrame struct {
	cols         int
	rows         []string
	cells        []smokeCell
	footerStyles []smokeFooterStyle
	alt          bool
}

func (f smokeFrame) textAt(x, y, width int) string {
	var text strings.Builder
	for column := x; column < x+width; column++ {
		text.WriteString(f.cells[y*f.cols+column].text)
	}
	return text.String()
}

func (f smokeFrame) footer() string { return strings.TrimSpace(f.rows[len(f.rows)-1]) }

// Assert the public two-column layout: whole padded hint pairs on the left,
// status two cells from the right edge, and a two-cell gap between them.
func smokeFooterLayout(cols int, hints []smokeKeyHint, status string) (string, []smokeFooterSpan) {
	status = ansi.Truncate(status, max(0, cols-4), "")
	limit := cols - 2
	if status != "" {
		limit -= ansi.StringWidth(status) + 2
	}
	text, x := "  ", 2
	var spans []smokeFooterSpan
	for i, hint := range hints {
		gap := 0
		if i > 0 {
			gap = 2
		}
		key, label := " "+hint.key+" ", ""
		if hint.label != "" {
			label = " " + hint.label
		}
		keyWidth, labelWidth := ansi.StringWidth(key), ansi.StringWidth(label)
		if x+gap+keyWidth+labelWidth > limit {
			break
		}
		text += strings.Repeat(" ", gap) + key + label
		x += gap
		role := "key"
		if hint.key == "LOCKED" {
			role = "locked"
		} else if hint.key == "UNLOCKED" {
			role = "unlocked"
		}
		spans = append(spans, smokeFooterSpan{x, keyWidth, role})
		if labelWidth > 0 {
			spans = append(spans, smokeFooterSpan{x + keyWidth, labelWidth, "label"})
		}
		x += keyWidth + labelWidth
	}
	if status != "" {
		statusX := cols - 2 - ansi.StringWidth(status)
		text += strings.Repeat(" ", statusX-x) + status
		spans = append(spans, smokeFooterSpan{statusX, ansi.StringWidth(status), "status"})
		x = cols - 2
	}
	return text + strings.Repeat(" ", cols-x), spans
}

func (f smokeFrame) hasFooter(hints []smokeKeyHint, status string) bool {
	want, _ := smokeFooterLayout(f.cols, hints, status)
	return f.textAt(0, len(f.rows)-1, f.cols) == want
}

func (f smokeFrame) hasPaneFooter(status string) bool {
	hints := smokePaneKeys
	if strings.Contains(status, " hidden | ") {
		hints = smokeHiddenKeys
	}
	return f.hasFooter(hints, status)
}

func smokeUnknownChordStatus(pane string) string {
	return "Unknown Ctrl+P chord \"?\";    " + smokeLeaderFooter + " | " + pane
}

// Use the newest rendered prompt, never a submitted command in older history.
// Wrapped rows are read from terminal cells so spaces at a wrap survive.
func (f smokeFrame) promptRow(prompt string) int {
	for y := len(f.rows) - 2; y >= 0; y-- {
		row := strings.TrimLeft(f.rows[y], " ")
		if row == prompt || strings.HasPrefix(row, prompt+" ") {
			return y
		}
	}
	return -1
}

func (f smokeFrame) hasInput(prompt, input string) bool {
	return f.hasInputIn(image.Rect(0, 0, f.cols, len(f.rows)-1), prompt, input)
}

// Restrict reads and wrapping to Bash's displayed area, excluding adjacent pane
// text. The latest prompt in that area must contain the complete editable line.
func (f smokeFrame) hasInputIn(area image.Rectangle, prompt, input string) bool {
	if area.Empty() || !area.In(image.Rect(0, 0, f.cols, len(f.rows)-1)) {
		return false
	}
	y := -1
	for row := area.Max.Y - 1; row >= area.Min.Y; row-- {
		text := strings.Trim(f.textAt(area.Min.X, row, area.Dx()), " ")
		if text == prompt || strings.HasPrefix(text, prompt+" ") {
			y = row
			break
		}
	}
	want := strings.TrimRight(prompt+" "+input, " ")
	height := max(1, (ansi.StringWidth(want)+area.Dx()-1)/area.Dx())
	if y < 0 || y+height > area.Max.Y {
		return false
	}
	var visible strings.Builder
	for row := y; row < y+height; row++ {
		visible.WriteString(f.textAt(area.Min.X, row, area.Dx()))
	}
	return strings.Trim(visible.String(), " ") == want
}

// Compare the editable text after a prompt marker, independent of preceding
// decoration. Preserve input bytes and wrap at the shell area's own boundaries.
func (f smokeFrame) hasInputAfter(area image.Rectangle, marker, input string) bool {
	if area.Empty() || !area.In(image.Rect(0, 0, f.cols, len(f.rows)-1)) {
		return false
	}
	x, y := -1, -1
	for row := area.Max.Y - 1; row >= area.Min.Y; row-- {
		for column := area.Min.X; column < area.Max.X; column++ {
			cell := f.cells[row*f.cols+column]
			if cell.text == marker && cell.width > 0 {
				x, y = column+cell.width, row
				break
			}
		}
		if y >= 0 {
			break
		}
	}
	if y < 0 || x >= area.Max.X || f.textAt(x, y, 1) != " " {
		return false
	}
	x++ // PS1's separator is not part of the editable input.
	remaining := ansi.StringWidth(input)
	var visible strings.Builder
	for remaining > 0 {
		if y >= area.Max.Y {
			return false
		}
		width := min(remaining, area.Max.X-x)
		visible.WriteString(f.textAt(x, y, width))
		x, remaining = x+width, remaining-width
		if remaining > 0 {
			x, y = area.Min.X, y+1
		}
	}
	if visible.String() != input {
		return false
	}
	for _, cell := range f.cells[y*f.cols+x : y*f.cols+area.Max.X] {
		if cell.text != "" && cell.text != " " {
			return false // Reject extra suffix text, not just a matching prefix.
		}
	}
	return true
}

func smokeColor(c color.Color) color.Color {
	if c == nil {
		return nil
	}
	return color.RGBAModel.Convert(c)
}

func assertSmokeFooterTheme(t *testing.T, f smokeFrame, hints []smokeKeyHint, status string, want ...smokeKeymapPalette) {
	t.Helper()
	if !f.hasFooter(hints, status) {
		t.Fatalf("footer keymap/status layout mismatch: %q", f.footer())
	}
	_, spans := smokeFooterLayout(f.cols, hints, status)
	palette := smokeKeymapPalette{}
	if len(want) > 0 {
		palette = want[0]
	} else {
		seen := map[string]bool{}
		for _, span := range spans {
			if seen[span.role] {
				continue
			}
			seen[span.role] = true
			style := f.footerStyles[span.x]
			switch span.role {
			case "key":
				palette.key, palette.bg = style.fg, style.bg
			case "label":
				palette.label = style.fg
			case "locked":
				palette.label, palette.bg = style.fg, style.bg
			case "unlocked":
				palette.bg, palette.success = style.fg, style.bg
			case "status":
				palette.status = style.fg
			}
		}
	}
	for x, style := range f.footerStyles {
		role := ""
		for _, span := range spans {
			if x >= span.x && x < span.x+span.width {
				role = span.role
				break
			}
		}
		var fg, bg color.Color
		var attrs uint8
		switch role {
		case "key":
			fg, bg, attrs = palette.key, palette.bg, uv.AttrBold
		case "label":
			fg = palette.label
		case "locked":
			fg, bg, attrs = palette.label, palette.bg, uv.AttrBold
		case "unlocked":
			fg, bg, attrs = palette.bg, palette.success, uv.AttrBold
		case "status":
			fg, attrs = palette.status, uv.AttrBold
		}
		if smokeColor(style.bg) != smokeColor(bg) || style.attrs != attrs || smokeColor(style.fg) != smokeColor(fg) {
			t.Fatalf("footer column %d (%s): fg=%v bg=%v attrs=%d, want fg=%v bg=%v attrs=%d", x, role, smokeColor(style.fg), smokeColor(style.bg), style.attrs, smokeColor(fg), smokeColor(bg), attrs)
		}
	}
}

func (f smokeFrame) starshipStatus(input, status string, timed bool) bool {
	y := f.promptRow("界❯")
	if y < 2 || !f.hasInput("界❯", input) || f.rows[y-1] != "│" ||
		!strings.HasPrefix(f.rows[y-2], "STARSHIP-REAL 界é🙂 ") || !strings.Contains(" "+f.rows[y-2]+" ", " "+status+" ") {
		return false
	}
	return !timed || strings.Contains(f.rows[y-2], "time:") && !strings.Contains(f.rows[y-2], "time:0ms")
}

func awaitSmokeStatus(s *smokeOuterPTY, prompt, label string, want int) {
	s.t.Helper()
	// Query Bash itself; the host footer no longer exposes command exit status.
	s.send(fmt.Sprintf("printf 'STATUS:%s:%%s\\n' \"$?\"\r", label))
	s.await("shell exit status for "+label, func(f smokeFrame) bool {
		return f.hasLine(fmt.Sprintf("STATUS:%s:%d", label, want)) && f.hasInput(prompt, "") && f.footer() == smokeBaseFooter
	})
}

func (f smokeFrame) hasLine(want string) bool {
	for _, row := range f.rows[:len(f.rows)-1] {
		if row == want {
			return true
		}
	}
	return false
}

func (f smokeFrame) panel(surface string) bool {
	width := min(100, f.cols-f.cols/5)
	contentRows := len(f.rows) - 1
	height := min(30, contentRows-contentRows/5)
	x, y := (f.cols-width)/2, (contentRows-height)/2
	return strings.HasPrefix(f.textAt(x, y, width), "LEADER_"+strings.ToUpper(surface)+":")
}

// This emulator sees only the real binary's outer PTY output. It is deliberately
// independent of libghostty-vt and never inspects host or prompt state.
type smokeOuterPTY struct {
	t           *testing.T
	cmd         *exec.Cmd
	ptmx        *os.File
	emulator    *vt.Emulator
	replyPipe   io.Closer
	initialTTY  *term.State
	mu          sync.Mutex
	writeMu     sync.Mutex
	traceMu     sync.Mutex
	output      []byte
	replies     []byte
	faults      []string
	stopping    chan struct{}
	readerDone  chan struct{}
	repliesDone chan struct{}
	processDone chan struct{}
	waitErr     error // published by closing processDone; Wait has exactly one owner
}

// Short fixture commands keep readline input on one row. The read checkpoints
// hold a real foreground command in each screen state until the test advances
// it; screen markers cannot be mistaken for echoed command text. Wide glyphs
// occupy the same columns on both screens: x/vt's DCH implementation erases
// wide continuations when shifting them, so changed state markers are ASCII.
const smokePaintFunction = `BASH_FUNC_smoke_paint%%=() { builtin printf '\e[2J\e[H\e[3;1HVISIBLE:界é🙂END\e[4;1HCOMBINING:éEND\e[12;40HUNDERLAY-CONTENT\e[18;1H'; }`

const smokeScreensFunction = `BASH_FUNC_smoke_screens%%=() {
    stty -echo
    builtin printf '\e[2J\e[HPRIMARY-RESTORED\e[3;1HVISIBLE:界é🙂END\e[12;40HPRIMARY-UNDERLAY\e[18;1H'
    builtin printf '\e[?1049h\e[2J\e[HALT-READY\e[3;1HVISIBLE:界é🙂END\e[12;40HALT-UNDERLAY\e[999;1HCHILD-FOOTER-ATTACK\e[20;1H'
    IFS= builtin read -r gate
    builtin printf '\e[2J\e[HALT-CLEARED\e[3;1HVISIBLE:界é🙂END\e[12;40HALT-UNDERLAY\e[999;1HCHILD-CLEAR-ATTACK\e[20;1H'
    IFS= builtin read -r gate
    builtin printf '\e[5;1HSIZE:'
    stty size
    IFS= builtin read -r gate
    builtin printf '\e[?1049l'
    stty echo
}`

func smokeEnvironment(home string) []string {
	var env []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "BASH_FUNC_") || strings.HasPrefix(name, "LC_") {
			continue
		}
		switch name {
		case "HOME", "TERM", "LANG", "LANGUAGE", "NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE",
			"SHELLOPTS", "BASHOPTS", "HISTCONTROL", "HISTIGNORE", "HISTSIZE",
			"BASH_ENV", "ENV", "SSH_CLIENT", "SSH_CONNECTION", "XDG_CACHE_HOME", "PRELUDE_MOTD_PURE":
			continue
		}
		env = append(env, entry)
	}
	locale := "C.UTF-8"
	if runtime.GOOS == "darwin" {
		locale = "en_US.UTF-8"
	}
	return append(env, "HOME="+home, "XDG_CACHE_HOME="+filepath.Join(home, "cache"), "TERM=xterm-256color", "LANG="+locale, "LC_ALL="+locale,
		"HISTCONTROL=", "HISTIGNORE=", "HISTSIZE=100", smokePaintFunction, smokeScreensFunction)
}

func startSmokeOuterPTY(t *testing.T, binary string, args ...string) *smokeOuterPTY {
	t.Helper()
	return startSmokeOuterPTYEnv(t, binary, smokeEnvironment(t.TempDir()), args...)
}

func startSmokeOuterPTYEnv(t *testing.T, binary string, env []string, args ...string) *smokeOuterPTY {
	t.Helper()
	return startSmokeOuterPTYSize(t, binary, env, ghosttySmokeCols, ghosttySmokeRows, args...)
}

func startSmokeOuterPTYSize(t *testing.T, binary string, env []string, cols, rows int, args ...string) *smokeOuterPTY {
	t.Helper()
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal("open outer PTY:", err)
	}
	defer master.Close()
	defer slave.Close()
	// Capture cooked mode before the binary can change it, not after Start.
	initial, err := term.GetState(int(slave.Fd()))
	if err != nil {
		t.Fatal("initial outer TTY state:", err)
	}
	if err := pty.Setsize(master, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)}); err != nil {
		t.Fatal("initial outer PTY size:", err)
	}
	// A pollable master gives both writes a deadline and reads a cancellable
	// Close. Do not call Fd on this new os.File (it disables Go's poller).
	fd, err := unix.Dup(int(master.Fd()))
	if err != nil {
		t.Fatal("duplicate outer PTY:", err)
	}
	if err := unix.SetNonblock(fd, true); err != nil {
		_ = unix.Close(fd)
		t.Fatal("pollable outer PTY:", err)
	}
	ptmx := os.NewFile(uintptr(fd), master.Name())
	cmd := exec.Command(binary, args...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		_ = ptmx.Close()
		t.Fatal("start binary on outer PTY:", err)
	}
	emulator := vt.NewEmulator(cols, rows)
	// x/vt answers DA/DSR/color queries, but does not implement Kitty keyboard
	// negotiation. Report legacy flags instead of leaving Bubble Tea waiting.
	emulator.RegisterCsiHandler(ansi.Command('?', 0, 'u'), func(ansi.Params) bool {
		_, _ = io.WriteString(emulator.InputPipe(), "\x1b[?0u")
		return true
	})
	s := &smokeOuterPTY{
		t: t, cmd: cmd, ptmx: ptmx, emulator: emulator,
		replyPipe: emulator.InputPipe().(io.Closer), initialTTY: initial,
		stopping: make(chan struct{}), readerDone: make(chan struct{}),
		repliesDone: make(chan struct{}), processDone: make(chan struct{}),
	}
	go func() {
		s.waitErr = cmd.Wait()
		close(s.processDone)
	}()
	go s.reply()
	go s.read()
	t.Cleanup(s.close)
	return s
}

func (s *smokeOuterPTY) trace(data []byte, reply bool) {
	s.traceMu.Lock()
	defer s.traceMu.Unlock()
	target := &s.output
	if reply {
		target = &s.replies
	}
	*target = append(*target, data...)
	if len(*target) > 16*1024 {
		*target = append([]byte(nil), (*target)[len(*target)-16*1024:]...)
	}
}

func (s *smokeOuterPTY) fault(err error) {
	select {
	case <-s.stopping:
		return
	case <-s.processDone:
		return
	default:
	}
	s.traceMu.Lock()
	s.faults = append(s.faults, err.Error())
	s.traceMu.Unlock()
}

func (s *smokeOuterPTY) write(data []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.ptmx.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
		return err
	}
	n, err := s.ptmx.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return err
}

func (s *smokeOuterPTY) reply() {
	defer close(s.repliesDone)
	// If a reply fails, unblock any parser Write waiting on the reply pipe too.
	defer s.replyPipe.Close()
	buffer := make([]byte, 4096)
	for {
		n, err := s.emulator.Read(buffer)
		if n > 0 {
			s.trace(buffer[:n], true)
			if err := s.write(buffer[:n]); err != nil {
				s.fault(fmt.Errorf("capability reply: %w", err))
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
				s.fault(fmt.Errorf("read capability reply: %w", err))
			}
			return
		}
	}
}

func (s *smokeOuterPTY) read() {
	defer close(s.readerDone)
	buffer := make([]byte, 32*1024)
	for {
		n, err := s.ptmx.Read(buffer)
		if n > 0 {
			s.trace(buffer[:n], false)
			s.mu.Lock()
			_, parseErr := s.emulator.Write(buffer[:n])
			s.mu.Unlock()
			if parseErr != nil {
				s.fault(fmt.Errorf("parse outer output: %w", parseErr))
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EIO) && !errors.Is(err, os.ErrClosed) {
				s.fault(fmt.Errorf("read outer PTY: %w", err))
			}
			return
		}
	}
}

func (s *smokeOuterPTY) frame() smokeFrame {
	s.mu.Lock()
	defer s.mu.Unlock()
	cols, rows := s.emulator.Width(), s.emulator.Height()
	frame := smokeFrame{cols: cols, rows: make([]string, rows), cells: make([]smokeCell, cols*rows), footerStyles: make([]smokeFooterStyle, cols), alt: s.emulator.IsAltScreen()}
	for y := range rows {
		var row strings.Builder
		for x := range cols {
			cell := smokeCell{text: " ", width: 1}
			if c := s.emulator.CellAt(x, y); c != nil {
				cell = smokeCell{text: c.Content, width: c.Width}
				if y == rows-1 {
					frame.footerStyles[x] = smokeFooterStyle{fg: c.Style.Fg, bg: c.Style.Bg, attrs: c.Style.Attrs}
				}
			}
			frame.cells[y*cols+x] = cell
			row.WriteString(cell.text)
		}
		frame.rows[y] = strings.TrimRight(row.String(), " ")
	}
	return frame
}

func (s *smokeOuterPTY) diagnostics() string {
	frame := s.frame()
	s.traceMu.Lock()
	defer s.traceMu.Unlock()
	status := "running"
	select {
	case <-s.processDone:
		status = fmt.Sprintf("exited: %v", s.waitErr)
	default:
	}
	return fmt.Sprintf("process %s; outer %dx%d, alternate=%v; I/O faults=%v\nouter screen:\n%s\noutput tail: %q\ncapability replies: %q",
		status, frame.cols, len(frame.rows), frame.alt, s.faults, strings.Join(frame.rows, "\n"), s.output, s.replies)
}

func (s *smokeOuterPTY) send(input string) {
	s.t.Helper()
	if err := s.write([]byte(input)); err != nil {
		s.t.Fatalf("send %q: %v\n%s", input, err, s.diagnostics())
	}
}

func (s *smokeOuterPTY) await(what string, check func(smokeFrame) bool) smokeFrame {
	s.t.Helper()
	deadline := time.Now().Add(smokeWait)
	for time.Now().Before(deadline) {
		frame := s.frame()
		if check(frame) {
			return frame
		}
		select {
		case <-s.processDone:
			s.t.Fatalf("host exited before %s\n%s", what, s.diagnostics())
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("timed out waiting for %s\n%s", what, s.diagnostics())
	return smokeFrame{}
}

func (s *smokeOuterPTY) resize(cols, rows int) {
	s.t.Helper()
	s.mu.Lock()
	// Resize the independent terminal before the host can repaint at its new
	// size. TIOCSWINSZ also delivers SIGWINCH to the outer foreground process.
	s.emulator.Resize(cols, rows)
	raw, err := s.ptmx.SyscallConn()
	var ioctlErr error
	if err == nil {
		err = raw.Control(func(fd uintptr) {
			ioctlErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, &unix.Winsize{Col: uint16(cols), Row: uint16(rows)})
		})
	}
	s.mu.Unlock()
	if err := errors.Join(err, ioctlErr); err != nil {
		s.t.Fatalf("resize outer PTY: %v\n%s", err, s.diagnostics())
	}
}

func smokeJoined(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	case <-time.After(smokeJoin):
		return false
	}
}

func (s *smokeOuterPTY) expectExit(want int) {
	s.t.Helper()
	if !smokeJoined(s.processDone) {
		s.t.Fatalf("host did not exit after Bash\n%s", s.diagnostics())
	}
	code := 0
	if s.waitErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(s.waitErr, &exitErr) {
			s.t.Fatalf("wait host: %v\n%s", s.waitErr, s.diagnostics())
		}
		code = exitErr.ExitCode()
	}
	if code != want {
		s.t.Fatalf("host exit status = %d, want %d\n%s", code, want, s.diagnostics())
	}
	if !smokeJoined(s.readerDone) {
		s.t.Fatalf("outer output reader did not drain after exit\n%s", s.diagnostics())
	}
	_ = s.replyPipe.Close()
	if !smokeJoined(s.repliesDone) {
		s.t.Fatal("outer capability reply reader did not stop")
	}
	if s.frame().alt {
		s.t.Fatalf("host left the outer alternate screen active\n%s", s.diagnostics())
	}
	var restored *term.State
	var stateErr error
	raw, err := s.ptmx.SyscallConn()
	if err == nil {
		err = raw.Control(func(fd uintptr) { restored, stateErr = term.GetState(int(fd)) })
	}
	if err := errors.Join(err, stateErr); err != nil {
		s.t.Fatalf("outer TTY state after exit: %v", err)
	}
	if !reflect.DeepEqual(restored, s.initialTTY) {
		s.t.Fatalf("host did not restore outer TTY mode: before=%+v after=%+v\n%s", s.initialTTY, restored, s.diagnostics())
	}
	s.traceMu.Lock()
	faults := append([]string(nil), s.faults...)
	s.traceMu.Unlock()
	if len(faults) != 0 {
		s.t.Fatalf("outer PTY I/O failed: %v\n%s", faults, s.diagnostics())
	}
}

func (s *smokeOuterPTY) close() {
	// Give Bash a chance to exit even after an assertion failed mid-command.
	select {
	case <-s.processDone:
	default:
		_ = s.write([]byte("\x03exit\r"))
		if !smokeJoined(s.processDone) {
			_ = s.cmd.Process.Kill()
		}
	}
	close(s.stopping)
	_ = s.replyPipe.Close()
	_ = s.ptmx.Close()
	joined := true
	for _, worker := range []struct {
		name string
		done <-chan struct{}
	}{{"process waiter", s.processDone}, {"output reader", s.readerDone}, {"capability replies", s.repliesDone}} {
		if !smokeJoined(worker.done) {
			s.t.Errorf("outer PTY %s did not stop within %s", worker.name, smokeJoin)
			joined = false
		}
	}
	if joined {
		// Close mutates vt's unsynchronized closed flag. Close its thread-safe
		// pipe first, then join Read/Write before calling emulator.Close.
		s.mu.Lock()
		_ = s.emulator.Close()
		s.mu.Unlock()
		if s.t.Failed() {
			s.t.Log(s.diagnostics())
		}
	}
}

func TestGhosttyBinarySmoke(t *testing.T) {
	// Build once, lazily when this smoke is selected; both scenarios share it.
	binary := filepath.Join(t.TempDir(), "prelude-workspace")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build prelude-workspace binary: %v (context: %v)\n%s", err, ctx.Err(), output)
	}

	t.Run("ReadlineAndRetainedFloatingPane", func(t *testing.T) {
		leaderSmokeTools(t)
		s := startSmokeOuterPTY(t, binary)
		first := s.await("initial shell and footer", func(f smokeFrame) bool {
			return f.alt && f.footer() == smokeBaseFooter && f.hasInput("prelude $", "")
		})
		assertSmokeFooterTheme(t, first, smokeNavigationKeys, "")
		s.send("echo typed-XY")
		s.await("typed command in readline", func(f smokeFrame) bool {
			return f.hasInput("prelude $", "echo typed-XY") && f.footer() == smokeBaseFooter
		})
		s.send("\x7f\x7fok")
		s.await("backspace edits in readline", func(f smokeFrame) bool {
			return f.hasInput("prelude $", "echo typed-ok") && f.footer() == smokeBaseFooter
		})
		s.send("\x01" + strings.Repeat("\x1b[C", 5) + "\x1b[3~")
		s.await("middle deletion preserving suffix", func(f smokeFrame) bool {
			return f.hasInput("prelude $", "echo yped-ok") && f.footer() == smokeBaseFooter
		})
		s.send("t\r")
		s.await("executed edited command", func(f smokeFrame) bool {
			return f.hasLine("typed-ok") && f.hasInput("prelude $", "") && f.footer() == smokeBaseFooter
		})
		s.send("\x1b[A")
		s.await("readline history", func(f smokeFrame) bool {
			return f.hasInput("prelude $", "echo typed-ok") && f.footer() == smokeBaseFooter
		})
		s.send("\x15")
		s.await("cleared recalled input", func(f smokeFrame) bool { return f.hasInput("prelude $", "") && f.footer() == smokeBaseFooter })
		s.send("smoke_paint\r")
		before := s.await("composed Unicode and panel underlay", func(f smokeFrame) bool {
			return f.rows[2] == "VISIBLE:界é🙂END" && strings.Contains(f.rows[11], "UNDERLAY-CONTENT") && f.hasInput("prelude $", "") && f.footer() == smokeBaseFooter
		})
		// x/vt itself drops decomposed accents while parsing. Assert against
		// the real emitted bytes so that limitation cannot mask a host regression.
		s.traceMu.Lock()
		accentEmitted := strings.Contains(string(s.output), "COMBINING:e\u0301END")
		s.traceMu.Unlock()
		if !accentEmitted {
			t.Fatalf("host dropped a combining mark before the outer terminal\n%s", s.diagnostics())
		}
		for _, want := range []struct {
			x    int
			cell smokeCell
		}{{8, smokeCell{"界", 2}}, {10, smokeCell{"é", 1}}, {11, smokeCell{"🙂", 2}}, {13, smokeCell{"E", 1}}} {
			if got := before.cells[2*before.cols+want.x]; got != want.cell {
				t.Fatalf("composed Unicode cell at (%d,2) = %+v, want %+v\n%s", want.x, got, want.cell, s.diagnostics())
			}
		}
		body := image.Rect(9, 2, 81, 21)
		s.send("\x1bx")
		shown := s.await("Alt+x opens and focuses the real floating menu pane", func(f smokeFrame) bool {
			return f.panel("menu") && strings.Contains(leaderSmokeBody(f, body), "LEADER_MENU:") &&
				f.hasPaneFooter("menu | floating | focus:pane | running")
		})
		assertSmokeFooterTheme(t, shown, smokePaneKeys, "menu | floating | focus:pane | running")
		if text := leaderSmokeBody(shown, body); strings.ContainsAny(text, "┌┐└┘│─") || strings.Contains(text, " | floating |") || strings.Contains(text, "UNDERLAY-CONTENT") {
			t.Fatalf("borderless pane gained host chrome or leaked text through blank child cells: %q", text)
		}
		identity := strings.TrimSpace(shown.textAt(body.Min.X, body.Min.Y, body.Dx()))
		var pid int
		if _, err := fmt.Sscanf(identity, "LEADER_MENU:%d", &pid); err != nil || pid <= 0 {
			t.Fatalf("menu PID marker: %v\n%s", err, s.diagnostics())
		}
		s.send("jk")
		s.await("focused pane receives navigation instead of Bash", func(f smokeFrame) bool {
			return strings.Contains(leaderSmokeBody(f, body), "BYTES:6a6b") && f.hasPaneFooter("menu | floating | focus:pane | running")
		})
		s.send("\x10t")
		s.await("hide restores exact Unicode underlay without killing the menu", func(f smokeFrame) bool {
			return f.hasPaneFooter("menu hidden | running") && syscall.Kill(pid, 0) == nil &&
				strings.Join(f.rows[:len(f.rows)-1], "\n") == strings.Join(before.rows[:len(before.rows)-1], "\n")
		})
		s.send("echo panel-live\r")
		s.await("Bash remains usable while pane is hidden", func(f smokeFrame) bool {
			return f.hasLine("panel-live") && f.hasInput("prelude $", "") && f.hasPaneFooter("menu hidden | running")
		})
		s.send("\x10tz")
		s.await("show refocuses the same PID with retained navigation state", func(f smokeFrame) bool {
			return f.panel("menu") && strings.TrimSpace(f.textAt(body.Min.X, body.Min.Y, body.Dx())) == identity &&
				strings.Contains(leaderSmokeBody(f, body), "BYTES:6a6b7a") && strings.Contains(f.footer(), "focus:pane | running")
		})
		s.send("\x10t")
		s.await("hide reveals current shell output, not a saved underlay", func(f smokeFrame) bool {
			return !f.panel("menu") && f.rows[11] == before.rows[11] && f.rows[2] == before.rows[2] && f.hasLine("panel-live") &&
				f.hasInput("prelude $", "") && f.hasPaneFooter("menu hidden | running")
		})
		s.send("\x10c")
		s.await("explicit close destroys the hidden pane and restores the base footer", func(f smokeFrame) bool {
			return f.footer() == smokeBaseFooter && f.hasInput("prelude $", "") && errors.Is(syscall.Kill(pid, 0), syscall.ESRCH)
		})
		s.send("exit 0\r")
		s.expectExit(0)
	})

	t.Run("ChildScreenIsolationResizeAndExitStatus", func(t *testing.T) {
		leaderSmokeTools(t)
		s := startSmokeOuterPTY(t, binary)
		s.await("initial prompt and footer", func(f smokeFrame) bool { return f.hasInput("prelude $", "") && f.footer() == smokeBaseFooter })
		s.send("smoke_paint\r")
		s.await("original primary screen", func(f smokeFrame) bool {
			return f.rows[2] == "VISIBLE:界é🙂END" && f.hasInput("prelude $", "") && f.footer() == smokeBaseFooter
		})
		s.send("\x10x\x10\t")
		s.await("visible menu leaves Bash focusable before destructive output", func(f smokeFrame) bool {
			return f.panel("menu") && strings.Contains(f.footer(), "focus:shell | running")
		})
		s.send("smoke_screens\r")
		const visibleRunning = "menu | floating | focus:shell | running"
		const hiddenRunning = "menu hidden | running"
		s.await("child alternate screen without losing chrome", func(f smokeFrame) bool {
			return f.rows[0] == "ALT-READY" && f.textAt(0, 2, 8) == "VISIBLE:" &&
				f.rows[ghosttySmokeRows-2] == "CHILD-FOOTER-ATTACK" && f.panel("menu") && f.hasPaneFooter(visibleRunning)
		})
		s.send("clear\r")
		s.await("child clear cannot erase footer or real menu window", func(f smokeFrame) bool {
			return f.rows[0] == "ALT-CLEARED" && f.textAt(0, 2, 8) == "VISIBLE:" && f.rows[ghosttySmokeRows-2] == "CHILD-CLEAR-ATTACK" &&
				f.panel("menu") && strings.Contains(leaderSmokeBody(f, image.Rect(9, 2, 81, 21)), "LEADER_MENU:") && f.hasPaneFooter(visibleRunning)
		})
		s.resize(104, 28)
		s.await("resized composition retains the menu and moves chrome", func(f smokeFrame) bool {
			if f.cols != 104 || len(f.rows) != 28 || !f.panel("menu") || !f.hasPaneFooter(visibleRunning) ||
				f.rows[0] != "ALT-CLEARED" || f.textAt(0, 2, 8) != "VISIBLE:" {
				return false
			}
			for _, row := range f.rows[:len(f.rows)-1] {
				if strings.Contains(row, smokeBaseFooter) || strings.Contains(row, visibleRunning) {
					return false // no stale footer left at the old terminal height
				}
			}
			return true
		})
		s.send("\x10t")
		s.await("hide exposes intact resized alternate screen beneath the menu", func(f smokeFrame) bool {
			return !f.panel("menu") && f.rows[0] == "ALT-CLEARED" && f.rows[2] == "VISIBLE:界é🙂END" &&
				f.rows[ghosttySmokeRows-2] == "CHILD-CLEAR-ATTACK" && strings.Contains(f.rows[11], "ALT-UNDERLAY") && f.hasPaneFooter(hiddenRunning)
		})
		s.send("size\r")
		s.await("outer resize propagated to inner PTY minus footer", func(f smokeFrame) bool {
			return f.rows[4] == "SIZE:27 104" && f.rows[0] == "ALT-CLEARED" && f.rows[2] == "VISIBLE:界é🙂END" && f.hasPaneFooter(hiddenRunning)
		})
		s.send("\x10t\x10\treturn\r")
		s.await("child leaves alternate screen with menu still visible", func(f smokeFrame) bool {
			return f.rows[0] == "PRIMARY-RESTORED" && f.hasInput("prelude $", "") && f.panel("menu") && f.hasPaneFooter("menu | floating | focus:shell | running")
		})
		s.send("\x10t")
		s.await("hide reveals restored primary screen without stale alternate output", func(f smokeFrame) bool {
			return !f.panel("menu") && f.rows[0] == "PRIMARY-RESTORED" && f.rows[2] == "VISIBLE:界é🙂END" &&
				strings.Contains(f.rows[11], "PRIMARY-UNDERLAY") && f.rows[4] == "" && f.hasInput("prelude $", "") && f.hasPaneFooter("menu hidden | running")
		})
		s.send("\x10c")
		s.await("close restores base footer on primary screen", func(f smokeFrame) bool { return f.hasInput("prelude $", "") && f.footer() == smokeBaseFooter })
		s.send("exit 37\r")
		s.expectExit(37)
	})
}
