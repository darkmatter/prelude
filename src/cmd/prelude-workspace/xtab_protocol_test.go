package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestXtabProtoOptIn(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  []string
		want bool
	}{
		{"unset", []string{"PATH=/bin"}, false},
		{"exact one", []string{"PRELUDE_X_TAB_PROTOTYPE=1"}, true},
		{"empty", []string{"PRELUDE_X_TAB_PROTOTYPE="}, false},
		{"true is not one", []string{"PRELUDE_X_TAB_PROTOTYPE=true"}, false},
		{"padded one", []string{"PRELUDE_X_TAB_PROTOTYPE= 1"}, false},
		{"other name", []string{"PRELUDE_X_TAB_PROTOTYPE_X=1"}, false},
		{"last wins off", []string{"PRELUDE_X_TAB_PROTOTYPE=1", "PRELUDE_X_TAB_PROTOTYPE=0"}, false},
		{"last wins on", []string{"PRELUDE_X_TAB_PROTOTYPE=0", "PRELUDE_X_TAB_PROTOTYPE=1"}, true},
	} {
		if got := xtabProtoOptIn(tc.env); got != tc.want {
			t.Errorf("%s: xtabProtoOptIn(%q) = %t, want %t", tc.name, tc.env, got, tc.want)
		}
	}
}

func TestXtabProtoNeedleSeesSequenceAcrossChunks(t *testing.T) {
	const seq = xtabProtoReadyOSC
	for _, tc := range []struct {
		name   string
		chunks []string
		want   []bool
	}{
		{"whole", []string{seq}, []bool{true}},
		{"embedded", []string{"abc" + seq + "def"}, []bool{true}},
		{"split in two", []string{seq[:5], seq[5:]}, []bool{false, true}},
		{"one byte per chunk tail", []string{seq[:len(seq)-1], seq[len(seq)-1:]}, []bool{false, true}},
		{"split in three", []string{"x" + seq[:1], seq[1:9], seq[9:] + "y"}, []bool{false, false, true}},
		{"restart on a fresh ESC", []string{seq[:6] + seq}, []bool{true}},
		{"interrupted then absent", []string{seq[:6], "zz" + seq[6:]}, []bool{false, false}},
		{"other OSC", []string{xtabProtoOutOSC}, []bool{false}},
		{"twice in one chunk then idle", []string{seq + seq, "plain"}, []bool{true, false}},
	} {
		needle := xtabProtoNeedle{seq: seq}
		for i, chunk := range tc.chunks {
			if got := needle.seen([]byte(chunk)); got != tc.want[i] {
				t.Errorf("%s: chunk %d %q seen = %t, want %t", tc.name, i, chunk, got, tc.want[i])
			}
		}
	}
}

func TestXtabProtoParseLine(t *testing.T) {
	for _, tc := range []struct {
		name          string
		line          string
		point         int
		prefix, query string
		cursor        int
		ok            bool
	}{
		{"caret at end", "x bu", 4, "x ", "bu", 2, true},
		{"caret inside token", "x bu", 3, "x ", "bu", 1, true},
		{"caret right after prefix", "x bu", 2, "x ", "bu", 0, true},
		{"empty query", "x ", 2, "x ", "", 0, true},
		{"leading spaces kept in prefix", "  x bu", 6, "  x ", "bu", 2, true},
		// READLINE_POINT counts characters: "界é" is two characters, five bytes.
		{"multibyte caret is in runes", "x 界é", 4, "x ", "界é", 2, true},
		{"multibyte caret mid token", "x 界é", 3, "x ", "界é", 1, true},
		{"byte offset past the rune count", "x 界é", 7, "", "", 0, false},
		{"caret inside prefix", "x bu", 1, "", "", 0, false},
		{"caret before leading spaces end", "  x bu", 3, "", "", 0, false},
		{"caret past end", "x bu", 5, "", "", 0, false},
		{"bare x", "x", 1, "", "", 0, false},
		{"other command", "xy bu", 5, "", "", 0, false},
		{"tab is not the prefix space", "x\tbu", 4, "", "", 0, false},
		{"second token", "x bu ild", 8, "", "", 0, false},
		{"trailing space", "x bu ", 5, "", "", 0, false},
		{"quote", "x 'bu", 5, "", "", 0, false},
		{"expansion", "x $bu", 5, "", "", 0, false},
		{"pipe", "x bu|cat", 8, "", "", 0, false},
		{"control character", "x b\x01u", 5, "", "", 0, false},
		{"empty line", "", 0, "", "", 0, false},
	} {
		prefix, query, cursor, ok := xtabProtoParseLine(tc.line, tc.point)
		if prefix != tc.prefix || query != tc.query || cursor != tc.cursor || ok != tc.ok {
			t.Errorf("%s: xtabProtoParseLine(%q, %d) = (%q, %q, %d, %t), want (%q, %q, %d, %t)",
				tc.name, tc.line, tc.point, prefix, query, cursor, ok, tc.prefix, tc.query, tc.cursor, tc.ok)
		}
	}
}

