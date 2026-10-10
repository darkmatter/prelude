package menu

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// Sources say where a task came from. Declared tasks arrive in the Config;
// each import adds its own source's tasks when the menu opens.
const (
	sourceDeclared = "declared"
	sourceJust     = "just"
	sourceScripts  = "scripts"
)

// importSources merges every enabled import into cfg in precedence order, so
// a Justfile recipe keeps a name that a package.json script also uses.
func importSources(cfg *Config) {
	importJust(cfg)
	importScripts(cfg)
}

// source names where t came from; the Config leaves declared tasks unmarked.
func (t Task) source() string {
	if t.Source == "" {
		return sourceDeclared
	}
	return t.Source
}

// taskNoun says what kind of entry t is in its source's terms, for the notes
// that report one entry hiding another.
func taskNoun(t Task) string {
	switch {
	case t.source() == sourceJust && len(t.Children) > 0:
		return "just module"
	case t.source() == sourceJust:
		return "just recipe"
	case t.source() == sourceScripts:
		return "package.json script"
	default:
		return "declared command"
	}
}

// hiddenEntry is an import that lost its key to a task merged before it.
type hiddenEntry struct {
	name string // the import's own key
	noun string // what it is in its source's terms
}

// mergeTasks adds one source's tasks to the catalogue. Sources merge in
// precedence order (declared, just, scripts), so a key stays with the task
// that claimed it first; that task records what it hid, for `x --list` and
// the picker's details. Each new task joins its group, or a new group after
// the existing ones, and every group is re-sorted by display name.
func mergeTasks(cfg *Config, tasks []Task) {
	if len(tasks) == 0 {
		return
	}

	type position struct{ group, task int }
	claimed := make(map[string]position)
	groupIndexes := make(map[string]int, len(cfg.Groups))
	for groupIndex, group := range cfg.Groups {
		groupIndexes[group.Title] = groupIndex
		for taskIndex, task := range group.Tasks {
			at := position{groupIndex, taskIndex}
			claimed[task.Name] = at
			// `x <word>` tries names before shortcuts, so an import named
			// like a declared shortcut would take it over.
			if task.Key != "" {
				claimed[task.Key] = at
			}
			if task.source() == sourceDeclared {
				// Children claim their mounted key and scoped argv routes;
				// keep hidden-import notes on the owning parent row.
				for _, child := range task.Children {
					claimed[child.Name] = at
					for _, parent := range []string{task.Name, task.Key} {
						if parent == "" {
							continue
						}
						for _, name := range []string{child.Label, child.Key} {
							if name != "" {
								claimed[parent+" "+name] = at
							}
						}
					}
				}
			}
		}
	}

	for _, task := range tasks {
		if at, found := claimed[task.Name]; found {
			owner := &cfg.Groups[at.group].Tasks[at.task]
			owner.hides = append(owner.hides, hiddenEntry{name: task.Name, noun: taskNoun(task)})
			continue
		}
		index, found := groupIndexes[task.group]
		if !found {
			index = len(cfg.Groups)
			cfg.Groups = append(cfg.Groups, Group{Title: task.group})
			groupIndexes[task.group] = index
		}
		cfg.Groups[index].Tasks = append(cfg.Groups[index].Tasks, task)
		claimed[task.Name] = position{index, len(cfg.Groups[index].Tasks) - 1}
	}

	for index := range cfg.Groups {
		sort.SliceStable(cfg.Groups[index].Tasks, func(i, j int) bool {
			left, right := cfg.Groups[index].Tasks[i], cfg.Groups[index].Tasks[j]
			if left.displayName() == right.displayName() {
				return left.Name < right.Name
			}
			return left.displayName() < right.displayName()
		})
	}
}

// hiddenNotes describes every import a task hid, in catalogue order. An
// import hidden by a shortcut names the command that owns the shortcut.
func hiddenNotes(cfg *Config) []string {
	var notes []string
	for _, group := range cfg.Groups {
		for _, task := range group.Tasks {
			for _, hidden := range task.hides {
				owner := taskNoun(task)
				if hidden.name != task.Name {
					owner += " " + task.Name
				}
				notes = append(notes, fmt.Sprintf("%s: %s hidden by %s", hidden.name, hidden.noun, owner))
			}
		}
	}
	return notes
}

// completableKey matches the keys completion can insert as one bare word:
// the characters a declared key may use.
var completableKey = regexp.MustCompile(`^[A-Za-z0-9:/_.-]+$`)

// writeImports prints each imported key `x` dispatches, tab-separated from
// its one-line description, in catalogue order. Shell completion reads it, so
// `x <TAB>` offers what the menu imported instead of rediscovering it. A just
// module recipe shown in its own group is reached through its module, not its
// namepath, so only the module is offered; a key that needs shell quoting
// (a script named "lint fix") stays in the picker but is not offered.
func writeImports(w io.Writer, cfg *Config) {
	oneLine := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")
	for _, group := range cfg.Groups {
		for _, task := range group.Tasks {
			if task.source() == sourceDeclared || len(task.justPath) > 0 || !completableKey.MatchString(task.Name) {
				continue
			}
			fmt.Fprintf(w, "%s\t%s\n", task.Name, oneLine.Replace(task.Description))
		}
	}
}
