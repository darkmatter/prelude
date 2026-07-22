package docs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"prelude/pkg/shared"
)

// Config is the normalized JSON boundary from the docs package bundle.
type Config struct {
	Project      string         `json:"project"`
	ColorProfile string         `json:"colorProfile"`
	Palette      shared.Palette `json:"palette"`
	// Nav is the expanded documentation tree (generate nodes already resolved).
	Nav []NavNode `json:"nav"`
	// HeroFile is a relative filename (resolved against the config dir) for the
	// build-time FIGlet wordmark baked into the bundle, or empty when no hero
	// is present (older bundles, empty project name). Loaded into Hero at init.
	HeroFile string `json:"heroFile,omitempty"`
	// Hero is the rendered FIGlet hero loaded from HeroFile at config load.
	// Empty when absent so the viewer falls back to the bold project name.
	Hero string `json:"-"`
}

// NavNode is one sidebar entry. Leaves hold Markdown; groups hold children.
type NavNode struct {
	Kind         string    `json:"kind"` // "leaf" | "group"
	Title        string    `json:"title"`
	MarkdownFile string    `json:"markdownFile,omitempty"`
	Children     []NavNode `json:"children,omitempty"`
	// GapBefore inserts a blank separator row above this entry in the sidebar
	// (used for the generated Options group).
	GapBefore bool `json:"gapBefore,omitempty"`
	// RootReadme is set in Nix when leaf text path equals prelude.docs.rootReadme.
	RootReadme bool `json:"rootReadme,omitempty"`

	// Markdown is filled after load from MarkdownFile (not in JSON).
	Markdown string `json:"-"`
}

func loadConfig(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("no config: pass --config or set PRELUDE_DOCS_CONFIG")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("docs config: %w", err)
	}
	if len(cfg.Nav) == 0 {
		// Back-compat: older fixtures may still use "pages".
		var legacy struct {
			Pages []struct {
				Text string `json:"text"`
			} `json:"pages"`
		}
		if err := json.Unmarshal(raw, &legacy); err == nil && len(legacy.Pages) > 0 {
			cfg.Nav = make([]NavNode, len(legacy.Pages))
			for i, p := range legacy.Pages {
				cfg.Nav[i] = NavNode{Kind: "leaf", Markdown: p.Text}
			}
		}
	}
	if len(cfg.Nav) == 0 {
		return nil, fmt.Errorf("docs: no pages configured")
	}
	base := filepath.Dir(path)
	if err := loadNavMarkdown(cfg.Nav, base); err != nil {
		return nil, err
	}
	if cfg.HeroFile != "" {
		heroPath := cfg.HeroFile
		if !filepath.IsAbs(heroPath) {
			heroPath = filepath.Join(base, heroPath)
		}
		hero, err := os.ReadFile(heroPath)
		if err != nil {
			return nil, fmt.Errorf("docs: read hero %s: %w", heroPath, err)
		}
		cfg.Hero = string(hero)
	}
	return &cfg, nil
}

func loadNavMarkdown(nodes []NavNode, base string) error {
	for i := range nodes {
		n := &nodes[i]
		if len(n.Children) > 0 || n.Kind == "group" {
			n.Kind = "group"
			if err := loadNavMarkdown(n.Children, base); err != nil {
				return err
			}
			continue
		}
		n.Kind = "leaf"
		if n.Markdown != "" {
			continue
		}
		if n.MarkdownFile == "" {
			return fmt.Errorf("docs: leaf %q has no markdownFile", n.Title)
		}
		path := n.MarkdownFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("docs: read %s: %w", path, err)
		}
		n.Markdown = string(body)
	}
	return nil
}
