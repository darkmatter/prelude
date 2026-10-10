package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// PATH wrappers exec this test binary as a deterministic PTY child. No Prelude
// surface implementation is copied, and no native engine runs in the child.
func TestPanePTYHelper(t *testing.T) {
	if os.Getenv("PRELUDE_GHOSTTY_PANE_TEST") != "1" {
		return
	}
	fail := func(err error) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(95)
	}
	if !term.IsTerminal(0) || !term.IsTerminal(1) {
		fail(errors.New("pane child needs a real PTY on stdin and stdout"))
	}
	if _, err := term.MakeRaw(0); err != nil {
		fail(err)
	}
	cols, rows, err := term.GetSize(0)
	if err != nil {
		fail(err)
	}
	args := flag.Args()
	if len(args) == 0 {
		fail(errors.New("missing wrapper identity"))
	}
	shownArgs := strings.Join(args[1:], " ")
	if args[0] == "x" {
		menuSelectionOutput(t, args[1:])
		// Keep the marker on one PTY row; the private path is checked above.
		shownArgs = strings.Join(args[1:3], " ")
	} else if args[0] == "docs" && len(args) != 1 {
		fail(fmt.Errorf("docs args = %q, want no arguments", args[1:]))
	}
	fmt.Fprintf(os.Stdout, "COMMAND:%s ARGS:%q CONFIG:%s SIZE:%dx%d\r\n",
		args[0], shownArgs, os.Getenv("PRELUDE_GHOSTTY_PANE_CONFIG"), cols, rows)
	readWant := func(want string) {
		data := make([]byte, len(want))
		if _, err := io.ReadFull(os.Stdin, data); err != nil {
			fail(err)
		}
		if string(data) != want {
			fail(fmt.Errorf("PTY input = %q, want %q", data, want))
		}
	}
	switch os.Getenv("PRELUDE_GHOSTTY_PANE_MODE") {
	case "hold":
		readWant("q")
		os.Exit(0)
	case "fd-probe":
		var fd int
		var dev, ino, rdev uint64
		if _, err := fmt.Sscanf(os.Getenv("PRELUDE_GHOSTTY_PANE_FD"), "%d:%d:%d:%d", &fd, &dev, &ino, &rdev); err != nil {
			fail(err)
		}
		var stat unix.Stat_t
		if unix.Fstat(fd, &stat) == nil && uint64(stat.Dev) == dev && uint64(stat.Ino) == ino && uint64(stat.Rdev) == rdev {
			fmt.Fprint(os.Stdout, "PTY_INHERITANCE:LEAK\r\n")
		} else {
			fmt.Fprint(os.Stdout, "PTY_INHERITANCE:CLEAN\r\n")
		}
		readWant("q")
		os.Exit(0)
	case "delayed-final":
		fmt.Fprint(os.Stdout, "DELAY_READY\r\n")
		readWant("q")
		fmt.Fprint(os.Stdout, "BUFFERED_BEFORE_FINAL\r\n")
		readWant("g")
		fmt.Fprint(os.Stdout, "\x1b[2J\x1b[HDELAYED_FINAL_IMAGE\r\n")
		os.Exit(0)
	case "descendant-parent", "exit-with-noisy-descendant":
		signal.Reset(syscall.SIGHUP)
		reader, writer, err := os.Pipe()
		if err != nil {
			fail(err)
		}
		child := exec.Command(os.Args[0], "-test.run=^TestPanePTYHelper$", "--", "descendant")
		child.Env = append(os.Environ(), "PRELUDE_GHOSTTY_PANE_MODE=stubborn-descendant")
		if os.Getenv("PRELUDE_GHOSTTY_PANE_MODE") == "exit-with-noisy-descendant" {
			child.Env = append(child.Env, "PRELUDE_GHOSTTY_PANE_NOISY=1")
		}
		child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
		child.ExtraFiles = []*os.File{writer}
		if err := child.Start(); err != nil {
			fail(err)
		}
		_ = writer.Close()
		var ready [1]byte
		if _, err := io.ReadFull(reader, ready[:]); err != nil {
			fail(err)
		}
		_ = reader.Close()
		if os.Getenv("PRELUDE_GHOSTTY_PANE_MODE") == "exit-with-noisy-descendant" {
			os.Exit(0)
		}
		if err := child.Wait(); err != nil {
			fail(err)
		}
		os.Exit(0)
	case "stubborn-descendant":
		signal.Ignore(syscall.SIGHUP, syscall.SIGPIPE)
		conn, err := net.DialTimeout("unix", os.Getenv("PRELUDE_GHOSTTY_PANE_SOCKET"), 2*time.Second)
		if err != nil {
			fail(err)
		}
		fmt.Fprintf(conn, "%d %d\n", os.Getpid(), unix.Getpgrp())
		fmt.Fprint(os.Stdout, "DESCENDANT_READY\r\n")
		ready := os.NewFile(3, "descendant-ready")
		if _, err := ready.Write([]byte{1}); err != nil {
			fail(err)
		}
		_ = ready.Close()
		if os.Getenv("PRELUDE_GHOSTTY_PANE_NOISY") == "1" {
			data := bytes.Repeat([]byte("NOISY_DESCENDANT\r\n"), outputChunk/18)
			for {
				_, _ = os.Stdout.Write(data)
				runtime.KeepAlive(conn)
			}
		}
		time.Sleep(10 * time.Second)
		_ = conn.Close()
		os.Exit(0)
	case "roundtrip":
		fmt.Fprint(os.Stdout, "\x1b[2J\x1b[HKEEP\r\n\x1b[3;4H\x1b[6n")
		readWant("\x1b[3;4R")
		fmt.Fprint(os.Stdout, "CURSOR_REPLY_OK\r\n\x1b[?2048hRESIZE_READY\r\n")
		readWant("\x1b[48;9;64;0;0t")
		cols, rows, err := term.GetSize(0)
		if err != nil || cols != 64 || rows != 9 {
			fail(fmt.Errorf("resized PTY = %dx%d, %v", cols, rows, err))
		}
		fmt.Fprint(os.Stdout, "RESIZE_REPLY_OK SIZE:64x9\r\n")
		readWant("go\r")
		fmt.Fprint(os.Stdout, "\r\nINPUT_OK\r\n")
		os.Exit(37)
	case "abandon":
		signal.Ignore(syscall.SIGHUP)
		fmt.Fprint(os.Stdout, "ABANDONED_PTY\r\n")
		_ = os.Stdin.Close()
		_ = os.Stdout.Close()
		_ = os.Stderr.Close()
		time.Sleep(5 * time.Second)
		os.Exit(96)
	case "noisy":
		signal.Ignore(syscall.SIGHUP)
		data := bytes.Repeat([]byte("NOISY_CHILD\r\n"), outputChunk/13)
		for {
			// Stay stubborn even after the master closes and writes fail.
			_, _ = os.Stdout.Write(data)
		}
	default:
		fail(errors.New("unknown pane fixture mode"))
	}
}

