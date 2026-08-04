package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

type childCursor struct {
	visible bool
	style   vt.CursorStyle
	blink   bool
}

type shellSession struct {
	cmd      *exec.Cmd
	pty      *os.File
	emulator *vt.Emulator
	replies  *terminalReplyPump

	cursor childCursor
	title  string

	waitDone chan struct{}
	waitMu   sync.Mutex
	waitErr  error
	close    sync.Once
	cleanup  func()
}

func startShell(cols, rows, scrollback int, catalog menuCatalog) (*shellSession, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	env := replaceEnv(os.Environ(), "PRELUDE_SHELL_VT_STATUS_SPIKE", "1")
	env = replaceEnv(env, "TERM", envOr("TERM", "xterm-256color"))
	cmd, cleanup, err := preludeShellCommand(shell, env, catalog)
	if err != nil {
		return nil, err
	}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(max(rows, 1)),
		Cols: uint16(max(cols, 1)),
	})
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("start shell PTY: %w", err)
	}

	session := &shellSession{
		cmd:      cmd,
		pty:      ptmx,
		emulator: vt.NewEmulator(max(cols, 1), max(rows, 1)),
		cursor: childCursor{
			visible: true,
			style:   vt.CursorBlock,
			blink:   true,
		},
		title:    "Prelude VT status spike",
		waitDone: make(chan struct{}),
		cleanup:  cleanup,
	}
	session.emulator.SetScrollbackSize(max(scrollback, 1))
	session.emulator.SetCallbacks(vt.Callbacks{
		Title: func(title string) {
			if title != "" {
				session.title = title
			}
		},
		CursorVisibility: func(visible bool) {
			session.cursor.visible = visible
		},
		CursorStyle: func(style vt.CursorStyle, blink bool) {
			session.cursor.style = style
			session.cursor.blink = blink
		},
	})

	session.replies = startTerminalReplyPump(session.pty, session.emulator)
	go func() {
		err := session.cmd.Wait()
		session.waitMu.Lock()
		session.waitErr = err
		session.waitMu.Unlock()
		close(session.waitDone)
	}()

	return session, nil
}

func (s *shellSession) Resize(cols, rows int) error {
	cols = max(cols, 1)
	rows = max(rows, 1)
	s.emulator.Resize(cols, rows)
	if err := pty.Setsize(s.pty, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}); err != nil {
		return fmt.Errorf("resize shell PTY: %w", err)
	}
	return nil
}

func (s *shellSession) SendKey(key uv.KeyPressEvent) { s.emulator.SendKey(key) }
func (s *shellSession) SendText(text string)         { s.emulator.SendText(text) }
func (s *shellSession) Paste(text string)            { s.emulator.Paste(text) }
func (s *shellSession) Focus()                       { s.emulator.Focus() }
func (s *shellSession) Blur()                        { s.emulator.Blur() }
func (s *shellSession) SendMouse(event vt.Mouse)     { s.emulator.SendMouse(event) }

func (s *shellSession) Wait() error {
	<-s.waitDone
	s.waitMu.Lock()
	defer s.waitMu.Unlock()
	return s.waitErr
}

func (s *shellSession) Close() {
	s.close.Do(func() {
		defer s.cleanup()
		_ = s.pty.Close()
		s.replies.Stop()
		_ = s.emulator.Close()

		select {
		case <-s.waitDone:
			return
		default:
		}
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Signal(syscall.SIGHUP)
		}
		select {
		case <-s.waitDone:
		case <-time.After(300 * time.Millisecond):
			if s.cmd.Process != nil {
				_ = s.cmd.Process.Kill()
			}
			<-s.waitDone
		}
	})
}

