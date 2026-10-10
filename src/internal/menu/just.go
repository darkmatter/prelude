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
	Attributes []json.RawMessage `json:"attributes"`
	Name       string            `json:"name"`
	Target     string            `json:"target"`
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

// justScope accumulates the public recipes and aliases of one Justfile scope,
// the root Justfile or a module, and the scopes of its submodules. The root's
// entries become top-level tasks and each module one container task.
type justScope struct {
	path    []string // module names from the root Justfile; empty for the root
	recipes []justRecipeEntry
	aliases []justAliasEntry
	modules map[string]*justScope
}

// module returns the scope of the submodule name, creating it on first use.
func (scope *justScope) module(name string) *justScope {
	if scope.modules == nil {
		scope.modules = make(map[string]*justScope)
	}
	module, found := scope.modules[name]
	if !found {
		path := append(scope.path[:len(scope.path):len(scope.path)], name)
		module = &justScope{path: path}
		scope.modules[name] = module
	}
	return module
}

// importJust merges the runtime Justfile recipes into cfg when enabled. A
// failed import keeps the configured catalogue and leaves a visible warning.
func importJust(cfg *Config) {
	if !cfg.Just.Enable {
		return
	}
	if tasks, err := loadJustTasks(cfg.Just, cfg.Root); err == nil {
		mergeTasks(cfg, tasks)
	} else {
		cfg.importWarnings = append(cfg.importWarnings, "just recipes unavailable; check that just and a Justfile are available")
	}
}

// loadJustTasks runs just in dir, the root, or in the current directory when
// dir is empty. It is a best-effort import: callers can keep the Nix-generated
// menu when just is not installed, no Justfile is present, or the Justfile
// cannot be parsed.
func loadJustTasks(cfg JustConfig, dir string) ([]Task, error) {
	if !cfg.Enable {
		return nil, nil
	}
	args := []string{"--dump", "--dump-format", "json"}
	if cfg.Justfile != nil && strings.TrimSpace(*cfg.Justfile) != "" {
		args = append([]string{"--justfile", *cfg.Justfile}, args...)
	}

	command := exec.Command("just", args...)
	command.Dir = dir
	command.Stderr = io.Discard
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("just dump: %w", err)
	}
	return parseJustDump(output, cfg)
}

// parseJustDump projects a just dump onto menu tasks. Root recipes stay
// top-level; a just module becomes one container row whose recipes, and
// submodules as containers of their own, are picked in nested subcommand
// pickers, so a module declutters the menu instead of adding one row per
// recipe. An explicit [group(…)] attribute on a module recipe keeps that
// recipe top-level in the named group — the escape hatch out of the module.
func parseJustDump(data []byte, cfg JustConfig) ([]Task, error) {
	var dump justDump
	if err := json.Unmarshal(data, &dump); err != nil {
		return nil, fmt.Errorf("parse just JSON: %w", err)
	}
	root := &justScope{}
	collectJustTasks(dump, root, root)
	return justScopeTasks(root, cfg), nil
}

// justScopeTasks builds the tasks of one scope, sorted by key: its public
// recipes and aliases, then a container for each submodule with any public
// entry. Below the root, tasks drop the group and show only their own name.
func justScopeTasks(scope *justScope, cfg JustConfig) []Task {
	tasks := make([]Task, 0, len(scope.recipes)+len(scope.aliases)+len(scope.modules))
	for _, entry := range scope.recipes {
		if entry.recipe.Private || strings.HasPrefix(entry.name, "_") {
			continue
		}
		tasks = append(tasks, justTask(entry.name, entry.recipe, cfg))
	}
	for _, entry := range scope.aliases {
		if entry.target.Private || strings.HasPrefix(entry.name, "_") || strings.HasPrefix(entry.target.Name, "_") {
			continue
		}
		tasks = append(tasks, justAliasTask(entry, cfg))
	}
	for _, module := range scope.modules {
		if children := justScopeTasks(module, cfg); len(children) > 0 {
			tasks = append(tasks, justModuleTask(module.path, children, cfg))
		}
	}
	if len(scope.path) > 0 {
		for index := range tasks {
			tasks[index].group = ""
			tasks[index].Label = justLastName(tasks[index].Name)
			tasks[index].justPath = nil
		}
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Name < tasks[j].Name })
	return tasks
}

