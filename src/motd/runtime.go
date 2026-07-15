package main

import (
	"bytes"
	"os/exec"
	"strings"
)

// Runtime isolates the only effectful values in an otherwise deterministic
// renderer. Probe/Check commands remain shell snippets because they are
// authored as such in configuration.
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
	shell, err := lookShell()
	if err != nil {
		return false, ""
	}
	output, err := exec.Command(shell, "-c", command).CombinedOutput()
	return err == nil, strings.TrimSpace(string(output))
}

func lookShell() (string, error) {
	shell, err := exec.LookPath("bash")
	if err != nil {
		return exec.LookPath("sh")
	}
	return shell, nil
}

func firstLine(output []byte) string {
	line, _, _ := bytes.Cut(output, []byte{'\n'})
	return strings.TrimSpace(string(line))
}
