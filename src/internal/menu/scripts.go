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
// put ahead of PATH. Keys stay verbatim, and the first `:` or `/` picks the
// group.
func parseScripts(data []byte, path string, cfg ScriptsConfig) ([]Task, error) {
	var manifest struct {
		Scripts map[string]json.RawMessage `json:"scripts"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	dir := filepath.Dir(path)
	bins := nodeModulesBins(dir)

	tasks := make([]Task, 0, len(manifest.Scripts))
	for name, raw := range manifest.Scripts {
		// Non-string entries are comments by convention ("//": [...]).
		var script string
		if name == "" || json.Unmarshal(raw, &script) != nil {
			continue
		}
		group, label := cfg.Group, name
		if separator := strings.IndexAny(name, ":/"); separator > 0 {
			group, label = name[:separator], name[separator+1:]
		}
		tasks = append(tasks, Task{
			Name:        name,
			Label:       label,
			Run:         script,
			Description: script,
			Usage:       script,
			Source:      sourceScripts,
			Dir:         dir,
			PathPrefix:  bins,
			group:       group,
		})
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Name < tasks[j].Name })
	return tasks, nil
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
