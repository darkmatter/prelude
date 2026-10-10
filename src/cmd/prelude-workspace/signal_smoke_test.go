package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestGhosttySignalBinarySmoke(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "prelude-workspace")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build prelude-workspace binary: %v (context: %v)\n%s", err, ctx.Err(), output)
	}

	for _, signal := range []struct {
		name string
		sig  syscall.Signal
	}{
		{"HUP", syscall.SIGHUP},
		{"TERM", syscall.SIGTERM},
		{"INT", syscall.SIGINT},
	} {
		t.Run(signal.name, func(t *testing.T) {
			leaderSmokeTools(t)
			privateDir := t.TempDir()
			t.Setenv("TMPDIR", privateDir)
			s := startSmokeOuterPTY(t, binary)
			s.await("initial shell and footer", func(f smokeFrame) bool {
				return f.alt && f.footer() == smokeBaseFooter && f.hasInput("prelude $", "")
			})
			// A separate foreground Bash waits for input while the menu stays live.
			s.send("bash --noprofile --norc -c 'echo SIGNAL_FOREGROUND_READY; read'\r")
			s.await("foreground process waiting for input", func(f smokeFrame) bool {
				return f.hasLine("SIGNAL_FOREGROUND_READY")
			})
			s.send("\x10x")
			s.await("active menu pane over the foreground process", func(f smokeFrame) bool {
				return f.panel("menu") && f.hasPaneFooter("menu | floating | focus:pane | running")
			})
			files, err := os.ReadDir(privateDir)
			if err != nil || len(files) == 0 {
				t.Fatalf("private files before signal: files=%v err=%v", files, err)
			}

			if err := s.cmd.Process.Signal(signal.sig); err != nil {
				t.Fatal("signal host:", err)
			}
			// Also checks alternate-screen exit, cooked TTY restoration and PTY drain.
			s.expectExit(128 + int(signal.sig))
			if remaining, err := os.ReadDir(privateDir); err != nil || len(remaining) != 0 {
				t.Fatalf("private files survived %s: files=%v err=%v (before: %v)", signal.name, remaining, err, files)
			}
		})
	}
}
