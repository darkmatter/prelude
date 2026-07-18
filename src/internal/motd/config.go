package motd

import (
	"fmt"

	"prelude/pkg/shared"
)

// Config is the normalized JSON boundary produced by motd.nix.
// Nix owns defaults and ordering; Go owns probes, layout, and rendering.
type Config struct {
	Project                  string         `json:"project"`
	Title                    string         `json:"title"`
	TitleAlign               string         `json:"titleAlign"`
	ColorProfile             string         `json:"colorProfile"`
	Palette                  shared.Palette `json:"palette"`
	Background               string         `json:"background"`
	BackgroundRelative       float64        `json:"backgroundRelative"`
	BackgroundBlend          float64        `json:"backgroundBlend"`
	BackgroundBlendSet       bool           `json:"backgroundBlendSet"`
	WindowBackground         string         `json:"windowBackground"`
	WindowBackgroundRelative float64        `json:"windowBackgroundRelative"`
	WindowBackgroundBlend    float64        `json:"windowBackgroundBlend"`
	WindowBackgroundBlendSet bool           `json:"windowBackgroundBlendSet"`
	ClearScreen              bool           `json:"clearScreen"`
	Margin                   Spacing        `json:"margin"`
	Align                    string         `json:"align"`
	// Padding: horizontal inset applies to tagline, middle content, and footer
	// shortcuts (header bar stays edge-to-edge). Top and bottom pad the whole
	// card, outside the title and shortcut rows.
	Padding        Spacing        `json:"padding"`
	Header         Header         `json:"header"`
	Description    StyledText     `json:"description"`
	Links          []Link         `json:"links"`
	Env            []EnvItem      `json:"env"`
	Commands       []Command      `json:"commands"`
	Recipes        []Recipe       `json:"recipes"`
	GettingStarted GettingStarted `json:"gettingStarted"`
	Shortcuts      []Shortcut     `json:"shortcuts"`
	Width          int            `json:"width"`    // 0 tracks the terminal width
	MaxWidth       int            `json:"maxWidth"` // 0 is unbounded

	// TerminalBackground is resolved at startup (OSC query with a palette bg
	// fallback), not read from JSON. It anchors the window background fade.
	TerminalBackground string `json:"-"`
	// StatusHint is derived from the async status cache for this render.
	StatusHint string `json:"-"`
	// StatusAge is the age of the cached async status (e.g. "2m ago",
	// "just now"), derived alongside StatusHint. It is rendered on the left,
	// appended to the status items, so the reload hint on the right stays clean.
	StatusAge string `json:"-"`
}

type Spacing struct {
	Top    int `json:"top"`
	Bottom int `json:"bottom"`
	Left   int `json:"left"`
	Right  int `json:"right"`
	// MinHeight gates the vertical sides: below this terminal height they
	// collapse to zero so breathing room never pushes content off short
	// screens. Zero disables the gate.
	MinHeight int `json:"minHeight"`
}

// collapseVertical zeroes Top/Bottom when the terminal is shorter than
// MinHeight. Horizontal sides are width concerns and stay untouched.
func (s Spacing) collapseVertical(terminalHeight int) Spacing {
	if s.MinHeight > 0 && terminalHeight < s.MinHeight {
		s.Top = 0
		s.Bottom = 0
	}
	return s
}

// Header is the hero bar: wordmark variant + status chips + tagline.
type Header struct {
	// TitleStyle: plain | spine | bracketed | label (default spine).
	TitleStyle string `json:"titleStyle"`
	Tagline    string `json:"tagline"`
	// Subtitle is a quiet second line under the tagline (e.g. "Your environment is ready").
	Subtitle string `json:"subtitle"`
	// TaglineLayout: "stack" (default) or "inline" (tagline · subtitle on one row).
	TaglineLayout string `json:"taglineLayout"`
	// TaglineAlign: "left" (default) or "center".
	TaglineAlign string `json:"taglineAlign"`
	// StatusHintLayout: "below" (default) or "inline" (lights left, hint right).
	StatusHintLayout string `json:"statusHintLayout"`
	// StatusHintLinks are OSC 8 hyperlinks appended after the reload hint
	// (e.g. the project repository).
	StatusHintLinks []Link         `json:"statusHintLinks"`
	Status          []HeaderStatus `json:"status"`
	// Background is an explicit header fill. Empty + BackgroundRaised paints the
	// lightened bar; empty + !Raised is transparent (fg-only).
	Background         string  `json:"background"`
	BackgroundRelative float64 `json:"backgroundRelative"`
	BackgroundRaised   bool    `json:"backgroundRaised"`
}

// HeaderStatus is one badge on the header: static text and/or a live check.
type HeaderStatus struct {
	Label  string `json:"label"`
	Status string `json:"status"` // static text, or resolved after Check runs
	Check  string `json:"check"`  // shell command; empty = static badge
	Async  bool   `json:"async"`  // resolve from cache; refresh outside foreground rendering
	Ok     string `json:"ok"`     // text when check exits 0 and stdout empty
	Fail   string `json:"fail"`   // text when check exits non-zero and stdout empty
	// FailLevel controls the semantic failure color: "error" (default) or
	// "warning".
	FailLevel string `json:"failLevel"`
	// Output controls what the badge displays after a check runs:
	//   ""        — default: configured ok/fail text, or first output line.
	//   "light"   — colored dot + label only; discard text and diagnostics.
	//   "diagnostic" — ok/fail text, plus captured output rendered below the MOTD.
	Output string `json:"output"`
	// Level is resolved at runtime: "success", "warning", "error", or "static".
	Level string `json:"-"`
}

type StyledText struct {
	Text               string  `json:"text"`
	Foreground         string  `json:"foreground"`
	Background         string  `json:"background"`
	BackgroundRelative float64 `json:"backgroundRelative"`
	Bold               bool    `json:"bold"`
	Italic             bool    `json:"italic"`
	Faint              bool    `json:"faint"`
	// Tips are optional follow-on lines. Wrap commands in backticks for accent.
	Tips []string `json:"tips"`
}

type Link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
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
