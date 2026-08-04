package wizard

import "testing"

func TestNormalizeFIGletOutputRemovesLineEndingWhitespace(t *testing.T) {
	got := normalizeFIGletOutput("foo  \nbar\t\n")
	want := "foo\nbar"
	if got != want {
		t.Fatalf("normalizeFIGletOutput() = %q, want %q", got, want)
	}
}

// figlet pads output to the font's full height; text without descenders
// leaves the descender rows blank. Those blank edge rows must be dropped so
// short titles don't carry phantom padding below (or above) the art.
func TestNormalizeFIGletOutputTrimsBlankEdgeRows(t *testing.T) {
	got := normalizeFIGletOutput("   \nfoo\nbar\n  \n")
	want := "foo\nbar"
	if got != want {
		t.Fatalf("normalizeFIGletOutput() = %q, want %q", got, want)
	}
}

// Interior blank rows are significant in fonts with multi-line glyphs and
// must survive the edge trim.
func TestNormalizeFIGletOutputKeepsInteriorBlankRows(t *testing.T) {
	got := normalizeFIGletOutput("foo\n  \nbar\n")
	want := "foo\n\nbar"
	if got != want {
		t.Fatalf("normalizeFIGletOutput() = %q, want %q", got, want)
	}
}
