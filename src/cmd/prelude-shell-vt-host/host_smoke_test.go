package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// outerScreen is what a user would actually see: the host binary's own output,
// parsed by a virtual terminal of the outer window's size. Assertions run
// against the reconstructed screen rather than the byte stream, so they test
// the composed picture instead of the escape sequences that produced it.
type outerScreen struct {
	t    *testing.T
	cmd  *exec.Cmd
	ptmx *os.File

	// mu guards the emulator: the reader goroutine writes into it while the
	// test goroutine renders it.
	mu       sync.Mutex
	emulator *vt.Emulator
}

const (
	smokeCols = 90
	smokeRows = 24
)

func startHostBinary(t *testing.T, args ...string) *outerScreen {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "prelude-shell-vt-host")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build host: %v\n%s", err, output)
	}

	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "PS1=$ ")
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: smokeCols, Rows: smokeRows})
	if err != nil {
		t.Fatalf("start host on a pty: %v", err)
	}

	screen := &outerScreen{t: t, cmd: cmd, ptmx: ptmx, emulator: vt.NewEmulator(smokeCols, smokeRows)}

	// The harness has to behave like a terminal, not just a tape recorder.
	// Bubble Tea probes for capabilities at startup and waits on the answers
	// before it paints, so the emulator's replies must go back to the host.
	go func() {
		buffer := make([]byte, 4096)
		for {
			count, err := screen.emulator.Read(buffer)
			if count > 0 {
				_, _ = ptmx.Write(buffer[:count])
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		buffer := make([]byte, 32*1024)
		for {
			count, readErr := ptmx.Read(buffer)
			if count > 0 {
				screen.mu.Lock()
				_, _ = screen.emulator.Write(buffer[:count])
				screen.mu.Unlock()
			}
			if readErr != nil {
				return
			}
		}
	}()

	t.Cleanup(func() {
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})
	return screen
}

func (s *outerScreen) send(input string) {
	s.t.Helper()
	if _, err := s.ptmx.WriteString(input); err != nil {
		s.t.Fatalf("write to host pty: %v", err)
	}
}

// rows renders the reconstructed outer screen, one string per row.
func (s *outerScreen) rows() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	lines := make([]string, smokeRows)
	for y := range smokeRows {
		var row strings.Builder
		for x := range smokeCols {
			if cell := s.emulator.CellAt(x, y); cell != nil {
				row.WriteString(cell.Content)
			}
		}
		lines[y] = strings.TrimRight(row.String(), " ")
	}
	return lines
}

func (s *outerScreen) text() string { return strings.Join(s.rows(), "\n") }

func (s *outerScreen) await(what string, check func([]string) bool) {
	s.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if check(s.rows()) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	s.t.Fatalf("timed out waiting for %s; outer screen:\n%s", what, s.text())
}

func anyRowContains(rows []string, want string) bool {
	for _, row := range rows {
		if strings.Contains(row, want) {
			return true
		}
	}
	return false
}

// docsStub writes a tiny interactive program to stand in for the docs viewer.
// It paints a banner, then echoes each key it is sent, which is what lets a
// test prove the pane is being driven rather than photographed.
func docsStub(t *testing.T, banner string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "docs-stub")
	script := "#!/bin/sh\n" +
		"printf '" + banner + "\\r\\n'\n" +
		"while IFS= read -r line; do printf 'KEY:%s\\r\\n' \"$line\"; done\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write docs stub: %v", err)
	}
	return path
}

// TestHostKeepsPinnedRowsThroughAChildScreenClear drives the real binary end to
// end, asserting against the reconstructed outer screen. The docs pin runs the
// stub above so the test never needs the real docs binary installed.
func TestHostKeepsPinnedRowsThroughAChildScreenClear(t *testing.T) {
	host := startHostBinary(t,
		"-shell", "/bin/sh",
		"-shell-rows", "6",
		"-docs", docsStub(t, "DOCS-PANE-LINE"),
	)

	host.await("the status row", func(rows []string) bool {
		return strings.Contains(rows[smokeRows-1], "pin")
	})

	host.send("echo smoke-before\r")
	host.await("the child's first output", func(rows []string) bool {
		return anyRowContains(rows, "smoke-before")
	})

	// Ctrl+G three times: off -> motd -> menu -> docs.
	for range 3 {
		host.send("\x07")
		time.Sleep(400 * time.Millisecond)
	}
	host.await("the live docs pane", func(rows []string) bool {
		return anyRowContains(rows, "DOCS-PANE-LINE")
	})

	pinnedBefore := host.rows()[:smokeRows-8]

	// The destructive case: the child erases its entire display and homes the
	// cursor. In a naive host those sequences would reach the outer terminal
	// and wipe the pinned rows.
	//
	// The wait is for the settled state, not just for smoke-after: the child's
	// line discipline echoes the command — smoke-after and all — before printf
	// runs, so "smoke-after is visible" is briefly true while smoke-before is
	// still on screen and the erase has not happened yet.
	host.send("printf '\\033[2J\\033[H'; echo smoke-after\r")
	host.await("the child's screen to be cleared and repainted", func(rows []string) bool {
		return anyRowContains(rows, "smoke-after") && !anyRowContains(rows, "smoke-before")
	})

	pinnedAfter := host.rows()[:smokeRows-8]
	for index := range pinnedBefore {
		if pinnedBefore[index] != pinnedAfter[index] {
			t.Fatalf("pinned row %d changed when the child cleared its screen:\n before: %q\n  after: %q\nouter screen:\n%s",
				index, pinnedBefore[index], pinnedAfter[index], host.text())
		}
	}

	if !anyRowContains(host.rows(), "DOCS-PANE-LINE") {
		t.Fatalf("the pinned docs pane was lost:\n%s", host.text())
	}
}

