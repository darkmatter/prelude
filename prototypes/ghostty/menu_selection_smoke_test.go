package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exercise the real binary and real x through their outer PTY, rather than
// reproducing the host's loader, prompt detection, or Selection representation.
func TestGhosttyMenuSelectionBinarySmoke(t *testing.T) {
	realX, err := exec.LookPath("x")
	if err != nil {
		t.Fatal("real configured x is required; run in nix develop path:.#ghostty-spike:", err)
	}
	binary := filepath.Join(t.TempDir(), "ghostty-spike")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build real Ghostty binary: %v (context: %v)\n%s", err, ctx.Err(), output)
	}

	for _, mode := range []struct {
		name   string
		args   []string
		prompt string
		ps2    string
	}{
		{"Fixed", nil, strings.TrimSpace(spikePrompt), "..."},
		{"Starship", []string{"--starship"}, "界❯", "∙"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			isolatedMotd(t)
			// This real x uses a test Config, not the generated completion catalogue.
			t.Setenv("PRELUDE_COMPLETION_INIT", "")
			bash, err := exec.LookPath("bash")
			if err != nil {
				t.Fatal(err)
			}
			quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
			dir := t.TempDir()
			workdir, prefix := filepath.Join(dir, "working dir's"), filepath.Join(dir, "command bin's")
			for _, path := range []string{workdir, prefix} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			log, mainContext, foregroundLog := filepath.Join(dir, "selection.calls"), filepath.Join(dir, "main.context"), filepath.Join(dir, "foreground.input")
			probe := fmt.Sprintf(`#!%s -p
builtin printf '%%s\0' "$PWD" "$PATH" "$(tty)" "$@" >> %s
builtin printf 'HANDOFF_IN_MAIN\n'
IFS= builtin read -r gate
builtin printf 'HANDOFF_RELEASED:%%s\n' "$gate"
exit 37
`, bash, quote(log))
			if err := os.WriteFile(filepath.Join(prefix, "handoff-probe"), []byte(probe), 0o700); err != nil {
				t.Fatal(err)
			}
			// Only Config changes. The wrapper forwards every host argument to
			// the installed x, including its private --select-output path.
			cfg, err := json.Marshal(map[string]any{
				"project": "handoff", "execute": true, "height": 6, "maxWidth": 90,
				"groups": []any{map[string]any{"title": "Selection", "tasks": []any{map[string]any{
					"name": "handoff", "description": "Run in the main Bash",
					"run": "# HANDOFF_SOURCE_FILE_ONLY\n[[ ${handoff_shell-} == \"$$\" && $(tty) == \"$handoff_tty\" ]] || exit 96\nhandoff-probe",
					"dir": workdir, "pathPrefix": []string{prefix},
					"args": []any{map[string]any{"token": "VALUE", "required": true}},
				}}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(dir, "menu.json")
			if err := os.WriteFile(config, cfg, 0o600); err != nil {
				t.Fatal(err)
			}
			wrapper := fmt.Sprintf("#!%s -p\nexec %s --config %s \"$@\"\n", bash, quote(realX), quote(config))
			if err := os.WriteFile(filepath.Join(dir, "x"), []byte(wrapper), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			starshipConfig := filepath.Join(dir, "starship.toml")
			if err := os.WriteFile(starshipConfig, []byte(smokeStarshipConfig), 0o600); err != nil {
				t.Fatal(err)
			}
			env := append(smokeEnvironment(t.TempDir()), "STARSHIP_CONFIG="+starshipConfig)
			s := startSmokeOuterPTYSize(t, binary, env, 160, 32, mode.args...)
			s.await("initial primary prompt", func(f smokeFrame) bool {
				return f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
			})
			// These unexported values can only be seen by source in the existing
			// Bash; the menu child and a new bash -c cannot see them.
			s.send(fmt.Sprintf("handoff_shell=$$; handoff_tty=$(tty); builtin printf '%%s\\0' \"$PWD\" \"$PATH\" \"$handoff_tty\" > %s; handoff_foreground() { stty -echo; builtin printf 'FOREGROUND_READY\\n'; local gate; IFS= builtin read -r gate; builtin printf '%%s\\0' \"$gate\" > %s; stty echo; builtin printf 'FOREGROUND_BYTES:%%s\\n' \"$gate\"; }; builtin printf 'HANDOFF_READY\\n'\r", quote(mainContext), quote(foregroundLog)))
			s.await("main-shell identity and foreground fixture ready", func(f smokeFrame) bool {
				return f.hasLine("HANDOFF_READY") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
			})
			contextData, err := os.ReadFile(mainContext)
			if err != nil {
				t.Fatal(err)
			}
			fields := strings.Split(strings.TrimSuffix(string(contextData), "\x00"), "\x00")
			if len(fields) != 3 || fields[2] == "" {
				t.Fatalf("main-shell context = %q", contextData)
			}
			const argumentLine = `'two words' '' 'quote"and' '$HOME'`
			wantFields := []string{workdir, prefix + string(os.PathListSeparator) + fields[1], fields[2], "two words", "", `quote"and`, "$HOME"}
			wantCall := strings.Join(wantFields, "\x00") + "\x00"
			calls := func(want int) {
				t.Helper()
				data, err := os.ReadFile(log)
				if err != nil || string(data) != strings.Repeat(wantCall, want) {
					t.Fatalf("selection must run exactly %d time(s) in the main PTY with its cwd/PATH/raw args: err=%v\ngot %q\nwant %q\n%s", want, err, data, strings.Repeat(wantCall, want), s.diagnostics())
				}
			}
			open := func() {
				t.Helper()
				s.send("\x10x")
				s.await("real x picker usable", func(f smokeFrame) bool {
					return strings.Contains(strings.Join(f.rows, "\n"), "handoff — command menu") &&
						f.hasPaneFooter("menu | floating | focus:pane | running")
				})
			}
			selectCommand := func() {
				t.Helper()
				open()
				s.send("\r")
				s.await("real x collects arguments", func(f smokeFrame) bool {
					return strings.Contains(strings.Join(f.rows, "\n"), "handoff handoff — enter arguments")
				})
				s.send(argumentLine)
				s.await("complete raw argument line in picker", func(f smokeFrame) bool {
					return strings.Contains(strings.Join(f.rows, "\n"), argumentLine)
				})
				s.send("\r")
			}
			menuClosed := func(f smokeFrame) bool {
				text := strings.Join(f.rows, "\n")
				return !strings.Contains(text, "handoff — command menu") &&
					!strings.Contains(text, "handoff handoff — enter arguments") && !strings.Contains(f.footer(), " | menu")
			}
			running := func(want int) {
				t.Helper()
				s.await("selection runs in main with no menu pane", func(f smokeFrame) bool {
					data, _ := os.ReadFile(log)
					return f.hasLine("HANDOFF_IN_MAIN") && f.footer() == smokeBaseFooter &&
						menuClosed(f) && strings.Count(string(data), "\x00") == len(wantFields)*want
				})
				calls(want)
				if strings.Contains(strings.Join(s.frame().rows, "\n"), "HANDOFF_SOURCE_FILE_ONLY") {
					t.Fatalf("raw multiline source was echoed as readline input instead of loaded from a command file\n%s", s.diagnostics())
				}
			}
			completed := func(input, label string) {
				t.Helper()
				s.await("selection exit status and primary-prompt restoration", func(f smokeFrame) bool {
					return f.footer() == smokeBaseFooter && f.hasInput(mode.prompt, input) &&
						(mode.name != "Starship" || f.starshipStatus(input, "status:37", true))
				})
				if input == "" {
					awaitSmokeStatus(s, mode.prompt, label, 37)
				}
			}

			const parked = `printf 'PARKED:%s:status:%s\n' 'abXYcd' "$?"`
			const beforeMark = `printf 'PARKED:%s:status:%s\n' 'ab`
			s.send("\x0c" + parked)
			s.await("parked input", func(f smokeFrame) bool { return f.hasInput(mode.prompt, parked) && f.footer() == smokeBaseFooter })
			// Mark before XY, point after XY. A temporary insertion acknowledges
			// that readline has consumed the cursor and mark keys before opening x.
			s.send("\x01" + strings.Repeat("\x1b[C", len(beforeMark)) + "\x1b " + "\x1b[C\x1b[C!")
			s.await("mid-line point and mark prepared", func(f smokeFrame) bool {
				return f.hasInput(mode.prompt, strings.Replace(parked, "XY", "XY!", 1)) && f.footer() == smokeBaseFooter
			})
			s.send("\x7f")
			s.await("parked line restored before handoff", func(f smokeFrame) bool { return f.hasInput(mode.prompt, parked) && f.footer() == smokeBaseFooter })
			selectCommand()
			running(1)
			s.send("first-selection\r")
			completed(parked, "parked-selection")
			s.send("-")
			s.await("parked mid-line caret preserved", func(f smokeFrame) bool {
				return f.hasInput(mode.prompt, strings.Replace(parked, "XY", "XY-", 1)) && f.footer() == smokeBaseFooter
			})
			s.send("\x18\x18M") // readline exchange-point-and-mark, then edit there.
			s.await("parked readline mark preserved", func(f smokeFrame) bool {
				return f.hasInput(mode.prompt, strings.Replace(parked, "XY", "MXY-", 1)) && f.footer() == smokeBaseFooter
			})
			s.send("\x05\r")
			s.await("restored parked input executes normally", func(f smokeFrame) bool {
				return f.hasLine("PARKED:abMXY-cd:status:37") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
			})
			calls(1)

			s.send("printf 'CANCEL_FAILURE_READY\\n'; false\r")
			s.await("failure command finished before cancellation", func(f smokeFrame) bool {
				return f.hasLine("CANCEL_FAILURE_READY") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter &&
					(mode.name != "Starship" || f.starshipStatus("", "status:1", false))
			})
			const cancelPrefix = `printf 'CANCEL:status:%s:%s\n' "$?" `
			const cancelInput = cancelPrefix + "cancel-preserved"
			s.send(cancelInput + "\x01" + strings.Repeat("\x1b[C", len(cancelPrefix)))
			s.await("input parked before cancellation", func(f smokeFrame) bool { return f.hasInput(mode.prompt, cancelInput) && f.footer() == smokeBaseFooter })
			open()
			s.send("\x1b")
			s.await("empty successful x result restores input without running", func(f smokeFrame) bool {
				return f.hasInput(mode.prompt, cancelInput) && f.footer() == smokeBaseFooter
			})
			s.send("-")
			s.await("cancellation retains the mid-line caret", func(f smokeFrame) bool {
				return f.hasInput(mode.prompt, cancelPrefix+"-cancel-preserved") && f.footer() == smokeBaseFooter
			})
			s.send("\x05\r")
			s.await("Bash preserves exit status across picker cancellation", func(f smokeFrame) bool {
				return f.hasLine("CANCEL:status:1:-cancel-preserved") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
			})
			calls(1)

			s.send("handoff_foreground\r")
			s.await("foreground application waiting for user input", func(f smokeFrame) bool {
				return f.hasLine("FOREGROUND_READY") && f.hasInput(mode.prompt, "handoff_foreground") && f.footer() == smokeBaseFooter
			})
			selectCommand()
			s.await("busy shell queues the result and closes menu", func(f smokeFrame) bool {
				return f.hasLine("FOREGROUND_READY") && menuClosed(f) && f.hasFooter(smokeNavigationKeys, "command queued")
			})
			calls(1)
			if data, err := os.ReadFile(foregroundLog); !os.IsNotExist(err) {
				t.Fatalf("handoff injected bytes into foreground read before user input: %q, %v", data, err)
			}
			s.send("only-user-input\r")
			running(2)
			data, err := os.ReadFile(foregroundLog)
			if err != nil || string(data) != "only-user-input\x00" {
				t.Fatalf("foreground application received command bytes: %q, %v\n%s", data, err, s.diagnostics())
			}
			s.send("busy-selection\r")
			completed("", "busy-selection")

			s.send("for handoff_item in one; do\r")
			s.await("secondary PS2 prompt", func(f smokeFrame) bool { return f.hasLine(mode.ps2) })
			selectCommand()
			s.await("PS2 queues selection instead of accepting the source as a loop body", func(f smokeFrame) bool {
				return f.hasLine(mode.ps2) && menuClosed(f) && f.hasFooter(smokeNavigationKeys, "command queued")
			})
			calls(2)
			s.send("printf 'PS2_BODY:%s\\n' \"$handoff_item\"\r")
			s.await("ordinary loop body still awaits done at PS2", func(f smokeFrame) bool {
				text := strings.Join(f.rows, "\n")
				if strings.Contains(text, mode.ps2+" builtin source ") {
					t.Fatalf("menu selection injected its loader at PS2 instead of waiting for a completed primary prompt; footer=%q\n%s", f.footer(), text)
				}
				return f.hasLine(mode.ps2) && strings.Contains(text, "printf 'PS2_BODY:")
			})
			calls(2)
			s.send("done\r")
			running(3)
			if !s.frame().hasLine("PS2_BODY:one") {
				t.Fatalf("queued selection displaced the unfinished shell command\n%s", s.diagnostics())
			}
			s.send("ps2-selection\r")
			completed("", "ps2-selection")
			calls(3)
			s.send("exit 0\r")
			s.expectExit(0)
		})
	}
}
