package main

import (
	"context"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// No mock Starship: this config exercises the installed binary's ANSI wrapping,
// multiline/wide PS1, status/PIPESTATUS handling, and PS0 command timer.
const smokeStarshipConfig = `add_newline = false
format = "[STARSHIP-REAL 界é🙂](bold green) $status$cmd_duration\n[│](blue)\n[界❯](bold cyan) "

[status]
disabled = false
format = "[status:$status](red) "
success_symbol = "0"
symbol = ""
pipestatus = true
pipestatus_format = "[status:$status pipe:$pipestatus](yellow) "
pipestatus_separator = ","
pipestatus_segment_format = "$status"

[cmd_duration]
min_time = 0
show_milliseconds = true
format = "[time:$duration](blue) "
`

// Editable-input checkpoints start at ❯; full colored/wide prompt and repaint
// coverage is asserted separately.
func (f smokeFrame) starshipInput(input string) bool {
	return f.hasInputAfter(image.Rect(0, 0, f.cols, len(f.rows)-1), "❯", input)
}

func starshipSmokeInput(s *smokeOuterPTY, input string) smokeFrame {
	s.t.Helper()
	return s.await("visible Starship readline input: "+input, func(f smokeFrame) bool {
		return f.footer() == smokeBaseFooter && f.starshipInput(input)
	})
}

func (f smokeFrame) generatedPromptKeymap() bool {
	row := f.promptRow("╰─")
	if row < 2 || f.rows[row-1] != "│" {
		return false
	}
	header := f.rows[row-2]
	const legend = "Alt + [m]─motd──[x]─menu──[d]─docs"
	return strings.Contains(header, "╭░▒▓ π ") && strings.Count(header, legend) == 1 &&
		!strings.Contains(header, "Ctrl+P") && !strings.Contains(header, "[?]")
}

func assertGeneratedPromptStyle(s *smokeOuterPTY, f smokeFrame) {
	s.t.Helper()
	const legend = "Alt + [m]─motd──[x]─menu──[d]─docs"
	y := f.promptRow("╰─") - 2
	header := f.rows[y]
	x := ansi.StringWidth(header[:strings.Index(header, legend)])
	s.mu.Lock()
	defer s.mu.Unlock()
	accent := s.emulator.CellAt(x+len("Alt + ["), y).Style.Fg
	muted := s.emulator.CellAt(x+len("Alt + "), y).Style.Fg
	if accent == nil || muted == nil || smokeColor(accent) == smokeColor(muted) {
		s.t.Fatal("generated prompt key/label colors must retain distinct accent/muted styling")
	}
	for _, key := range []string{"m", "x", "d"} {
		keyX := x + ansi.StringWidth(legend[:strings.Index(legend, "["+key+"]")]) + 1
		cell := s.emulator.CellAt(keyX, y)
		if cell.Content != key || cell.Style.Attrs != uv.AttrBold || cell.Style.Bg != nil || smokeColor(cell.Style.Fg) != smokeColor(accent) {
			s.t.Fatalf("generated bracketed shortcut %s lost its original bold accent style: %+v", key, cell)
		}
		for _, bracketX := range []int{keyX - 1, keyX + 1} {
			bracket := s.emulator.CellAt(bracketX, y)
			if bracket.Style.Attrs != 0 || bracket.Style.Bg != nil || smokeColor(bracket.Style.Fg) != smokeColor(muted) {
				s.t.Fatalf("generated shortcut bracket gained keycap-style background/bold: %+v", bracket)
			}
		}
	}
}

func TestGhosttyStarshipBinarySmoke(t *testing.T) {
	starship, err := exec.LookPath("starship")
	if err != nil {
		t.Fatal("real Starship is required; run in nix develop path:/home/cm/git/darkmatter/prelude#ghostty-spike:", err)
	}
	t.Logf("real Starship: %s", starship)
	binary := filepath.Join(t.TempDir(), "ghostty-spike")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build real Ghostty binary: %v (context: %v)\n%s", err, ctx.Err(), output)
	}

	t.Run("RealPromptStatusReadlineAndParkedPanes", func(t *testing.T) {
		leaderSmokeTools(t)
		home := t.TempDir()
		config := filepath.Join(home, "starship.toml")
		if err := os.WriteFile(config, []byte(smokeStarshipConfig), 0o600); err != nil {
			t.Fatal(err)
		}
		poison := filepath.Join(home, "poison.bash")
		for _, name := range []string{poison, filepath.Join(home, ".bashrc")} {
			if err := os.WriteFile(name, []byte("printf 'INHERITED_HOOK_RAN\\n'\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range []string{"fixture-alpha", "fixture-beta"} {
			if err := os.WriteFile(filepath.Join(home, name), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		env := append(smokeEnvironment(home), "STARSHIP_CONFIG="+config,
			"BASH_ENV="+poison, "ENV="+poison, "BLE_VERSION=400", "BLE_ATTACHED=1", "BLE_PIPESTATUS=73",
			"bash_preexec_imported=1", "__bp_imported=1", "BP_PIPESTATUS=99",
			"preexec_functions=poison", "precmd_functions=poison", "starship_precmd_user_func=spike_tool",
			"STARSHIP_PROMPT_COMMAND=printf INHERITED_HOOK_RAN", "STARSHIP_START_TIME=1",
			"BASH_FUNC_blehook%%=() { builtin printf 'INHERITED_HOOK_RAN\\n'; }",
			"BASH_FUNC_bleopt%%=() { builtin printf 'INHERITED_HOOK_RAN\\n'; }",
			"BASH_FUNC__ble_version%%=() { return 0; }",
			"BASH_FUNC_spike_tool%%=() { builtin printf 'PROJECT_TOOLS_OK\\n'; }")
		s := startSmokeOuterPTYEnv(t, binary, env, "--starship")
		first := s.await("colored multiline/wide Starship and hint footer", func(f smokeFrame) bool {
			return f.alt && f.starshipStatus("", "status:0", false) && f.footer() == smokeBaseFooter
		})
		for y, row := range first.rows {
			if strings.HasPrefix(row, "STARSHIP-REAL") {
				column := len("STARSHIP-REAL ")
				if got := first.cells[y*first.cols+column]; got != (smokeCell{"界", 2}) {
					t.Fatalf("Starship wide prompt cell = %+v\n%s", got, s.diagnostics())
				}
				s.mu.Lock()
				colored := s.emulator.CellAt(0, y).Style.Fg != nil
				s.mu.Unlock()
				if !colored {
					t.Fatal("Starship prompt lost its actual ANSI color")
				}
			}
		}
		for _, step := range []struct {
			command string
			status  string
		}{
			{"true", "status:0"},
			{"false", "status:1"},
			{"false | true", "status:0 pipe:1,0"},
			{"set -o pipefail; false | true", "status:1 pipe:1,0"},
			{"set +o pipefail; true | false", "status:1 pipe:0,1"},
		} {
			s.send(step.command + "\r")
			s.await("live Starship status for "+step.command, func(f smokeFrame) bool {
				return f.starshipStatus("", step.status, false) && f.footer() == smokeBaseFooter
			})
		}
		const timedCommand = "printf 'STARSHIP_TIMER_RUNNING\\n'; sleep 0.25; true"
		s.send(timedCommand + "\r")
		s.await("foreground command starts without losing submitted input", func(f smokeFrame) bool {
			return f.hasLine("STARSHIP_TIMER_RUNNING") && f.hasInput("界❯", timedCommand) && f.footer() == smokeBaseFooter
		})
		s.await("Starship PS0 timer and successful prompt", func(f smokeFrame) bool {
			return f.starshipStatus("", "status:0", true) && f.footer() == smokeBaseFooter
		})
		const interruptCommand = "printf 'STARSHIP_INTERRUPT_READY\\n'; sleep 5"
		s.send(interruptCommand + "\r")
		s.await("interruptible Starship command", func(f smokeFrame) bool {
			return f.hasLine("STARSHIP_INTERRUPT_READY") && f.hasInput("界❯", interruptCommand) && f.footer() == smokeBaseFooter
		})
		s.send("\x03")
		s.await("Ctrl+C foreground status", func(f smokeFrame) bool {
			return f.starshipStatus("", "status:130", false) && f.footer() == smokeBaseFooter
		})
		s.send("cancel-me")
		s.await("parked input before interrupt", func(f smokeFrame) bool {
			return f.starshipStatus("cancel-me", "status:130", false) && f.footer() == smokeBaseFooter
		})
		s.send("\x03")
		s.await("Ctrl+C parked input status", func(f smokeFrame) bool {
			return f.starshipStatus("", "status:130", false) && f.footer() == smokeBaseFooter
		})
		s.send("true\r")
		starshipSmokeInput(s, "")

		s.send("echo typed-XY")
		starshipSmokeInput(s, "echo typed-XY")
		s.send("\x7f\x7fok\x01" + strings.Repeat("\x1b[C", 5) + "\x1b[3~")
		starshipSmokeInput(s, "echo yped-ok")
		s.send("t\r")
		s.await("executed edited Starship input", func(f smokeFrame) bool {
			return f.hasLine("typed-ok") && f.starshipStatus("", "status:0", false) && f.footer() == smokeBaseFooter
		})
		s.send("\x1b[A")
		starshipSmokeInput(s, "echo typed-ok")
		s.send("\x15spike_too\t")
		starshipSmokeInput(s, "spike_tool ")
		s.send("\r")
		s.await("project tools survive isolated shell hooks", func(f smokeFrame) bool {
			return f.hasLine("PROJECT_TOOLS_OK") && f.starshipStatus("", "status:0", false) && f.footer() == smokeBaseFooter
		})
		s.send(fmt.Sprintf("cd %q; printf 'CWD_READY\\n'\r", home))
		s.await("completion fixture directory", func(f smokeFrame) bool {
			return f.hasLine("CWD_READY") && f.starshipStatus("", "status:0", false) && f.footer() == smokeBaseFooter
		})
		s.send("cat fixture-\t\t")
		s.await("ambiguous completion lists and redraws real Starship", func(f smokeFrame) bool {
			return strings.Contains(strings.Join(f.rows, "\n"), "fixture-alpha  fixture-beta") && f.hasInput("界❯", "cat fixture-") && f.footer() == smokeBaseFooter
		})
		s.send("\x15echo 界é🙂")
		starshipSmokeInput(s, "echo 界é🙂")
		s.send("\x0c")
		s.await("Ctrl+L redraw retains wide editable input", func(f smokeFrame) bool {
			return f.hasInput("界❯", "echo 界é🙂") && f.footer() == smokeBaseFooter
		})

		input := "echo " + strings.Repeat("abcd", 28) + "-WRAP-TAIL"
		s.send("\x15" + input)
		starshipSmokeInput(s, input)
		s.resize(60, 8)
		s.await("wrapped input scroll/reflow after outer shrink", func(f smokeFrame) bool {
			return f.cols == 60 && len(f.rows) == 8 && f.hasInput("界❯", input) && f.footer() == smokeBaseFooter
		})
		s.resize(180, 24)
		s.await("outer grow reconstructs the entire wrapped input", func(f smokeFrame) bool {
			return f.cols == 180 && f.hasInput("界❯", input) && f.footer() == smokeBaseFooter
		})
		s.send("\x01" + strings.Repeat("\x1b[C", 5) + "\x1b[3~")
		starshipSmokeInput(s, "echo "+strings.TrimPrefix(input, "echo a"))
		s.send("a\x05\x0c")
		starshipSmokeInput(s, input)
		// A tiny viewport crops the real prompt rather than reconstructing it
		// in a second input/status row. A fresh readline redraw recovers it.
		input = "echo " + strings.Repeat("abcd", 60) + "-OFFSCREEN-TAIL"
		s.send("\x15" + input)
		s.resize(90, 3)
		s.await("off-screen prompt leaves only native input tail and one footer", func(f smokeFrame) bool {
			return f.cols == 90 && len(f.rows) == 3 && f.promptRow("界❯") < 0 &&
				strings.Contains(strings.Join(f.rows[:len(f.rows)-1], ""), "-OFFSCREEN-TAIL") && f.footer() == smokeBaseFooter
		})
		s.resize(320, 24)
		s.send("\x0c")
		starshipSmokeInput(s, input)
		s.send("\r")
		s.await("recovered off-screen input executes without corruption", func(f smokeFrame) bool {
			return f.hasLine(strings.TrimPrefix(input, "echo ")) && f.hasInput("界❯", "") && f.footer() == smokeBaseFooter
		})
		s.send("echo parked-界")
		starshipSmokeInput(s, "echo parked-界")
		s.resize(160, 28)
		s.await("pane workspace resize completed", func(f smokeFrame) bool {
			return f.cols == 160 && len(f.rows) == 28 && f.hasInput("界❯", "echo parked-界") && f.footer() == smokeBaseFooter
		})
		parked := "echo parked-界"
		for _, surface := range []struct{ key, kind string }{{"x", "menu"}, {"d", "docs"}} {
			before := s.frame()
			s.send("\x10" + surface.key)
			shown := s.await("Starship input parked behind "+surface.kind, func(f smokeFrame) bool {
				return f.panel(surface.kind) && f.hasPaneFooter(surface.kind+" | floating | focus:pane | running")
			})
			marker := "LEADER_" + strings.ToUpper(surface.kind) + ":"
			var identity string
			for _, row := range shown.rows {
				// The floating pane can share this row with editable Bash text.
				// Only its marker/PID, not the underlay beside it, is identity.
				if _, suffix, found := strings.Cut(row, marker); found {
					if fields := strings.Fields(suffix); len(fields) > 0 {
						identity = marker + fields[0]
					}
				}
			}
			if identity == "" {
				t.Fatalf("pane PID marker missing after first paint: %s", s.diagnostics())
			}
			s.send("N\x10" + surface.key)
			s.await("hide restores exact parked Starship screen", func(f smokeFrame) bool {
				return strings.Join(f.rows[:len(f.rows)-1], "\n") == strings.Join(before.rows[:len(before.rows)-1], "\n") &&
					f.starshipInput(parked) && f.hasPaneFooter(surface.kind+" hidden | running")
			})
			s.send("-h")
			parked += "-h"
			s.await("hidden pane leaves Bash editable", func(f smokeFrame) bool {
				return f.starshipInput(parked) && f.hasPaneFooter(surface.kind+" hidden | running")
			})
			s.send("\x10" + surface.key)
			s.await("show retains same pane PID and input", func(f smokeFrame) bool {
				return strings.Contains(strings.Join(f.rows, "\n"), identity) && strings.Contains(strings.Join(f.rows, "\n"), "BYTES:4e") &&
					f.hasPaneFooter(surface.kind+" | floating | focus:pane | running")
			})
			if surface.kind == "menu" {
				s.send("\x1d")
				s.await("docking reflows parked Starship without taking its input", func(f smokeFrame) bool {
					shell := image.Rect(f.cols/2, 0, f.cols, len(f.rows)-1)
					pane := image.Rect(0, 0, f.cols/2, len(f.rows)-1)
					return f.hasPaneFooter("Placement: left | menu | left | focus:pane | running") &&
						f.hasInputAfter(shell, "❯", parked) && strings.TrimSpace(f.textAt(0, 0, pane.Dx())) == identity &&
						strings.Contains(leaderSmokeBody(f, pane), "BYTES:4e")
				})
				s.send("\x1c")
				s.await("undocking restores full wide Starship prompt and parked input", func(f smokeFrame) bool {
					return f.hasPaneFooter("Placement: floating | menu | floating | focus:pane | running") &&
						f.hasInput("界❯", parked) && f.panel("menu") && strings.Contains(strings.Join(f.rows, "\n"), identity) &&
						strings.Contains(strings.Join(f.rows, "\n"), "BYTES:4e")
				})
			}
			s.send("\x10c")
			s.await("pane close retains full wide Starship prompt and parked input", func(f smokeFrame) bool {
				return f.footer() == smokeBaseFooter && !f.panel(surface.kind) && f.hasInput("界❯", parked)
			})
			starshipSmokeInput(s, parked)
		}
		// Execution proves this edit landed at the restored caret, without
		// imposing a new decorative-glyph/renderer contract on the input check.
		parked += "-caret"
		s.send("-caret\r")
		s.await("parked input and restored caret still execute in Bash", func(f smokeFrame) bool {
			return f.hasLine(strings.TrimPrefix(parked, "echo ")) && f.starshipStatus("", "status:0", false) && f.footer() == smokeBaseFooter
		})
		s.traceMu.Lock()
		poisonRan := strings.Contains(string(s.output), "INHERITED_HOOK_RAN")
		s.traceMu.Unlock()
		if poisonRan {
			t.Fatalf("private Bash inherited a ble/bash-preexec/user-rc hook\n%s", s.diagnostics())
		}
		s.send("exit 0\r")
		s.expectExit(0)
	})

	t.Run("GeneratedPreludeConfig", func(t *testing.T) {
		leaderSmokeTools(t)
		config := os.Getenv("PRELUDE_GHOSTTY_TEST_STARSHIP_CONFIG")
		if config == "" {
			t.Fatal("generated Starship preset required; run in nix develop path:.#ghostty-spike")
		}
		if _, err := os.Stat(config); err != nil {
			t.Fatal("generated Prelude Starship config:", err)
		}
		t.Setenv("STARSHIP_CONFIG", config)
		s := startSmokeOuterPTYSize(t, binary, smokeEnvironment(t.TempDir()), 160, 28, "--starship")
		first := s.await("generated prompt restores bracketed Alt + m MOTD/x menu/d docs with LOCKED footer", func(f smokeFrame) bool {
			return f.generatedPromptKeymap() && f.hasInput("╰─", "") && f.hasFooter(smokeNavigationKeys, "")
		})
		assertSmokeFooterTheme(t, first, smokeNavigationKeys, "")
		assertGeneratedPromptStyle(s, first)
		s.send("echo prelude-界")
		s.await("Prelude prompt editable input boundary with LOCKED footer", func(f smokeFrame) bool {
			return f.hasInput("╰─", "echo prelude-界") && f.hasFooter(smokeNavigationKeys, "")
		})
		s.send("\x10")
		leader := s.await("generated prompt prefix shows the expanded host keymap", func(f smokeFrame) bool {
			return f.hasFooter(smokeLeaderKeys, "") && f.hasInput("╰─", "echo prelude-界")
		})
		assertSmokeFooterTheme(t, leader, smokeLeaderKeys, "")
		s.send("x")
		menu := s.await("generated prompt Ctrl+P x opens the menu and locks command mode", func(f smokeFrame) bool {
			return f.panel("menu") && f.hasPaneFooter("menu | floating | focus:pane | running")
		})
		body := image.Rect(30, 2, 130, 24)
		menuIdentity := strings.TrimSpace(menu.textAt(body.Min.X, body.Min.Y, body.Dx()))
		s.send("\x10t")
		s.await("Ctrl+P t hides the generated prompt's retained menu", func(f smokeFrame) bool {
			return !f.panel("menu") && f.hasPaneFooter("menu hidden | running") && f.hasInput("╰─", "echo prelude-界")
		})
		s.send("\x1bx")
		s.await("Alt+x restores the same menu process", func(f smokeFrame) bool {
			return f.panel("menu") && strings.TrimSpace(f.textAt(body.Min.X, body.Min.Y, body.Dx())) == menuIdentity &&
				f.hasPaneFooter("menu | floating | focus:pane | running")
		})
		s.send("\x10c")
		s.await("closing generated menu restores LOCKED footer and exact parked input", func(f smokeFrame) bool {
			return f.hasFooter(smokeNavigationKeys, "") && f.hasInput("╰─", "echo prelude-界")
		})
		s.send("\x1bd")
		s.await("generated prompt Alt+d opens docs", func(f smokeFrame) bool {
			return f.panel("docs") && f.hasPaneFooter("docs | floating | focus:pane | running")
		})
		s.send("\x10c")
		s.await("closing generated docs retains only command-lock footer", func(f smokeFrame) bool {
			return f.hasFooter(smokeNavigationKeys, "") && f.hasInput("╰─", "echo prelude-界")
		})
		s.send("\r")
		s.await("Prelude Starship command output", func(f smokeFrame) bool {
			return f.hasLine("prelude-界") && f.hasInput("╰─", "") && f.hasFooter(smokeNavigationKeys, "")
		})
		s.send("\x1b[A")
		s.await("Prelude Starship history recall", func(f smokeFrame) bool {
			return f.hasInput("╰─", "echo prelude-界") && f.hasFooter(smokeNavigationKeys, "")
		})
		s.resize(70, 12)
		s.send("\x0c")
		s.await("Prelude Powerline reflow/redraw retains input and LOCKED footer", func(f smokeFrame) bool {
			return f.cols == 70 && f.hasInput("╰─", "echo prelude-界") && f.hasFooter(smokeNavigationKeys, "")
		})
		s.send("\x15exit 0\r")
		s.expectExit(0)
	})

	t.Run("LockedFooterEveryPromptMode", func(t *testing.T) {
		config := os.Getenv("PRELUDE_GHOSTTY_TEST_STARSHIP_CONFIG")
		if config == "" {
			t.Fatal("generated Starship preset required; run in nix develop path:.#ghostty-spike")
		}
		custom := filepath.Join(t.TempDir(), "custom-owner.toml")
		if err := os.WriteFile(custom, []byte("add_newline = false\nformat = \"[CUSTOM-OWNER ? x d](bold green)\\n[custom❯](cyan) \"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		for _, mode := range []struct {
			name, config, prompt string
			args                 []string
		}{
			{"GeneratedPreset", config, "╰─", []string{"--starship"}},
			{"CustomOwner", custom, "custom❯", []string{"--starship"}},
			{"FixedPrompt", config, "spike $", nil},
		} {
			t.Run(mode.name, func(t *testing.T) {
				isolatedMotd(t)
				env := append(smokeEnvironment(t.TempDir()), "STARSHIP_CONFIG="+mode.config)
				s := startSmokeOuterPTYSize(t, binary, env, 160, 28, mode.args...)
				first := s.await("every prompt mode retains LOCKED command footer", func(f smokeFrame) bool {
					return f.hasInput(mode.prompt, "") && f.hasFooter(smokeNavigationKeys, "")
				})
				if mode.name == "CustomOwner" && !first.hasLine("CUSTOM-OWNER ? x d") {
					t.Fatalf("custom Starship config/aliases were overwritten\n%s", s.diagnostics())
				}
				assertSmokeFooterTheme(t, first, smokeNavigationKeys, "")
				s.send("echo marker-fallback\r")
				s.await("LOCKED footer survives a real shell command", func(f smokeFrame) bool {
					return f.hasLine("marker-fallback") && f.hasInput(mode.prompt, "") && f.hasFooter(smokeNavigationKeys, "")
				})
				s.send("exit 0\r")
				s.expectExit(0)
			})
		}
	})

	t.Run("MissingStarshipIsActionable", func(t *testing.T) {
		bash, err := exec.LookPath("bash")
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		if err := os.Symlink(bash, filepath.Join(dir, "bash")); err != nil {
			t.Fatal(err)
		}
		env := append(smokeEnvironment(t.TempDir()), "PATH="+dir)
		s := startSmokeOuterPTYEnv(t, binary, env, "--starship")
		s.expectExit(1)
		s.traceMu.Lock()
		output := string(s.output)
		s.traceMu.Unlock()
		if !strings.Contains(output, "--starship needs Starship on PATH") || !strings.Contains(output, "#ghostty-spike") {
			t.Fatalf("missing Starship did not explain how to start the spike: %q", output)
		}
	})
}
