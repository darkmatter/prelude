package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

const (
	outputChunk    = 32 * 1024
	finalOutputCap = 4 * outputChunk
	outputPace     = time.Second / 60
	closeGrace     = 300 * time.Millisecond
	writeTimeout   = 250 * time.Millisecond
)

type outputMsg struct {
	data []byte
	err  error
}
type childExitMsg struct{ err error }
type hostFailureMsg struct{ err error }

// Only the reader and waiter run in the background. Neither has access to the
// engine. One queued chunk plus one pending read command bounds noisy output.
type shellProcess struct {
	cmd                 *exec.Cmd
	ptmx                *os.File
	rc                  string
	commandFile         string // private source file owned by the main Bash lifecycle
	completionFile      string // private readline query responses
	completionApplyFile string // separate edit requests/acknowledgements
	output              chan outputMsg
	stop                chan struct{}
	readerDone          chan struct{}
	exited              chan struct{}
	waitErr             error     // published by closing exited
	exitedAt            time.Time // published by closing exited
	tailDrained         bool      // owned by the sequential nextOutput commands
	stopOnce            sync.Once
	closeErr            error
}

// startBash gives readline and foreground programs a real controlling terminal,
// not pipes. Bash still owns editing, command execution, signals, and job control;
// the private rc in prompt.go prints the entry MOTD before installing prompt
// metadata for the host.
func startBash(cols, rows int, env []string, starship bool) (*shellProcess, error) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		return nil, fmt.Errorf("find interactive Bash on PATH: %w", err)
	}
	env = shellEnvironment(env)
	content := bashRC
	if starship {
		path, err := exec.LookPath("starship")
		if err != nil {
			return nil, fmt.Errorf("--starship needs Starship on PATH; run in nix develop path:/home/cm/git/darkmatter/prelude#ghostty-spike or install Starship: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		init := exec.CommandContext(ctx, path, "init", "bash", "--print-full-init")
		init.Env = env
		init.WaitDelay = closeGrace
		script, err := init.Output()
		if err != nil {
			return nil, fmt.Errorf("initialize Starship for private Bash: %w (context: %v)", err, ctx.Err())
		}
		content = bashCommonRC + `if (( BASH_VERSINFO[0] < 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] < 4) )); then
    builtin printf '%s\n' 'ghostty spike: --starship requires Bash >=4.4 (PS0 timing)' >&2
    exit 1
fi
` + string(script) + "\n" + bashStarshipRC
	}
	commandFile, err := os.CreateTemp("", "prelude-ghostty-command-*")
	if err != nil {
		return nil, fmt.Errorf("private Bash command file: %w", err)
	}
	if err := commandFile.Close(); err != nil {
		_ = os.Remove(commandFile.Name())
		return nil, err
	}
	started := false
	defer func() {
		if !started {
			_ = os.Remove(commandFile.Name())
		}
	}()
	completionFile, err := os.CreateTemp("", "prelude-ghostty-completion-*")
	if err != nil {
		return nil, fmt.Errorf("private Bash completion file: %w", err)
	}
	if err := completionFile.Close(); err != nil {
		_ = os.Remove(completionFile.Name())
		return nil, err
	}
	defer func() {
		if !started {
			_ = os.Remove(completionFile.Name())
		}
	}()
	completionApplyFile, err := os.CreateTemp("", "prelude-ghostty-completion-apply-*")
	if err != nil {
		return nil, fmt.Errorf("private Bash completion apply file: %w", err)
	}
	if err := completionApplyFile.Close(); err != nil {
		_ = os.Remove(completionApplyFile.Name())
		return nil, err
	}
	defer func() {
		if !started {
			_ = os.Remove(completionApplyFile.Name())
		}
	}()
	content = "__prelude_ghostty_completion_apply_file=" + shellQuote(completionApplyFile.Name()) + "\n" +
		"__prelude_ghostty_command_file=" + shellQuote(commandFile.Name()) + "\n" +
		"__prelude_ghostty_completion_file=" + shellQuote(completionFile.Name()) + "\n" + content
	rc, err := os.CreateTemp("", "prelude-ghostty-*.bashrc")
	if err != nil {
		return nil, fmt.Errorf("private Bash rc: %w", err)
	}
	_, writeErr := rc.WriteString(content)
	closeErr := rc.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(rc.Name())
		return nil, fmt.Errorf("write private Bash rc: %w", err)
	}
	s, err := startPTY(bash, []string{"--noprofile", "--rcfile", rc.Name(), "-i"}, cols, rows, env, rc.Name())
	if err != nil {
		return nil, err
	}
	s.commandFile, s.completionFile, s.completionApplyFile, started = commandFile.Name(), completionFile.Name(), completionApplyFile.Name(), true
	return s, nil
}