// Check the public handoff at the child boundary, without depending on pane
// fields or the tempfile's name. A fresh picker must receive an empty, private
// regular file, never a symlink or a shared output channel.
func menuSelectionOutput(t *testing.T, args []string) string {
	t.Helper()
	if len(args) != 3 || args[0] != "--embedded" || args[1] != "--select-output" || !filepath.IsAbs(args[2]) {
		t.Fatalf("menu args = %q, want --embedded --select-output <absolute private tempfile>", args)
	}
	info, err := os.Lstat(args[2])
	if err != nil {
		t.Fatal("menu selection output:", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() != 0 {
		t.Fatalf("menu selection output must start empty and private: mode=%v size=%d", info.Mode(), info.Size())
	}
	return args[2]
}

func paneTestPATH(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, command := range []string{"x", "docs"} {
		wrapper := fmt.Sprintf("#!%s\nexec '%s' '-test.run=^TestPanePTYHelper$' -- '%s' \"$@\"\n",
			sh, strings.ReplaceAll(executable, "'", "'\\''"), command)
		if err := os.WriteFile(filepath.Join(dir, command), []byte(wrapper), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func newPaneTest(t *testing.T, kind surfaceKind, cols, rows int, mode string) *pane {
	t.Helper()
	env := append(os.Environ(), "PRELUDE_GHOSTTY_PANE_TEST=1", "PRELUDE_GHOSTTY_PANE_CONFIG=fixture-config",
		"PRELUDE_GHOSTTY_PANE_MODE="+mode)
	p, err := startPane(kind, cols, rows, env)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := p.close(); err != nil {
			t.Error(err)
		}
	})
	return p
}

func paneTestFrameText(frame terminalFrame) string {
	var text strings.Builder
	for y := range frame.Rows {
		for x := range frame.Cols {
			text.WriteString(frame.Cells[y*frame.Cols+x].Content)
		}
		text.WriteByte('\n')
	}
	return text.String()
}

// Drive output commands only when bytes or lifecycle results are available, so
// the test has a deadline without adding another worker. All native operations,
// including cleanup, stay on this test's equivalent of the host UI loop.
func awaitPaneTest(t *testing.T, p *pane, what string, check func() bool) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	tick := time.NewTicker(outputPace)
	defer tick.Stop()
	for !check() {
		if p.done {
			t.Fatalf("pane finished before %s: code=%d err=%v\n%s", what, p.exitCode, p.err, paneTestFrameText(p.frame))
		}
		ready := len(p.process.output) != 0
		select {
		case <-p.process.exited:
			ready = true
		default:
		}
		select {
		case <-p.process.readerDone:
			ready = true
		default:
		}
		if ready {
			switch message := p.nextOutput().(type) {
			case paneOutputMsg:
				if message.pane != p {
					t.Fatal("output lost its pane identity")
				}
				if len(message.output.data) != 0 {
					if err := p.absorb(message.output.data); err != nil {
						t.Fatal(err)
					}
				}
				if err := message.output.err; err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EIO) {
					_ = p.finish(fmt.Errorf("read pane PTY: %w", err))
				}
			case paneExitMsg:
				if message.pane != p {
					t.Fatal("exit lost its pane identity")
				}
				_ = p.finish(message.err)
			case paneFailureMsg:
				if message.pane != p {
					t.Fatal("failure lost its pane identity")
				}
				_ = p.finish(message.err)
			default:
				t.Fatalf("output command stopped before %s", what)
			}
		}
		select {
		case <-timer.C:
			t.Fatalf("waiting for %s: err=%v\n%s", what, p.err, paneTestFrameText(p.frame))
		default:
		}
		if !ready {
			select {
			case <-tick.C:
			case <-timer.C:
				t.Fatalf("waiting for %s\n%s", what, paneTestFrameText(p.frame))
			}
		}
	}
}

func paneTestProcessClosed(t *testing.T, p *pane) {
	t.Helper()
	for name, done := range map[string]<-chan struct{}{
		"reader": p.process.readerDone, "waiter": p.process.exited, "commands": p.process.stop,
	} {
		select {
		case <-done:
		default:
			t.Fatalf("%s survived process cleanup", name)
		}
	}
	if _, err := p.process.ptmx.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("PTY master survived cleanup: %v", err)
	}
}

