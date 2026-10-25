package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func envMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, entry := range env {
		name, value, _ := strings.Cut(entry, "=")
		out[name] = value
	}
	return out
}

// The workspace keeps the user's readline/history setup: inherited values
// pass through, and absent ones stay absent so Bash defaults (~/.inputrc,
// ~/.bash_history) apply instead of /dev/null.
func TestShellEnvironmentHonoursUserReadlineAndHistory(t *testing.T) {
	got := envMap(shellEnvironment([]string{
		"PATH=/bin", "INPUTRC=/custom/inputrc", "HISTFILE=/custom/history",
	}))
	if got["INPUTRC"] != "/custom/inputrc" || got["HISTFILE"] != "/custom/history" {
		t.Fatalf("inherited setup = INPUTRC=%q HISTFILE=%q", got["INPUTRC"], got["HISTFILE"])
	}

	got = envMap(shellEnvironment([]string{"PATH=/bin"}))
	if _, ok := got["INPUTRC"]; ok {
		t.Fatalf("INPUTRC forced to %q, want absent for ~/.inputrc default", got["INPUTRC"])
	}
	if _, ok := got["HISTFILE"]; ok {
		t.Fatalf("HISTFILE forced to %q, want absent for ~/.bash_history default", got["HISTFILE"])
	}
}

// Children keep the outer TERM only when its terminfo resolves; otherwise
// they get the fallback the inner renderer implements.
func TestShellEnvironmentTERMResolution(t *testing.T) {
	terminfo := t.TempDir()
	dir := filepath.Join(terminfo, "x")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "xterm-workspace-fixture"), []byte("compiled"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := []string{"PATH=/bin", "HOME=" + t.TempDir(), "TERMINFO_DIRS=" + terminfo}

	for _, tc := range []struct {
		name string
		term string
		want string
	}{
		{"resolvable outer TERM kept", "xterm-workspace-fixture", "xterm-workspace-fixture"},
		{"unresolvable outer TERM falls back", "xterm-no-such-entry-xyz", fallbackTERM},
		{"dumb falls back", "dumb", fallbackTERM},
		{"empty falls back", "", fallbackTERM},
		{"path-like TERM falls back", "../xterm-workspace-fixture", fallbackTERM},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := envMap(shellEnvironment(append(append([]string(nil), base...), "TERM="+tc.term)))
			if got["TERM"] != tc.want {
				t.Fatalf("TERM = %q, want %q", got["TERM"], tc.want)
			}
		})
	}
}