// startPTY owns the same bounded lifecycle for Bash and public surface wrappers.
// rc is an optional cleanup path, not a file loaded by this starter; panes leave
// it empty and execute their wrapper directly, without a private Bash rc.
func startPTY(command string, args []string, cols, rows int, env []string, rc string) (*shellProcess, error) {

	cmd := exec.Command(command, args...)
	cmd.Env = env
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	if err != nil {
		if rc != "" {
			_ = os.Remove(rc)
		}
		return nil, fmt.Errorf("start PTY for %q: %w", command, err)
	}
	s := &shellProcess{
		cmd: cmd, ptmx: ptmx, rc: rc, output: make(chan outputMsg, 1),
		stop: make(chan struct{}), readerDone: make(chan struct{}), exited: make(chan struct{}),
	}
	go func() {
		s.waitErr = cmd.Wait()
		s.exitedAt = time.Now()
		close(s.exited)
	}()
	// NewFile must see O_NONBLOCK when it registers the descriptor with Go's
	// poller. A dup preserves this PTY, makes deadlines work, and lets Close
	// interrupt the reader. Keep CLOEXEC so later panes cannot inherit this
	// master. Never call Fd on the pollable master afterward.
	fd, err := unix.FcntlInt(ptmx.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	if err == nil {
		err = unix.SetNonblock(fd, true)
		if err == nil {
			s.ptmx = os.NewFile(uintptr(fd), ptmx.Name())
			_ = ptmx.Close()
		} else {
			_ = unix.Close(fd)
		}
	}
	if err != nil {
		close(s.readerDone)
		_ = s.close()
		return nil, fmt.Errorf("pollable PTY for %q: %w", command, err)
	}
	go s.read()
	return s, nil
}

// Keep project tools/configuration, but let the private rc own interactive
// startup and prompt state instead of inheriting a parent's shell setup.
func shellEnvironment(env []string) []string {
	result := make([]string, 0, len(env)+3)
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		// Exported project functions remain available, but inherited shell-hook
		// frameworks must not redirect Starship into blehook/bleopt/preexec.
		if strings.HasPrefix(name, "BLE_") || strings.HasPrefix(name, "_ble_") ||
			strings.HasPrefix(name, "BP_") || strings.HasPrefix(name, "__bp_") ||
			strings.HasPrefix(name, "BASH_FUNC_ble") || strings.HasPrefix(name, "BASH_FUNC__ble") ||
			strings.HasPrefix(name, "BASH_FUNC___bp_") || strings.HasPrefix(name, "BASH_FUNC_bash_preexec") ||
			strings.HasPrefix(name, "BASH_FUNC_starship_") || strings.HasPrefix(name, "BASH_FUNC__starship_") ||
			strings.HasPrefix(name, "BASH_FUNC___prelude_ghostty_") {
			continue
		}
		switch name {
		case "TERM", "INPUTRC", "BASH_ENV", "ENV", "PROMPT_COMMAND", "PS0", "PS1", "PS2", "HISTFILE",
			"bash_preexec_imported", "preexec_functions", "precmd_functions", "starship_precmd_user_func",
			"starship_preexec_user_func", "STARSHIP_PROMPT_COMMAND", "STARSHIP_DEBUG_TRAP", "STARSHIP_START_TIME",
			"STARSHIP_END_TIME", "STARSHIP_DURATION", "STARSHIP_CMD_STATUS", "STARSHIP_PIPE_STATUS",
			"STARSHIP_PREEXEC_READY", "STARSHIP_SHELL", "STARSHIP_SESSION_KEY":
			continue
		}
		result = append(result, entry)
	}
	return append(result, "TERM=xterm-256color", "INPUTRC=/dev/null", "HISTFILE=/dev/null")
}

