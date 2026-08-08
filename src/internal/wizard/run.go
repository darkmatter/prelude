package wizard

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/term"
)

// Run executes prelude-title and returns a process exit code.
func Run(defaultConfigPath string, args []string) int {
	flags := flag.NewFlagSet("prelude-title", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	configPath := flags.String("config", defaultConfigPath, "path to the generated font config")
	recipePath := flags.String("recipe", "", "optional title recipe used to prefill text and font")
	var outputPath string
	flags.StringVar(&outputPath, "o", "", "title mode: write title here instead of stdout; wizard mode: write config here (default: prelude.nix)")
	flags.StringVar(&outputPath, "output", "", "title mode: write title here instead of stdout; wizard mode: write config here (default: prelude.nix)")
	generate := flags.Bool("generate", false, "render without opening the chooser")
	interactive := flags.Bool("interactive", false, "open the chooser even when a terminal is not detected")
	wizard := flags.Bool("wizard", false, "extend the chooser into a setup wizard that writes a ready-to-use prelude config (and a sibling title.txt)")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: prelude-title [--recipe path] [-o path] [--generate|--interactive|--wizard]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "prelude-title: unexpected argument: %s\n", flags.Arg(0))
		return 2
	}
	if *generate && *interactive {
		fmt.Fprintln(os.Stderr, "prelude-title: --generate and --interactive are mutually exclusive")
		return 2
	}
	if *wizard && *generate {
		fmt.Fprintln(os.Stderr, "prelude-title: --wizard and --generate are mutually exclusive")
		return 2
	}
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "prelude-title: no font config was provided")
		return 1
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		return fail(err)
	}
	recipe, err := initialRecipe(cfg, *recipePath)
	if err != nil {
		return fail(err)
	}

	if *wizard {
		return runWizard(cfg, recipe, outputPath, *interactive)
	}
	useChooser := *interactive || (!*generate && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd())))
	if !useChooser {
		return generateTitle(cfg, recipe, outputPath)
	}

	model := newChooser(cfg, recipe, renderFIGlet)
	final, err := tea.NewProgram(model).Run()
	if err != nil {
		return fail(err)
	}
	chosen, ok := final.(chooserModel)
	if !ok {
		return fail(errors.New("chooser returned an unexpected model"))
	}
	if chosen.canceled || !chosen.done {
		fmt.Fprintln(os.Stderr, "prelude-title: canceled")
		return 130
	}

	return generateTitle(cfg, chosen.selectedRecipe(), outputPath)
}

func initialRecipe(cfg Config, path string) (Recipe, error) {
	if path != "" {
		recipe, err := loadRecipe(path)
		if err != nil {
			return Recipe{}, err
		}
		if cfg.fontIndex(recipe.Font) < 0 {
			return Recipe{}, fmt.Errorf("unknown font %q", recipe.Font)
		}
		return recipe, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return Recipe{}, err
	}
	text := filepath.Base(cwd)
	if text == "." || text == string(filepath.Separator) || text == "" {
		text = "prelude"
	}
	return Recipe{Text: text, Font: cfg.DefaultFont}, nil
}

func generateTitle(cfg Config, recipe Recipe, outputPath string) int {
	index := cfg.fontIndex(recipe.Font)
	if index < 0 {
		return fail(fmt.Errorf("unknown font %q", recipe.Font))
	}
	rendered, err := renderFIGlet(cfg.Fonts[index], recipe.Text)
	if err != nil {
		return fail(err)
	}
	data := []byte(rendered + "\n")
	if outputPath == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			return fail(fmt.Errorf("write stdout: %w", err))
		}
		return 0
	}
	if err := writeAtomic(outputPath, data); err != nil {
		return fail(fmt.Errorf("write %s: %w", outputPath, err))
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", outputPath)
	return 0
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "prelude-title:", err)
	return 1
}

