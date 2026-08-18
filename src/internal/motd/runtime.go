package motd

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// Runtime isolates shell effects used only by Preflight.
// Probe/Check commands remain shell snippets because they are authored as such
// in configuration. Pure Render never uses Runtime.
type Runtime interface {
	// Probe runs a command and returns its first stdout line (env chips).
	Probe(command string) (string, error)
	// Check runs a command for a status badge. ok is true when exit status is 0.
	// output is the first non-empty line of combined stdout/stderr.
	Check(command string) (ok bool, output string)
}

type systemRuntime struct{}

func (systemRuntime) Probe(command string) (string, error) {
	shell, err := lookShell()
	if err != nil {
		return "", err
	}
	output, err := exec.Command(shell, "-c", command).Output()
	if err != nil {
		return "", err
	}
	return firstLine(output), nil
}

func (systemRuntime) Check(command string) (bool, string) {
	return checkCommand(context.Background(), command)
}

func lookShell() (string, error) {
	shell, err := exec.LookPath("bash")
	if err != nil {
		return exec.LookPath("sh")
	}
	return shell, nil
}

// CheckCommandWithTimeout applies a bounded execution window to a configured
// check. It is for detached prompt refreshes, which must not accumulate hung
// child processes across shell prompts.
func CheckCommandWithTimeout(command string, timeout time.Duration) (bool, string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return checkCommand(ctx, command)
}

func checkCommand(ctx context.Context, command string) (bool, string) {
	shell, err := lookShell()
	if err != nil {
		return false, ""
	}
	cmd := exec.CommandContext(ctx, shell, "-c", command)
	// Each configured check gets an isolated group so cancellation includes
	// pipeline and background children rather than only the shell launcher.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	// A deliberately detached child can escape its shell process group while
	// retaining an output pipe. Bound that final pipe wait as well.
	cmd.WaitDelay = 100 * time.Millisecond
	output, err := cmd.CombinedOutput()
	return err == nil, strings.TrimSpace(string(output))
}

func firstLine(output []byte) string {
	line, _, _ := bytes.Cut(output, []byte{'\n'})
	return strings.TrimSpace(string(line))
}
