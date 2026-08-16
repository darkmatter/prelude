package docs

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestParsePrintArgs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    []string
		want    printRequest
		wantErr bool
	}{
		{name: "page only", args: []string{"2"}, want: printRequest{kind: printKindPage, page: 2, offset: 0}},
		{name: "page and offset", args: []string{"2", "20"}, want: printRequest{kind: printKindPage, page: 2, offset: 20}},
		{name: "next", args: []string{"next"}, want: printRequest{kind: printKindNext}},
		{name: "prev", args: []string{"prev"}, want: printRequest{kind: printKindPrev}},
		{name: "title token", args: []string{"foo"}, wantErr: true},
		{name: "bad offset", args: []string{"2", "x"}, wantErr: true},
		{name: "extra args", args: []string{"1", "0", "junk"}, wantErr: true},
		{name: "page zero", args: []string{"0"}, wantErr: true},
		{name: "negative page", args: []string{"-1"}, wantErr: true},
		{name: "negative offset", args: []string{"1", "-1"}, wantErr: true},
		{name: "empty", args: []string{}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePrintArgs(test.args)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parsePrintArgs(%q) = %#v, want error", test.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePrintArgs(%q): %v", test.args, err)
			}
			if got != test.want {
				t.Fatalf("parsePrintArgs(%q) = %#v, want %#v", test.args, got, test.want)
			}
		})
	}
}

func TestPrintViewport(t *testing.T) {
	t.Parallel()
	if got, want := wrapWidth(120), 96; got != want {
		t.Fatalf("wrapWidth(120) = %d, want %d", got, want)
	}
	if got, want := wrapWidth(40), 40; got != want {
		t.Fatalf("wrapWidth(40) = %d, want %d", got, want)
	}
	if got, want := wrapWidth(8), 16; got != want {
		t.Fatalf("wrapWidth(8) = %d, want %d", got, want)
	}
	if got, want := windowHeight(24), 22; got != want {
		t.Fatalf("windowHeight(24) = %d, want %d", got, want)
	}
	if got, want := windowHeight(1), 1; got != want {
		t.Fatalf("windowHeight(1) = %d, want %d", got, want)
	}
}

func TestPrintPageOffsetSlicesWindow(t *testing.T) {
	cfg := numberedDocs(t, 1, 60)
	var stdout, stderr bytes.Buffer
	env := testPrintEnv(t, &stdout, &stderr)
	env.cols, env.rows = 80, 24

	if err := runPrint(cfg, []string{"1", "20"}, env); err != nil {
		t.Fatal(err)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr: %s", stderr.String())
	}

	got := printedLines(stdout.String())
	if len(got) != 22 {
		t.Fatalf("printed %d lines, want 22:\n%s", len(got), stdout.String())
	}
	if got[0] == "" || got[len(got)-1] == "" {
		t.Fatalf("printed an empty content line: %q", got)
	}

	state := readPagerState(t, env.statePath)
	if state.Page != 1 || state.Start != 20 || state.End != 42 {
		t.Fatalf("state = %+v, want page=1 start=20 end=42", state)
	}
	if state.Width != 80 {
		t.Fatalf("state width = %d, want 80", state.Width)
	}
	if state.Config != env.configPath {
		t.Fatalf("state config = %q, want %q", state.Config, env.configPath)
	}
}

func TestPrintPageOffsetClampsToLastWindow(t *testing.T) {
	cfg := numberedDocs(t, 1, 30)
	var stdout bytes.Buffer
	env := testPrintEnv(t, &stdout, ioDiscard{})
	env.cols, env.rows = 80, 12 // H = 10

	if err := runPrint(cfg, []string{"1", "999"}, env); err != nil {
		t.Fatal(err)
	}
	got := printedLines(stdout.String())
	if len(got) != 10 {
		t.Fatalf("printed %d lines, want 10:\n%s", len(got), stdout.String())
	}
	state := readPagerState(t, env.statePath)
	if state.Page != 1 || state.Start != state.End-10 {
		t.Fatalf("state = %+v, want a final ten-line window", state)
	}
}

func TestPrintPageOutOfRange(t *testing.T) {
	cfg := numberedDocs(t, 1, 4)
	var stderr bytes.Buffer
	env := testPrintEnv(t, ioDiscard{}, &stderr)

	err := runPrint(cfg, []string{"3"}, env)
	if err == nil {
		t.Fatal("expected out-of-range error")
	}
	if !strings.Contains(err.Error(), "page 3 out of range (1-1)") {
		t.Fatalf("error = %v", err)
	}
}

