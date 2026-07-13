package main

import (
	"fmt"

	"prelude/shared"
)

// Config is the normalized JSON boundary produced by motd.nix.
// Nix owns defaults and ordering; Go owns probes, layout, and rendering.
type Config struct {
	Project          string         `json:"project"`
	ColorProfile     string         `json:"colorProfile"`
	Palette          shared.Palette `json:"palette"`
	Background       string         `json:"background"`
	WindowBackground string         `json:"windowBackground"`
	ClearScreen      bool           `json:"clearScreen"`
	Margin           Spacing        `json:"margin"`
	Align            string         `json:"align"`
	// Padding insets middle content. Header and shortcuts stay edge-to-edge.
	Padding        Spacing        `json:"padding"`
	Header         Header         `json:"header"`
	Description    StyledText     `json:"description"`
	Env            []EnvItem      `json:"env"`
	Commands       []Command      `json:"commands"`
	Recipes        []Recipe       `json:"recipes"`
	Git            bool           `json:"git"`
	GettingStarted GettingStarted `json:"gettingStarted"`
	Shortcuts      []Shortcut     `json:"shortcuts"`
	Width          int            `json:"width"`    // 0 tracks the terminal width
	MaxWidth       int            `json:"maxWidth"` // 0 is unbounded
}

type Spacing struct {
	Top    int `json:"top"`
	Bottom int `json:"bottom"`
	Left   int `json:"left"`
	Right  int `json:"right"`
}

// Header is the filled hero bar: wordmark variant + status + tagline beneath.
type Header struct {
	// TitleStyle: plain | spine | bracketed | label (default spine).
	TitleStyle         string `json:"titleStyle"`
	Tagline            string `json:"tagline"`
	StatusLabel        string `json:"statusLabel"`
	StatusLabelCompact string `json:"statusLabelCompact"`
	StatusText         string `json:"statusText"`
}

type StyledText struct {
	Text       string   `json:"text"`
	Foreground string   `json:"foreground"`
	Background string   `json:"background"`
	Bold       bool     `json:"bold"`
	Italic     bool     `json:"italic"`
	Faint      bool     `json:"faint"`
	// Tips are optional follow-on lines. Wrap commands in backticks for accent.
	Tips []string `json:"tips"`
}

type EnvItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Probe string `json:"probe"`
}

type Command struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// Recipe is one titled codeblock in the examples group.
type Recipe struct {
	Title string       `json:"title"`
	Steps []RecipeStep `json:"steps"`
}

// RecipeStep is either a command or a comment caption (exactly one side set).
type RecipeStep struct {
	Command string `json:"command"`
	Comment string `json:"comment"`
}

// Shortcut is a quiet discoverability chip in the closing line.
type Shortcut struct {
	Command string `json:"command"`
	Alias   string `json:"alias"`
}

// GettingStarted labels the unified commands + examples region.
type GettingStarted struct {
	Heading       string `json:"heading"`
	CommandsLabel string `json:"commandsLabel"`
	ExamplesLabel string `json:"examplesLabel"`
}

func loadConfig(path string) (Config, error) {
	if path == "" {
		return Config{}, fmt.Errorf("no config: pass --config or set PRELUDE_MOTD_CONFIG")
	}
	cfg, err := shared.LoadJSON[Config](path)
	if err != nil {
		return Config{}, err
	}
	return *cfg, nil
}