func TestXtabProtoReadFile(t *testing.T) {
	for _, tc := range []struct {
		name    string
		data    string
		rest    string
		caret   int
		wantErr string
	}{
		{"point and line", "4\nx bu", "x bu", 4, ""},
		{"empty line", "0\n", "", 0, ""},
		{"missing newline", "3", "", 3, ""},
		{"multibyte line", "4\nx 界é", "x 界é", 4, ""},
		{"only the first newline splits", "1\na\nb", "a\nb", 1, ""},
		{"at the size limit", "1\n" + strings.Repeat("a", xtabProtoMaxData-2), strings.Repeat("a", xtabProtoMaxData-2), 1, ""},
		{"over the size limit", "1\n" + strings.Repeat("a", xtabProtoMaxData-1), "", 0, "exceeds limit"},
		{"NUL", "1\na\x00b", "", 0, "contains NUL"},
		{"invalid UTF-8", "1\na\xffb", "", 0, "not UTF-8"},
		{"empty file", "", "", 0, "invalid caret"},
		{"negative caret", "-1\nx", "", 0, "invalid caret"},
		{"non-numeric caret", "x bu\n", "", 0, "invalid caret"},
	} {
		path := filepath.Join(t.TempDir(), "data")
		if err := os.WriteFile(path, []byte(tc.data), 0o600); err != nil {
			t.Fatal(err)
		}
		rest, caret, err := xtabProtoReadFile(path)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("%s: error = %v, want one containing %q", tc.name, err, tc.wantErr)
			}
			continue
		}
		if err != nil || rest != tc.rest || caret != tc.caret {
			t.Errorf("%s: got (%d bytes, %d, %v), want (%d bytes, %d, nil)", tc.name, len(rest), caret, err, len(tc.rest), tc.caret)
		}
	}
	if _, _, err := xtabProtoReadFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Error("a missing file must be an error")
	}
}

// A reader acting on an ack must never observe a truncated file: the write
// replaces the whole file by rename and leaves no temporary sibling behind.
func TestXtabProtoWriteFileReplacesWholeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data")
	for _, content := range []string{"12\nx a-long-first-query", "1\nx", ""} {
		if err := xtabProtoWriteFile(path, []byte(content)); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != content {
			t.Fatalf("read back %q, %v; want %q", got, err, content)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "data" {
		t.Fatalf("directory after writes = %v, %v; want only the data file", entries, err)
	}
	if err := xtabProtoWriteFile(filepath.Join(dir, "missing", "data"), []byte("1\nx")); err == nil {
		t.Fatal("writing into a missing directory must fail")
	}
}

func TestXtabProtoKeyClasses(t *testing.T) {
	for _, tc := range []struct {
		key            tea.KeyPressMsg
		submit, prefix bool
	}{
		{tea.KeyPressMsg{Code: tea.KeyEnter}, true, false},
		{tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}, true, false},
		{tea.KeyPressMsg{Code: 'm', Mod: tea.ModCtrl}, true, false},
		{tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl}, true, false},
		{tea.KeyPressMsg{Code: tea.KeyEscape}, false, true},
		{tea.KeyPressMsg{Code: '[', Mod: tea.ModCtrl}, false, true},
		{tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}, false, true},
		{tea.KeyPressMsg{Code: 'x', Text: "x"}, false, false},
		{tea.KeyPressMsg{Code: 'x', Mod: tea.ModAlt}, false, false},
		{tea.KeyPressMsg{Code: tea.KeyTab}, false, false},
		{tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl}, false, false},
	} {
		if got := xtabProtoSubmitKey(tc.key); got != tc.submit {
			t.Errorf("xtabProtoSubmitKey(%s) = %t, want %t", tc.key, got, tc.submit)
		}
		if got := xtabProtoPrefixKey(tc.key); got != tc.prefix {
			t.Errorf("xtabProtoPrefixKey(%s) = %t, want %t", tc.key, got, tc.prefix)
		}
	}
}
