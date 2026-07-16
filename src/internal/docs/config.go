package docs

import (
	"fmt"

	"prelude/pkg/shared"
)

// Config is the normalized JSON boundary from docs.nix.
type Config struct {
	Project      string         `json:"project"`
	ColorProfile string         `json:"colorProfile"`
	Palette      shared.Palette `json:"palette"`
	Pages        []Page         `json:"pages"`
}

// Page is one Markdown file embedded by Nix at build time.
type Page struct {
	Text string `json:"text"`
}

func loadConfig(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("no config: pass --config or set PRELUDE_DOCS_CONFIG")
	}
	cfg, err := shared.LoadJSON[Config](path)
	if err != nil {
		return nil, err
	}
	if len(cfg.Pages) == 0 {
		return nil, fmt.Errorf("docs: no pages configured")
	}
	return cfg, nil
}
