package main

import (
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

var spikeNavigationKeys = []keyHint{}
var spikeWindowKeys = []keyHint{lockedBadge}
var spikeLeaderWindowKeys = []keyHint{unlockedBadge, layoutKeys, {key: "Esc", text: "lock"}}
var spikeHiddenKeys = spikeNavigationKeys

func writeChromeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "menu.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestChromePaletteLoad(t *testing.T) {
	path := writeChromeConfig(t, `{"project":"custom","groups":[],"palette":{"fg":"#13579b","muted":"#abc","accent":212,"accent2":"33","warning":"","error":null,"bg":"#010203","success":"#23ab45"}}`)
	want := chromePalette{
		Fg: ansi.HexColor("#13579b"), Muted: ansi.HexColor("#abc"),
		Accent: ansi.IndexedColor(212), Accent2: ansi.IndexedColor(33),
		Bg: ansi.HexColor("#010203"), Success: ansi.HexColor("#23ab45"),
	}
	for _, tc := range []struct {
		name string
		env  []string
		want chromePalette
	}{
		{"Absent", nil, chromePalette{}},
		{"UnrelatedVariable", []string{"OTHER_PRELUDE_MENU_CONFIG=" + path}, chromePalette{}},
		{"LastAssignmentWins", []string{"PRELUDE_MENU_CONFIG=/missing/menu.json", "PRELUDE_MENU_CONFIG=" + path}, want},
		{"EmptyOverridesPath", []string{"PRELUDE_MENU_CONFIG=" + path, "PRELUDE_MENU_CONFIG="}, chromePalette{}},
		{"UnsetOverridesPath", []string{"PRELUDE_MENU_CONFIG=" + path, "PRELUDE_MENU_CONFIG"}, chromePalette{}},
		{"OptionalTokens", []string{"PRELUDE_MENU_CONFIG=" + writeChromeConfig(t, `{"palette":{"fg":1}}`)}, chromePalette{Fg: ansi.BasicColor(1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := loadChromePalette(tc.env)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("loadChromePalette = %+v, %v; want %+v", got, err, tc.want)
			}
		})
	}
}

func TestChromePaletteInvalidConfig(t *testing.T) {
	for _, tc := range []struct {
		name, content, detail string
	}{
		{"MalformedJSON", `{"palette":`, "decode menu config"},
		{"NotAnObject", `[]`, "decode menu config"},
		{"MissingPalette", `{}`, ".palette object"},
		{"NullPalette", `{"palette":null}`, ".palette object"},
		{"InvalidHex", `{"palette":{"fg":"#123456","accent":"#oops"}}`, ".palette.accent"},
		{"IndexOutOfRange", `{"palette":{"accent":256}}`, ".palette.accent"},
		{"NegativeIndex", `{"palette":{"accent":"-1"}}`, ".palette.accent"},
		{"FractionalIndex", `{"palette":{"accent":1.5}}`, ".palette.accent"},
		{"WrongColorShape", `{"palette":{"accent":true}}`, ".palette.accent"},
		{"InvalidKeycapBackground", `{"palette":{"bg":"#oops"}}`, ".palette.bg"},
		{"InvalidSuccessColor", `{"palette":{"success":256}}`, ".palette.success"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeChromeConfig(t, tc.content)
			got, err := loadChromePalette([]string{"PRELUDE_MENU_CONFIG=" + path})
			if err == nil || !strings.Contains(err.Error(), "PRELUDE_MENU_CONFIG") ||
				!strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), tc.detail) {
				t.Fatalf("invalid config must identify its path and problem: %v", err)
			}
			if !reflect.DeepEqual(got, chromePalette{}) {
				t.Fatalf("invalid config returned a partial palette: %+v", got)
			}
		})
	}
	path := filepath.Join(t.TempDir(), "missing.json")
	if _, err := loadChromePalette([]string{"PRELUDE_MENU_CONFIG=" + path}); !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), path) {
		t.Fatalf("missing explicit config must retain the path and I/O error: %v", err)
	}
}

