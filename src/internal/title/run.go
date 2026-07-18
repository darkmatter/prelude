package title

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	flags.StringVar(&outputPath, "o", "", "write the generated title to this path instead of stdout")
	flags.StringVar(&outputPath, "output", "", "write the generated title to this path instead of stdout")
	generate := flags.Bool("generate", false, "render without opening the chooser")
	interactive := flags.Bool("interactive", false, "open the chooser even when a terminal is not detected")
	wizard := flags.Bool("wizard", false, "extend the chooser into a setup wizard that prints a ready-to-use prelude config to stdout")
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

// runWizard drives the setup-wizard iteration of the chooser. The TUI renders
// on stderr so stdout stays reserved for the generated config, which makes
// `prelude-title --wizard > prelude.nix` work naturally.
func runWizard(cfg Config, recipe Recipe, outputPath string, force bool) int {
	if len(cfg.Themes) == 0 {
		return fail(errors.New("config contains no themes; rebuild prelude-title from the current module"))
	}
	// The config references the title by path, so the wizard always writes a
	// file; stdout is reserved for the config itself. The default lands in
	// docs/ (next to the starter page) to keep the repo root uncluttered.
	if outputPath == "" {
		outputPath = "docs/title.txt"
	}
	// Validate before the TUI runs: an unrepresentable path would otherwise
	// surface only after the user walked every step.
	if !nixPathLiteralPattern.MatchString(outputPath) {
		return fail(fmt.Errorf("output path %q cannot be written as a Nix path literal (letters, digits, and ./+_- only)", outputPath))
	}
	if !force && !(term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))) {
		return fail(errors.New("the wizard needs an interactive terminal (stdout may still be redirected)"))
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
	return finishWizard(cfg, renderFIGlet, finished.result(), outputPath, os.Stdout, os.Stderr)
}

// finishWizard materializes a completed wizard: the rendered title file, the
// starter docs page when the docs viewer was enabled, and the config on
// stdout. Split from runWizard so the file/emission contract is testable
// without a terminal.
func finishWizard(cfg Config, render renderFunc, result wizardResult, outputPath string, stdout, stderr io.Writer) int {
	index := cfg.fontIndex(result.Recipe.Font)
	if index < 0 {
		return fail(fmt.Errorf("unknown font %q", result.Recipe.Font))
	}
	rendered, err := render(cfg.Fonts[index], result.Recipe.Text)
	if err != nil {
		return fail(err)
	}
	if err := writeAtomic(outputPath, []byte(rendered+"\n")); err != nil {
		return fail(fmt.Errorf("write %s: %w", outputPath, err))
	}
	fmt.Fprintf(stderr, "wrote %s\n", outputPath)

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

	if _, err := io.WriteString(stdout, renderWizardConfig(result, outputPath)); err != nil {
		return fail(fmt.Errorf("write stdout: %w", err))
	}
	return 0
}
