package main

import "testing"

func TestStripFormatLeadingNewlines(t *testing.T) {
	in := `add_newline = false
format = '''


[powerline]
$character'''
palette = "prelude"
`
	got := stripFormatLeadingNewlines(in)
	want := `add_newline = false
format = '''[powerline]
$character'''
palette = "prelude"
`
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestStripFormatLeadingNewlinesNoChange(t *testing.T) {
	in := `format = '''[powerline]
$character'''
`
	if got := stripFormatLeadingNewlines(in); got != in {
		t.Fatalf("unexpected change:\n%s", got)
	}
}