// preludeShellCommand recreates the prompt initialization performed by Prelude's
// nix develop setup hook, then layers the ble.sh integration on top. STARSHIP_CONFIG
// remains untouched: the child resolves the same generated configuration as its
// parent environment. ble.sh is bash-only, so the integration requires bash.
func preludeShellCommand(shell string, env []string, catalog menuCatalog) (*exec.Cmd, func(), error) {
	shellKind := filepath.Base(shell)
	if shellKind != "bash" {
		return nil, nil, fmt.Errorf("prelude ble.sh integration requires bash (got %q); run inside nix develop", shell)
	}

	starship, err := exec.LookPath("starship")
	if err != nil {
		return nil, nil, fmt.Errorf("starship not on PATH (run inside nix develop): %w", err)
	}
	var initScript []byte
	if setupHook := adjacentPromptSetupHook(starship); setupHook != "" {
		// This is the canonical nix develop path. The package-owned function
		// includes ble.sh, palette faces, Darwin stty handling, and the
		// shell-specific Starship initialization.
		initScript = []byte(fmt.Sprintf("source %q\npreludePromptInit\n", setupHook))
	} else {
		// Standalone fallback for a user-installed Starship. ble.sh will not be
		// loaded here, so prelude-ble.sh's BLE_VERSION guard no-ops gracefully.
		initScript, err = exec.Command(starship, "init", "bash").Output()
		if err != nil {
			return nil, nil, fmt.Errorf("starship init bash: %w", err)
		}
	}
	lineEditing := exec.Command(shell, "-c", "set -o emacs")
	lineEditing.Env = env
	if err := lineEditing.Run(); err != nil {
		return nil, nil, fmt.Errorf("bash has no interactive line editing; enter nix develop interactively before running the spike")
	}

	dir, err := os.MkdirTemp("", "prelude-shell-vt-starship-*")
	if err != nil {
		return nil, nil, fmt.Errorf("starship startup directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	if err := os.WriteFile(filepath.Join(dir, "prelude-catalog.sh"), catalog.emitShellCatalog(), 0o600); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("write prelude-catalog.sh: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prelude-ble.sh"), []byte(preludeBleScript), 0o600); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("write prelude-ble.sh: %w", err)
	}

	// Source the generated catalog first so the integration script sees the
	// arrays, then source the version-controlled ble.sh glue. $PRELUDE_RC_DIR
	// avoids interpolating the temp path into the script body.
	initScript = append(initScript,
		[]byte("\n# Prelude × ble.sh integration.\nexport PRELUDE_RC_DIR=")...)
	initScript = append(initScript, []byte(shellSingle(dir))...)
	initScript = append(initScript, []byte("\nsource \"$PRELUDE_RC_DIR/prelude-catalog.sh\"\nsource \"$PRELUDE_RC_DIR/prelude-ble.sh\"\n")...)

	rcPath := filepath.Join(dir, "bashrc")
	if err := os.WriteFile(rcPath, initScript, 0o600); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("starship startup file: %w", err)
	}

	cmd := exec.Command(shell, "--noprofile", "--rcfile", rcPath, "-i")
	cmd.Env = env
	return cmd, cleanup, nil
}

// adjacentPromptSetupHook resolves the canonical prompt initializer shipped by
// Prelude's aggregate Nix package. A standalone Starship has no such hook.
func adjacentPromptSetupHook(starship string) string {
	candidate := filepath.Join(filepath.Dir(filepath.Dir(starship)), "nix-support", "setup-hook")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

// terminalReplyPump carries input and terminal-query responses produced by an
// emulator back to its PTY. Stop wakes the blocking Emulator.Read before
// Emulator.Close, avoiding the data race in x/vt's raw Read/Close pair.
type terminalReplyPump struct {
	destination io.Writer
	emulator    *vt.Emulator
	stop        chan struct{}
	done        chan struct{}
	once        sync.Once
}

func startTerminalReplyPump(destination io.Writer, emulator *vt.Emulator) *terminalReplyPump {
	pump := &terminalReplyPump{
		destination: destination,
		emulator:    emulator,
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
	go pump.run()
	return pump
}

func (pump *terminalReplyPump) run() {
	defer close(pump.done)
	buffer := make([]byte, 4*1024)
	for {
		count, err := pump.emulator.Read(buffer)
		if err != nil {
			return
		}
		select {
		case <-pump.stop:
			return
		default:
		}
		if count > 0 {
			_, _ = pump.destination.Write(buffer[:count])
		}
	}
}

func (pump *terminalReplyPump) Stop() {
	pump.once.Do(func() {
		close(pump.stop)
		// Emulator.Read has no cancellation API. This byte is consumed by the
		// stopped pump and is never forwarded to the child.
		pump.emulator.SendText("\x00")
		<-pump.done
	})
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func replaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	replaced := false
	out := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			if !replaced {
				out = append(out, prefix+value)
				replaced = true
			}
			continue
		}
		out = append(out, entry)
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}
