package menu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestXtabProtoRead(t *testing.T) {
	for _, tc := range []struct {
		name   string
		data   string
		query  string
		cursor int
		ok     bool
	}{
		{"caret at end", "2\nbu", "bu", 2, true},
		{"caret at start", "0\nbu", "bu", 0, true},
		{"empty query", "0\n", "", 0, true},
		{"multibyte caret counts runes", "2\n界é", "界é", 2, true},
		{"caret past the rune count", "3\n界é", "", 0, false},
		{"negative caret", "-1\nbu", "", 0, false},
		{"non-numeric caret", "x\nbu", "", 0, false},
		{"empty file", "", "", 0, false},
		{"no newline", "2", "", 0, false},
		{"second line", "1\na\nb", "", 0, false},
		{"carriage return", "1\na\r", "", 0, false},
		{"NUL", "1\na\x00", "", 0, false},
		{"invalid UTF-8", "1\na\xff", "", 0, false},
		{"at the size limit", "1\n" + strings.Repeat("a", xtabProtoMaxData-2), strings.Repeat("a", xtabProtoMaxData-2), 1, true},
		{"over the size limit", "1\n" + strings.Repeat("a", xtabProtoMaxData-1), "", 0, false},
	} {
		path := filepath.Join(t.TempDir(), "in")
		if err := os.WriteFile(path, []byte(tc.data), 0o600); err != nil {
			t.Fatal(err)
		}
		query, cursor, ok := xtabProtoRead(path)
		if query != tc.query || cursor != tc.cursor || ok != tc.ok {
			t.Errorf("%s: got (%d bytes, %d, %t), want (%d bytes, %d, %t)",
				tc.name, len(query), cursor, ok, len(tc.query), tc.cursor, tc.ok)
		}
	}
	if _, _, ok := xtabProtoRead(filepath.Join(t.TempDir(), "missing")); ok {
		t.Error("a missing file must not be a seed")
	}
}

// xtabTestModel returns an opted-in root picker and the path the host would
// read its published filter from.
func xtabTestModel(t *testing.T) (model, string) {
	t.Helper()
	dir := t.TempDir()
	in, out := filepath.Join(dir, "in"), filepath.Join(dir, "out")
	t.Setenv("PRELUDE_X_TAB_PROTOTYPE", "1")
	t.Setenv("PRELUDE_XTAB_PROTO_IN", in)
	t.Setenv("PRELUDE_XTAB_PROTO_OUT", out)
	cfg := testMenuConfig(subcommandTestTasks()...)
	return newModel(cfg, newStyles(cfg, false), nil), out
}

func xtabKey(text string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: []rune(text)[0], Text: text}
}

// The out-file is what the host applies to Readline, so each step states
// whether the picker published and exactly what the host would read.
func TestXtabProtoPublishBaseline(t *testing.T) {
	trigger := tea.KeyPressMsg{Code: '^', Mod: tea.ModCtrl}
	type step struct {
		name string
		seed string // written to the in-file before msg when non-empty
		msg  tea.Msg
		want string // out-file content; "" means nothing was published
	}
	for _, tc := range []struct {
		name  string
		steps []step
	}{
		{"nothing is published before an edit", []step{
			{"resize", "", tea.WindowSizeMsg{Width: 80, Height: 24}, ""},
			{"focus", "", tea.FocusMsg{}, ""},
			{"list navigation", "", tea.KeyPressMsg{Code: tea.KeyDown}, ""},
		}},
		{"typing publishes text and rune caret", []step{
			{"first rune", "", xtabKey("d"), "1\nd"},
			{"multibyte rune", "", xtabKey("界"), "2\nd界"},
			{"caret-only move", "", tea.KeyPressMsg{Code: tea.KeyLeft}, "1\nd界"},
			{"unchanged after navigation", "", tea.KeyPressMsg{Code: tea.KeyDown}, ""},
			{"backspace", "", tea.KeyPressMsg{Code: tea.KeyBackspace}, "0\n界"},
		}},
		{"a host seed is not echoed back", []step{
			{"seed applied", "2\ndb", trigger, ""},
			{"unchanged after navigation", "", tea.KeyPressMsg{Code: tea.KeyDown}, ""},
			{"edit after the seed", "", xtabKey("x"), "3\ndbx"},
			{"reseed replaces the edit", "1\ndb", trigger, ""},
			{"edit at the seeded caret", "", xtabKey("y"), "2\ndyb"},
		}},
		{"an unreadable seed keeps the filter", []step{
			{"typed", "", xtabKey("d"), "1\nd"},
			{"seed with a caret past its text", "9\nzz", trigger, ""},
			{"next edit continues the old text", "", xtabKey("b"), "2\ndb"},
		}},
		{"subcommand pickers never publish", []step{
			{"typed", "", xtabKey("d"), "1\nd"},
			{"typed", "", xtabKey("b"), "2\ndb"},
			{"enter the db picker", "", tea.KeyPressMsg{Code: tea.KeyEnter}, ""},
			{"filter inside the picker", "", xtabKey("m"), ""},
			{"seed inside the picker is ignored", "1\nzz", trigger, ""},
			{"leave the picker", "", tea.KeyPressMsg{Code: tea.KeyEsc}, ""},
			{"root navigation after leaving", "", tea.KeyPressMsg{Code: tea.KeyDown}, ""},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, out := xtabTestModel(t)
			for _, s := range tc.steps {
				if s.seed != "" {
					if err := os.WriteFile(m.xtab.inFile, []byte(s.seed), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				next, _ := m.Update(s.msg)
				m = next.(model)
				got, err := os.ReadFile(out)
				if err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				if string(got) != s.want {
					t.Fatalf("%s: published %q, want %q (filter %q)", s.name, got, s.want, m.prompt.Value())
				}
			}
		})
	}
}

func TestXtabProtoInertWithoutOptIn(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	t.Setenv("PRELUDE_X_TAB_PROTOTYPE", "true") // only exactly "1" opts in
	t.Setenv("PRELUDE_XTAB_PROTO_IN", filepath.Join(dir, "in"))
	t.Setenv("PRELUDE_XTAB_PROTO_OUT", out)
	cfg := testMenuConfig(subcommandTestTasks()...)
	m := newModel(cfg, newStyles(cfg, false), nil)
	m, _ = press(t, m, xtabKey("d"))
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("picker published without the opt-in: %v", err)
	}
	if m.prompt.Value() != "d" {
		t.Fatalf("filter = %q, want the typed text", m.prompt.Value())
	}
}

// The host reads on the ack that follows a write, so a write must replace
// the whole file and leave nothing else in the pane's directory.
func TestXtabProtoWriteReplacesWholeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out")
	for _, content := range []string{"9\na-longer-filter", "1\nx"} {
		if err := xtabProtoWrite(path, []byte(content)); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != content {
			t.Fatalf("read back %q, %v; want %q", got, err, content)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("directory after writes = %v, %v; want only the out file", entries, err)
	}
	if err := xtabProtoWrite(filepath.Join(dir, "missing", "out"), []byte("1\nx")); err == nil {
		t.Fatal("writing into a missing directory must fail")
	}
}