func (s *shellProcess) read() {
	defer close(s.readerDone)
	for {
		// Each queued message owns its bytes. Reusing a read buffer could
		// overwrite output before the UI loop feeds it into Ghostty.
		data := make([]byte, outputChunk)
		n, err := s.ptmx.Read(data)
		if n > 0 || err != nil {
			select {
			case s.output <- outputMsg{data: data[:n], err: err}:
			case <-s.stop:
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// Pace consumption rather than accumulating an output transcript: a busy child
// encounters bounded PTY/channel backpressure instead of growing UI memory.
func (s *shellProcess) nextOutput() tea.Msg {
	timer := time.NewTimer(outputPace)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-s.stop:
		return nil
	}
	select {
	case <-s.exited:
	default:
		select {
		case message := <-s.output:
			return message
		case <-s.stop:
			return nil
		case <-s.exited:
		case <-s.readerDone:
			// The final read is queued before readerDone closes. Drain it
			// before bounding the wait for a child that abandoned its PTY.
			select {
			case message := <-s.output:
				return message
			default:
			}
			return s.waitExit()
		}
	}
	// Drain final output, but do not let a background job holding the slave
	// open keep the host alive after the direct child exits.
	remaining := time.Until(s.exitedAt.Add(closeGrace))
	if remaining <= 0 {
		return s.finalOutput()
	}
	timer.Reset(remaining)
	select {
	case message := <-s.output:
		return message
	case <-timer.C:
		return s.finalOutput()
	case <-s.stop:
		return nil
	}
}

// Take one bounded tail after the grace period, including the queued chunk and
// a reader blocked behind it. Neither a delayed UI nor a continuously writing
// descendant can turn this into an unlimited transcript or extend every read.
func (s *shellProcess) finalOutput() tea.Msg {
	if s.tailDrained {
		return childExitMsg{err: s.waitErr}
	}
	s.tailDrained = true
	deadline := time.Now().Add(outputPace)
	timer := time.NewTimer(outputPace)
	defer timer.Stop()
	var tail outputMsg
	for {
		var message outputMsg
		select {
		case message = <-s.output:
		default:
			select {
			case message = <-s.output:
			case <-timer.C:
				if len(tail.data) == 0 {
					return childExitMsg{err: s.waitErr}
				}
				return tail
			case <-s.stop:
				return nil
			}
		}
		length := min(len(message.data), finalOutputCap-len(tail.data))
		tail.data = append(tail.data, message.data[:length]...)
		tail.err = message.err
		if tail.err != nil || len(tail.data) == finalOutputCap || !time.Now().Before(deadline) {
			return tail
		}
	}
}

func (s *shellProcess) waitExit() tea.Msg {
	timer := time.NewTimer(closeGrace)
	defer timer.Stop()
	select {
	case <-s.exited:
		return childExitMsg{err: s.waitErr}
	case <-s.stop:
		return nil
	case <-timer.C:
		return hostFailureMsg{err: fmt.Errorf("child closed its PTY without exiting within %s", closeGrace)}
	}
}

func (s *shellProcess) send(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if err := s.ptmx.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return fmt.Errorf("PTY write deadline: %w", err)
	}
	n, err := s.ptmx.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		select {
		case <-s.exited:
			return nil // normal exit wins over a simultaneous final reply/input
		default:
		}
		return fmt.Errorf("write PTY: %w", err)
	}
	return nil
}

func (s *shellProcess) resize(cols, rows int) error {
	raw, err := s.ptmx.SyscallConn()
	if err != nil {
		return err
	}
	var ioctlErr error
	err = raw.Control(func(fd uintptr) {
		ioctlErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, &unix.Winsize{Col: uint16(cols), Row: uint16(rows)})
	})
	return errors.Join(err, ioctlErr)
}

func (s *shellProcess) close() error {
	s.stopOnce.Do(func() {
		close(s.stop)
		// Capture the foreground job before closing the master. Bash may be
		// waiting for an editor or command in a separate process group.
		foreground := 0
		if raw, err := s.ptmx.SyscallConn(); err == nil {
			_ = raw.Control(func(fd uintptr) {
				foreground, _ = unix.IoctlGetInt(int(fd), unix.TIOCGPGRP)
			})
		}
		_ = s.ptmx.Close()
		childGroup, hostGroup := s.cmd.Process.Pid, unix.Getpgrp()
		signal := func(sig syscall.Signal) {
			if foreground > 0 && foreground != childGroup && foreground != hostGroup {
				_ = syscall.Kill(-foreground, sig)
			}
			// StartWithSize makes the child a session/group leader. Its ordinary
			// descendants can remain in that group after the leader is reaped.
			if childGroup > 0 && childGroup != hostGroup {
				_ = syscall.Kill(-childGroup, sig)
			} else {
				select {
				case <-s.exited:
				default:
					_ = s.cmd.Process.Signal(sig)
				}
			}
		}
		signal(syscall.SIGHUP)
		select {
		case <-s.exited:
		case <-time.After(closeGrace):
		}
		// Escalate both owned groups even if HUP already exited the direct child.
		signal(syscall.SIGKILL)
		select {
		case <-s.exited:
		case <-time.After(closeGrace):
			s.closeErr = fmt.Errorf("child did not exit after SIGKILL")
		}
		select {
		case <-s.readerDone:
		case <-time.After(closeGrace):
			s.closeErr = errors.Join(s.closeErr, fmt.Errorf("PTY reader did not stop"))
		}
		for _, path := range []string{s.rc, s.commandFile, s.completionFile, s.completionApplyFile} {
			if path != "" {
				if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					s.closeErr = errors.Join(s.closeErr, fmt.Errorf("remove private Bash file: %w", err))
				}
			}
		}
	})
	return s.closeErr
}