func TestHostPropagatesTheChildExitStatus(t *testing.T) {
	host := startHostBinary(t, "-shell", "/bin/sh")

	host.await("the status row", func(rows []string) bool {
		return strings.Contains(rows[smokeRows-1], "pin")
	})

	host.send("exit 7\r")

	done := make(chan error, 1)
	go func() { done <- host.cmd.Wait() }()

	var waitErr error
	select {
	case waitErr = <-done:
	case <-time.After(15 * time.Second):
		t.Fatalf("host did not exit after the child did; outer screen:\n%s", host.text())
	}

	// The host reports the child's status rather than inventing one, so a
	// wrapper script can tell a failed command from a broken host.
	exitErr, ok := waitErr.(*exec.ExitError)
	if !ok {
		t.Fatalf("host exited with %v, want the child's status 7", waitErr)
	}
	if code := exitErr.ExitCode(); code != 7 {
		t.Fatalf("host exit code = %d, want the child's 7", code)
	}
}

// pinToDocs cycles the real binary to the docs pin: off -> motd -> menu -> docs.
func pinToDocs(host *outerScreen, banner string) {
	for range 3 {
		host.send("\x07")
		time.Sleep(400 * time.Millisecond)
	}
	host.await("the live docs pane", func(rows []string) bool {
		return anyRowContains(rows, banner)
	})
}

func TestHostDrivesAPinnedPaneWithTheKeyboard(t *testing.T) {
	// The point of a live pane: keys reach the pinned program, not the shell.
	const banner = "DOCS-PANE-READY"
	host := startHostBinary(t,
		"-shell", "/bin/sh",
		"-shell-rows", "6",
		"-docs", docsStub(t, banner),
	)

	host.await("the status row", func(rows []string) bool {
		return strings.Contains(rows[smokeRows-1], "pin")
	})
	pinToDocs(host, banner)

	// Unfocused, typing still belongs to the shell.
	host.send("echo shell-still-mine\r")
	host.await("the shell to take the keystrokes", func(rows []string) bool {
		return anyRowContains(rows, "shell-still-mine")
	})
	if anyRowContains(host.rows(), "KEY:echo shell-still-mine") {
		t.Fatalf("the pane swallowed input while unfocused:\n%s", host.text())
	}

	// Ctrl+O hands the keyboard to the pane.
	host.send("\x0f")
	host.await("the header to report pane focus", func(rows []string) bool {
		return anyRowContains(rows, "input:PANE")
	})

	host.send("navigate-me\r")
	host.await("the pane to receive the keystrokes", func(rows []string) bool {
		return anyRowContains(rows, "KEY:navigate-me")
	})

	// The shell must not have seen any of that.
	if anyRowContains(host.rows(), "navigate-me: command not found") {
		t.Fatalf("input leaked to the shell while the pane was focused:\n%s", host.text())
	}

	// Ctrl+O gives it back.
	host.send("\x0f")
	host.await("the header to report shell focus", func(rows []string) bool {
		return anyRowContains(rows, "input:shell")
	})
	host.send("echo shell-again\r")
	host.await("the shell to take keystrokes again", func(rows []string) bool {
		return anyRowContains(rows, "shell-again")
	})
}

func TestHostReturnsFocusWhenThePaneChildExits(t *testing.T) {
	// A pane whose program quits must not leave a frozen last frame holding
	// the keyboard hostage.
	const banner = "DOCS-PANE-READY"
	host := startHostBinary(t,
		"-shell", "/bin/sh",
		"-shell-rows", "6",
		"-docs", docsStub(t, banner),
	)

	host.await("the status row", func(rows []string) bool {
		return strings.Contains(rows[smokeRows-1], "pin")
	})
	pinToDocs(host, banner)

	host.send("\x0f")
	host.await("pane focus", func(rows []string) bool {
		return anyRowContains(rows, "input:PANE")
	})

	// The stub exits when its stdin reaches EOF; Ctrl+D does that.
	host.send("\x04")
	host.await("the pin to drop when its child exits", func(rows []string) bool {
		return !anyRowContains(rows, banner) && strings.Contains(rows[smokeRows-1], "pin:off")
	})

	// And the shell has the keyboard back.
	host.send("echo shell-recovered\r")
	host.await("the shell to take keystrokes again", func(rows []string) bool {
		return anyRowContains(rows, "shell-recovered")
	})
}