func TestPanePublicCommandsHaveSeparatePTYs(t *testing.T) {
	paneTestPATH(t)
	var panes []*pane
	for _, kind := range []surfaceKind{surfaceMenu, surfaceDocs} {
		p := newPaneTest(t, kind, 96, 8, "hold")
		panes = append(panes, p)
		command, args := kind.String(), ""
		if kind == surfaceMenu {
			command, args = "x", "--embedded --select-output"
		}
		want := fmt.Sprintf("COMMAND:%s ARGS:%q CONFIG:fixture-config SIZE:96x8", command, args)
		awaitPaneTest(t, p, "public wrapper invocation", func() bool {
			return strings.Contains(paneTestFrameText(p.frame), want)
		})
		if p.kind != kind || p.process.rc != "" {
			t.Fatalf("pane kind=%s rc=%q", p.kind, p.process.rc)
		}
		for _, previous := range panes[:len(panes)-1] {
			if p.process == previous.process || p.process.ptmx == previous.process.ptmx ||
				p.process.cmd.Process.Pid == previous.process.cmd.Process.Pid || p.terminal == previous.terminal {
				t.Fatal("surfaces shared a process, PTY, or terminal engine")
			}
		}
	}
	if err := panes[0].forward([]byte("q"), nil); err != nil {
		t.Fatal(err)
	}
	awaitPaneTest(t, panes[0], "menu exit", func() bool { return panes[0].done })
	if panes[0].exitCode != 0 || panes[0].err != nil {
		t.Fatalf("menu exit = %d, %v", panes[0].exitCode, panes[0].err)
	}
	paneTestProcessClosed(t, panes[0])
	for _, p := range panes[1:] {
		select {
		case <-p.process.exited:
			t.Fatalf("closing the menu also exited %s", p.kind)
		default:
		}
	}
}