func TestChromePaletteHostView(t *testing.T) {
	for _, tc := range []struct {
		name, config string
		want         chromePalette
	}{
		{
			name:   "CustomHexAndANSI",
			config: `{"palette":{"fg":"#13579b","muted":"#2468ac","accent":212,"accent2":"33","warning":"#c98f12","error":"#d43a5e","bg":"#010203","success":"#23ab45"}}`,
			want: chromePalette{
				Fg: ansi.HexColor("#13579b"), Muted: ansi.HexColor("#2468ac"),
				Accent: ansi.IndexedColor(212), Accent2: ansi.IndexedColor(33),
				Warning: ansi.HexColor("#c98f12"), Error: ansi.HexColor("#d43a5e"),
				Bg: ansi.HexColor("#010203"), Success: ansi.HexColor("#23ab45"),
			},
		},
		{name: "EmptyConfigOverride"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := ""
			if tc.config != "" {
				path = writeChromeConfig(t, tc.config)
			}
			h := newSpikeTestHost(t, "PRELUDE_MENU_CONFIG="+path)
			if !reflect.DeepEqual(h.palette, tc.want) {
				t.Fatalf("host startup palette = %+v, want %+v", h.palette, tc.want)
			}
			if path != "" {
				// Later views must use the startup snapshot, not reread.
				if err := os.WriteFile(path, []byte("invalid after startup"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			assertChromeFooter(t, h, spikeNavigationKeys, spikeClosedPill())
			h.notice = "theme notice"
			assertChromeFooter(t, h, spikeNavigationKeys, "theme notice "+spikeClosedPill())
			h.notice = ""
			h.err = errors.New("theme failure")
			assertChromeFooter(t, h, nil, "error: theme failure ●")
			h.err = nil
			spikeKey(h, 'u', tea.ModCtrl)
			h.Update(tea.PasteMsg{Content: `printf '\033[999;1HCHROME-CHILD-OUTPUT\033[H'`})
			spikeKey(h, tea.KeyEnter, 0)
			awaitSpike(t, h, "child output at its bottom margin", func() bool {
				lastRow := strings.Split(spikeFrameText(h.frame), "\n")[h.frame.Rows-1]
				return h.prompt.ready && strings.HasPrefix(lastRow, "CHROME-CHILD-OUTPUT")
			})
			assertChromeFooter(t, h, spikeNavigationKeys, spikeClosedPill())
		})
	}
}

func assertChromeColor(t *testing.T, got, want color.Color) {
	t.Helper()
	if (got == nil) != (want == nil) || want != nil && color.RGBAModel.Convert(got) != color.RGBAModel.Convert(want) {
		t.Fatalf("color = %v, want %v", got, want)
	}
}

func assertChromeFooter(t *testing.T, h *host, hints []keyHint, status string) (*vt.Emulator, []string) {
	t.Helper()
	assertSpikeFooter(t, h, spikeFooterKeyText(hints), status)
	view := h.View()
	lines := strings.Split(ansi.Strip(view.Content), "\n")
	screen := vt.NewEmulator(h.cols, h.rows)
	t.Cleanup(func() { screen.Close() })
	if _, err := screen.Write([]byte(strings.ReplaceAll(view.Content, "\n", "\r\n"))); err != nil {
		t.Fatal(err)
	}
	assertChromeFooterCells(t, screen, h, hints, spikeStatusSpans(h, status))
	for y := range h.rows - 1 {
		for x := range h.cols {
			if cell := screen.CellAt(x, y); cell == nil || cell.Style.Bg != nil {
				t.Fatalf("keycap palette leaked a backdrop into the native canvas/candidates at %d,%d: %+v", x, y, cell)
			}
		}
	}
	return screen, lines
}

// spikeStatusSpans returns the host's own status spans trimmed to the text
// the footer layout actually drew, so cell assertions check that drawFooter
// paints exactly those spans (colors, bold, reverse) at the right cells.
func spikeStatusSpans(h *host, status string) []statusSpan {
	_, spans, drawn, _ := h.footerLayout()
	var out []statusSpan
	remaining := ansi.StringWidth(drawn)
	for _, span := range spans {
		if remaining <= 0 {
			break
		}
		if w := ansi.StringWidth(span.text); w > remaining {
			span.text = ansi.Truncate(span.text, remaining, "")
		}
		remaining -= ansi.StringWidth(span.text)
		out = append(out, span)
	}
	return out
}

func assertChromeFooterCells(t *testing.T, screen *vt.Emulator, h *host, hints []keyHint, spans []statusSpan) {
	t.Helper()
	caps := make([]bool, h.cols)
	x, y := 2, h.rows-1
	for i, hint := range hints {
		if i > 0 {
			x += 2
		}
		keyFG, keyBG := h.palette.Accent2, h.palette.Bg
		modeFG, modeBG := h.palette.Muted, h.palette.Bg
		if hint.badge && hint.active {
			keyFG, keyBG = h.palette.Bg, h.palette.Accent
			modeFG, modeBG = keyFG, keyBG
		}
		for _, char := range " " + hint.key + " " {
			cell := screen.CellAt(x, y)
			if cell == nil || cell.Content != string(char) || cell.Style.Attrs != uv.AttrBold {
				t.Fatalf("padded keycap glyph/style at %d,%d = %+v, want %q bold", x, y, cell, char)
			}
			assertChromeColor(t, cell.Style.Fg, keyFG)
			assertChromeColor(t, cell.Style.Bg, keyBG)
			caps[x] = true
			x++
		}
		if hint.badge {
			for _, char := range hint.text + " " {
				cell := screen.CellAt(x, y)
				if cell == nil || cell.Content != string(char) || cell.Style.Attrs != uv.AttrBold {
					t.Fatalf("badge mode glyph/style at %d,%d = %+v, want %q bold", x, y, cell, char)
				}
				assertChromeColor(t, cell.Style.Fg, modeFG)
				assertChromeColor(t, cell.Style.Bg, modeBG)
				caps[x] = true
				x++
			}
			continue
		}
		if hint == layoutKeys {
			for _, where := range placements {
				fg, attrs := h.palette.Muted, uint8(0)
				if where == h.placement {
					fg, attrs = h.palette.Accent, uv.AttrBold
				}
				for _, char := range " " + where.glyph() {
					cell := screen.CellAt(x, y)
					if cell == nil || cell.Content != string(char) || cell.Style.Bg != nil || cell.Style.Attrs != attrs {
						t.Fatalf("layout strip glyph/style at %d,%d = %+v, want %q attrs=%d without a background", x, y, cell, char, attrs)
					}
					assertChromeColor(t, cell.Style.Fg, fg)
					x++
				}
			}
			continue
		}
		if hint.text == "" {
			continue
		}
		for _, char := range " " + hint.text {
			cell := screen.CellAt(x, y)
			if cell == nil || cell.Content != string(char) || cell.Style.Bg != nil || cell.Style.Attrs != 0 {
				t.Fatalf("label glyph/style at %d,%d = %+v, want %q muted without a background", x, y, cell, char)
			}
			assertChromeColor(t, cell.Style.Fg, h.palette.Muted)
			x++
		}
	}
	painted := make([]bool, h.cols)
	statusX := h.cols - 2 - ansi.StringWidth(spansText(spans))
	for _, span := range spans {
		var attrs uint8
		if span.bold {
			attrs = uv.AttrBold
		}
		if span.reverse {
			attrs |= uv.AttrReverse
		}
		for _, char := range span.text {
			cell := screen.CellAt(statusX, y)
			if cell == nil || cell.Content != string(char) || cell.Style.Attrs != attrs {
				t.Fatalf("right-aligned status at %d,%d = %+v, want %q attrs=%d", statusX, y, cell, char, attrs)
			}
			assertChromeColor(t, cell.Style.Fg, span.fg)
			assertChromeColor(t, cell.Style.Bg, span.bg)
			painted[statusX] = span.bg != nil
			statusX++
		}
	}
	for x := range h.cols {
		if cell := screen.CellAt(x, y); cell == nil || !caps[x] && !painted[x] && cell.Style.Bg != nil {
			t.Fatalf("footer background escaped the padded keycaps at %d,%d: %+v", x, y, cell)
		}
	}
}