// collectJustTasks walks one dump scope. Its recipes and aliases join scope,
// and each submodule a nested scope, unless an explicit [group(…)] attribute
// moves a module recipe to top, the root scope. An alias inherits its
// target's placement.
func collectJustTasks(dump justDump, scope *justScope, top *justScope) {
	prefix := strings.Join(scope.path, "::")
	for key, recipe := range dump.Recipes {
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
		owner := scope
		if len(attributeGroups(recipe)) > 0 {
			owner = top
		}
		owner.recipes = append(owner.recipes, justRecipeEntry{name: name, recipe: recipe})
	}

	for key, child := range dump.Modules {
		collectJustTasks(child, scope.module(key), top)
	}

	for key, alias := range dump.Aliases {
		name := alias.Name
		if name == "" {
			name = key
		}
		// Alias targets resolve within their own Justfile scope; a dangling
		// target cannot be invoked, so it never becomes a menu entry.
		target, found := dump.Recipes[alias.Target]
		if !found || aliasPrivate(alias) {
			continue
		}
		if prefix != "" && !strings.Contains(name, "::") {
			name = prefix + "::" + name
		}
		owner := scope
		if len(attributeGroups(target)) > 0 {
			owner = top
		}
		owner.aliases = append(owner.aliases, justAliasEntry{name: name, target: target})
	}
}

// justIdentity places a top-level just task: the configured group, and a
// label of the namepath beneath its root module ("deploy::staging" shows as
// "staging"), which only an explicitly grouped module recipe has.
func justIdentity(name string, cfg JustConfig) (group string, label string) {
	group = cfg.Group
	if group == "" {
		group = "just"
	}
	if separator := strings.Index(name, "::"); separator > 0 {
		return group, name[separator+2:]
	}
	return group, name
}

// justLastName is the last segment of a just namepath: the word that selects
// a module's entry beneath it.
func justLastName(name string) string {
	if separator := strings.LastIndex(name, "::"); separator >= 0 {
		return name[separator+2:]
	}
	return name
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

// aliasPrivate reports whether an alias carries the [private] attribute. The
// dump renders alias attributes as plain strings.
func aliasPrivate(alias justAlias) bool {
	for _, raw := range alias.Attributes {
		var value string
		if err := json.Unmarshal(raw, &value); err == nil && value == "private" {
			return true
		}
	}
	return false
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
	// An explicit `[group('ops')]` attribute overrides the configured default
	// group.
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
		Source:      sourceJust,
		group:       group,
		justPath:    justModulePath(name),
	}
}

func justModulePath(name string) []string {
	if !strings.Contains(name, "::") {
		return nil
	}
	return strings.Split(name, "::")
}

// justModuleTask builds the container row for one just module. Enter opens a
// subcommand picker over its children, and the details pane indexes them. Its
// run is just's dispatch form for the module path (`just e2e sub`), so words
// the menu does not know pass through for just to resolve.
func justModuleTask(path []string, children []Task, cfg JustConfig) Task {
	group := cfg.Group
	if group == "" {
		group = "just"
	}
	words := make([]string, len(path))
	for index, word := range path {
		words[index] = shellWord(word)
	}
	run := justPrefix(cfg) + " " + strings.Join(words, " ")
	var details strings.Builder
	for _, child := range children {
		details.WriteString(child.displayName())
		if child.Description != "" {
			details.WriteString(" — " + child.Description)
		}
		details.WriteString("\n")
	}
	return Task{
		Name:        strings.Join(path, "::"),
		Label:       path[len(path)-1],
		Run:         run,
		Command:     run,
		Description: subcommandsDescription(len(children)),
		Usage:       run + " <subcommand>",
		Details:     strings.TrimRight(details.String(), "\n"),
		Children:    children,
		Source:      sourceJust,
		group:       group,
		justModule:  true,
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
		Source:      sourceJust,
		group:       group,
		justPath:    justModulePath(entry.name),
	}
}

func justAliasTarget(entry justAliasEntry) string {
	return entry.target.Name
}

// justPrefix is how every imported task invokes just; a pinned Justfile
// travels with it.
func justPrefix(cfg JustConfig) string {
	prefix := "just"
	if cfg.Justfile != nil && strings.TrimSpace(*cfg.Justfile) != "" {
		prefix += " --justfile " + shellWord(*cfg.Justfile)
	}
	return prefix
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
