package main

import (
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

func completionStarshipPromptRow(rows []string) int {
	for y := len(rows) - 3; y >= 0; y-- {
		if strings.HasPrefix(strings.TrimSpace(rows[y]), "STARSHIP-REAL 界é🙂") &&
			strings.TrimSpace(rows[y+1]) == "│" && strings.HasPrefix(strings.TrimSpace(rows[y+2]), "界❯") {
			return y
		}
	}
	return -1
}

// Wait for real Bash output before asserting that a chooser is absent. A Tab
// still sitting in Readline's queue must not look like safe suppression.
func awaitHostCompletionQuery(t *testing.T, h *host) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	query := regexp.MustCompile(`\x1b\]133;Q;[0-9]+(?:\x07|\x1b\\)`)
	var output []byte
	for !query.Match(output) {
		select {
		case msg := <-h.child.output:
			output = append(output, msg.data...)
			h.Update(msg)
			if h.err != nil {
				t.Fatal(h.err)
			}
		case <-h.child.exited:
			t.Fatal("Bash exited before the completion query response")
		case <-timer.C:
			t.Fatal("waiting for the real completion query response")
		}
	}
}

// Observe the real PTY, independent native snapshot, and rendered outer screen.
// Reading TIOCGWINSZ observes the same kernel geometry as child `stty size`
// without dismissing the chooser to run a probe command.
func assertCompletionOverlay(t *testing.T, h *host, baseline terminalFrame, input string, popup, shifted bool) int {
	t.Helper()
	native, err := h.terminal.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if size := frontendPTYSize(t, h.child); size != [2]int{baseline.Cols, baseline.Rows} ||
		native.Cols != baseline.Cols || native.Rows != baseline.Rows || h.frame.Cols != baseline.Cols || h.frame.Rows != baseline.Rows {
		t.Fatalf("completion changed Bash PTY/Ghostty geometry: PTY=%v native=%dx%d cached=%dx%d, want %dx%d", size, native.Cols, native.Rows, h.frame.Cols, h.frame.Rows, baseline.Cols, baseline.Rows)
	}
	cached := h.frame
	cached.Cells = append([]uv.Cell(nil), cached.Cells...)
	view := h.View()
	after, err := h.terminal.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cached, h.frame) || !reflect.DeepEqual(native, after) {
		t.Fatal("display-only projection mutated the cached or native terminal snapshot")
	}
	nativeRows := strings.Split(spikeFrameText(native), "\n")[:native.Rows]
	rows := strings.Split(ansi.Strip(view.Content), "\n")
	nativeStart, displayedStart := completionStarshipPromptRow(nativeRows), completionStarshipPromptRow(rows[:len(rows)-1])
	if nativeStart < 0 || displayedStart < 0 {
		t.Fatalf("completion lost Starship's header, middle, or final row: native=%d display=%d\nnative:\n%s\ndisplay:\n%s", nativeStart, displayedStart, spikeFrameText(native), ansi.Strip(view.Content))
	}
	shift := nativeStart - displayedStart
	if shifted && shift <= 0 || !shifted && shift != 0 {
		t.Fatalf("prompt display shift=%d, want shifted=%t\n%s", shift, shifted, ansi.Strip(view.Content))
	}
	inputRows := (ansi.StringWidth("界❯ "+input) + native.Cols - 1) / native.Cols
	blockRows := 2 + inputRows
	if nativeStart+blockRows > native.Rows || displayedStart+blockRows >= len(rows) {
		t.Fatal("full multiline prompt and wrapped input do not fit before the footer")
	}
	outer := vt.NewEmulator(h.cols, h.rows)
	defer outer.Close()
	if _, err := outer.Write([]byte(strings.ReplaceAll(view.Content, "\n", "\r\n"))); err != nil {
		t.Fatal(err)
	}
	for dy := 0; dy < blockRows; dy++ {
		if strings.TrimRight(nativeRows[nativeStart+dy], " ") != strings.TrimRight(rows[displayedStart+dy], " ") {
			t.Fatalf("projection changed prompt/input row %d\n%s", dy, ansi.Strip(view.Content))
		}
		for x := 0; x < native.Cols; x++ {
			want := native.Cells[(nativeStart+dy)*native.Cols+x]
			if want.Width <= 0 || strings.TrimSpace(want.Content) == "" {
				continue
			}
			got := outer.CellAt(x, displayedStart+dy)
			if got == nil || got.Content != want.Content || got.Width != want.Width || got.Style.Attrs != want.Style.Attrs ||
				(got.Style.Fg == nil) != (want.Style.Fg == nil) || want.Style.Fg != nil && color.RGBAModel.Convert(got.Style.Fg) != color.RGBAModel.Convert(want.Style.Fg) {
				t.Fatalf("projection lost native glyph/style at %d,%d: got=%+v want=%+v", x, dy, got, want)
			}
		}
		if dy < 3 && native.Cells[(nativeStart+dy)*native.Cols].Style.Fg == nil {
			t.Fatalf("Starship fixture row %d is not colored", dy)
		}
	}
	if view.Cursor == nil || view.Cursor.X != native.CursorX || view.Cursor.Y != native.CursorY-shift || view.Cursor.Shape != tea.CursorBar {
		t.Fatalf("cursor did not follow display-only prompt projection: got=%+v native=%d,%d shift=%d", view.Cursor, native.CursorX, native.CursorY, shift)
	}
	frame := smokeFrame{cols: h.cols, rows: rows}
	selectedKey, _ := smokeCompletionSelected(frame)
	visible := selectedKey != "" || strings.Contains(rows[len(rows)-1], completionHint)
	if visible != popup {
		t.Fatalf("popup visible=%t, want %t\n%s", visible, popup, ansi.Strip(view.Content))
	}
	layout := h.layout()
	if len(rows) != h.rows || layout.FooterY != h.rows-1 || layout.Shell.Dy() != baseline.Rows {
		t.Fatalf("completion changed the full shell or protected footer geometry: layout=%+v rows=%d", layout, len(rows))
	}
	if popup {
		key, selectedRow := smokeCompletionSelected(frame)
		area := h.completionArea(layout)
		wantHeight := min(len(h.completion.snapshot.candidates), 8, native.Rows-blockRows)
		if key == "" || selectedRow < displayedStart+blockRows || area.Min.Y != displayedStart+blockRows ||
			area.Dy() != wantHeight || area.Dy() < 1 || area.Max.Y > layout.FooterY {
			t.Fatalf("completion covered prompt/input or reserved an extra hint row: selected=%q row=%d area=%v wantHeight=%d\n%s", key, selectedRow, area, wantHeight, ansi.Strip(view.Content))
		}
		for y := area.Min.Y; y < area.Max.Y; y++ {
			if strings.TrimSpace(rows[y]) == "" || strings.Contains(rows[y], completionHint) {
				t.Fatalf("chooser row %d is not a candidate: %q", y, rows[y])
			}
		}
		if strings.Count(ansi.Strip(view.Content), completionHint) != 1 || !strings.Contains(rows[layout.FooterY], completionHint) ||
			strings.Contains(strings.Join(rows[:layout.FooterY], "\n"), completionHint) {
			t.Fatalf("completion must have exactly one hint footer at the last row, never below candidates\n%s", ansi.Strip(view.Content))
		}
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				if cell := outer.CellAt(x, y); cell == nil || cell.Style.Bg != nil {
					t.Fatalf("completion cell %d,%d lost its transparent style: %+v", x, y, cell)
				}
			}
		}
	} else if footer, want := strings.TrimSpace(rows[layout.FooterY]), strings.TrimSpace(baseFooter); footer != want {
		t.Fatalf("completion changed the base hint footer: %q, want %q", footer, want)
	}
	hints := spikeNavigationKeys
	if popup {
		hints = spikeCompletionKeys
	}
	assertChromeFooterCells(t, outer, h, hints, "", nil)
	// bind-x redisplay may normalize erased blank-cell styles. Its output is
	// real Bash output; projection itself must still leave both full snapshots
	// byte-for-byte intact across View, as checked above.
	if popup && spikeFrameText(baseline) != spikeFrameText(native) {
		t.Fatal("showing/cycling the overlay changed the native shell text")
	}
	return shift
}