func TestPaneRepliesResizeAndExitKeepTerminal(t *testing.T) {
	paneTestPATH(t)
	p := newPaneTest(t, surfaceMenu, 42, 8, "roundtrip")
	awaitPaneTest(t, p, "cursor reply and resize mode", func() bool {
		return strings.Contains(paneTestFrameText(p.frame), "RESIZE_READY")
	})
	process, engine, pid := p.process, p.terminal, p.process.cmd.Process.Pid
	before, beforeText := p.frame, paneTestFrameText(p.frame)
	if err := p.resize(64, 9); err != nil {
		t.Fatal(err)
	}
	if p.process != process || p.terminal != engine || p.process.cmd.Process.Pid != pid ||
		p.frame.Cols != 64 || p.frame.Rows != 9 || !strings.Contains(paneTestFrameText(p.frame), "KEEP") {
		t.Fatal("resize replaced the child/engine or lost its contents")
	}
	if paneTestFrameText(before) != beforeText {
		t.Fatal("resize mutated the previous owned frame")
	}
	awaitPaneTest(t, p, "native resize reply and PTY geometry", func() bool {
		return strings.Contains(paneTestFrameText(p.frame), "RESIZE_REPLY_OK SIZE:64x9")
	})
	if err := p.forward(p.terminal.Paste("go")); err != nil {
		t.Fatal(err)
	}
	if err := p.forward(p.terminal.Key(tea.Key{Code: tea.KeyEnter})); err != nil {
		t.Fatal(err)
	}
	awaitPaneTest(t, p, "nonzero exit and final output", func() bool { return p.done })
	if p.exitCode != 37 || p.err != nil || !strings.Contains(paneTestFrameText(p.frame), "INPUT_OK") {
		t.Fatalf("pane exit = %d, %v\n%s", p.exitCode, p.err, paneTestFrameText(p.frame))
	}
	paneTestProcessClosed(t, p)
	if p.closed || p.terminal.open() != nil {
		t.Fatal("finish closed the native terminal")
	}
	if err := p.resize(72, 10); err != nil {
		t.Fatal("resize completed pane:", err)
	}
	if p.process != process || p.terminal != engine || p.frame.Cols != 72 || p.frame.Rows != 10 ||
		!strings.Contains(paneTestFrameText(p.frame), "KEEP") || !strings.Contains(paneTestFrameText(p.frame), "INPUT_OK") {
		t.Fatal("completed pane lost its retained process identity or frame")
	}
	if err := p.forward([]byte("ignored after exit"), nil); err != nil {
		t.Fatal(err)
	}
	if err := p.finish(errors.New("stale exit")); err != nil || p.exitCode != 37 {
		t.Fatal("a repeated finish changed the exit status:", err)
	}
	if err := p.close(); err != nil {
		t.Fatal(err)
	}
	if !p.closed || p.terminal.open() == nil {
		t.Fatal("close retained the native terminal")
	}
	if err := p.close(); err != nil || p.nextOutput() != nil {
		t.Fatal("close was not idempotent or output survived cleanup:", err)
	}
}

