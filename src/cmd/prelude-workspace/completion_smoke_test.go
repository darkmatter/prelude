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
)

// No manual source or copied completion implementation: the real binary must
// load Nix's generated completion-only init before either kind of first prompt.
func TestGhosttyCompletionInitBinarySmoke(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "prelude-workspace")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build prelude-workspace binary: %v (context: %v)\n%s", err, ctx.Err(), output)
	}

	for _, mode := range []struct {
		name   string
		args   []string
		prompt string
	}{
		{"Fixed", nil, strings.TrimSpace(workspacePrompt)},
		{"Starship", []string{"--starship"}, "界❯"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			for _, initMode := range []string{"GeneratedCatalogueAndHandoff", "InheritedNoclobber", "Unset", "Empty", "Missing", "Failing"} {
				t.Run(initMode, func(t *testing.T) {
					generated := initMode == "GeneratedCatalogueAndHandoff" || initMode == "InheritedNoclobber"
					dir, home := t.TempDir(), t.TempDir()
					motdLog := filepath.Join(dir, "motd.calls")
					writeMotdFixture(t, dir, fmt.Sprintf("builtin printf 'args:%%s\\n' \"$*\" >> %q\nbuiltin printf 'COMPLETION_STARTUP_MOTD\\n'", motdLog))
					// Shadow only MOTD. x and docs remain the actual configured wrappers.
					t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
					if err := os.WriteFile(filepath.Join(home, "completion-file-unique"), []byte("COMPLETION_FILE_OK\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					for _, name := range []string{"ambiguous-alpha", "ambiguous-beta"} {
						if err := os.WriteFile(filepath.Join(home, name), nil, 0o600); err != nil {
							t.Fatal(err)
						}
					}
					starshipConfig := filepath.Join(home, "starship.toml")
					if err := os.WriteFile(starshipConfig, []byte(smokeStarshipConfig), 0o600); err != nil {
						t.Fatal(err)
					}
					// Remove the inherited init only for these explicit path scenarios.
					// Generated coverage below restores the real Nix-provided value.
					var env []string
					for _, entry := range smokeEnvironment(home) {
						if !strings.HasPrefix(entry, "PRELUDE_COMPLETION_INIT=") {
							env = append(env, entry)
						}
					}
					env = append(env, "STARSHIP_CONFIG="+starshipConfig, "GHOSTTY_COMPLETION_HOME="+home,
						`BASH_FUNC_completion_prepare%%=() { builtin cd "$GHOSTTY_COMPLETION_HOME"; builtin printf 'COMPLETION_CWD_READY\n'; }`,
						`BASH_FUNC_completion_tool%%=() { builtin printf 'COMPLETION_FUNCTION_OK\n'; }`)
					initPath := ""
					switch initMode {
					case "GeneratedCatalogueAndHandoff", "InheritedNoclobber":
						initPath = os.Getenv("PRELUDE_COMPLETION_INIT")
						if initPath == "" {
							t.Fatal("real PRELUDE_COMPLETION_INIT is required; run in nix develop path:.#workspace")
						}
						if _, err := os.Stat(initPath); err != nil {
							t.Fatal("generated completion-only init:", err)
						}
						t.Logf("real generated completion init: %s", initPath)
						for _, command := range []string{"x", "docs"} {
							if _, err := exec.LookPath(command); err != nil {
								t.Fatal("real configured "+command+":", err)
							}
						}
					case "Missing":
						initPath = filepath.Join(dir, "missing-completion.bash")
					case "Failing":
						initPath = filepath.Join(dir, "failing-completion.bash")
						if err := os.WriteFile(initPath, []byte("builtin printf 'INIT_FAILURE_SEEN\\n'\nreturn 23\n"), 0o600); err != nil {
							t.Fatal(err)
						}
					}
					if initMode != "Unset" {
						env = append(env, "PRELUDE_COMPLETION_INIT="+initPath)
					}
					if initMode == "InheritedNoclobber" {
						env = append(env, "SHELLOPTS=braceexpand:hashall:interactive-comments:noclobber")
					}
					s := startSmokeOuterPTYSize(t, binary, env, 160, 40, mode.args...)
					first := s.await("unchanged first prompt and hint footer after automatic init", func(f smokeFrame) bool {
						return f.hasLine("COMPLETION_STARTUP_MOTD") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
					})
					text := strings.ToLower(strings.Join(first.rows, "\n"))
					warning := strings.Contains(text, "prelude-workspace:") && strings.Contains(text, "completion")
					wantWarning := initMode == "Missing" || initMode == "Failing"
					if warning != wantWarning || initMode == "Failing" && !first.hasLine("INIT_FAILURE_SEEN") {
						t.Fatalf("completion init warning=%v, want %v; mode=%s\n%s", warning, wantWarning, initMode, s.diagnostics())
					}
					assertMotdCalls(t, motdLog, 1)
					s.send("printf 'FIRST_STATUS:%s\\n' \"$?\"\r")
					s.await("init never prevents an ordinary shell command", func(f smokeFrame) bool {
						return f.hasLine("FIRST_STATUS:0") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
					})
					s.send("completion_prepare\r")
					s.await("file completion fixture directory", func(f smokeFrame) bool {
						return f.hasLine("COMPLETION_CWD_READY") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
					})
					input := func(want string) {
						t.Helper()
						s.await("real readline completion: "+want, func(f smokeFrame) bool {
							return !smokeCompletionVisible(f) && f.footer() == smokeBaseFooter && f.hasInput(mode.prompt, want)
						})
					}
					s.send("cat completion-f\t")
					input("cat completion-file-unique")
					s.send("\r")
					s.await("ordinary file completion still executes", func(f smokeFrame) bool {
						return f.hasLine("COMPLETION_FILE_OK") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
					})
					s.send("completion_too\t")
					input("completion_tool")
					s.send("\r")
					s.await("ordinary function completion still executes", func(f smokeFrame) bool {
						return f.hasLine("COMPLETION_FUNCTION_OK") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
					})

					if generated {
						// One outer-PTY write keeps both Tabs queued while the first
						// completion callback is still in flight. Native Bash must
						// display its ambiguous filename list, not lose the second Tab.
						s.send("cat ambiguous-\t\t")
						s.await("rapid double-Tab preserves native ambiguous filename listing", func(f smokeFrame) bool {
							text := strings.Join(f.rows[:len(f.rows)-1], "\n")
							return strings.Contains(text, "ambiguous-alpha") && strings.Contains(text, "ambiguous-beta") &&
								!smokeCompletionVisible(f) && f.hasInput(mode.prompt, "cat ambiguous-") && f.footer() == smokeBaseFooter
						})
						s.send("\x15")
						exerciseSmokeCompletionPopup(s, mode.prompt)
						s.send("x\t")
						awaitSmokeCompletion(s, mode.prompt, "x")
						s.send("\x1b")
						awaitSmokeCompletionInput(s, mode.prompt, "x")
						s.send("\x15x bu\t")
						acceptSmokeCompletion(s, mode.prompt, "x build")
						s.send("\x15x build .#prelude-d\t")
						input("x build .#prelude-docs")
						s.send("\x15x go:t\t")
						acceptSmokeCompletion(s, mode.prompt, "x go:test")
						// docs is a menu/dispatcher built-in, not a generated catalogue
						// completion key. Dispatch it normally after completing x keys.
						s.send("\x15x docs")
						input("x docs")
						s.send("\r")
						s.await("ordinary x dispatch still executes after completion", func(f smokeFrame) bool {
							text := strings.Join(f.rows[:len(f.rows)-1], "\n")
							return strings.Contains(text, "PAGES") && strings.Contains(text, "DOCS") &&
								f.footer() == smokeBaseFooter && !smokeCompletionVisible(f) && !strings.Contains(f.footer(), " | menu")
						})
						s.send("q")
						s.await("actual x/docs returns to the original shell footer", func(f smokeFrame) bool {
							return f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
						})
					}
					s.send("if [[ ${BLE_VERSION+x} || ${BLE_ATTACHED+x} ]] || compgen -A function -- ble >/dev/null || compgen -A function -- __ble >/dev/null; then printf 'COMPLETION_BLE_LOADED\\n'; else printf 'COMPLETION_NO_BLE\\n'; fi\r")
					s.await("completion-only startup never loads ble.sh or its hooks", func(f smokeFrame) bool {
						return f.hasLine("COMPLETION_NO_BLE") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
					})
					const timedCommand = "printf 'COMPLETION_RUNNING\\n'; sleep 0.15; false"
					s.send(timedCommand + "\r")
					s.await("foreground command starts after completion", func(f smokeFrame) bool {
						return f.hasLine("COMPLETION_RUNNING") && f.hasInput(mode.prompt, timedCommand) && f.footer() == smokeBaseFooter
					})
					s.await("normal exit status and prompt remain functional", func(f smokeFrame) bool {
						return f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter &&
							(mode.name != "Starship" || f.starshipStatus("", "status:1", true))
					})
					awaitSmokeStatus(s, mode.prompt, "completion-failure", 1)

					if generated {
						s.send("echo completion-handoff")
						s.await("parked shell input after completion", func(f smokeFrame) bool {
							return f.hasInput(mode.prompt, "echo completion-handoff") && f.footer() == smokeBaseFooter
						})
						s.send("\x1bx")
						s.await("configured picker still works after completion", func(f smokeFrame) bool {
							return strings.Contains(strings.Join(f.rows, "\n"), "prelude — command menu") && f.hasPaneFooter("menu | floating | focus:pane | running")
						})
						s.send("docs")
						s.await("real picker filters to docs", func(f smokeFrame) bool {
							return strings.Contains(strings.Join(f.rows, "\n"), "browse project documentation")
						})
						s.send("\r")
						s.await("selected docs runs in main without a menu pane", func(f smokeFrame) bool {
							text := strings.Join(f.rows[:len(f.rows)-1], "\n")
							return strings.Contains(text, "PAGES") && strings.Contains(text, "DOCS") &&
								f.footer() == smokeBaseFooter && !strings.Contains(f.footer(), " | menu")
						})
						s.send("q")
						s.await("completion and menu handoff preserve parked readline input", func(f smokeFrame) bool {
							return f.footer() == smokeBaseFooter && f.hasInput(mode.prompt, "echo completion-handoff")
						})
						s.send("\r")
						s.await("parked input still executes", func(f smokeFrame) bool {
							return f.hasLine("completion-handoff") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
						})
					}
					if initMode == "InheritedNoclobber" {
						s.send("[[ -o noclobber ]] && printf 'COMPLETION_NOCLOBBER_KEPT\\n'\r")
						s.await("completion preserves the inherited shell option", func(f smokeFrame) bool {
							return f.hasLine("COMPLETION_NOCLOBBER_KEPT") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
						})
					}
					assertMotdCalls(t, motdLog, 1)
					s.send("exit 0\r")
					s.expectExit(0)
				})
			}
		})
	}
}