// defaultWizardConfigPath is the setup output. Always a sidecar module —
// never flake.nix — so existing flakes import it instead of being replaced.
const defaultWizardConfigPath = "prelude.nix"

const (
	wizardEnvrcPath     = ".envrc"
	wizardEnvrcContents = "use flake\n"
)

// runWizard drives the setup-wizard iteration of the chooser. The TUI renders
// on stderr. -o selects the config path (default prelude.nix); the FIGlet
// wordmark is written as title.txt next to that file.
func runWizard(cfg Config, recipe Recipe, outputPath string, force bool) int {
	if len(cfg.Themes) == 0 {
		return fail(errors.New("config contains no themes; rebuild prelude-title from the current module"))
	}
	// -o is the config destination. Title always lands beside it as title.txt
	// so the emitted module can reference ./title.txt relative to itself.
	if outputPath == "" {
		outputPath = defaultWizardConfigPath
	}
	if isFlakeNixPath(outputPath) {
		return fail(fmt.Errorf("refusing to write %s — setup emits a separate importable module (default %s) so existing flakes are not overwritten; import it from flake.nix instead", outputPath, defaultWizardConfigPath))
	}
	titlePath := titlePathBesideConfig(outputPath)
	// Validate before the TUI runs: an unrepresentable path would otherwise
	// surface only after the user walked every step.
	if !nixPathLiteralPattern.MatchString(titlePath) {
		return fail(fmt.Errorf("title path %q cannot be written as a Nix path literal (letters, digits, and ./+_- only)", titlePath))
	}
	if !nixPathLiteralPattern.MatchString(outputPath) {
		return fail(fmt.Errorf("config path %q cannot be written as a Nix path literal (letters, digits, and ./+_- only)", outputPath))
	}
	if !force && !(term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))) {
		return fail(errors.New("the wizard needs an interactive terminal"))
	}

	model := newWizard(cfg, recipe, renderFIGlet)
	final, err := tea.NewProgram(model, tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return fail(err)
	}
	finished, ok := final.(wizardModel)
	if !ok {
		return fail(errors.New("wizard returned an unexpected model"))
	}
	if finished.canceled || !finished.done {
		fmt.Fprintln(os.Stderr, "prelude-title: canceled")
		return 130
	}
	return finishWizard(cfg, renderFIGlet, finished.result(), outputPath, os.Stderr)
}

// titlePathBesideConfig returns title.txt in the same directory as configPath.
func titlePathBesideConfig(configPath string) string {
	dir := filepath.Dir(configPath)
	if dir == "." || dir == "" {
		return "title.txt"
	}
	return filepath.ToSlash(filepath.Join(dir, "title.txt"))
}

// finishWizard materializes a completed wizard: the rendered title file beside
// the config, the starter docs page when the docs viewer was enabled, the
// optional project-root .envrc, and the full options-template config at -o.
// Split from runWizard so the file contract is testable without a terminal.
func finishWizard(cfg Config, render renderFunc, result wizardResult, configPath string, stderr io.Writer) int {
	index := cfg.fontIndex(result.Recipe.Font)
	if index < 0 {
		return fail(fmt.Errorf("unknown font %q", result.Recipe.Font))
	}
	rendered, err := render(cfg.Fonts[index], result.Recipe.Text)
	if err != nil {
		return fail(err)
	}
	titlePath := titlePathBesideConfig(configPath)
	if err := writeAtomic(titlePath, []byte(rendered+"\n")); err != nil {
		return fail(fmt.Errorf("write %s: %w", titlePath, err))
	}
	fmt.Fprintf(stderr, "wrote %s\n", titlePath)

	if result.Docs {
		// The emitted config references this page, so create it — but never
		// clobber docs a project already has.
		if _, err := os.Stat(starterDocsPath); errors.Is(err, os.ErrNotExist) {
			if err := writeAtomic(starterDocsPath, []byte(starterDocsPage)); err != nil {
				return fail(fmt.Errorf("write %s: %w", starterDocsPath, err))
			}
			fmt.Fprintf(stderr, "wrote %s\n", starterDocsPath)
		} else {
			fmt.Fprintf(stderr, "kept existing %s\n", starterDocsPath)
		}
	}

	if result.Envrc {
		if err := materializeWizardEnvrc(stderr); err != nil {
			return fail(err)
		}
	}

	// Config always references the sibling title by name so the path is valid
	// relative to the config file, regardless of directory.
	if isFlakeNixPath(configPath) {
		return fail(fmt.Errorf("refusing to write %s — setup emits a separate importable module (default %s)", configPath, defaultWizardConfigPath))
	}
	nixData := renderWizardConfig(result, "title.txt")
	if err := writeAtomic(configPath, []byte(nixData)); err != nil {
		return fail(fmt.Errorf("write %s: %w", configPath, err))
	}
	fmt.Fprintf(stderr, "wrote %s\n", configPath)
	printWizardNextSteps(stderr, configPath, result.Motd)
	return 0
}

