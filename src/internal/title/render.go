package title

import (
	"fmt"
	"os/exec"
	"strings"
)

type renderFunc func(Font, string) (string, error)

func renderFIGlet(font Font, text string) (string, error) {
	cmd := exec.Command("figlet", "-f", font.Path, "--", text)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("render %s: %s", font.Name, strings.TrimSpace(string(output)))
	}
	return normalizeFIGletOutput(string(output)), nil
}

func normalizeFIGletOutput(output string) string {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}
