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
)

const fixtureMotd = `builtin printf 'MOTD_SHELL:%s\n' "$*"`

// The ordinary command still runs on a real PTY. Only fixture PATH is changed:
// unrelated shell/pane tests must not run project Preflight or write its Cache.
func writeMotdFixture(t *testing.T, dir, body string) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("#!%s -p\n%s\n", bash, body)
	if err := os.WriteFile(filepath.Join(dir, "motd"), []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func isolatedMotd(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	writeMotdFixture(t, dir, fixtureMotd)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func assertMotdCalls(t *testing.T, path string, want int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("MOTD invocation log:", err)
	}
	// This also proves zero arguments: --pure or any other startup flag changes
	// the record, even if a repaint happens to show an identical banner.
	if expected := strings.Repeat("args:\n", want); string(data) != expected {
		t.Fatalf("MOTD invocations = %q, want %q", data, expected)
	}
}

func TestGhosttyStartupMotdBinarySmoke(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "prelude-workspace")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build prelude-workspace binary: %v (context: %v)\n%s", err, ctx.Err(), output)
	}
	config := filepath.Join(t.TempDir(), "starship.toml")
	if err := os.WriteFile(config, []byte(smokeStarshipConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STARSHIP_CONFIG", config)

	for _, mode := range []struct {
		name   string
		args   []string
		prompt string
	}{
		{"Fixed", nil, strings.TrimSpace(workspacePrompt)},
		{"Starship", []string{"--starship"}, "界❯"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			t.Run("OnceBeforePromptAndManualReprint", func(t *testing.T) {
				leaderSmokeTools(t)
				dir := t.TempDir()
				log := filepath.Join(dir, "motd.calls")
				writeMotdFixture(t, dir, fmt.Sprintf(`if [[ ! -e %q ]]; then
    builtin printf 'args:%%s\n' "$*" >> %q
    builtin printf 'STARTUP_MOTD_ONCE\n'
else
    builtin printf 'args:%%s\n' "$*" >> %q
    builtin printf 'MANUAL_MOTD_REPRINT\n'
fi`, log, log, log))
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
				s := startSmokeOuterPTY(t, binary, mode.args...)
				first := s.await("startup banner before first prompt", func(f smokeFrame) bool {
					return f.hasLine("STARTUP_MOTD_ONCE") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
				})
				text := strings.Join(first.rows, "\n")
				if strings.Count(text, "STARTUP_MOTD_ONCE") != 1 || strings.Index(text, "STARTUP_MOTD_ONCE") >= strings.Index(text, mode.prompt) {
					t.Fatalf("banner was duplicated or not before the first prompt\n%s", s.diagnostics())
				}
				assertMotdCalls(t, log, 1)

				const pending = smokeBaseFooter
				s.send("echo parked")
				s.await("parked Bash input after startup MOTD", func(f smokeFrame) bool { return f.hasInput(mode.prompt, "echo parked") && f.footer() == pending })
				for _, surface := range []struct{ key, kind string }{{"x", "menu"}, {"d", "docs"}} {
					s.send("\x10" + surface.key)
					shown := s.await("MOTD stays outside the "+surface.kind+" pane", func(f smokeFrame) bool {
						return f.panel(surface.kind) && f.hasPaneFooter(surface.kind+" | floating | focus:pane | running")
					})
					if body := leaderSmokeBody(shown, image.Rect(9, 2, 81, 21)); strings.Contains(body, "MOTD_") {
						t.Fatalf("MOTD appeared in the surface pane: %q", body)
					}
					if surface.kind == "menu" && shown.rows[0] != "STARTUP_MOTD_ONCE" {
						t.Fatalf("startup banner was not ordinary main-shell output\n%s", s.diagnostics())
					}
					s.send("\x10" + surface.key)
					s.await("hide "+surface.kind+" without replaying startup", func(f smokeFrame) bool {
						return f.hasInput(mode.prompt, "echo parked") && f.hasPaneFooter(surface.kind+" hidden | running")
					})
					s.send("\x10t")
					s.await("show "+surface.kind+" without replaying startup", func(f smokeFrame) bool {
						return f.panel(surface.kind) && f.hasPaneFooter(surface.kind+" | floating | focus:pane | running")
					})
					s.send("\x1d")
					s.await("docked "+surface.kind+" with parked Bash input", func(f smokeFrame) bool {
						return f.hasPaneFooter("Placement: left | " + surface.kind + " | left | focus:pane | running")
					})
					s.resize(104, 28)
					s.await("outer resize does not rerun MOTD", func(f smokeFrame) bool {
						return f.cols == 104 && len(f.rows) == 28 && f.hasPaneFooter("Placement: left | "+surface.kind+" | left | focus:pane | running")
					})
					s.send("\x1c")
					s.await("floating "+surface.kind+" restored", func(f smokeFrame) bool {
						return f.panel(surface.kind) && f.hasPaneFooter("Placement: floating | "+surface.kind+" | floating | focus:pane | running")
					})
					s.send("\x10c")
					s.await("close "+surface.kind+" leaves MOTD in Bash only", func(f smokeFrame) bool { return f.hasInput(mode.prompt, "echo parked") && f.footer() == pending })
					s.resize(ghosttySmokeCols, ghosttySmokeRows)
					s.await("original shell geometry restored", func(f smokeFrame) bool {
						return f.cols == ghosttySmokeCols && len(f.rows) == ghosttySmokeRows && f.hasInput(mode.prompt, "echo parked") && f.footer() == pending
					})
					assertMotdCalls(t, log, 1)
				}
				s.send("\x0c")
				s.await("readline redraw clears banner without replaying it", func(f smokeFrame) bool {
					return !f.hasLine("STARTUP_MOTD_ONCE") && f.hasInput(mode.prompt, "echo parked") && f.footer() == pending
				})
				s.send("\r")
				s.await("ordinary command after startup", func(f smokeFrame) bool {
					return f.hasLine("parked") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
				})
				s.send("false\r")
				awaitSmokeStatus(s, mode.prompt, "motd-user-failure", 1)
				assertMotdCalls(t, log, 1)
				s.send("motd\r")
				s.await("manual argument-free MOTD remains a shell command", func(f smokeFrame) bool {
					return f.hasLine("MANUAL_MOTD_REPRINT") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
				})
				assertMotdCalls(t, log, 2)
				s.send("exit 0\r")
				s.expectExit(0)
			})

			t.Run("AltMReadlineHandoff", func(t *testing.T) {
				leaderSmokeTools(t)
				dir := t.TempDir()
				log, ttyLog := filepath.Join(dir, "motd.calls"), filepath.Join(dir, "motd.tty")
				mainTTY, foregroundLog := filepath.Join(dir, "main.tty"), filepath.Join(dir, "foreground.input")
				writeMotdFixture(t, dir, fmt.Sprintf(`builtin printf 'args:%%s\n' "$*" >> %q
builtin printf '%%s\n' "$(tty)" >> %q
calls=0
while IFS= builtin read -r record; do ((calls+=1)); done < %q
builtin printf 'ALTM_MOTD:%%s\n' "$calls"`, log, ttyLog, log))
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
				env := append(smokeEnvironment(t.TempDir()), fmt.Sprintf(`BASH_FUNC_altm_foreground%%%%=() { stty -echo; builtin printf 'ALTM_FOREGROUND_READY\n'; local gate; IFS= builtin read -r gate; builtin printf '%%s' "$gate" > %q; stty echo; builtin printf 'ALTM_FOREGROUND_DONE\n'; }`, foregroundLog))
				s := startSmokeOuterPTYSize(t, binary, env, 160, 28, mode.args...)
				s.await("ordinary startup MOTD before shortcuts", func(f smokeFrame) bool {
					return f.hasLine("ALTM_MOTD:1") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
				})
				s.send(fmt.Sprintf("tty > %q; printf 'ALTM_READY\\n'\r", mainTTY))
				s.await("main-shell TTY identity", func(f smokeFrame) bool { return f.hasLine("ALTM_READY") && f.hasInput(mode.prompt, "") })
				const parked = `printf 'ALTM_PARKED:%s\n' 'abXYcd'`
				const beforeMark = `printf 'ALTM_PARKED:%s\n' 'ab`
				s.send(parked + "\x01" + strings.Repeat("\x1b[C", len(beforeMark)) + "\x1b \x1b[C\x1b[C!")
				s.await("MOTD shortcut fixture has exact point/mark", func(f smokeFrame) bool { return f.hasInput(mode.prompt, strings.Replace(parked, "XY", "XY!", 1)) })
				s.send("\x7f")
				s.await("parked line before Alt+M", func(f smokeFrame) bool { return f.hasInput(mode.prompt, parked) })
				s.send("\x1bx")
				pane := s.await("Alt+x opens retained native menu", func(f smokeFrame) bool {
					return f.panel("menu") && f.hasPaneFooter("menu | floating | focus:pane | running")
				})
				body := image.Rect(30, 2, 130, 24)
				identity := strings.TrimSpace(pane.textAt(body.Min.X, body.Min.Y, body.Dx()))
				s.send("R")
				s.await("pane retains its own input", func(f smokeFrame) bool { return strings.Contains(leaderSmokeBody(f, body), "BYTES:52") })
				s.send("\x1bm")
				s.await("Alt+M prints in main shell, hides pane and restores exact parked input", func(f smokeFrame) bool {
					return f.hasLine("ALTM_MOTD:2") && f.hasInput(mode.prompt, parked) && !f.panel("menu") && f.hasPaneFooter("menu hidden | running")
				})
				s.send("-")
				s.await("Alt+M preserves mid-line point", func(f smokeFrame) bool { return f.hasInput(mode.prompt, strings.Replace(parked, "XY", "XY-", 1)) })
				s.send("\x18\x18M")
				s.await("Alt+M preserves Readline mark", func(f smokeFrame) bool { return f.hasInput(mode.prompt, strings.Replace(parked, "XY", "MXY-", 1)) })
				s.send("\x05\r")
				s.await("restored input executes byte-for-byte", func(f smokeFrame) bool { return f.hasLine("ALTM_PARKED:abMXY-cd") && f.hasInput(mode.prompt, "") })
				s.send("\x10t")
				s.await("MOTD hid rather than replaced the same pane", func(f smokeFrame) bool {
					return f.panel("menu") && strings.TrimSpace(f.textAt(body.Min.X, body.Min.Y, body.Dx())) == identity && strings.Contains(leaderSmokeBody(f, body), "BYTES:52")
				})
				s.send("\x10")
				s.await("MOTD command visibly unlocks", func(f smokeFrame) bool { return f.hasFooter(smokeLeaderKeys, "") })
				s.send("m")
				s.await("unlocked m performs one MOTD action and locks", func(f smokeFrame) bool {
					return f.hasLine("ALTM_MOTD:3") && f.hasInput(mode.prompt, "") && f.hasPaneFooter("menu hidden | running")
				})
				s.send("altm_foreground\r")
				s.await("foreground read owns main-shell input", func(f smokeFrame) bool {
					return f.hasLine("ALTM_FOREGROUND_READY") && f.hasInput(mode.prompt, "altm_foreground")
				})
				s.send("\x1bm")
				s.await("Alt+M queues rather than injecting into foreground input", func(f smokeFrame) bool { return f.hasPaneFooter("menu hidden | running | command queued") })
				assertMotdCalls(t, log, 3)
				if data, err := os.ReadFile(foregroundLog); !os.IsNotExist(err) {
					t.Fatalf("Alt+M injected foreground bytes before user input: %q, %v", data, err)
				}
				s.send("only-user-input\r")
				s.await("queued foreground MOTD runs after the next primary prompt", func(f smokeFrame) bool {
					return f.hasLine("ALTM_MOTD:4") && f.hasLine("ALTM_FOREGROUND_DONE") && f.hasInput(mode.prompt, "") && f.hasPaneFooter("menu hidden | running")
				})
				if data, err := os.ReadFile(foregroundLog); err != nil || string(data) != "only-user-input" {
					t.Fatalf("foreground reader received shortcut bytes: %q, %v", data, err)
				}
				ps2 := "..."
				if mode.name == "Starship" {
					ps2 = "∙"
				}
				s.send("for motd_item in one; do\r")
				s.await("PS2 owns unfinished shell command", func(f smokeFrame) bool { return f.hasLine(ps2) })
				s.send("\x1bm")
				s.await("Alt+M safely queues at PS2", func(f smokeFrame) bool {
					return f.hasLine(ps2) && f.hasPaneFooter("menu hidden | running | command queued")
				})
				assertMotdCalls(t, log, 4)
				s.send("printf 'ALTM_PS2_BODY:%s\\n' \"$motd_item\"\r")
				s.await("PS2 body remains an ordinary unfinished command", func(f smokeFrame) bool {
					return f.hasLine(ps2) && strings.Contains(strings.Join(f.rows, "\n"), "printf 'ALTM_PS2_BODY:")
				})
				assertMotdCalls(t, log, 4)
				s.send("done\r")
				s.await("queued PS2 MOTD waits until the loop really completes", func(f smokeFrame) bool {
					return f.hasLine("ALTM_PS2_BODY:one") && f.hasLine("ALTM_MOTD:5") && f.hasInput(mode.prompt, "") && f.hasPaneFooter("menu hidden | running")
				})
				assertMotdCalls(t, log, 5)
				wantTTY, err := os.ReadFile(mainTTY)
				if err != nil {
					t.Fatal(err)
				}
				if got, err := os.ReadFile(ttyLog); err != nil || string(got) != strings.Repeat(string(wantTTY), 5) {
					t.Fatalf("every MOTD must use the main Bash PTY, not a pane: got %q, main %q, err=%v", got, wantTTY, err)
				}
				s.send("\x10c")
				s.await("retained pane closes without disturbing main shell", func(f smokeFrame) bool { return f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter })
				s.send("exit 0\r")
				s.expectExit(0)
			})

			for _, failure := range []string{"Failure", "Missing"} {
				t.Run(failure, func(t *testing.T) {
					dir := t.TempDir()
					log := filepath.Join(dir, "motd.calls")
					if failure == "Failure" {
						writeMotdFixture(t, dir, fmt.Sprintf("builtin printf 'args:%%s\\n' \"$*\" >> %q\nexit 73", log))
						t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
					} else {
						// A private PATH makes the command genuinely missing, rather
						// than replacing it with a successful production skip flag.
						commands := []string{"bash"}
						if len(mode.args) != 0 {
							commands = append(commands, "starship")
						}
						for _, command := range commands {
							path, err := exec.LookPath(command)
							if err != nil {
								t.Fatal(err)
							}
							if err := os.Symlink(path, filepath.Join(dir, command)); err != nil {
								t.Fatal(err)
							}
						}
						t.Setenv("PATH", dir)
					}
					s := startSmokeOuterPTY(t, binary, mode.args...)
					s.await("MOTD failure still yields a clean first prompt", func(f smokeFrame) bool {
						return f.hasLine("prelude-workspace: startup motd failed; continuing with the shell") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
					})
					if failure == "Failure" {
						assertMotdCalls(t, log, 1)
					}
					s.send("echo first-status:$?\r")
					s.await("startup failure did not leak into user exit status", func(f smokeFrame) bool {
						return f.hasLine("first-status:0") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
					})
					writeMotdFixture(t, dir, "builtin printf 'MANUAL_MOTD_RECOVERED\\n'")
					s.send("motd\r")
					s.await("manual MOTD works after startup failure", func(f smokeFrame) bool {
						return f.hasLine("MANUAL_MOTD_RECOVERED") && f.hasInput(mode.prompt, "") && f.footer() == smokeBaseFooter
					})
					s.send("exit 0\r")
					s.expectExit(0)
				})
			}
		})
	}
}
