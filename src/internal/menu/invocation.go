package menu

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
	collectSubcommandInvocation
)

// invocationDecision carries exactly one variant: command is meaningful for
// commandInvocation, while task is meaningful for collectArgumentsInvocation
// and collectSubcommandInvocation. Callers must check kind because an empty
// command is still a valid decision.
type invocationDecision struct {
	kind    invocationKind
	command string
	task    Task
}

// resolveXInvocation shares the TUI's task assembler. Imported module recipes
// use a space-separated path; their Just namepaths are not public selectors.
func resolveXInvocation(cfg *Config, args []string) (invocationDecision, error) {
	if len(args) == 0 {
		return invocationDecision{}, fmt.Errorf("missing command name")
	}
	name := args[0]
	task := findXTask(cfg, name)
	if task == nil || len(task.Children) > 0 {
		if child, consumed := findModuleRecipe(cfg, args); child != nil {
			extra := args[consumed:]
			if len(extra) > 0 && extra[0] == "--" {
				extra = extra[1:]
			}
			return resolveTaskInvocation(*child, extra), nil
		}
	}
	if task == nil {
		return invocationDecision{}, fmt.Errorf("unknown command %q", name)
	}
	extra := args[1:]
	if len(extra) > 0 && extra[0] == "--" {
		extra = extra[1:]
	}
	// Unknown subcommands retain Just's native module-dispatch form.
	if len(task.Children) > 0 && len(extra) > 0 {
		if child, consumed := findSubcommand(*task, extra); child != nil {
			return resolveTaskInvocation(*child, extra[consumed:]), nil
		}
	}
	return resolveTaskInvocation(*task, extra), nil
}

func resolveTaskInvocation(task Task, extra []string) invocationDecision {
	if len(extra) > 0 {
		return commandDecision(assembleInvocation(task, strings.Join(extra, " ")))
	}
	return beginInvocation(task)
}

// findXTask resolves the public x selector. Exact catalogue name always wins;
// single-key accelerators are a second pass so a key never shadows a name.
// Imported module namepaths are internal, including explicitly grouped recipes.
func findXTask(cfg *Config, name string) *Task {
	for groupIndex := range cfg.Groups {
		for taskIndex := range cfg.Groups[groupIndex].Tasks {
			task := &cfg.Groups[groupIndex].Tasks[taskIndex]
			if task.Name == name && len(task.justPath) == 0 {
				return task
			}
		}
	}
	for groupIndex := range cfg.Groups {
		for taskIndex := range cfg.Groups[groupIndex].Tasks {
			task := &cfg.Groups[groupIndex].Tasks[taskIndex]
			if task.Key != "" && task.Key == name {
				return task
			}
		}
	}
	return nil
}

func findModuleRecipe(cfg *Config, args []string) (*Task, int) {
	for gi := range cfg.Groups {
		for ti := range cfg.Groups[gi].Tasks {
			task := &cfg.Groups[gi].Tasks[ti]
			if len(task.justPath) > 0 && routeMatches(task.justPath, args) {
				return task, len(task.justPath)
			}
			if len(args) > 1 && task.Name == args[0] {
				if child, consumed := findSubcommand(*task, args[1:]); child != nil {
					return child, consumed + 1
				}
			}
		}
	}
	return nil, 0
}

func routeMatches(path, args []string) bool {
	if len(args) < len(path) {
		return false
	}
	for i, part := range path {
		if part != args[i] {
			return false
		}
	}
	return true
}

func findSubcommand(parent Task, args []string) (*Task, int) {
	for index := range parent.Children {
		child := &parent.Children[index]
		path := child.justPath
		if len(path) > 1 {
			path = path[1:]
		} else {
			path = []string{child.Label}
		}
		if routeMatches(path, args) {
			return child, len(path)
		}
	}
	return nil, 0
}

// beginInvocation prepares a task selected in the TUI. Subcommand parents open
// the subcommand picker; declaring any arguments opens argument-entry mode;
// otherwise the task is immediately executable.
func beginInvocation(task Task) invocationDecision {
	if len(task.Children) > 0 {
		return invocationDecision{kind: collectSubcommandInvocation, task: task}
	}
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
