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
	Aliases map[string]justAlias  `json:"aliases"`
}

type justRecipe struct {
	Attributes []json.RawMessage `json:"attributes"`
	Doc        *string           `json:"doc"`
	Name       string            `json:"name"`
	Namepath   string            `json:"namepath"`
	Parameters []justParameter   `json:"parameters"`
	Private    bool              `json:"private"`
}

// justAttribute is one structured recipe attribute. Group membership and
// metadata (the examples carrier) are read; both accept a single string or a
// list of strings.
type justAttribute struct {
	Group    json.RawMessage `json:"group"`
	Metadata json.RawMessage `json:"metadata"`
}

type justAlias struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

type justParameter struct {
	Default  *string         `json:"default"`
	Flag     bool            `json:"flag"`
	Help     *string         `json:"help"`
	Kind     string          `json:"kind"`
	Long     *string         `json:"long"`
	Max      *int            `json:"max"`
	Min      *int            `json:"min"`
	Multiple bool            `json:"multiple"`
	Name     string          `json:"name"`
	Pattern  json.RawMessage `json:"pattern"`
	Short    *string         `json:"short"`
	Value    *string         `json:"value"`
}

type justRecipeEntry struct {
	name   string
	recipe justRecipe
}

type justAliasEntry struct {
	name   string
	target justRecipe
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
	aliases := make([]justAliasEntry, 0)
	collectJustTasks(dump, "", &entries, &aliases)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})
	sort.Slice(aliases, func(i, j int) bool {
		return aliases[i].name < aliases[j].name
	})

	tasks := make([]Task, 0, len(entries)+len(aliases))
	for _, entry := range entries {
		if entry.recipe.Private || strings.HasPrefix(entry.name, "_") {
			continue
		}
		tasks = append(tasks, justTask(entry.name, entry.recipe, cfg))
	}
	for _, entry := range aliases {
		if entry.target.Private || strings.HasPrefix(entry.name, "_") || strings.HasPrefix(entry.target.Name, "_") {
			continue
		}
		tasks = append(tasks, justAliasTask(entry, cfg))
	}
	return tasks, nil
}

func collectJustTasks(module justDump, prefix string, entries *[]justRecipeEntry, aliases *[]justAliasEntry) {
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
		collectJustTasks(child, childPrefix, entries, aliases)
	}

	for key, alias := range module.Aliases {
		name := alias.Name
		if name == "" {
			name = key
		}
		if prefix != "" {
			name = prefix + "::" + name
		}
		// Alias targets resolve within their own Justfile scope; a dangling
		// target cannot be invoked, so it never becomes a menu entry.
		target, found := module.Recipes[alias.Target]
		if !found {
			continue
		}
		*aliases = append(*aliases, justAliasEntry{name: name, target: target})
	}
}

// justIdentity splits a just namepath: module recipes ("db::migrate") group
// under the module name; flat names group under cfg.Group.
func justIdentity(name string, cfg JustConfig) (group string, label string) {
	if separator := strings.Index(name, "::"); separator > 0 {
		return name[:separator], name[separator+2:]
	}
	if separator := strings.IndexByte(name, ':'); separator > 0 {
		return name[:separator], name[separator+1:]
	}
	group = cfg.Group
	if group == "" {
		group = "just"
	}
	return group, name
}

// attributeGroups returns a recipe's declared groups in dump order. A recipe
// may carry several group attributes; the first one owns the menu placement.
// Plain-string attributes ([private], [unix]) are skipped — the dump mixes
// them into the same array as structured ones.
func attributeGroups(recipe justRecipe) []string {
	groups := make([]string, 0, len(recipe.Attributes))
	for _, raw := range recipe.Attributes {
		var attribute justAttribute
		if err := json.Unmarshal(raw, &attribute); err != nil {
			continue
		}
		for _, group := range decodeGroupValue(attribute.Group) {
			if group != "" && !justContains(groups, group) {
				groups = append(groups, group)
			}
		}
	}
	return groups
}

// attributeMetadata returns the recipe's `[metadata(...)]` entries — the
// just-native carrier for worked example invocations, rendered by the menu as
// the Examples section.
func attributeMetadata(recipe justRecipe) []string {
	examples := make([]string, 0, len(recipe.Attributes))
	for _, raw := range recipe.Attributes {
		var attribute justAttribute
		if err := json.Unmarshal(raw, &attribute); err != nil {
			continue
		}
		for _, entry := range decodeGroupValue(attribute.Metadata) {
			if entry != "" {
				examples = append(examples, entry)
			}
		}
	}
	if len(examples) == 0 {
		return nil
	}
	return examples
}

