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

// justImported is a dump split by menu placement: flat entries stay top-level,
// and per-module entries become one parent task's subcommand children.
type justImported struct {
	flat    []justRecipeEntry
	aliases []justAliasEntry
	modules map[string]*justModule
}

// justModule accumulates one just module's public recipes and aliases.
type justModule struct {
	name    string
	recipes []justRecipeEntry
	aliases []justAliasEntry
}

func (imported *justImported) module(name string) *justModule {
	if imported.modules == nil {
		imported.modules = make(map[string]*justModule)
	}
	module, found := imported.modules[name]
	if !found {
		module = &justModule{name: name}
		imported.modules[name] = module
	}
	return module
}

// moduleRoot returns the first namepath segment: the module that owns a nested
// entry ("e2e::sub::x" belongs to e2e).
func moduleRoot(name string) string {
	if separator := strings.Index(name, "::"); separator > 0 {
		return name[:separator]
	}
	return name
}

// importJust merges the runtime Justfile recipes into cfg when enabled. A
// failed import keeps the configured catalogue and leaves a visible warning.
func importJust(cfg *Config) {
	if !cfg.Just.Enable {
		return
	}
	if tasks, err := loadJustTasks(cfg.Just); err == nil {
		mergeTasks(cfg, tasks)
	} else {
		cfg.importWarnings = append(cfg.importWarnings, "just recipes unavailable; check that just and a Justfile are available")
	}
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

// parseJustDump projects a just dump onto menu tasks. Root recipes stay
// top-level; a just module contributes one parent row whose subcommands are
// picked in a submenu, so a module declutters the menu instead of adding one
// row per recipe. An explicit [group(…)] attribute on a module recipe keeps
// that recipe top-level in the named group — the escape hatch out of the
// submenu.
func parseJustDump(data []byte, cfg JustConfig) ([]Task, error) {
	var dump justDump
	if err := json.Unmarshal(data, &dump); err != nil {
		return nil, fmt.Errorf("parse just JSON: %w", err)
	}

	imported := justImported{}
	collectJustTasks(dump, "", &imported)

	tasks := make([]Task, 0, len(imported.flat)+len(imported.aliases)+len(imported.modules))
	for _, entry := range imported.flat {
		if entry.recipe.Private || strings.HasPrefix(entry.name, "_") {
			continue
		}
		tasks = append(tasks, justTask(entry.name, entry.recipe, cfg))
	}
	for _, entry := range imported.aliases {
		if entry.target.Private || strings.HasPrefix(entry.name, "_") || strings.HasPrefix(entry.target.Name, "_") {
			continue
		}
		tasks = append(tasks, justAliasTask(entry, cfg))
	}
	for _, module := range imported.modules {
		children := make([]Task, 0, len(module.recipes)+len(module.aliases))
		for _, entry := range module.recipes {
			if entry.recipe.Private || strings.HasPrefix(entry.name, "_") {
				continue
			}
			children = append(children, justChildTask(entry.name, entry.recipe, cfg))
		}
		for _, entry := range module.aliases {
			if entry.target.Private || strings.HasPrefix(entry.name, "_") || strings.HasPrefix(entry.target.Name, "_") {
				continue
			}
			children = append(children, justChildAliasTask(entry, cfg))
		}
		if len(children) == 0 {
			continue
		}
		sort.Slice(children, func(i, j int) bool { return children[i].Name < children[j].Name })
		tasks = append(tasks, justParentTask(module.name, children, cfg))
	}

	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Name < tasks[j].Name })
	return tasks, nil
}

// collectJustTasks walks one dump scope (the root Justfile or a module).
// Recipes and aliases of the root scope stay top-level; recipes inside a
// module join that module's parent unless an explicit [group(…)] attribute
// keeps them top-level. An alias inherits its target's placement.
func collectJustTasks(module justDump, prefix string, imported *justImported) {
	for key, recipe := range module.Recipes {
		name := recipe.Namepath
		if name == "" {
			name = recipe.Name
		}
		if name == "" {
			name = key
		}
		if prefix != "" {
			if !strings.Contains(name, "::") {
				name = prefix + "::" + name
			}
			if len(attributeGroups(recipe)) == 0 {
				child := imported.module(moduleRoot(name))
				child.recipes = append(child.recipes, justRecipeEntry{name: name, recipe: recipe})
				continue
			}
		}
		imported.flat = append(imported.flat, justRecipeEntry{name: name, recipe: recipe})
	}

	for key, child := range module.Modules {
		childPrefix := key
		if prefix != "" {
			childPrefix = prefix + "::" + key
		}
		collectJustTasks(child, childPrefix, imported)
	}

	for key, alias := range module.Aliases {
		name := alias.Name
		if name == "" {
			name = key
		}
		// Alias targets resolve within their own Justfile scope; a dangling
		// target cannot be invoked, so it never becomes a menu entry.
		target, found := module.Recipes[alias.Target]
		if !found || aliasPrivate(alias) {
			continue
		}
		if prefix != "" {
			if !strings.Contains(name, "::") {
				name = prefix + "::" + name
			}
			// The alias inherits its target's placement: a grouped target
			// keeps the alias top-level with it.
			if len(attributeGroups(target)) == 0 {
				child := imported.module(moduleRoot(name))
				child.aliases = append(child.aliases, justAliasEntry{name: name, target: target})
				continue
			}
		}
		imported.aliases = append(imported.aliases, justAliasEntry{name: name, target: target})
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

// justChildTask keeps the Just namepath for execution, but the submenu shows
// only the portion beneath the root module.
func justChildTask(name string, recipe justRecipe, cfg JustConfig) Task {
	task := justTask(name, recipe, cfg)
	task.group = ""
	task.haystack = taskHaystack(task)
	return task
}

// justChildAliasTask builds one module subcommand from an alias. Alias-of
// fields carry over from the flat alias shape.
func justChildAliasTask(entry justAliasEntry, cfg JustConfig) Task {
	task := justAliasTask(entry, cfg)
	task.group = ""
	task.haystack = taskHaystack(task)
	return task
}

// justParentTask builds the single menu row for one just module. Enter opens a
// subcommand picker over the children, and the details pane indexes them.
func justParentTask(module string, children []Task, cfg JustConfig) Task {
	group := cfg.Group
	if group == "" {
		group = "just"
	}
	run := justPrefix(cfg) + " " + shellWord(module)
	description := fmt.Sprintf("%d subcommands", len(children))
	if len(children) == 1 {
		description = "1 subcommand"
	}
	var details strings.Builder
	for _, child := range children {
		details.WriteString(child.displayName())
		if child.Description != "" {
			details.WriteString(" — " + child.Description)
		}
		details.WriteString("\n")
	}
	return Task{
		Name:        module,
		Label:       module,
		Run:         run,
		Command:     run,
		Description: description,
		Usage:       run + " <subcommand>",
		Details:     strings.TrimRight(details.String(), "\n"),
		Children:    children,
		Source:      sourceJust,
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
