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

// invocationDecision carries exactly one variant: command and line are
// meaningful for commandInvocation, while task is meaningful for every kind —
// the task to collect input for, or the task a command was assembled from.
// Callers must check kind because an empty command is still a valid decision.
type invocationDecision struct {
	kind    invocationKind
	command string
	task    Task
	line    string // argument text appended to task.Run
	// trail is the parents dispatch descended through to reach task, root
	// first, so a picker opened for task sits under theirs.
	trail []Task
}

// selection projects a command decision for library hosts, which dispatch on
// the task and its argument text rather than exec'ing the assembled command.
func (d invocationDecision) selection() *Selection {
	return &Selection{
		Name:       d.task.Name,
		Line:       d.line,
		Command:    d.command,
		Source:     d.task.source(),
		Dir:        d.task.Dir,
		PathPrefix: d.task.PathPrefix,
	}
}

// resolveXInvocation shares the TUI's task assembler. `x a b c …` finds the
// top-level task a, then descends while the next word is a child's label; the
// words left over are the reached task's arguments. A container has none to
// take, so a leftover word there is an unknown command, except under a just
// module, which hands it to just. Just namepaths are not public selectors: an
// explicitly grouped module recipe keeps its module route (`x deploy staging`)
// and wins only where it matches more words than the descent.
func resolveXInvocation(cfg *Config, args []string) (invocationDecision, error) {
	if len(args) == 0 {
		return invocationDecision{}, fmt.Errorf("missing command name")
	}
	top := findXTask(cfg, args[0])
	task, consumed := top, 0
	var trail []Task
	if top != nil {
		consumed = 1
		for consumed < len(args) && args[consumed] != "--" {
			child := findChild(task, args[consumed])
			if child == nil {
				break
			}
			trail = append(trail, *task)
			task = child
			consumed++
		}
	}
	if top == nil || len(top.Children) > 0 {
		if recipe := findModuleRecipe(cfg, args); recipe != nil && len(recipe.justPath) > consumed {
			task, consumed, trail = recipe, len(recipe.justPath), nil
		}
	}
	if task == nil {
		return invocationDecision{}, fmt.Errorf("unknown command %q", args[0])
	}

	extra := args[consumed:]
	if len(extra) > 0 && extra[0] == "--" {
		extra = extra[1:]
	}
	if len(extra) > 0 && task.Run == "" && len(task.Children) > 0 {
		// Name the words as typed up to the first that matched nothing.
		return invocationDecision{}, fmt.Errorf("unknown command %q", strings.Join(args[:consumed+1], " "))
	}
	decision := resolveTaskInvocation(*task, extra)
	decision.trail = trail
	return decision, nil
}

func resolveTaskInvocation(task Task, extra []string) invocationDecision {
	if len(extra) > 0 {
		return commandDecision(task, strings.Join(extra, " "))
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

// findChild returns the subcommand of parent whose label is word.
func findChild(parent *Task, word string) *Task {
	for index := range parent.Children {
		if parent.Children[index].displayName() == word {
			return &parent.Children[index]
		}
	}
	return nil
}

// findModuleRecipe returns the explicitly grouped just module recipe whose
// module route args begin with, the longest when routes share a prefix.
func findModuleRecipe(cfg *Config, args []string) *Task {
	var found *Task
	for groupIndex := range cfg.Groups {
		for taskIndex := range cfg.Groups[groupIndex].Tasks {
			task := &cfg.Groups[groupIndex].Tasks[taskIndex]
			if len(task.justPath) > 0 && routeMatches(task.justPath, args) &&
				(found == nil || len(task.justPath) > len(found.justPath)) {
				found = task
			}
		}
	}
	return found
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

// beginInvocation prepares a task selected in the TUI. A container opens its
// subcommand picker; declaring any arguments opens argument-entry mode;
// otherwise the task, a runnable parent included, is immediately executable.
func beginInvocation(task Task) invocationDecision {
	if task.isContainer() {
		return invocationDecision{kind: collectSubcommandInvocation, task: task}
	}
	if len(task.Args) > 0 {
		return invocationDecision{kind: collectArgumentsInvocation, task: task}
	}
	return commandDecision(task, "")
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
	return commandDecision(task, argumentLine), nil
}

// commandDecision records command presence separately from command contents.
// This preserves empty commands instead of collapsing them into "no action".
func commandDecision(task Task, line string) invocationDecision {
	return invocationDecision{
		kind:    commandInvocation,
		command: assembleInvocation(task, line),
		task:    task,
		line:    line,
	}
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
