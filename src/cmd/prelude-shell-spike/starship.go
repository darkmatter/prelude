package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// compactStarshipConfig returns a temp STARSHIP_CONFIG with leading blank lines
// stripped from `format`. Prelude's default format starts with "\n\n", which
// leaves a 2-row gap above the status bar when the shell is bottom-anchored.
//
// Spike-only: does not change the user's on-disk config. Empty path means leave
// STARSHIP_CONFIG unchanged (unset or already compact).
func compactStarshipConfig() (path string, cleanup func(), err error) {
	src := os.Getenv("STARSHIP_CONFIG")
	if src == "" {
		return "", func() {}, nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", nil, fmt.Errorf("read STARSHIP_CONFIG: %w", err)
	}

	compacted := stripFormatLeadingNewlines(string(data))
	if compacted == string(data) {
		return src, func() {}, nil
	}

	dir, err := os.MkdirTemp("", "prelude-shell-spike-starship-*")
	if err != nil {
		return "", nil, err
	}
	out := filepath.Join(dir, "starship.toml")
	if err := os.WriteFile(out, []byte(compacted), 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, err
	}
	return out, func() { _ = os.RemoveAll(dir) }, nil
}

// stripFormatLeadingNewlines removes leading \n/\r from the format string value
// so the powerline is the first thing starship paints.
//
// Handles TOML multiline format = '''…''' and format = """…""" (no backrefs;
// Go's regexp RE2 does not support \2).
func stripFormatLeadingNewlines(toml string) string {
	for _, quote := range []string{`'''`, `"""`} {
		if out, ok := stripOneFormatQuote(toml, quote); ok {
			return out
		}
	}
	return toml
}

func stripOneFormatQuote(toml, quote string) (string, bool) {
	idx := indexFormatAssign(toml, quote)
	if idx < 0 {
		return toml, false
	}
	// idx points at start of opening quote
	openEnd := idx + len(quote)
	closeIdx := strings.Index(toml[openEnd:], quote)
	if closeIdx < 0 {
		return toml, false
	}
	closeIdx += openEnd
	body := toml[openEnd:closeIdx]
	trimmed := strings.TrimLeft(body, "\r\n")
	if trimmed == body {
		return toml, true // found but already compact
	}
	return toml[:openEnd] + trimmed + toml[closeIdx:], true
}

// indexFormatAssign returns the index of the opening quote after `format =`.
func indexFormatAssign(toml, quote string) int {
	// Scan for format = with optional spaces
	for i := 0; i < len(toml); i++ {
		if !hasPrefixAt(toml, i, "format") {
			continue
		}
		// word boundary: start or non-ident before
		if i > 0 && isIdentByte(toml[i-1]) {
			continue
		}
		j := i + len("format")
		for j < len(toml) && (toml[j] == ' ' || toml[j] == '\t') {
			j++
		}
		if j >= len(toml) || toml[j] != '=' {
			continue
		}
		j++
		for j < len(toml) && (toml[j] == ' ' || toml[j] == '\t') {
			j++
		}
		if hasPrefixAt(toml, j, quote) {
			return j
		}
	}
	return -1
}

func hasPrefixAt(s string, i int, prefix string) bool {
	return i+len(prefix) <= len(s) && s[i:i+len(prefix)] == prefix
}

func isIdentByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