func TestPaneAbandonedPTYRetainsFailureAndFrame(t *testing.T) {
	paneTestPATH(t)
	p := newPaneTest(t, surfaceDocs, 64, 8, "abandon")
	start := time.Now()
	awaitPaneTest(t, p, "abandoned PTY failure", func() bool { return p.done })
	if p.exitCode != 1 || p.err == nil || !strings.Contains(p.err.Error(), "without exiting") ||
		!strings.Contains(paneTestFrameText(p.frame), "ABANDONED_PTY") {
		t.Fatalf("abandoned pane = %d, %v\n%s", p.exitCode, p.err, paneTestFrameText(p.frame))
	}
	if time.Since(start) > 4*closeGrace {
		t.Fatal("abandoned PTY failure and cleanup were not bounded")
	}
	paneTestProcessClosed(t, p)
	failure := p.err
	if err := p.finish(nil); err != failure || p.err != failure || p.exitCode != 1 {
		t.Fatal("finish lost the original non-exit error")
	}
	if err := p.resize(70, 9); err != nil || p.terminal.open() != nil {
		t.Fatal("failed pane did not retain a resizable native frame:", err)
	}
}

func TestPaneBoundsWriteAndNoisyStubbornCleanup(t *testing.T) {
	paneTestPATH(t)
	p := newPaneTest(t, surfaceMenu, 64, 8, "noisy")
	awaitPaneTest(t, p, "noisy child", func() bool {
		return strings.Contains(paneTestFrameText(p.frame), "NOISY_CHILD")
	})
	start := time.Now()
	err := p.forward(bytes.Repeat([]byte("i"), 8*outputChunk), nil)
	if !errors.Is(err, os.ErrDeadlineExceeded) || time.Since(start) > 4*writeTimeout {
		t.Fatalf("unread PTY input escaped its write deadline: %v", err)
	}
	start = time.Now()
	if err := p.close(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 4*closeGrace {
		t.Fatal("noisy stubborn child escaped the shutdown bound")
	}
	paneTestProcessClosed(t, p)
	if err := p.close(); err != nil || p.nextOutput() != nil {
		t.Fatal("closed noisy pane still had a live output command:", err)
	}
}

func TestPaneChildDoesNotInheritOtherPTY(t *testing.T) {
	paneTestPATH(t)
	first := newPaneTest(t, surfaceMenu, 96, 8, "hold")
	awaitPaneTest(t, first, "first pane ready", func() bool {
		return strings.Contains(paneTestFrameText(first.frame), "COMMAND:x")
	})
	raw, err := first.process.ptmx.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var identity string
	var statErr error
	if err := raw.Control(func(fd uintptr) {
		var stat unix.Stat_t
		statErr = unix.Fstat(int(fd), &stat)
		identity = fmt.Sprintf("%d:%d:%d:%d", fd, uint64(stat.Dev), uint64(stat.Ino), uint64(stat.Rdev))
	}); err != nil {
		t.Fatal(err)
	}
	if statErr != nil {
		t.Fatal(statErr)
	}
	t.Setenv("PRELUDE_GHOSTTY_PANE_FD", identity)
	second := newPaneTest(t, surfaceDocs, 96, 8, "fd-probe")
	awaitPaneTest(t, second, "child descriptor probe", func() bool {
		return strings.Contains(paneTestFrameText(second.frame), "PTY_INHERITANCE:")
	})
	if !strings.Contains(paneTestFrameText(second.frame), "PTY_INHERITANCE:CLEAN") {
		t.Fatalf("subsequent pane inherited another pane's PTY master:\n%s", paneTestFrameText(second.frame))
	}
	if first.done {
		t.Fatal("descriptor probe stopped the first pane")
	}
}

func TestPaneDelayedFinalImageSurvives(t *testing.T) {
	paneTestPATH(t)
	p := newPaneTest(t, surfaceDocs, 64, 8, "delayed-final")
	awaitPaneTest(t, p, "final-image fixture ready", func() bool {
		return strings.Contains(paneTestFrameText(p.frame), "DELAY_READY")
	})
	if err := p.forward([]byte("q"), nil); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(p.process.output) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("fixture did not queue its first tail chunk")
		}
		time.Sleep(outputPace)
	}
	// Leave one chunk queued while the reader acquires the final image behind it.
	if err := p.forward([]byte("g"), nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.process.exited:
	case <-time.After(2 * time.Second):
		t.Fatal("final-image fixture did not exit")
	}
	time.Sleep(closeGrace + 2*outputPace)
	awaitPaneTest(t, p, "delayed final pane exit", func() bool { return p.done })
	if p.err != nil || p.exitCode != 0 || !strings.Contains(paneTestFrameText(p.frame), "DELAYED_FINAL_IMAGE") {
		t.Fatalf("delayed final pane image lost: code=%d err=%v\n%s", p.exitCode, p.err, paneTestFrameText(p.frame))
	}
	paneTestProcessClosed(t, p)
	if err := p.resize(72, 9); err != nil || !strings.Contains(paneTestFrameText(p.frame), "DELAYED_FINAL_IMAGE") {
		t.Fatal("delayed final image did not survive reflow:", err)
	}
}

