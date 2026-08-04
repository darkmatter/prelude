package main

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

const livePanelCloseGrace = 300 * time.Millisecond

// livePanelSession owns an interactive pinned command exactly as shellSession
// owns the child shell: process, PTY, virtual terminal, replies, and cleanup.
// Neither child ever writes directly to the physical terminal.
type livePanelSession struct {
	cmd      *exec.Cmd
	pty      *os.File
	emulator *vt.Emulator
	replies  *terminalReplyPump
	cursor   childCursor

	waitDone chan struct{}
	waitMu   sync.Mutex
	waitErr  error
	close    sync.Once
}

func startLivePanelCommand(name string, args []string, cols, rows int, env []string) (*livePanelSession, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil, fmt.Errorf("%s not on PATH", name)
	}

	cols = max(cols, 1)
	rows = max(rows, 1)
	cmd := exec.Command(path, args...)
	cmd.Env = filterEnv(env, "PRELUDE_MOTD_PURE")
	cmd.Env = replaceEnv(cmd.Env, "TERM", envValue(cmd.Env, "TERM", "xterm-256color"))
	cmd.Env = replaceEnv(cmd.Env, "COLUMNS", fmt.Sprintf("%d", cols))
	cmd.Env = replaceEnv(cmd.Env, "LINES", fmt.Sprintf("%d", rows))

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		return nil, fmt.Errorf("start %s PTY: %w", name, err)
	}

	session := &livePanelSession{
		cmd:      cmd,
		pty:      ptmx,
		emulator: vt.NewEmulator(cols, rows),
		cursor: childCursor{
			visible: true,
			style:   vt.CursorBlock,
			blink:   true,
		},
		waitDone: make(chan struct{}),
	}
	session.emulator.SetDefaultForegroundColor(pinForeground)
	session.emulator.SetDefaultBackgroundColor(pinBackground)
	session.emulator.SetCallbacks(vt.Callbacks{
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

func (session *livePanelSession) Resize(cols, rows int) error {
	cols = max(cols, 1)
	rows = max(rows, 1)
	session.emulator.Resize(cols, rows)
	if err := pty.Setsize(session.pty, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}); err != nil {
		return fmt.Errorf("resize panel PTY: %w", err)
	}
	return nil
}

func (session *livePanelSession) SendKey(key uv.KeyPressEvent) { session.emulator.SendKey(key) }
func (session *livePanelSession) SendText(text string)         { session.emulator.SendText(text) }
func (session *livePanelSession) Paste(text string)            { session.emulator.Paste(text) }
func (session *livePanelSession) Focus()                       { session.emulator.Focus() }
func (session *livePanelSession) Blur()                        { session.emulator.Blur() }
func (session *livePanelSession) SendMouse(event vt.Mouse)     { session.emulator.SendMouse(event) }

func (session *livePanelSession) Wait() error {
	<-session.waitDone
	session.waitMu.Lock()
	defer session.waitMu.Unlock()
	return session.waitErr
}

func (session *livePanelSession) Close() {
	if session == nil {
		return
	}
	session.close.Do(func() {
		_ = session.pty.Close()
		session.replies.Stop()
		_ = session.emulator.Close()

		select {
		case <-session.waitDone:
			return
		default:
		}
		if session.cmd.Process != nil {
			_ = session.cmd.Process.Signal(syscall.SIGHUP)
		}
		select {
		case <-session.waitDone:
		case <-time.After(livePanelCloseGrace):
			if session.cmd.Process != nil {
				_ = session.cmd.Process.Kill()
			}
			<-session.waitDone
		}
	})
}
