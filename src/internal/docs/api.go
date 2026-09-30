package docs

// Library entry points for hosts that show docs in their own process
// (libprelude, which the TypeScript API loads through bun:ffi). A host passes
// pages inline as Markdown instead of a Nix bundle directory; any relative
// markdownFile paths resolve against the working directory.

import (
	tea "charm.land/bubbletea/v2"

	"prelude/pkg/manual"
	"prelude/pkg/shared"
)

// ParseConfig decodes a docs config held in memory.
func ParseConfig(raw []byte) (*Config, error) {
	return parseConfig(raw, "")
}

// View runs the full-screen docs viewer until the user quits.
func View(cfg *Config) error {
	options := []tea.ProgramOption{}
	if profile, ok := shared.ConfiguredColorProfile(cfg.ColorProfile); ok {
		options = append(options, tea.WithColorProfile(profile))
	}
	_, err := tea.NewProgram(manual.New(manualDocument(cfg), cfg.Palette), options...).Run()
	return err
}

// RenderPage renders one page (1-based, depth-first through the nav) for a
// terminal cols wide, exactly as `docs <page>` prints it, and reports how many
// pages the document has.
func RenderPage(cfg *Config, page, cols int) (lines []string, pages int, err error) {
	document := manualDocument(cfg)
	lines, err = manual.RenderLeafLines(document, cfg.Palette, page, wrapWidth(cols))
	return lines, manual.LeafCount(document), err
}