// Socket EOF observes death even if an orphan is briefly a zombie. A short
// directory name also fits macOS's smaller Unix socket path limit.
func newPaneDescendantTest(t *testing.T, mode string) (*pane, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "pane-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "s")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if err := listener.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PRELUDE_GHOSTTY_PANE_SOCKET", path)
	p := newPaneTest(t, surfaceMenu, 64, 8, mode)
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	var pid, group int
	if _, err := fmt.Sscanf(line, "%d %d", &pid, &group); err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			_ = syscall.Kill(pid, syscall.SIGKILL) // also clean up a failing regression
		}
		_ = conn.Close()
	})
	if group != p.process.cmd.Process.Pid {
		t.Fatalf("fixture descendant group=%d, want pane group=%d", group, p.process.cmd.Process.Pid)
	}
	return p, func() {
		t.Helper()
		if err := conn.SetReadDeadline(time.Now().Add(closeGrace)); err != nil {
			t.Fatal(err)
		}
		var data [1]byte
		if _, err := conn.Read(data[:]); !errors.Is(err, io.EOF) {
			t.Fatalf("same-group descendant survived pane cleanup: %v", err)
		}
		stopped = true
	}
}

func TestPaneCloseStopsSameGroupDescendant(t *testing.T) {
	paneTestPATH(t)
	p, assertStopped := newPaneDescendantTest(t, "descendant-parent")
	awaitPaneTest(t, p, "same-group descendant ready", func() bool {
		return strings.Contains(paneTestFrameText(p.frame), "DESCENDANT_READY")
	})
	start := time.Now()
	if err := p.close(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 4*closeGrace {
		t.Fatal("pane close exceeded its teardown bound")
	}
	paneTestProcessClosed(t, p)
	assertStopped()
}

func TestPaneExitDrainBoundsNoisyDescendant(t *testing.T) {
	paneTestPATH(t)
	p, assertStopped := newPaneDescendantTest(t, "exit-with-noisy-descendant")
	select {
	case <-p.process.exited:
	case <-time.After(2 * time.Second):
		t.Fatal("noisy descendant's direct parent did not exit")
	}
	time.Sleep(closeGrace + 2*outputPace)
	start := time.Now()
	awaitPaneTest(t, p, "bounded noisy descendant drain", func() bool { return p.done })
	if time.Since(start) > 4*closeGrace || p.err != nil || p.exitCode != 0 {
		t.Fatalf("noisy descendant escaped drain/cleanup bound: elapsed=%s code=%d err=%v", time.Since(start), p.exitCode, p.err)
	}
	paneTestProcessClosed(t, p)
	assertStopped()
}

func TestPaneTaggedMessagesNeedNoNativeTerminal(t *testing.T) {
	newProcess := func() *shellProcess {
		return &shellProcess{output: make(chan outputMsg, 1), stop: make(chan struct{}),
			readerDone: make(chan struct{}), exited: make(chan struct{})}
	}
	p := &pane{process: newProcess()}
	failure := errors.New("read fixture")
	p.process.output <- outputMsg{data: []byte("output"), err: failure}
	output, ok := p.nextOutput().(paneOutputMsg)
	if !ok || output.pane != p || string(output.output.data) != "output" || output.output.err != failure {
		t.Fatalf("tagged output = %+v", output)
	}
	p.process.waitErr, p.process.exitedAt = failure, time.Now().Add(-closeGrace)
	close(p.process.exited)
	exit, ok := p.nextOutput().(paneExitMsg)
	if !ok || exit.pane != p || exit.err != failure {
		t.Fatalf("tagged exit = %+v", exit)
	}
	close(p.process.stop)
	if message := p.nextOutput(); message != nil {
		t.Fatalf("nil output became %T", message)
	}
	p = &pane{process: newProcess()}
	close(p.process.readerDone)
	failed, ok := p.nextOutput().(paneFailureMsg)
	if !ok || failed.pane != p || failed.err == nil {
		t.Fatalf("tagged failure = %+v", failed)
	}
}

func TestPaneStartupErrorsAndSurfaceLabels(t *testing.T) {
	dir := paneTestPATH(t)
	for kind, want := range map[surfaceKind]string{
		surfaceNone: "none", surfaceMenu: "menu", surfaceDocs: "docs",
	} {
		if kind.String() != want {
			t.Fatalf("surface %d label = %q, want %q", kind, kind.String(), want)
		}
	}
	for _, kind := range []surfaceKind{surfaceNone, surfaceKind(255)} {
		if p, err := startPane(kind, 32, 8, os.Environ()); p != nil || err == nil {
			t.Fatalf("unsupported surface %d = %v, %v", kind, p, err)
		}
	}
	t.Run("missing public command", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		for _, kind := range []surfaceKind{surfaceMenu, surfaceDocs} {
			p, err := startPane(kind, 32, 8, os.Environ())
			if p != nil || err == nil || !strings.Contains(err.Error(), kind.String()+" pane command") ||
				!strings.Contains(err.Error(), "PATH") {
				t.Fatalf("missing %s = %v, %v", kind, p, err)
			}
		}
	})
	t.Run("invalid geometry", func(t *testing.T) {
		if p, err := startPane(surfaceMenu, 0, 8, os.Environ()); p != nil || err == nil {
			t.Fatalf("invalid geometry = %v, %v", p, err)
		}
	})
	t.Run("unstartable wrapper", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(dir, "docs"), []byte("#!/no/such/pane-interpreter\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		p, err := startPane(surfaceDocs, 32, 8, os.Environ())
		if p != nil || err == nil || !strings.Contains(err.Error(), "start docs pane command") {
			t.Fatalf("unstartable wrapper = %v, %v", p, err)
		}
	})
}