// decodeGroupValue accepts the two dump shapes: `"ops"` and `["ops", "admin"]`.
func decodeGroupValue(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err == nil {
		return many
	}
	return nil
}

// patternOptions turns a `[arg(pattern)]` constraint into pickable options
// when it is a plain alternation of literals ("--help|--version"); regex
// metacharacters mean the pattern is not presentable as a fixed choice set.
func patternOptions(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	patterns := decodeGroupValue(raw)
	for _, pattern := range patterns {
		if pattern == "" || strings.ContainsAny(pattern, "()[]{}+*?.^$\\") {
			return nil
		}
	}
	options := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		for _, option := range strings.Split(pattern, "|") {
			if option != "" && !justContains(options, option) {
				options = append(options, option)
			}
		}
	}
	if len(options) < 2 {
		return nil
	}
	return options
}

func justContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func justTask(name string, recipe justRecipe, cfg JustConfig) Task {
	group, label := justIdentity(name, cfg)
	// An explicit `[group('ops')]` attribute overrides both the module
	// namepath and the configured default group.
	if declared := attributeGroups(recipe); len(declared) > 0 {
		group = declared[0]
	}

	run := justPrefix(cfg) + " " + shellWord(name)
	examples := attributeMetadata(recipe)
	args := make([]Arg, 0, len(recipe.Parameters))
	usageArgs := make([]string, 0, len(recipe.Parameters))
	for _, parameter := range recipe.Parameters {
		token := parameter.Name
		long := parameter.Name
		if parameter.Long != nil && strings.TrimSpace(*parameter.Long) != "" {
			long = *parameter.Long
		}
		if parameter.Flag || parameter.Long != nil {
			// Typed `[arg(long…)]` options surface as --long tokens, matching
			// the invocation form just itself parses and validates.
			token = "--" + strings.TrimLeft(long, "-")
		}
		description := ""
		if parameter.Help != nil {
			description = *parameter.Help
		}
		// `min` forces a minimum count, so min > 0 makes the option required.
		required := !parameter.Flag && parameter.Default == nil &&
			parameter.Kind != "star" && (parameter.Min == nil || *parameter.Min > 0)
		args = append(args, Arg{
			Token:       token,
			Description: description,
			Required:    required,
			Boolean:     parameter.Flag,
			Options:     patternOptions(parameter.Pattern),
			Default:     parameter.Default,
		})
		switch {
		case required:
			usageArgs = append(usageArgs, "<"+token+">")
		case parameter.Flag:
			usageArgs = append(usageArgs, "["+token+"]")
		case parameter.Default != nil && *parameter.Default != "":
			usageArgs = append(usageArgs, "["+token+"="+*parameter.Default+"]")
		case parameter.Default != nil:
			usageArgs = append(usageArgs, "["+token+"]")
		case parameter.Kind == "star" || parameter.Multiple:
			usageArgs = append(usageArgs, "["+token+"...]")
		default:
			usageArgs = append(usageArgs, "["+token+"]")
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
		Examples:    examples,
		group:       group,
	}
}

// justAliasTask surfaces `alias cc := container-config` as its own entry: the
// alias keeps the target's group and description, with the target recorded in
// the details pane.
func justAliasTask(entry justAliasEntry, cfg JustConfig) Task {
	group, label := justIdentity(entry.name, cfg)
	if declared := attributeGroups(entry.target); len(declared) > 0 {
		group = declared[0]
	}

	run := justPrefix(cfg) + " " + shellWord(entry.name)
	description := "alias of " + justAliasTarget(entry)
	if entry.target.Doc != nil && strings.TrimSpace(*entry.target.Doc) != "" {
		description = *entry.target.Doc
	}
	return Task{
		Name:        entry.name,
		Label:       label,
		Run:         run,
		Command:     run,
		Description: description,
		Usage:       run,
		Details:     "Alias of " + justAliasTarget(entry),
		group:       group,
	}
}

func justAliasTarget(entry justAliasEntry) string {
	return entry.target.Name
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
