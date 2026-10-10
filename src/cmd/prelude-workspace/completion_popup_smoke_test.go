package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

const smokeCompletionHint = "Tab  next   Shift+Tab  prev   Enter  accept   Esc  dismiss"

func smokeCompletionVisible(f smokeFrame) bool {
	return f.footer() == smokeCompletionHint
}

// Read the UI's selected marker, allowing either keys or full x invocations
// and arbitrary panel chrome/descriptions. No controller state is inspected.
func smokeCompletionSelected(f smokeFrame) (string, int) {
	if !smokeCompletionVisible(f) {
		return "", -1
	}
	// Search from the popup end so a redirection in older shell history is
	// not mistaken for the current selected marker.
	for y := len(f.rows) - 2; y >= 0; y-- {
		if _, suffix, ok := strings.Cut(f.rows[y], "> "); ok {
			words := strings.Fields(suffix)
			if len(words) > 0 && words[0] == "x" {
				words = words[1:]
			}
			if len(words) > 0 {
				return words[0], y
			}
		}
	}
	return "", -1
}

func smokeCompletionStarshipRows(t *testing.T, f smokeFrame, prompt, input string) {
	t.Helper()
	if prompt != "界❯" {
		return
	}
	row := f.promptRow(prompt)
	if row < 2 || !strings.HasPrefix(f.rows[row-2], "STARSHIP-REAL 界é🙂 ") || f.rows[row-1] != "│" || !f.hasInput(prompt, input) {
		t.Fatalf("completion lost part of the live multiline Starship prompt/input\n%s", strings.Join(f.rows, "\n"))
	}
}

func smokeCompletionBounds(t *testing.T, f smokeFrame, prompt, input string) {
	t.Helper()
	smokeCompletionStarshipRows(t, f, prompt, input)
	promptRow := f.promptRow(prompt)
	key, selectedRow := smokeCompletionSelected(f)
	if promptRow < 0 || !f.hasInput(prompt, input) || key == "" || selectedRow <= promptRow || selectedRow >= len(f.rows)-1 {
		t.Fatalf("completion candidates must be below the live prompt and above the protected footer: prompt=%d selected=%d key=%q\n%s", promptRow, selectedRow, key, strings.Join(f.rows, "\n"))
	}
	if f.footer() != smokeCompletionHint || strings.Count(strings.Join(f.rows, "\n"), smokeCompletionHint) != 1 {
		t.Fatalf("completion hint must occupy only the single protected final row\n%s", strings.Join(f.rows, "\n"))
	}
	assertSmokeFooterTheme(t, f, smokeCompletionKeys, "")
	candidates := 0
	for _, row := range f.rows[promptRow+1 : len(f.rows)-1] {
		if strings.TrimSpace(row) != "" {
			candidates++
		}
	}
	if candidates < 1 || candidates > 8 {
		t.Fatalf("completion must show 1–8 candidate rows without a separate hint row; got %d\n%s", candidates, strings.Join(f.rows, "\n"))
	}
}

func awaitSmokeCompletion(s *smokeOuterPTY, prompt, input string) smokeFrame {
	s.t.Helper()
	f := s.await("host completion popup with a single hint footer", func(f smokeFrame) bool {
		key, _ := smokeCompletionSelected(f)
		return key != "" && f.hasInput(prompt, input) && f.footer() == smokeCompletionHint
	})
	smokeCompletionBounds(s.t, f, prompt, input)
	return f
}

func awaitSmokeCompletionInput(s *smokeOuterPTY, prompt, input string) smokeFrame {
	s.t.Helper()
	f := s.await("completion leaves editable readline input: "+input, func(f smokeFrame) bool {
		return !smokeCompletionVisible(f) && f.hasInput(prompt, input) && f.footer() == smokeBaseFooter
	})
	smokeCompletionStarshipRows(s.t, f, prompt, input)
	return f
}

// A unique result may insert immediately, or may require popup acceptance.
// Both routes must leave it in readline, not run a main-shell command.
func acceptSmokeCompletion(s *smokeOuterPTY, prompt, want string) {
	s.t.Helper()
	words := strings.Fields(want)
	key := words[1]
	f := s.await("unique completion inserts or offers the correct command", func(f smokeFrame) bool {
		selected, _ := smokeCompletionSelected(f)
		return selected == key || !smokeCompletionVisible(f) && f.hasInput(prompt, want) && f.footer() == smokeBaseFooter
	})
	if smokeCompletionVisible(f) {
		s.send("\r")
	}
	awaitSmokeCompletionInput(s, prompt, want)
}

