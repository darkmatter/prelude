// prelude-docs: man-style project manual with a CONTENTS sidebar.
//
//	docs --config cfg.json
//
// Content is hand-authored in Nix (prelude.docs.sections); this binary only
// lays out and navigates it. Digits 1–9 jump to sections; j/k scroll; q quits.
package main

import (
	"flag"
	"fmt"
	"os"

	"prelude/manual"
	"prelude/shared"

	tea "charm.land/bubbletea/v2"
)

var defaultConfigPath string

func main() {
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
