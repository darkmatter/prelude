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

var spikeNavigationKeys = []keyHint{{"LOCKED", ""}, {"Ctrl+P", "unlock"}}
var spikeLeaderKeys = []keyHint{
	{"UNLOCKED", ""}, {"m", "motd"}, {"x", "menu"}, {"d", "docs"}, {"t", "window"},
	{"Tab", "focus"}, {"v", "layout"}, {"c", "close"}, {"Esc", "lock"}, {"Ctrl+P", "lock"},
}
var spikeCompletionKeys = []keyHint{{"Tab", "next"}, {"Shift+Tab", "prev"}, {"Enter", "accept"}, {"Esc", "dismiss"}}
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
				// Later views and completion must use the startup snapshot, not reread.
				if err := os.WriteFile(path, []byte("invalid after startup"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			assertChromeFooter(t, h, spikeNavigationKeys, "", nil)
			h.leader = true
			assertChromeFooter(t, h, spikeLeaderKeys[:7], "", nil)
			h.leader = false
			h.notice = "theme notice"
			assertChromeFooter(t, h, spikeNavigationKeys, "theme notice", tc.want.Warning)
			h.notice = ""
			h.pendingCommand = "echo queued"
			assertChromeFooter(t, h, spikeNavigationKeys, "command queued", tc.want.Warning)
			h.pendingCommand = ""
			h.err = errors.New("theme failure")
			assertChromeFooter(t, h, nil, "error: theme failure", tc.want.Error)
			h.err = nil

			h.Update(tea.PasteMsg{Content: "x go:"})
			awaitSpike(t, h, "completion prefix", func() bool { return spikeInput(h) == "x go:" })
			spikeKey(h, tea.KeyTab, 0)
			awaitSpike(t, h, "themed command candidates", func() bool { return h.completionVisible() || h.notice != "" })
			if !h.completionVisible() {
				t.Fatalf("completion was suppressed: notice=%q ready=%t anchor=%d cursor=%d,%d frame=%dx%d", h.notice, h.prompt.ready, h.prompt.anchor, h.frame.CursorX, h.frame.CursorY, h.frame.Cols, h.frame.Rows)
			}
			screen, lines := assertChromeFooter(t, h, spikeCompletionKeys, "", nil)
			if strings.Count(strings.Join(lines, "\n"), completionHint) != 1 {
				t.Fatal("completion hints must appear only on the protected last row")
			}
			area := h.layout().Completion
			c := h.completion
			if area.Empty() || len(c.snapshot.candidates) < 2 || area.Max.Y > h.rows-1 {
				t.Fatalf("candidates must fit above the footer: area=%v candidates=%d", area, len(c.snapshot.candidates))
			}
			for row := range min(area.Dy(), len(c.snapshot.candidates)) {
				selected := row == c.selected
				fg := tc.want.Fg
				if selected {
					fg = tc.want.Accent
				}
				assertChromeColor(t, screen.CellAt(area.Min.X+2, area.Min.Y+row).Style.Fg, fg)
				for x := area.Min.X; x < area.Max.X; x++ {
					cell := screen.CellAt(x, area.Min.Y+row)
					if cell.Style.Bg != nil || strings.TrimSpace(cell.Content) != "" && (cell.Style.Attrs&uv.AttrBold != 0) != selected {
						t.Fatalf("candidate must be transparent, with only the selection bold: %+v", cell)
					}
				}
			}
			spikeKey(h, tea.KeyEscape, 0)
			spikeKey(h, 'u', tea.ModCtrl)
			h.Update(tea.PasteMsg{Content: `printf '\033[999;1HCHROME-CHILD-OUTPUT\033[H'`})
			spikeKey(h, tea.KeyEnter, 0)
			awaitSpike(t, h, "child output at its bottom margin", func() bool {
				lastRow := strings.Split(spikeFrameText(h.frame), "\n")[h.frame.Rows-1]
				return h.prompt.ready && strings.HasPrefix(lastRow, "CHROME-CHILD-OUTPUT")
			})
			assertChromeFooter(t, h, spikeNavigationKeys, "", nil)
		})
	}
}

func assertChromeColor(t *testing.T, got, want color.Color) {
	t.Helper()
	if (got == nil) != (want == nil) || want != nil && color.RGBAModel.Convert(got) != color.RGBAModel.Convert(want) {
		t.Fatalf("color = %v, want %v", got, want)
	}
}

func assertChromeFooter(t *testing.T, h *host, hints []keyHint, status string, fg color.Color) (*vt.Emulator, []string) {
	t.Helper()
	assertSpikeFooter(t, h, spikeFooterKeyText(hints), status)
	view := h.View()
	lines := strings.Split(ansi.Strip(view.Content), "\n")
	screen := vt.NewEmulator(h.cols, h.rows)
	t.Cleanup(func() { screen.Close() })
	if _, err := screen.Write([]byte(strings.ReplaceAll(view.Content, "\n", "\r\n"))); err != nil {
		t.Fatal(err)
	}
	assertChromeFooterCells(t, screen, h, hints, status, fg)
	for y := 0; y < h.rows-1; y++ {
		for x := range h.cols {
			if cell := screen.CellAt(x, y); cell == nil || cell.Style.Bg != nil {
				t.Fatalf("keycap palette leaked a backdrop into the native canvas/candidates at %d,%d: %+v", x, y, cell)
			}
		}
	}
	return screen, lines
}

func assertChromeFooterCells(t *testing.T, screen *vt.Emulator, h *host, hints []keyHint, status string, fg color.Color) {
	t.Helper()
	caps := make([]bool, h.cols)
	x, y := 2, h.rows-1
	for i, hint := range hints {
		if i > 0 {
			x += 2
		}
		keyFG, keyBG := h.palette.Accent2, h.palette.Bg
		switch hint.key {
		case "LOCKED":
			keyFG = h.palette.Muted
		case "UNLOCKED":
			keyFG, keyBG = h.palette.Bg, h.palette.Success
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
	for x := range h.cols {
		if cell := screen.CellAt(x, y); cell == nil || !caps[x] && cell.Style.Bg != nil {
			t.Fatalf("footer background escaped the padded keycaps at %d,%d: %+v", x, y, cell)
		}
	}
	statusX := h.cols - 2 - ansi.StringWidth(status)
	for _, char := range status {
		cell := screen.CellAt(statusX, y)
		if cell == nil || cell.Content != string(char) || cell.Style.Bg != nil || cell.Style.Attrs != uv.AttrBold {
			t.Fatalf("right-aligned status at %d,%d = %+v, want %q bold without a background", statusX, y, cell, char)
		}
		assertChromeColor(t, cell.Style.Fg, fg)
		statusX++
	}
}
