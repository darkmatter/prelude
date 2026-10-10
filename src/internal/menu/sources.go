package menu

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

// openCatalogue settles where cfg reads project files and runs commands, then
// imports from there. A task with no directory of its own runs in the root,
// with the menu's PATH prefix after its own.
//
// Only a bound menu takes PRELUDE_ROOT, so a devshell's menu started from a
// bound menu's command keeps working in its own checkout. A bound menu takes
// it when someone set it for it, or when it handed it down itself (reopened
// through its own `x`), but not when another bound menu handed it on.
func openCatalogue(cfg *Config) {
	root := os.Getenv("PRELUDE_ROOT")
	handedBy := os.Getenv("PRELUDE_ROOT_MENU")
	if root != "" && cfg.Root != "" && (handedBy == "" || handedBy == cfg.path) {
		if absolute, err := filepath.Abs(root); err == nil {
			root = absolute
		}
		cfg.Root = root
	}
	importSources(cfg)
	if cfg.Root != "" || len(cfg.PathPrefix) > 0 {
		for index := range cfg.Groups {
			placeTasks(cfg.Groups[index].Tasks, cfg)
		}
	}
}

func placeTasks(tasks []Task, cfg *Config) {
	for index := range tasks {
		task := &tasks[index]
		if task.Dir == "" {
			task.Dir = cfg.Root
		}
		task.PathPrefix = slices.Concat(task.PathPrefix, cfg.PathPrefix)
		placeTasks(task.Children, cfg)
	}
}

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
	case t.justModule:
		return "just module"
	case t.source() == sourceJust:
		return "just recipe"
	case t.source() == sourceScripts && t.Run == "" && len(t.Children) > 0:
		// A name prefix the scripts share, not a script of its own.
		return "package.json scripts"
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
			owner.hides = append(owner.hides, hiddenEntries(task)...)
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

// hiddenEntries names what a claimed import hides, by each import's own key:
// a name prefix scripts share (`tools` for `tools/status`) is no script of
// its own, so the scripts beneath it are named instead.
func hiddenEntries(task Task) []hiddenEntry {
	var entries []hiddenEntry
	if task.source() != sourceScripts || task.Run != "" || len(task.Children) == 0 {
		entries = append(entries, hiddenEntry{name: task.Name, noun: taskNoun(task)})
	}
	for _, child := range task.Children {
		entries = append(entries, hiddenEntries(child)...)
	}
	return entries
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

// completableWord matches a route word completion can insert bare: the
// characters a declared key's words may use.
var completableWord = regexp.MustCompile(`^[A-Za-z0-9:_.-]+$`)

// oneLine keeps a description on its completion line.
var oneLine = strings.NewReplacer("\t", " ", "\n", " ", "\r", " ")

// writeImports prints the route of every imported node `x` dispatches, at
// every depth, tab-separated from its one-line description, in catalogue
// order. A route is the node's public words joined by spaces (`e2e sub x`).
// Shell completion reads it, so `x <TAB>` offers what the menu imported
// instead of rediscovering it. An explicitly grouped just module recipe is
// reached by its module route (`deploy staging`), not its namepath; a word
// that needs shell quoting (a script named "lint fix") stays in the picker
// but is not offered, and neither is anything beneath it.
func writeImports(w io.Writer, cfg *Config) {
	for _, group := range cfg.Groups {
		for _, task := range group.Tasks {
			if task.source() == sourceDeclared {
				continue
			}
			route := []string{task.Name}
			if len(task.justPath) > 0 {
				route = task.justPath
			}
			writeImportRoutes(w, task, route)
		}
	}
}

// writeImportRoutes prints task's route and then each subcommand's beneath it.
func writeImportRoutes(w io.Writer, task Task, route []string) {
	for _, word := range route {
		if !completableWord.MatchString(word) {
			return
		}
	}
	fmt.Fprintf(w, "%s\t%s\n", strings.Join(route, " "), oneLine.Replace(task.Description))
	for _, child := range task.Children {
		writeImportRoutes(w, child, append(route[:len(route):len(route)], child.displayName()))
	}
}
