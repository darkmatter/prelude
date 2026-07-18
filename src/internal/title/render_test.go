package title

import "testing"

func TestNormalizeFIGletOutputRemovesLineEndingWhitespace(t *testing.T) {
	got := normalizeFIGletOutput("   \nfoo  \nbar\t\n")
	want := "\nfoo\nbar"
	if got != want {
		t.Fatalf("normalizeFIGletOutput() = %q, want %q", got, want)
	}
}
