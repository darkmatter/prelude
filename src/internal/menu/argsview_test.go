package menu

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestArgsViewKeepsLaterFieldsAndChipsVisible(t *testing.T) {
	for _, tc := range []struct {
		name           string
		height         int
		terminalHeight int
		fullText       bool
	}{
		{name: "reported height", height: 12, terminalHeight: 24},
		{name: "ample height", height: 16, terminalHeight: 34, fullText: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := Task{
				Name: "run", Run: "echo",
				Args: []Arg{
					{Token: "NAME", Description: strings.Repeat("x", 108)},
					{Token: "ENV", Required: true, Options: []string{"prod", "stage"}},
				},
			}
			cfg := testMenuConfig(task)
			cfg.Height = tc.height
			m := newModel(cfg, newStyles(cfg, false), &task)
			m.applyLayout(40, tc.terminalHeight)

			view := ansi.Strip(m.View().Content)
			for _, text := range []string{"NAME", "ENV", "REQUIRED", "prod", "stage"} {
				if !strings.Contains(view, text) {
					t.Fatalf("argument form lost %q:\n%s", text, view)
				}
			}
			form := ansi.Strip(m.args.View(m.frame, m.layout.listHeight-2))
			lines := strings.Split(form, "\n")
			if len(lines) != tc.height {
				t.Fatalf("argument form height = %d, want %d:\n%s", len(lines), tc.height, form)
			}
			for _, line := range lines {
				if ansi.StringWidth(line) != 40 {
					t.Fatalf("argument form row should retain its frame width: %q", line)
				}
			}
			if tc.fullText {
				if strings.Count(form, "x") != 108 || strings.Contains(form, "…") {
					t.Fatalf("ample height should preserve the complete description:\n%s", view)
				}
			} else if !strings.Contains(form, "…") || strings.Count(form, "x") >= 108 {
				t.Fatalf("bounded description should mark omitted text with an ellipsis:\n%s", view)
			}

			next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
			m = next.(model)
			next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			m = next.(model)
			if m.prompt.Value() != "prod" {
				t.Fatalf("later option chip did not populate the argument line: %q", m.prompt.Value())
			}
			next, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if sel := next.(model).chosen; sel == nil || sel.Name != "run" || sel.Line != "prod" || sel.Command != "echo prod" {
				t.Fatalf("later option chip selection = %+v", sel)
			}
		})
	}
}

func TestArgsViewWrapsDescriptions(t *testing.T) {
	defaultValue := "staging"
	for _, tc := range []struct {
		name        string
		description string
		defaultVal  *string
		want        []string
	}{
		{
			name:        "words and default",
			description: "alpha beta gamma delta",
			defaultVal:  &defaultValue,
			want:        []string{"alpha beta gamma", "delta (default:", "staging)"},
		},
		{
			name:        "explicit newline",
			description: "alpha beta\ngamma delta",
			want:        []string{"alpha beta", "gamma delta"},
		},
		{
			name:        "wide characters",
			description: "alpha 🐈 beta gamma delta",
			want:        []string{"alpha 🐈 beta", "gamma delta"},
		},
		{
			name:        "long word",
			description: strings.Repeat("x", 40),
			want:        []string{strings.Repeat("x", 18), strings.Repeat("x", 18), "xxxx"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := Task{Name: "run", Args: []Arg{{Token: "ENV", Required: true, Description: tc.description, Default: tc.defaultVal}}}
			cfg := testMenuConfig(task)
			st := newStyles(cfg, false)
			args := newArgsView(st).WithSize(38).EnterArg(task)
			frame := Frame{st: st}.WithSize(38)

			view := ansi.Strip(args.View(frame, 8))
			lines := strings.Split(view, "\n")
			for i, line := range lines {
				if !strings.Contains(line, "REQUIRED") {
					continue
				}
				for j, want := range tc.want {
					if i+j >= len(lines) {
						t.Fatalf("missing description continuation:\n%s", view)
					}
					row := lines[i+j]
					if ansi.StringWidth(row) != 40 || !strings.HasPrefix(row, "│  ") || !strings.HasSuffix(row, "  │") {
						t.Fatalf("description row should retain padding and frame rails: %q\n%s", row, view)
					}
					if j > 0 && !strings.HasPrefix(row, "│"+strings.Repeat(" ", 18)) {
						t.Fatalf("continuation should align with the description column: %q\n%s", row, view)
					}
					if got := strings.TrimSpace(ansi.Cut(row, 19, 37)); got != want {
						t.Fatalf("description line = %q, want %q\n%s", got, want, view)
					}
				}
				return
			}
			t.Fatalf("argument row missing:\n%s", view)
		})
	}
}