func TestPrintNestedGroupLeafIndex(t *testing.T) {
	cfg := &Config{Nav: []NavNode{
		{
			Kind:  "group",
			Title: "Guides",
			Children: []NavNode{
				{Kind: "leaf", Title: "One", Markdown: "body one"},
				{Kind: "leaf", Title: "Two", Markdown: "body two"},
			},
		},
	}}
	var stdout bytes.Buffer
	env := testPrintEnv(t, &stdout, ioDiscard{})

	if err := runPrint(cfg, []string{"2"}, env); err != nil {
		t.Fatal(err)
	}
	plain := ansi.Strip(stdout.String())
	if !strings.Contains(plain, "body two") {
		t.Fatalf("missing second leaf:\n%s", plain)
	}
	if strings.Contains(plain, "body one") {
		t.Fatalf("first leaf leaked:\n%s", plain)
	}
}
func TestPrintNextAndPrevPageWindows(t *testing.T) {
	cfg := numberedDocs(t, 2, 30)
	var stdout bytes.Buffer
	env := testPrintEnv(t, &stdout, ioDiscard{})
	env.cols, env.rows = 80, 12 // H = 10

	if err := runPrint(cfg, []string{"1", "0"}, env); err != nil {
		t.Fatal(err)
	}
	first := readPagerState(t, env.statePath)
	if first.Page != 1 || first.Start != 0 {
		t.Fatalf("first state = %+v", first)
	}

	stdout.Reset()
	if err := runPrint(cfg, []string{"next"}, env); err != nil {
		t.Fatal(err)
	}
	second := readPagerState(t, env.statePath)
	if second.Page != 1 || second.Start != first.End {
		t.Fatalf("next state = %+v, want page 1 start %d", second, first.End)
	}

	stdout.Reset()
	if err := runPrint(cfg, []string{"prev"}, env); err != nil {
		t.Fatal(err)
	}
	previous := readPagerState(t, env.statePath)
	if previous.Page != 1 || previous.Start != first.Start {
		t.Fatalf("prev state = %+v, want first state %+v", previous, first)
	}
}

func TestPrintNextCrossesToNextLeaf(t *testing.T) {
	cfg := numberedDocs(t, 2, 4)
	var stdout bytes.Buffer
	env := testPrintEnv(t, &stdout, ioDiscard{})
	env.cols, env.rows = 80, 12

	if err := runPrint(cfg, []string{"1", "0"}, env); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := runPrint(cfg, []string{"next"}, env); err != nil {
		t.Fatal(err)
	}
	state := readPagerState(t, env.statePath)
	if state.Page != 2 || state.Start != 0 {
		t.Fatalf("cross-page next state = %+v, want page 2 start 0", state)
	}
}

func TestPrintEmptyStateStartsAtFirstPage(t *testing.T) {
	cfg := numberedDocs(t, 2, 4)
	var stdout bytes.Buffer
	env := testPrintEnv(t, &stdout, ioDiscard{})
	env.cols, env.rows = 80, 12

	if err := runPrint(cfg, []string{"prev"}, env); err != nil {
		t.Fatal(err)
	}
	state := readPagerState(t, env.statePath)
	if state.Page != 1 || state.Start != 0 {
		t.Fatalf("empty prev state = %+v, want page 1 start 0", state)
	}
}

func TestPrintNextRebasesAfterWidthChange(t *testing.T) {
	cfg := numberedDocs(t, 1, 60)
	var stdout bytes.Buffer
	env := testPrintEnv(t, &stdout, ioDiscard{})
	env.cols, env.rows = 40, 24
	state := pagerState{Page: 1, Start: 20, End: 42, Width: 80, Config: env.configPath}
	if err := savePagerState(env.statePath, state); err != nil {
		t.Fatal(err)
	}

	if err := runPrint(cfg, []string{"next"}, env); err != nil {
		t.Fatal(err)
	}
	got := readPagerState(t, env.statePath)
	if got.Start == 42 {
		t.Fatalf("next reused old-width offset after resize: %+v", got)
	}
	if got.Width != 40 {
		t.Fatalf("rebased state width = %d, want 40", got.Width)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func testPrintEnv(t *testing.T, stdout, stderr interface{ Write([]byte) (int, error) }) printEnv {
	t.Helper()
	dir := t.TempDir()
	return printEnv{
		cols:       80,
		rows:       24,
		statePath:  filepath.Join(dir, "state.json"),
		configPath: filepath.Join(dir, "config.json"),
		stdout:     stdout,
		stderr:     stderr,
		environ:    []string{"NO_COLOR=1"},
	}
}

func numberedDocs(t *testing.T, pages, lines int) *Config {
	t.Helper()
	nav := make([]NavNode, pages)
	for page := range pages {
		var b strings.Builder
		for line := range lines {
			if line > 0 {
				b.WriteByte('\n')
			}
			b.WriteString("- L")
			if line < 10 {
				b.WriteByte('0')
			}
			b.WriteString(itoa(line))
		}
		nav[page] = NavNode{Kind: "leaf", Title: "p" + itoa(page+1), Markdown: b.String()}
	}
	return &Config{Nav: nav}
}

func printedLines(out string) []string {
	plain := strings.TrimRight(ansi.Strip(out), "\n")
	if plain == "" {
		return nil
	}
	return strings.Split(plain, "\n")
}

func readPagerState(t *testing.T, path string) pagerState {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state pagerState
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("state %s: %v", raw, err)
	}
	return state
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
