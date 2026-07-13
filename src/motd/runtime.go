package main

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

// Runtime isolates the only effectful values in an otherwise deterministic
// renderer. Probe commands remain shell snippets because they are authored as
// such in configuration; the UI itself is never assembled or rendered by a
// generated shell script.
type Runtime interface {
	Probe(command string) (string, error)
	Git() (GitInfo, bool)
}

type GitInfo struct {
	Branch string
	Ahead  int
	Dirty  int
}

type systemRuntime struct{}

func (systemRuntime) Probe(command string) (string, error) {
	shell, err := exec.LookPath("bash")
	if err != nil {
		shell, err = exec.LookPath("sh")
		if err != nil {
			return "", err
		}
	}
	output, err := exec.Command(shell, "-c", command).Output()
	if err != nil {
		return "", err
	}
	return firstLine(output), nil
}

func (systemRuntime) Git() (GitInfo, bool) {
	if _, err := exec.LookPath("git"); err != nil {
		return GitInfo{}, false
	}
	inside, err := gitOutput("rev-parse", "--is-inside-work-tree")
	if err != nil || inside != "true" {
		return GitInfo{}, false
	}

	branch, err := gitOutput("symbolic-ref", "--short", "HEAD")
	if err != nil {
		branch, err = gitOutput("rev-parse", "--short", "HEAD")
		if err != nil {
			branch = "?"
		}
	}

	dirty := 0
	if status, statusErr := exec.Command("git", "status", "--porcelain").Output(); statusErr == nil {
		dirty = len(bytes.FieldsFunc(status, func(r rune) bool { return r == '\n' }))
	}

	ahead := 0
	if value, aheadErr := gitOutput("rev-list", "--count", "@{upstream}..HEAD"); aheadErr == nil {
		ahead, _ = strconv.Atoi(value)
	}
	return GitInfo{Branch: branch, Ahead: ahead, Dirty: dirty}, true
}

func gitOutput(args ...string) (string, error) {
	output, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func firstLine(output []byte) string {
	line, _, _ := bytes.Cut(output, []byte{'\n'})
	return strings.TrimSpace(string(line))
}
