package main

import (
	"fmt"
	"strings"
)

// invocationKind is the closed set of decisions the adapters can act on.
// The zero value is invalid so an error can never be mistaken for a command.
type invocationKind uint8

const (
	invalidInvocation invocationKind = iota
	commandInvocation
	collectArgumentsInvocation
)

// invocationDecision carries exactly one variant: command is meaningful for
// commandInvocation, while task is meaningful for collectArgumentsInvocation.
// Callers must check kind because an empty command is still a valid decision.
type invocationDecision struct {
	kind    invocationKind
	command string
	task    Task
}

// resolveInvocation prepares a direct CLI selection. The length check is
// intentional: an explicitly supplied empty argv element still counts as an
// argument and must bypass interactive argument collection. Argument strings
// are joined without quoting because Prelude's existing contract is shell text,
// not an argv-preserving process invocation.
func resolveInvocation(cfg *Config, selector string, extra []string) (invocationDecision, error) {
	task := findInvocationTask(cfg, selector)
	if task == nil {
		return invocationDecision{}, fmt.Errorf("unknown task %q", selector)
	}
	if len(extra) > 0 {
		return commandDecision(assembleInvocation(*task, strings.Join(extra, " "))), nil
	}
	return beginInvocation(*task), nil
}

// beginInvocation prepares a task selected in the TUI. Declaring any arguments
// opens argument-entry mode; otherwise the task is immediately executable.
func beginInvocation(task Task) invocationDecision {
	if len(task.Args) > 0 {
		return invocationDecision{kind: collectArgumentsInvocation, task: task}
	}
	return commandDecision(assembleInvocation(task, ""))
}

// completeInvocation validates and assembles text submitted from argument-entry
// mode. The text remains opaque shell source: declarations only require that
// wholly blank input is rejected, and the first required token wins.
func completeInvocation(task Task, argumentLine string) (invocationDecision, error) {
	argumentLine = strings.TrimSpace(argumentLine)
	if argumentLine == "" {
		for _, arg := range task.Args {
			if arg.Required {
				return invocationDecision{}, fmt.Errorf(
					"%s: missing required argument %s",
					task.Name,
					arg.Token,
				)
			}
		}
	}
	return commandDecision(assembleInvocation(task, argumentLine)), nil
}

// commandDecision records command presence separately from command contents.
// This preserves empty commands instead of collapsing them into "no action".
func commandDecision(command string) invocationDecision {
	return invocationDecision{kind: commandInvocation, command: command}
}

// findInvocationTask uses two full passes so an exact name in any group always
// outranks a key in any group. Combining the checks per group would make group
// order incorrectly affect name-versus-key precedence.
func findInvocationTask(cfg *Config, selector string) *Task {
	for groupIndex := range cfg.Groups {
		for taskIndex := range cfg.Groups[groupIndex].Tasks {
			task := &cfg.Groups[groupIndex].Tasks[taskIndex]
			if task.Name == selector {
				return task
			}
		}
	}
	for groupIndex := range cfg.Groups {
		for taskIndex := range cfg.Groups[groupIndex].Tasks {
			task := &cfg.Groups[groupIndex].Tasks[taskIndex]
			if task.Key != "" && task.Key == selector {
				return task
			}
		}
	}
	return nil
}

// assembleInvocation trims only the complete command. Interactive callers pass
// normalized text, while direct CLI callers pass the raw one-space argv join;
// avoiding an inner trim preserves spaces contained in explicit CLI arguments.
func assembleInvocation(task Task, argumentLine string) string {
	if argumentLine == "" {
		return strings.TrimSpace(task.Run)
	}
	return strings.TrimSpace(task.Run + " " + argumentLine)
}

// invocationToken converts a suggested value into the shell text inserted by a
// chip: booleans insert their flag, long options insert flag plus value, and
// positional arguments insert only the value.
func invocationToken(arg Arg, option string) string {
	if arg.Boolean {
		return arg.Token
	}
	if strings.HasPrefix(arg.Token, "--") {
		return arg.Token + " " + option
	}
	return option
}