func exerciseSmokeCompletionPopup(s *smokeOuterPTY, prompt string) {
	s.t.Helper()
	// Force the prompt to the bottom of the child viewport. The host must
	// project it upward without resizing Bash or covering its editable input.
	s.send("printf 'POPUP_FILL\\n%.0s' {1..48}\r")
	s.await("prompt near the viewport bottom", func(f smokeFrame) bool {
		return f.hasLine("POPUP_FILL") && f.hasInput(prompt, "") && f.footer() == smokeBaseFooter
	})
	s.send("x go:\t")
	first := awaitSmokeCompletion(s, prompt, "x go:")
	firstKey, _ := smokeCompletionSelected(first)
	s.send("\t")
	next := s.await("Tab cycles the visible selected candidate", func(f smokeFrame) bool {
		key, _ := smokeCompletionSelected(f)
		return key != "" && key != firstKey && f.hasInput(prompt, "x go:") && f.footer() == smokeCompletionHint
	})
	nextKey, _ := smokeCompletionSelected(next)
	s.send("\x1b[Z")
	s.await("Shift+Tab returns to the previous selection", func(f smokeFrame) bool {
		key, _ := smokeCompletionSelected(f)
		return key == firstKey && f.hasInput(prompt, "x go:") && f.footer() == smokeCompletionHint
	})
	s.send("\t")
	s.await("cycling remains stable without changing readline", func(f smokeFrame) bool {
		key, _ := smokeCompletionSelected(f)
		return key == nextKey && f.hasInput(prompt, "x go:") && f.footer() == smokeCompletionHint
	})
	s.send("\x1b")
	closed := awaitSmokeCompletionInput(s, prompt, "x go:")
	if text := strings.Join(closed.rows[:len(closed.rows)-1], "\n"); strings.Contains(text, "go:test") || strings.Contains(text, "go:vet") {
		s.t.Fatalf("dismissed popup left completion candidates echoed into the native shell\n%s", s.diagnostics())
	}

	// Extra Tab queued during the query is navigation, not a second native
	// completion that could echo candidates into the child terminal.
	s.send("\t\t")
	s.await("rapid double-Tab navigates a multiple-match popup", func(f smokeFrame) bool {
		key, _ := smokeCompletionSelected(f)
		return key == nextKey && f.hasInput(prompt, "x go:") && f.footer() == smokeCompletionHint
	})
	s.send("\x1b")
	awaitSmokeCompletionInput(s, prompt, "x go:")
	s.send("\t")
	shown := awaitSmokeCompletion(s, prompt, "x go:")
	acceptedKey, _ := smokeCompletionSelected(shown)
	s.send("\r")
	awaitSmokeCompletionInput(s, prompt, "x "+acceptedKey)

	// Complete the full colon-containing prefix at a mid-line caret, keeping
	// an existing suffix byte-for-byte. Editing at that caret proves its point.
	const prefix, suffix = "x go:t", " -- suffix-kept"
	s.send("\x15" + prefix + suffix + "\x01" + strings.Repeat("\x1b[C", len(prefix)) + "\t")
	acceptSmokeCompletion(s, prompt, "x go:test"+suffix)
	s.send("!")
	awaitSmokeCompletionInput(s, prompt, "x go:test!"+suffix)

	s.send("\x05\x15x go:\tZ")
	awaitSmokeCompletionInput(s, prompt, "x go:Z")
	s.send("\x15printf 'POPUP_STALE_SAFE\\n'\r")
	s.await("typed input survives the completion response and a real command roundtrip", func(f smokeFrame) bool {
		return f.hasLine("POPUP_STALE_SAFE") && f.hasInput(prompt, "") && !smokeCompletionVisible(f) && f.footer() == smokeBaseFooter
	})

	s.send("x go:\t")
	awaitSmokeCompletion(s, prompt, "x go:")
	s.send("\x16\tZ")
	s.await("quoted Tab dismisses the popup and inserts literal whitespace", func(f smokeFrame) bool {
		prefix := prompt + " x go:"
		row := f.promptRow(prompt)
		if row < 0 || !strings.HasPrefix(f.rows[row], prefix) || f.footer() != smokeBaseFooter || smokeCompletionVisible(f) {
			return false
		}
		tail := strings.TrimPrefix(f.rows[row], prefix)
		return len(tail) > 1 && strings.TrimSpace(tail) == "Z"
	})
	// Replace only x go:, leaving the quoted byte in place, then let Bash
	// write it. Readline displays literal Tab as whitespace, not necessarily ^I.
	quotedTab := filepath.Join(s.t.TempDir(), "quoted-tab")
	s.send("\x01" + strings.Repeat("\x1b[3~", len("x go:")) + "\x1b[200~printf '%s' '\x1b[201~\x05" + fmt.Sprintf("' > %q\r", quotedTab))
	s.await("literal quoted Tab survives popup dismissal", func(f smokeFrame) bool {
		data, err := os.ReadFile(quotedTab)
		return err == nil && string(data) == "\tZ" && f.hasInput(prompt, "") && !smokeCompletionVisible(f) && f.footer() == smokeBaseFooter
	})
	s.send("\x15x go:\t")
	awaitSmokeCompletion(s, prompt, "x go:")
	s.send("\x1b[200~Z\x1b[201~")
	awaitSmokeCompletionInput(s, prompt, "x go:Z")

	s.send("\x15x go:\t")
	awaitSmokeCompletion(s, prompt, "x go:")
	s.send("\x1b[O")
	awaitSmokeCompletionInput(s, prompt, "x go:")
	s.send("\x1b[I")
	s.send("\t")
	awaitSmokeCompletion(s, prompt, "x go:")
	s.send("\x1d")
	s.await("layout change dismisses completion without changing input", func(f smokeFrame) bool {
		return !smokeCompletionVisible(f) && f.hasInput(prompt, "x go:") && f.hasFooter(smokeNavigationKeys, "Placement: left")
	})
	s.send("\x1c") // Return left -> floating for the existing menu tests.
	s.send("\x15x go:\t")
	awaitSmokeCompletion(s, prompt, "x go:")
	s.resize(80, 8)
	f := s.await("small workspace keeps prompt and existing footer usable", func(f smokeFrame) bool {
		return f.cols == 80 && len(f.rows) == 8 && f.hasInput(prompt, "x go:") && (f.footer() == smokeBaseFooter || f.footer() == smokeCompletionHint)
	})
	// A width change invalidates measured prompt height. Establish a fresh,
	// complete PS1 rather than requiring projection from unknown reflow data.
	if smokeCompletionVisible(f) {
		smokeCompletionBounds(s.t, f, prompt, "x go:")
		s.send("\x1b")
		awaitSmokeCompletionInput(s, prompt, "x go:")
	}
	s.send("\x0c")
	s.await("fresh full prompt after width reflow", func(f smokeFrame) bool {
		if f.footer() != smokeBaseFooter || !f.hasInput(prompt, "x go:") {
			return false
		}
		if prompt == "界❯" {
			row := f.promptRow(prompt)
			return row >= 2 && strings.HasPrefix(f.rows[row-2], "STARSHIP-REAL 界é🙂 ") && f.rows[row-1] == "│"
		}
		return true
	})
	s.send("\t")
	awaitSmokeCompletion(s, prompt, "x go:")
	s.send("\x1b")
	awaitSmokeCompletionInput(s, prompt, "x go:")

	// Exactly one candidate row must fit below the complete prompt. The old
	// popup-local hint consumed this row and suppressed the chooser entirely.
	tinyRows := 3
	if prompt == "界❯" {
		tinyRows = 5
	}
	s.resize(80, tinyRows)
	s.send("\x0c")
	tiny := s.await("full prompt leaves exactly one candidate row", func(f smokeFrame) bool {
		if f.cols != 80 || len(f.rows) != tinyRows || !f.hasInput(prompt, "x go:") || f.footer() != smokeBaseFooter {
			return false
		}
		if prompt == "界❯" {
			row := f.promptRow(prompt)
			return row >= 2 && strings.HasPrefix(f.rows[row-2], "STARSHIP-REAL 界é🙂 ") && f.rows[row-1] == "│"
		}
		return true
	})
	smokeCompletionStarshipRows(s.t, tiny, prompt, "x go:")
	s.send("\t")
	one := awaitSmokeCompletion(s, prompt, "x go:")
	_, selectedRow := smokeCompletionSelected(one)
	if selectedRow != len(one.rows)-2 || one.promptRow(prompt) != selectedRow-1 {
		s.t.Fatalf("one-row chooser changed prompt/footer geometry\n%s", s.diagnostics())
	}
	s.send("\x1b")
	awaitSmokeCompletionInput(s, prompt, "x go:")
	s.send("\x15printf 'POPUP_SIZE:'; stty size\r")
	s.await("completion preserves the native PTY height minus the single footer", func(f smokeFrame) bool {
		return f.hasLine(fmt.Sprintf("POPUP_SIZE:%d 80", tinyRows-1)) && f.hasInput(prompt, "") && f.footer() == smokeBaseFooter
	})
	s.resize(160, 40)
	// Bash can redraw only PS1's tail during external reflow. A new primary
	// prompt supplies all native rows; the host must not reconstruct PS1 source.
	s.send("\x15true\r")
	s.await("ordinary command restores a full prompt after outer grow", func(f smokeFrame) bool {
		if f.cols != 160 || len(f.rows) != 40 || smokeCompletionVisible(f) || !f.hasInput(prompt, "") || f.footer() != smokeBaseFooter {
			return false
		}
		if prompt == "界❯" {
			row := f.promptRow(prompt)
			return row >= 2 && strings.HasPrefix(f.rows[row-2], "STARSHIP-REAL 界é🙂 ") && f.rows[row-1] == "│"
		}
		return true
	})
}

