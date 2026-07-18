package title

import (
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

//go:embed templates/*.tmpl
var configTemplateFS embed.FS

// Defaults and option surface in the templates mirror src/prelude/defaults.nix
// and src/prelude/options/*.nix — keep those files aligned when options change.

// configData is the view model for the setup-generated Nix templates.
type configData struct {
	Theme         string
	ColorProfile  string
	Project       string
	TitlePath     string // already a Nix path literal
	Motd          bool
	Menu          bool
	Prompt        bool
	Docs          bool
	Commands      []commandData
	PaletteTokens []string
}

// commandData is one prelude.commands entry for the template.
type commandData struct {
	Key          string // Nix attr key (quoted when needed)
	Pad          string // leading indent for the entry block
	Exec         string // empty → emit commented inferred default
	InferredExec string
	Description  string
	MotdOrder    int // 0 → commented null; else active sort order
}

// paletteTokens matches options/shared.nix palette submodule fields.
var paletteTokens = []string{
	"fg", "muted", "dim", "border", "accentBorder", "accent", "accent2",
	"success", "warning", "info", "error", "selectionFg", "bg", "surface", "secondary",
}

var configTemplates = mustParseConfigTemplates()

func mustParseConfigTemplates() *template.Template {
	// Root is built first so FuncMap closures can ExecuteTemplate against it.
	root := template.New("root")
	funcMap := template.FuncMap{
		"nixString": nixString,
		"bool": func(v bool) string {
			if v {
				return "true"
			}
			return "false"
		},
		"include": func(name string, data any) (string, error) {
			var buf bytes.Buffer
			if err := root.ExecuteTemplate(&buf, name, data); err != nil {
				return "", err
			}
			return buf.String(), nil
		},
		"commentLines": commentLines,
		"indent":       indentLines,
	}
	return template.Must(root.Funcs(funcMap).ParseFS(configTemplateFS, "templates/*.tmpl"))
}

// renderWizardConfig emits the ready-to-use config for the collected result:
// a flake-parts module, or standalone prelude.lib builder calls when
// flake-parts was toggled off. titlePath is the path of the rendered
// wordmark relative to the config file (setup always uses sibling title.txt).
func renderWizardConfig(r wizardResult, titlePath string) string {
	data := newConfigData(r, titlePath, r.FlakeParts)
	name := "flake_parts.nix.tmpl"
	if !r.FlakeParts {
		name = "standalone.nix.tmpl"
	}
	var buf bytes.Buffer
	if err := configTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		// Templates are compile-time assets; a runtime failure is a programming error.
		panic(fmt.Sprintf("render setup config %s: %v", name, err))
	}
	return buf.String()
}

func newConfigData(r wizardResult, titlePath string, flakeParts bool) configData {
	pad := "    "
	if flakeParts {
		pad = "      "
	}
	commands := make([]commandData, len(r.Commands))
	for i, command := range r.Commands {
		entry := commandData{
			Key:          nixAttrKey(command.Name),
			Pad:          pad,
			Exec:         command.Exec,
			InferredExec: inferredCommandExec(command.Name),
			Description:  command.Description,
		}
		// Advertise the first few commands on the MOTD Getting Started list.
		if r.Motd && i < 3 {
			entry.MotdOrder = (i + 1) * 100
		}
		commands[i] = entry
	}
	return configData{
		Theme:         r.Theme,
		ColorProfile:  r.ColorProfile,
		Project:       r.Project,
		TitlePath:     nixPath(titlePath),
		Motd:          r.Motd,
		Menu:          r.Menu,
		Prompt:        r.Prompt,
		Docs:          r.Docs,
		Commands:      commands,
		PaletteTokens: paletteTokens,
	}
}

// inferredCommandExec mirrors prelude's exec default: the segment after the
// first colon, or the whole key when ungrouped.
func inferredCommandExec(name string) string {
	if i := strings.IndexByte(name, ':'); i >= 0 && i+1 < len(name) {
		return name[i+1:]
	}
	return name
}

// commentLines prefixes every line with "# " so a whole block can be shipped
// as a commented example (empty lines become "#").
func commentLines(s string) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = "#"
		} else {
			lines[i] = "# " + line
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// indentLines prefixes every line with n spaces.
func indentLines(n int, s string) string {
	if s == "" {
		return ""
	}
	pad := strings.Repeat(" ", n)
	// Preserve a trailing newline without indenting past the final empty split.
	trailing := strings.HasSuffix(s, "\n")
	s = strings.TrimRight(s, "\n")
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	out := strings.Join(lines, "\n")
	if trailing {
		out += "\n"
	}
	return out
}

// nixString quotes a value as a Nix double-quoted string literal.
func nixString(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"${", `\${`,
		"\n", `\n`,
	)
	return `"` + replacer.Replace(value) + `"`
}

// nixAttrKey renders an attrset key, quoting it only when it is not a plain
// Nix identifier (public keys containing colons, such as `test:unit`, need quotes).
func nixAttrKey(name string) string {
	if nixIdentifierPattern.MatchString(name) {
		return name
	}
	return nixString(name)
}

var nixIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_'-]*$`)

// commandKeyPattern accepts public keys such as `test:unit` and
// `test:unit:watch`. The first colon derives menu grouping; the complete key
// remains callable through x.
var commandKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+(:[A-Za-z0-9_.-]+)*$`)

// nixPath emits a Nix path literal. Relative paths gain the mandatory ./
// prefix; absolute paths pass through.
func nixPath(path string) string {
	cleaned := filepath.ToSlash(filepath.Clean(path))
	if strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "./") || strings.HasPrefix(cleaned, "../") {
		return cleaned
	}
	return "./" + cleaned
}

// nixPathLiteralPattern matches the characters Nix accepts in an unquoted
// path literal. Anything else (spaces most commonly) would emit a config
// that fails to parse, so output paths are validated up front.
var nixPathLiteralPattern = regexp.MustCompile(`^[A-Za-z0-9._+/-]+$`)

// starterDocsPath is both written by setup and referenced by the emitted
// config, so the generated module builds without manual steps.
const starterDocsPath = "docs/getting-started.md"

const starterDocsPage = `# Getting started

This page was created by ` + "`prelude setup`" + `.

Every Markdown file listed under ` + "`prelude.docs.pages`" + ` becomes one page in
the ` + "`docs`" + ` viewer; the first heading is the sidebar label. Replace this
page with real onboarding notes and add more pages as the project grows.
`