// materializeWizardEnvrc installs the default direnv entrypoint in the current
// project directory without replacing an existing file or symlink.
func materializeWizardEnvrc(stderr io.Writer) error {
	if _, err := os.Lstat(wizardEnvrcPath); err == nil {
		fmt.Fprintf(stderr, "kept existing %s\n", wizardEnvrcPath)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect %s: %w", wizardEnvrcPath, err)
	}

	if err := writeAtomic(wizardEnvrcPath, []byte(wizardEnvrcContents)); err != nil {
		return fmt.Errorf("write %s: %w", wizardEnvrcPath, err)
	}
	fmt.Fprintf(stderr, "wrote %s\n", wizardEnvrcPath)
	return nil
}

// isFlakeNixPath reports paths whose base name is flake.nix (with or without
// a directory). Setup must never target these: the product is always a
// sidecar module consumers import.
func isFlakeNixPath(path string) bool {
	return filepath.Base(path) == "flake.nix"
}

// printWizardNextSteps reminds the user how to attach the sidecar module to
// an existing flake without replacing flake.nix. When the MOTD is enabled it
// also prints the direnv log-silencing snippet (direnv noise prints between
// the MOTD and the prompt).
func printWizardNextSteps(stderr io.Writer, configPath string, motdEnabled bool) {
	importPath := configPath
	if !strings.HasPrefix(importPath, "./") && !strings.HasPrefix(importPath, "/") {
		importPath = "./" + importPath
	}
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "Next: import this module from your flake.nix (do not replace flake.nix):")
	fmt.Fprintln(stderr)
	fmt.Fprintf(stderr, "  imports = [\n")
	fmt.Fprintf(stderr, "    inputs.prelude.flakeModules.default\n")
	fmt.Fprintf(stderr, "    %s\n", importPath)
	fmt.Fprintf(stderr, "  ];\n")
	fmt.Fprintln(stderr)
	fmt.Fprintln(stderr, "Add the prelude input if needed, put config.packages.prelude on your")
	fmt.Fprintln(stderr, "devShell, and shellHook = \"motd\"; — see docs/your-own-repo.md.")
	if motdEnabled {
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Tip: direnv log lines can print between the MOTD and your prompt.")
		fmt.Fprintln(stderr, "Silence them once per user account:")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "  mkdir -p ~/.config/direnv && touch ~/.config/direnv/direnv.toml")
		fmt.Fprintln(stderr, "  grep -q 'log_format = \"-\"' ~/.config/direnv/direnv.toml \\")
		fmt.Fprintf(stderr, "    || printf '%%s\\n' '[global]' 'log_format = \"-\"' 'log_filter = \"^$\"' \\\n")
		fmt.Fprintln(stderr, "    >> ~/.config/direnv/direnv.toml")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Pass this along to your users — direnv logging is per-user config.")
	}
}
