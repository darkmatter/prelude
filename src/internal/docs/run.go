package docs

import (
	"flag"
	"fmt"
	"os"

	"prelude/pkg/manual"
	"prelude/pkg/shared"

	tea "charm.land/bubbletea/v2"
)

// Run is the binary entry point. defaultConfigPath is injected by Nix at link
// time via ldflags; it acts as the fallback when PRELUDE_DOCS_CONFIG is unset.
func Run(defaultConfigPath string) {
	configPathDefault := os.Getenv("PRELUDE_DOCS_CONFIG")
	if configPathDefault == "" {
		configPathDefault = defaultConfigPath
	}
	configPath := flag.String("config", configPathDefault, "path to the docs config JSON")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "docs:", err)
		os.Exit(1)
	}

	if args := flag.Args(); len(args) > 0 {
		env := printEnv{
			statePath:  defaultPagerStatePath(),
			configPath: *configPath,
			stdout:     os.Stdout,
			stderr:     os.Stderr,
			environ:    os.Environ(),
		}
		if err := runPrint(cfg, args, env); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	viewer := manual.New(manualDocument(cfg), cfg.Palette)

	options := []tea.ProgramOption{}
	if profile, ok := shared.ConfiguredColorProfile(cfg.ColorProfile); ok {
		options = append(options, tea.WithColorProfile(profile))
	}
	p := tea.NewProgram(viewer, options...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "docs:", err)
		os.Exit(1)
	}
}
