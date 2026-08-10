package menu

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
)

// justDump is the part of `just --dump --dump-format json` that the menu
// needs. The decoder intentionally ignores additional fields so changes to
// just's dump format do not make the menu unusable.
type justDump struct {
	Recipes map[string]justRecipe `json:"recipes"`
	Modules map[string]justDump   `json:"modules"`
}

type justRecipe struct {
	Doc        *string         `json:"doc"`
	Name       string          `json:"name"`
	Namepath   string          `json:"namepath"`
	Parameters []justParameter `json:"parameters"`
	Private    bool            `json:"private"`
}

type justParameter struct {
	Default *string `json:"default"`
	Flag    bool    `json:"flag"`
	Help    *string `json:"help"`
	Kind    string  `json:"kind"`
	Long    *string `json:"long"`
	Name    string  `json:"name"`
}

type justRecipeEntry struct {
	name   string
	recipe justRecipe
}

// loadJustTasks runs just in the user's current shell directory. It is a
// best-effort import: callers can keep the Nix-generated menu when just is not
// installed, no Justfile is present, or the Justfile cannot be parsed.
func loadJustTasks(cfg JustConfig) ([]Task, error) {
	if !cfg.Enable {
		return nil, nil
	}
	args := []string{"--dump", "--dump-format", "json"}
	if cfg.Justfile != nil && strings.TrimSpace(*cfg.Justfile) != "" {
		args = append([]string{"--justfile", *cfg.Justfile}, args...)
	}

	command := exec.Command("just", args...)
	command.Stderr = io.Discard
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("just dump: %w", err)
	}
	return parseJustDump(output, cfg)
}

func parseJustDump(data []byte, cfg JustConfig) ([]Task, error) {
	var dump justDump
	if err := json.Unmarshal(data, &dump); err != nil {
		return nil, fmt.Errorf("parse just JSON: %w", err)
	}

	entries := make([]justRecipeEntry, 0)
	collectJustRecipes(dump, "", &entries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	tasks := make([]Task, 0, len(entries))
	for _, entry := range entries {
		if entry.recipe.Private || strings.HasPrefix(entry.name, "_") {
			continue
		}
		tasks = append(tasks, justTask(entry.name, entry.recipe, cfg))
	}
	return tasks, nil
}

func collectJustRecipes(module justDump, prefix string, entries *[]justRecipeEntry) {
	for key, recipe := range module.Recipes {
		name := recipe.Namepath
		if name == "" {
			name = recipe.Name
		}
		if name == "" {
			name = key
		}
		if prefix != "" && !strings.Contains(name, "::") {
			name = prefix + "::" + name
		}
		*entries = append(*entries, justRecipeEntry{name: name, recipe: recipe})
	}

	for key, child := range module.Modules {
		childPrefix := key
		if prefix != "" {
			childPrefix = prefix + "::" + key
		}
		collectJustRecipes(child, childPrefix, entries)
	}
}

func justTask(name string, recipe justRecipe, cfg JustConfig) Task {
	group := cfg.Group
	if group == "" {
		group = "just"
	}
	label := name
	if separator := strings.IndexByte(name, ':'); separator > 0 {
		group = name[:separator]
		label = name[separator+1:]
	}

	run := justPrefix(cfg) + " " + shellWord(name)
	args := make([]Arg, 0, len(recipe.Parameters))
	usageArgs := make([]string, 0, len(recipe.Parameters))
	for _, parameter := range recipe.Parameters {
		token := parameter.Name
		if parameter.Flag {
			long := parameter.Name
			if parameter.Long != nil && strings.TrimSpace(*parameter.Long) != "" {
				long = *parameter.Long
			}
			token = "--" + strings.TrimLeft(long, "-")
		}
		description := ""
		if parameter.Help != nil {
			description = *parameter.Help
		}
		required := !parameter.Flag && parameter.Default == nil && parameter.Kind != "star"
		args = append(args, Arg{
			Token:       token,
			Description: description,
			Required:    required,
			Boolean:     parameter.Flag,
		})
		if required {
			usageArgs = append(usageArgs, "<"+parameter.Name+">")
		} else if parameter.Flag {
			usageArgs = append(usageArgs, "["+token+"]")
		} else {
			usageArgs = append(usageArgs, "["+parameter.Name+"]")
		}
	}

	description := ""
	if recipe.Doc != nil {
		description = *recipe.Doc
	}
	usage := run
	if len(usageArgs) > 0 {
		usage += " " + strings.Join(usageArgs, " ")
	}
	return Task{
		Name:        name,
		Label:       label,
		Run:         run,
		Command:     run,
		Description: description,
		Usage:       usage,
		Args:        args,
		group:       group,
	}
}

// mergeJustTasks appends imported recipes to the baked catalogue. Existing
// tasks win by name, so explicit Nix declarations remain the override surface.
func justPrefix(cfg JustConfig) string {
	prefix := "just"
	if cfg.Justfile != nil && strings.TrimSpace(*cfg.Justfile) != "" {
		prefix += " --justfile " + shellWord(*cfg.Justfile)
	}
	return prefix
}

func mergeJustTasks(cfg *Config, tasks []Task) {
	if len(tasks) == 0 {
		return
	}

	existing := make(map[string]struct{})
	groupIndexes := make(map[string]int, len(cfg.Groups))
	for index, group := range cfg.Groups {
		groupIndexes[group.Title] = index
		for _, task := range group.Tasks {
			existing[task.Name] = struct{}{}
		}
	}

	for _, task := range tasks {
		if _, found := existing[task.Name]; found {
			continue
		}
		existing[task.Name] = struct{}{}

		index, found := groupIndexes[taskGroup(task)]
		if !found {
			index = len(cfg.Groups)
			cfg.Groups = append(cfg.Groups, Group{Title: taskGroup(task)})
			groupIndexes[taskGroup(task)] = index
		}
		cfg.Groups[index].Tasks = append(cfg.Groups[index].Tasks, task)
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

func taskGroup(task Task) string {
	if task.group != "" {
		return task.group
	}
	if separator := strings.IndexByte(task.Name, ':'); separator > 0 {
		return task.Name[:separator]
	}
	return "just"
}

func shellWord(value string) string {
	if value != "" {
		safe := true
		for _, character := range value {
			if !((character >= 'a' && character <= 'z') ||
				(character >= 'A' && character <= 'Z') ||
				(character >= '0' && character <= '9') ||
				strings.ContainsRune("_:.-", character)) {
				safe = false
				break
			}
		}
		if safe {
			return value
		}
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