func TestHostCompletionOverlayKeepsNativeStarshipGeometry(t *testing.T) {
	if os.Getenv("PRELUDE_COMPLETION_INIT") == "" {
		t.Fatal("real generated completion init is required; run in nix develop path:.#ghostty-spike")
	}
	for _, tc := range []struct {
		name, prefix, suffix string
		rows                 int
		bottom, hidden       bool
	}{
		{name: "NaturalBlankSpace", rows: 14, prefix: "x go:"},
		{name: "BottomWrappedInput", rows: 14, prefix: "x go:", suffix: " -- " + strings.Repeat("wrap-", 22) + "TAIL", bottom: true},
		{name: "NearSmallWindow", rows: 8, prefix: "x", bottom: true},
		{name: "OneCandidateRow", rows: 5, prefix: "x", bottom: true},
		{name: "NoSafeSpace", rows: 4, prefix: "x", bottom: true, hidden: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			motdDir := t.TempDir()
			motd := fixtureMotd
			if tc.bottom {
				// Fill naturally before the first PS1, without an extra command or
				// prompt status transition that could obscure projection side effects.
				motd = "builtin printf 'OVERLAY_FILL\\n%.0s' {1..48}"
			}
			writeMotdFixture(t, motdDir, motd)
			t.Setenv("PATH", motdDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			home := t.TempDir()
			config := filepath.Join(home, "starship.toml")
			if err := os.WriteFile(config, []byte(smokeStarshipConfig), 0o600); err != nil {
				t.Fatal(err)
			}
			runLog := filepath.Join(home, "x.executions")
			env := append(smokeEnvironment(home), "STARSHIP_CONFIG="+config, "GHOSTTY_COMPLETION_EXECUTION_LOG="+runLog,
				`BASH_FUNC_x%%=() { if [[ ${1-} == --imports ]]; then command x "$@"; else builtin printf '%s\n' "$*" >> "$GHOSTTY_COMPLETION_EXECUTION_LOG"; return 23; fi; }`)
			h, err := newHost(100, tc.rows, env, true)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := h.close(); err != nil {
					t.Error(err)
				}
			})
			awaitSpike(t, h, "real multiline Starship prompt", func() bool { return h.prompt.ready && spikeInput(h) == "" })
			input := tc.prefix + tc.suffix
			h.Update(tea.PasteMsg{Content: input})
			awaitSpike(t, h, "editable input before completion", func() bool { return spikeInput(h) == input })
			if tc.suffix != "" {
				spikeKey(h, 'a', tea.ModCtrl)
				for range len(tc.prefix) {
					spikeKey(h, tea.KeyRight, 0)
				}
				awaitSpike(t, h, "mid-line caret before wrapped suffix", func() bool { return h.frame.CursorX == 4+len(tc.prefix) })
			}
			baseline, err := h.terminal.Snapshot()
			if err != nil {
				t.Fatal(err)
			}
			assertCompletionOverlay(t, h, baseline, input, false, false)
			spikeKey(h, tea.KeyTab, 0)
			if tc.hidden {
				awaitHostCompletionQuery(t, h)
				awaitSpike(t, h, "Readline redraw after small-window query", func() bool { return h.prompt.ready && spikeInput(h) == input })
				view := strings.Split(ansi.Strip(h.View().Content), "\n")
				if strings.Contains(view[len(view)-1], completionHint) || completionStarshipPromptRow(view[:len(view)-1]) != 0 ||
					spikeFrameText(baseline) != spikeFrameText(h.frame) {
					t.Fatalf("unsafe chooser covered the prompt or changed the native screen\n%s", strings.Join(view, "\n"))
				}
				// Dismiss any pre-existing resize notice before the exact footer check.
				spikeKey(h, tea.KeyEscape, 0)
				assertCompletionOverlay(t, h, baseline, input, false, false)
				return
			}
			awaitSpike(t, h, "popup below the complete redrawn prompt", func() bool {
				return h.prompt.ready && spikeInput(h) == input && strings.Contains(ansi.Strip(h.View().Content), completionHint)
			})
			shift := assertCompletionOverlay(t, h, baseline, input, true, tc.bottom)
			first, _ := smokeCompletionSelected(smokeFrame{rows: strings.Split(ansi.Strip(h.View().Content), "\n")})
			spikeKey(h, tea.KeyTab, 0)
			if got, _ := smokeCompletionSelected(smokeFrame{rows: strings.Split(ansi.Strip(h.View().Content), "\n")}); got == first || got == "" {
				t.Fatal("Tab did not cycle the displayed candidate")
			}
			if got := assertCompletionOverlay(t, h, baseline, input, true, tc.bottom); got != shift {
				t.Fatal("cycling shifted the native viewport again")
			}
			spikeKey(h, tea.KeyTab, tea.ModShift)
			assertCompletionOverlay(t, h, baseline, input, true, tc.bottom)
			spikeKey(h, tea.KeyEscape, 0)
			assertCompletionOverlay(t, h, baseline, input, false, false)
			if spikeFrameText(baseline) != spikeFrameText(h.frame) {
				t.Fatal("opening/cycling/cancelling changed the native shell text")
			}
			spikeKey(h, tea.KeyTab, 0)
			awaitSpike(t, h, "reopened display-only popup with redrawn input", func() bool {
				return h.prompt.ready && spikeInput(h) == input && strings.Contains(ansi.Strip(h.View().Content), completionHint)
			})
			assertCompletionOverlay(t, h, baseline, input, true, tc.bottom)
			spikeKey(h, tea.KeyEnter, 0)
			want := "x " + first + tc.suffix
			awaitSpike(t, h, "acceptance edits Readline without executing", func() bool {
				return h.prompt.ready && strings.TrimSpace(spikeInput(h)) == want && !strings.Contains(ansi.Strip(h.View().Content), completionHint)
			})
			assertCompletionOverlay(t, h, baseline, want, false, false)
			if _, err := os.Stat(runLog); !os.IsNotExist(err) {
				t.Fatalf("completion acceptance executed its candidate instead of editing Readline: %v", err)
			}
			if tc.suffix != "" {
				spikeKey(h, 'Z', 0)
				want = "x " + first + "Z" + tc.suffix
				awaitSpike(t, h, "accepted completion retains mid-line caret and suffix", func() bool { return spikeInput(h) == want })
				assertCompletionOverlay(t, h, baseline, want, false, false)
			}
		})
	}
}
