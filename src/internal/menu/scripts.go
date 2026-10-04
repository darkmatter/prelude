package menu

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// importScripts merges package.json scripts into cfg when enabled. A failed
// import keeps the rest of the catalogue and leaves a visible warning.
func importScripts(cfg *Config) {
	if !cfg.Scripts.Enable {
		return
	}
	tasks, err := loadScriptTasks(cfg.Scripts)
	if err != nil {
		cfg.importWarnings = append(cfg.importWarnings, "package.json scripts unavailable: "+err.Error())
		return
	}
	mergeTasks(cfg, tasks)
}

func loadScriptTasks(cfg ScriptsConfig) ([]Task, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	path, err := packageJSONPath(cfg, cwd)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseScripts(data, path, cfg)
}

// packageJSONPath resolves the package.json to import from the working
// directory: the configured path, else the nearest one at or above it.
func packageJSONPath(cfg ScriptsConfig, cwd string) (string, error) {
	if cfg.PackageJSON != nil && strings.TrimSpace(*cfg.PackageJSON) != "" {
		path := *cfg.PackageJSON
		if filepath.IsAbs(path) {
			return path, nil
		}
		base := cwd
		if root, found := nearestWith(cwd, "flake.nix"); found {
			base = root
		}
		return filepath.Join(base, path), nil
	}
	dir, found := nearestWith(cwd, "package.json")
	if !found {
		return "", fmt.Errorf("no package.json in %s or above", cwd)
	}
	return filepath.Join(dir, "package.json"), nil
}

// nearestWith returns dir, or the nearest directory above it, that holds name.
func nearestWith(dir, name string) (string, bool) {
	for {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// parseScripts projects a package.json's scripts onto menu tasks. Each runs
// exactly as written, with no package manager in front: from the
// package.json's directory, with the node_modules/.bin directories npm would
// put ahead of PATH. Every script lands in the configured group, and a `/`
// nests it under the scripts sharing its prefix (`db/migrate` is `migrate`
// under `db`); `:` and spaces are ordinary name characters.
func parseScripts(data []byte, path string, cfg ScriptsConfig) ([]Task, error) {
	var manifest struct {
		Scripts map[string]json.RawMessage `json:"scripts"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	dir := filepath.Dir(path)
	bins := nodeModulesBins(dir)

	root := &scriptNode{}
	for name, raw := range manifest.Scripts {
		// Non-string entries are comments by convention ("//": [...]).
		var script string
		if name == "" || json.Unmarshal(raw, &script) != nil {
			continue
		}
		node := root.descend(scriptPath(name))
		node.task = &Task{
			Name:        name,
			Run:         script,
			Description: script,
			Usage:       script,
			Source:      sourceScripts,
			Dir:         dir,
			PathPrefix:  bins,
		}
	}
	tasks := root.tasks()
	for index := range tasks {
		tasks[index].group = cfg.Group
	}
	return tasks, nil
}

// scriptPath splits a script name into its words at each `/`. A name with an
// empty word (`/x`, `a//b`, `x/`) has no parent to nest under, so it stays
// one word.
func scriptPath(name string) []string {
	path := strings.Split(name, "/")
	for _, word := range path {
		if word == "" {
			return []string{name}
		}
	}
	return path
}

// scriptNode is one word of the scripts tree: the script named by the path to
// it, if there is one, and the words below it.
type scriptNode struct {
	path     []string
	task     *Task
	children map[string]*scriptNode
}

// descend returns the node at path below n, creating the missing ones.
func (n *scriptNode) descend(path []string) *scriptNode {
	node := n
	for _, word := range path {
		if node.children == nil {
			node.children = make(map[string]*scriptNode)
		}
		child, found := node.children[word]
		if !found {
			child = &scriptNode{path: append(node.path[:len(node.path):len(node.path)], word)}
			node.children[word] = child
		}
		node = child
	}
	return node
}

// tasks builds the tasks below n, sorted by label then name. A word no script
// names itself becomes a container that only opens the scripts beneath it,
// described by how many there are.
func (n *scriptNode) tasks() []Task {
	if len(n.children) == 0 {
		return nil
	}
	tasks := make([]Task, 0, len(n.children))
	for _, child := range n.children {
		task := Task{Name: strings.Join(child.path, "/"), Source: sourceScripts}
		if child.task != nil {
			task = *child.task
		}
		task.Label = child.path[len(child.path)-1]
		task.Children = child.tasks()
		if child.task == nil {
			task.Description = subcommandsDescription(len(task.Children))
		}
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Label != tasks[j].Label {
			return tasks[i].Label < tasks[j].Label
		}
		return tasks[i].Name < tasks[j].Name
	})
	return tasks
}

// nodeModulesBins lists node_modules/.bin in dir and in every directory above
// it, nearest first: the PATH entries npm gives a script.
func nodeModulesBins(dir string) []string {
	var bins []string
	for {
		bins = append(bins, filepath.Join(dir, "node_modules", ".bin"))
		parent := filepath.Dir(dir)
		if parent == dir {
			return bins
		}
		dir = parent
	}
}
