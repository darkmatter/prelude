package main

import (
	"fmt"

	"prelude/shared"
)

// Config is the normalized JSON boundary from docs.nix.
// Content is hand-authored in Nix; the viewer only lays it out.
type Config struct {
	Project      string         `json:"project"`
	ColorProfile string         `json:"colorProfile"`
	Palette      shared.Palette `json:"palette"`
	Sections     []Section      `json:"sections"`
}

// Section is one sidebar entry + body.
type Section struct {
	Title  string  `json:"title"`
	Blocks []Block `json:"blocks"`
}

// Block is a hand-authored content unit.
//
//	type=lead     term (accent2 bold) + " — " + text (fg)
//	type=para     wrapped muted paragraph
//	type=option   term (accent2 bold) + indented muted text
//	type=command  term (accent2 bold) + indented muted text
//	type=shell    "$ " + command (accent/fg); optional note (dim)
//	type=blank    empty row
type Block struct {
	Type    string `json:"type"`
	Term    string `json:"term"`
	Text    string `json:"text"`
	Command string `json:"command"`
	Note    string `json:"note"`
}

func loadConfig(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("no config: pass --config or set PRELUDE_DOCS_CONFIG")
	}
	cfg, err := shared.LoadJSON[Config](path)
	if err != nil {
		return nil, err
	}
	if len(cfg.Sections) == 0 {
		return nil, fmt.Errorf("docs: no sections configured")
	}
	return cfg, nil
}
