package main

import (
	"bufio"
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

const vtpinShellFunction = `
vtpin() {
  if [ "$#" -ne 1 ]; then
    printf '%s\n' 'usage: vtpin {motd|docs|off}' >&2
    return 2
  fi

  case "$1" in
    motd|docs|off)
      printf '%s\n' "$1" >&"$PRELUDE_VTPIN_FD"
      ;;
    *)
      printf 'vtpin: unsupported target: %s\n' "$1" >&2
      printf '%s\n' 'usage: vtpin {motd|docs|off}' >&2
      return 2
      ;;
  esac
}
`

type childCursor struct {
	visible bool
	style   vt.CursorStyle
	blink   bool
}

type shellSession struct {
	cmd      *exec.Cmd
	pty      *os.File
	pinPipe  *os.File
	pinInput *bufio.Reader
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

func startShell(cols, rows int) (*shellSession, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	env := replaceEnv(os.Environ(), "PRELUDE_SHELL_VT_SPIKE", "1")
	env = replaceEnv(env, "TERM", envOr("TERM", "xterm-256color"))
	pinReader, pinWriter, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("vtpin control pipe: %w", err)
	}
	cmd, cleanup, err := starshipShellCommand(shell, env)
	if err != nil {
		_ = pinReader.Close()
		_ = pinWriter.Close()
		return nil, err
	}
	vtpinControlFD := 3 + len(cmd.ExtraFiles)
	cmd.Env = replaceEnv(cmd.Env, "PRELUDE_VTPIN_FD", fmt.Sprintf("%d", vtpinControlFD))
	cmd.ExtraFiles = append(cmd.ExtraFiles, pinWriter)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(max(rows, 1)),
		Cols: uint16(max(cols, 1)),
	})
	_ = pinWriter.Close()
	if err != nil {
		_ = pinReader.Close()
		cleanup()
		return nil, fmt.Errorf("start shell PTY: %w", err)
	}

	session := &shellSession{
		cmd:      cmd,
		pty:      ptmx,
		pinPipe:  pinReader,
		pinInput: bufio.NewReader(pinReader),
		emulator: vt.NewEmulator(max(cols, 1), max(rows, 1)),
		cursor: childCursor{
			visible: true,
			style:   vt.CursorBlock,
			blink:   true,
		},
		title:    "Prelude VT shell spike",
		waitDone: make(chan struct{}),
		cleanup:  cleanup,
	}
	session.emulator.SetScrollbackSize(2_000)
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

func (s *shellSession) ReadPinTarget() (string, error) {
	target, err := s.pinInput.ReadString('\n')
	return strings.TrimSpace(target), err
}

func (s *shellSession) Wait() error {
	<-s.waitDone
	s.waitMu.Lock()
	defer s.waitMu.Unlock()
	return s.waitErr
}

func (s *shellSession) Close() {
	s.close.Do(func() {
		defer s.cleanup()
		_ = s.pinPipe.Close()
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

// starshipShellCommand recreates the prompt initialization performed by
// Prelude's nix develop setup hook. STARSHIP_CONFIG remains untouched: the
// child resolves the same generated configuration as its parent environment.
func starshipShellCommand(shell string, env []string) (*exec.Cmd, func(), error) {
	shellKind := filepath.Base(shell)
	if shellKind != "bash" && shellKind != "zsh" {
		return nil, nil, fmt.Errorf("starship child shell must be bash or zsh, got %q", shell)
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
		// Standalone fallback for a user-installed Starship.
		initCommand := exec.Command(starship, "init", shellKind)
		initCommand.Env = env
		initScript, err = initCommand.Output()
		if err != nil {
			return nil, nil, fmt.Errorf("starship init %s: %w", shellKind, err)
		}
	}
	initScript = append(initScript, vtpinShellFunction...)
	if shellKind == "bash" {
		lineEditing := exec.Command(shell, "-c", "set -o emacs")
		lineEditing.Env = env
		if err := lineEditing.Run(); err != nil {
			return nil, nil, fmt.Errorf("bash has no interactive line editing; enter nix develop interactively before running the spike")
		}
	}

	dir, err := os.MkdirTemp("", "prelude-shell-vt-starship-*")
	if err != nil {
		return nil, nil, fmt.Errorf("starship startup directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	rcName := "bashrc"
	if shellKind == "zsh" {
		rcName = ".zshrc"
	}
	rcPath := filepath.Join(dir, rcName)
	if err := os.WriteFile(rcPath, initScript, 0o600); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("starship startup file: %w", err)
	}

	var cmd *exec.Cmd
	if shellKind == "bash" {
		cmd = exec.Command(shell, "--noprofile", "--rcfile", rcPath, "-i")
	} else {
		env = replaceEnv(env, "ZDOTDIR", dir)
		cmd = exec.Command(shell, "-i")
	}
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
