package wizard

import (
	"fmt"
	"os/exec"
	"strings"
)

type renderFunc func(Font, string) (string, error)

func renderFIGlet(font Font, text string) (string, error) {
	cmd := exec.Command("figlet", "-k", "-f", font.Path, "--", text)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("render %s: %s", font.Name, strings.TrimSpace(string(output)))
	}
	return normalizeFIGletOutput(string(output)), nil
}

// normalizeFIGletOutput right-trims every row, then drops fully-blank edge
// rows. figlet always emits the font's full height, so descender rows stay
// blank for text without g/j/p/q/y — measuring the rendered rows (rather
// than trusting font metrics) removes that phantom padding for every font.
// Interior blank rows are kept: some fonts use them inside multi-line glyphs.
func normalizeFIGletOutput(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