func TestGhosttyCompletionPopupOwnershipBinarySmoke(t *testing.T) {
	initPath := os.Getenv("PRELUDE_COMPLETION_INIT")
	if initPath == "" {
		t.Fatal("real generated completion init is required; run in nix develop path:.#workspace")
	}
	realX, err := exec.LookPath("x")
	if err != nil {
		t.Fatal(err)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "prelude-workspace")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build prelude-workspace binary: %v (context: %v)\n%s", err, ctx.Err(), output)
	}
	for _, mode := range []struct {
		name, prompt, ps2 string
		args              []string
	}{
		{"Fixed", strings.TrimSpace(workspacePrompt), "...", nil},
		{"Starship", "界❯", "∙", []string{"--starship"}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			dir, home := t.TempDir(), t.TempDir()
			motdLog, inputLog := filepath.Join(dir, "motd.calls"), filepath.Join(dir, "foreground.input")
			writeMotdFixture(t, dir, fmt.Sprintf("builtin printf 'args:%%s\\n' \"$*\" >> %q\nbuiltin printf 'POPUP_MOTD\\n'", motdLog))
			for name, body := range map[string]string{
				"popup-reader": fmt.Sprintf("export GHOSTTY_HOST_SELECTION_READER=1\nexec %q -test.run=^TestHostSelectionPTYHelper$", helper),
				"docs":         fmt.Sprintf("export GHOSTTY_LEADER_SMOKE_TOOL=DOCS\nexec %q -test.run=^TestGhosttyLeaderSmokeToolHelper$", helper),
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(fmt.Sprintf("#!%s -p\n%s\n", bash, body)), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			if err := os.WriteFile(filepath.Join(home, "child-native-only"), nil, 0o600); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(home, "starship.toml")
			if err := os.WriteFile(config, []byte(smokeStarshipConfig), 0o600); err != nil {
				t.Fatal(err)
			}
			palettePath := filepath.Join(dir, "menu.json")
			if err := os.WriteFile(palettePath, []byte(`{"palette":{"fg":"#13579b","muted":"#2468ac","accent":"#d46cb8","accent2":"#3388cc","success":"#39b878","warning":"#c98f12","error":"#d43a5e","bg":"#010203"}}`), 0o600); err != nil {
				t.Fatal(err)
			}
			// Only bare x is a foreground reader in this ownership fixture;
			// completion imports and all argument dispatch still call real x.
			env := append(smokeEnvironment(home), "COLORTERM=truecolor", "STARSHIP_CONFIG="+config, "PRELUDE_COMPLETION_INIT="+initPath, "PRELUDE_MENU_CONFIG="+palettePath,
				"GHOSTTY_HOST_SELECTION_READY="+filepath.Join(dir, "foreground.ready"), "GHOSTTY_HOST_SELECTION_INPUT="+inputLog,
				fmt.Sprintf(`BASH_FUNC_x%%%%=() { if (( $# == 0 )); then command popup-reader; else command %q "$@"; fi; }`, realX))
			s := startSmokeOuterPTYSize(t, binary, env, 160, 24, mode.args...)
			first := s.await("initial prompt with canonical completion loaded", func(f smokeFrame) bool {
				return f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
			})
			palette := smokeKeymapPalette{key: ansi.HexColor("#3388cc"), label: ansi.HexColor("#2468ac"), bg: ansi.HexColor("#010203"), success: ansi.HexColor("#39b878")}
			assertSmokeFooterTheme(t, first, smokeNavigationKeys, "", palette)
			s.send("x\r")
			s.await("foreground child owns input even though its submitted command was x", func(f smokeFrame) bool {
				return f.hasLine("RACE_FOREGROUND_READY") && f.hasInput(mode.prompt, "x") && !smokeCompletionVisible(f) && f.footer() == smokeBaseFooter
			})
			s.send("\tforeground-user\r")
			s.await("foreground Tab was not intercepted by completion", func(f smokeFrame) bool {
				return f.hasLine("RACE_FOREGROUND_FINISHED") && f.hasInput(mode.prompt, "") && !smokeCompletionVisible(f) && f.footer() == smokeBaseFooter
			})
			if data, err := os.ReadFile(inputLog); err != nil || string(data) != "\tforeground-user" {
				t.Fatalf("foreground child received query bytes instead of its Tab: %q, %v", data, err)
			}

			s.send(fmt.Sprintf("cd %q; printf 'PS2_CWD_READY\\n'\r", home))
			s.await("isolated native PS2 completion directory", func(f smokeFrame) bool {
				return f.hasLine("PS2_CWD_READY") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
			})
			s.send("for popup_item in one; do\r")
			s.await("secondary prompt owns input", func(f smokeFrame) bool { return f.hasLine(mode.ps2) })
			s.send("x child-na\t!")
			s.await("Tab at PS2 stays native, not a host popup", func(f smokeFrame) bool {
				return f.hasLine(mode.ps2+" x child-native-only !") && !smokeCompletionVisible(f)
			})
			s.send("\x03")
			s.await("PS2 cancellation returns to a usable primary prompt", func(f smokeFrame) bool { return f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter })
			awaitSmokeStatus(s, mode.prompt, "ps2-cancel", 130)

			s.send("x go:\t")
			popup := awaitSmokeCompletion(s, mode.prompt, "x go:")
			assertSmokeFooterTheme(t, popup, smokeCompletionKeys, "", palette)
			s.send("\x10")
			leader := s.await("leader replaces completion hints on the same footer", func(f smokeFrame) bool {
				return f.hasFooter(smokeLeaderKeys, "") && !smokeCompletionVisible(f)
			})
			assertSmokeFooterTheme(t, leader, smokeLeaderKeys, "", palette)
			s.send("d")
			pane := s.await("opening a pane dismisses completion and transfers focus", func(f smokeFrame) bool {
				return f.panel("docs") && !smokeCompletionVisible(f) && f.hasPaneFooter("docs | floating | focus:pane | running")
			})
			palette.status = ansi.HexColor("#39b878")
			assertSmokeFooterTheme(t, pane, smokePaneKeys, "docs | floating | focus:pane | running", palette)
			s.send("\t\x1b[Z")
			s.await("pane owns Tab and Shift+Tab bytes", func(f smokeFrame) bool {
				return strings.Contains(strings.Join(f.rows, "\n"), "BYTES:091b5b5a") && !smokeCompletionVisible(f)
			})
			s.send("\x10?")
			notice := s.await("notice uses the palette warning color on the protected footer", func(f smokeFrame) bool {
				return f.hasPaneFooter(smokeUnknownChordStatus("docs | floating | focus:pane | running")) && f.panel("docs")
			})
			palette.status = ansi.HexColor("#c98f12")
			assertSmokeFooterTheme(t, notice, smokePaneKeys, smokeUnknownChordStatus("docs | floating | focus:pane | running"), palette)
			s.send("\x10c")
			awaitSmokeCompletionInput(s, mode.prompt, "x go:")
			s.send("\x15exit 0\r")
			s.expectExit(0)
			assertMotdCalls(t, motdLog, 1)
		})
	}
}
