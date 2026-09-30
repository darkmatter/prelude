package motd

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderHostedRecordsHostOutcomesAndRunsShellChecks(t *testing.T) {
	cfg := Config{
		Project: "acme",
		Header: Header{
			TitleStyle: titleStyleSpine,
			Status: []HeaderStatus{
				// Evaluated by the host (a TypeScript function); never run here.
				// Run as shell, `ts:db` would fail and render "down".
				{Label: "db", Check: "ts:db", Ok: "up", Fail: "down"},
				// A shell check the host left to Go, exactly as Preflight runs it.
				{Label: "shell", Check: "echo from-shell"},
			},
		},
		Env: []EnvItem{
			{Label: "node", Probe: "ts:node"},
			{Label: "shell-env", Probe: "echo probed"},
		},
		Width: 72,
	}
	results := Results{
		Status: []CheckResult{{Check: "ts:db", OK: true}},
		Env:    []ProbeResult{{Probe: "ts:node", Value: "v24"}},
	}

	output := ansi.Strip(RenderHosted(cfg, results, 72, 24))
	for _, want := range []string{"up", "from-shell", "v24", "probed"} {
		if !strings.Contains(output, want) {
			t.Errorf("hosted render is missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "down") {
		t.Fatalf("host-evaluated check was re-run as shell:\n%s", output)
	}
}

func TestParseConfigRejectsUnknownFields(t *testing.T) {
	if _, err := ParseConfig([]byte(`{"project":"acme"}`)); err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if _, err := ParseConfig([]byte(`{"project":"acme","bogus":1}`)); err == nil {
		t.Fatal("ParseConfig accepted an unknown field")
	}
}
