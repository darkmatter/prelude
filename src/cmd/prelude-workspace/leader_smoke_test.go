package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/term"
)

// Separate markers from frontendTools: these children report their PID, actual
// PTY size, and accumulated raw input through the binary's outer terminal only.
func leaderSmokeTools(t *testing.T) {
	t.Helper()
	// This x is a PTY routing fixture, not the generated catalogue dispatcher.
	t.Setenv("PRELUDE_COMPLETION_INIT", "")
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for command, label := range map[string]string{"x": "MENU", "docs": "DOCS"} {
		// Ignore inherited BASH_ENV/ENV in this fixture interpreter; shell
		// startup isolation is tested in the private interactive Bash itself.
		content := fmt.Sprintf("#!%s -p\nexport GHOSTTY_LEADER_SMOKE_TOOL=%s\nexec %q -test.run=^TestGhosttyLeaderSmokeToolHelper$ -- \"$@\"\n", bash, label, binary)
		if err := os.WriteFile(filepath.Join(dir, command), []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeMotdFixture(t, dir, fixtureMotd)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestGhosttyLeaderSmokeToolHelper(t *testing.T) {
	label := os.Getenv("GHOSTTY_LEADER_SMOKE_TOOL")
	if label == "" {
		return
	}

	args := flag.Args()
	var selectionOutput string
	if label == "MENU" {
		selectionOutput = menuSelectionOutput(t, args)
	} else if len(args) != 0 {
		t.Fatalf("docs args = %q, want no arguments", args)
	}
	if _, err := term.MakeRaw(int(os.Stdin.Fd())); err != nil {
		t.Fatal(err)
	}
	fmt.Print("\x1b[?2004h")
	var input []byte
	for {
		cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("\x1b[2J\x1b[HLEADER_%s:%d\r\nSIZE:%dx%d\r\nBYTES:%x", label, os.Getpid(), cols, rows, input)
		var data [1]byte
		if _, err := os.Stdin.Read(data[:]); err != nil {
			os.Exit(0)
		}
		if data[0] == 'q' {
			if selectionOutput != "" {
				if err := os.WriteFile(selectionOutput, []byte("builtin printf 'FAILED_MENU_SOURCE_RAN\\n'\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			os.Exit(7)
		}
		input = append(input, data[0])
	}
}

func leaderSmokeBody(f smokeFrame, body image.Rectangle) string {
	if body.Empty() || !body.In(image.Rect(0, 0, f.cols, len(f.rows))) {
		return ""
	}
	var rows []string
	for y := body.Min.Y; y < body.Max.Y; y++ {
		rows = append(rows, f.textAt(body.Min.X, y, body.Dx()))
	}
	return strings.Join(rows, "\n")
}

func TestGhosttyLeaderBinarySmoke(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "prelude-workspace")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build prelude-workspace binary: %v (context: %v)\n%s", err, ctx.Err(), output)
	}

	t.Run("ConfiguredSurfacesAndMenuSelection", func(t *testing.T) {
		for _, command := range []string{"x", "docs", "motd"} {
			path, err := exec.LookPath(command)
			if err != nil {
				t.Fatalf("configured %s wrapper missing: %v; run in nix develop path:.#workspace (configured first-paint coverage is required)", command, err)
			}
			t.Logf("configured %s: %s", command, path)
		}

		// Keep the real normal MOTD/Preflight integration here, not in the
		// unrelated fixtures. Start large enough to retain its whole banner.
		const motdIdentity = "Devshell UI for Nix flakes"
		const menuIdentity = "prelude — command menu"
		s := startSmokeOuterPTYSize(t, binary, smokeEnvironment(t.TempDir()), 120, 60)
		s.await("actual configured startup MOTD before first prompt", func(f smokeFrame) bool {
			text := strings.Join(f.rows, "\n")
			return strings.Contains(text, motdIdentity) && strings.Contains(text, "Prelude's own devshell") &&
				!strings.Contains(text, "MOTD_SHELL:") && f.hasInput("prelude $", "") && f.footer() == smokeBaseFooter
		})
		s.resize(120, 42)
		s.await("larger outer terminal", func(f smokeFrame) bool { return f.cols == 120 && len(f.rows) == 42 && f.hasLine("prelude $") })
		s.send("echo native-main")
		s.await("parked Bash input", func(f smokeFrame) bool {
			return f.hasInput("prelude $", "echo native-main") && f.footer() == smokeBaseFooter
		})
		body := image.Rect(12, 5, 108, 35) // Full borderless 96x30 panel inside 120x42.
		const shellFooter = smokeBaseFooter
		paint := func(what, kind, status string, markers ...string) {
			s.await(what, func(f smokeFrame) bool {
				text := leaderSmokeBody(f, body)
				for _, marker := range markers {
					if !strings.Contains(text, marker) {
						return false
					}
				}
				return f.alt && f.hasPaneFooter(kind+" | floating | "+status)
			})
		}
		s.send("\x1bx")
		// These strings come from menu/view_list.go and the configured catalogue,
		// not the host's pane label or a placeholder surface.
		paint("configured Alt+x menu first paint", "menu", "focus:pane | running", menuIdentity, "docs")
		s.send("docs")
		paint("interactive menu filters to docs", "menu", "focus:pane | running", menuIdentity, "browse project documentation")
		s.send("\x10x")
		s.await("Ctrl+P x hides the Alt+x-opened configured menu", func(f smokeFrame) bool {
			return f.hasPaneFooter("menu hidden | running") && !strings.Contains(leaderSmokeBody(f, body), menuIdentity)
		})
		s.send("\x10t")
		paint("show preserves configured menu filter and selection", "menu", "focus:pane | running", menuIdentity, "browse project documentation")
		s.send("\r")
		s.await("menu selection opens actual docs in the main terminal with no menu pane", func(f smokeFrame) bool {
			text := strings.Join(f.rows[:len(f.rows)-1], "\n")
			return strings.Contains(text, "PAGES") && strings.Contains(text, "DOCS") &&
				f.footer() == smokeBaseFooter && !strings.Contains(f.footer(), " | menu")
		})
		s.send("q")
		s.await("exiting selected docs restores parked input in the main Bash", func(f smokeFrame) bool {
			return f.footer() == shellFooter && f.hasInput("prelude $", "echo native-main")
		})
		s.send("\x10x")
		paint("reopened configured picker is usable after selected docs exits", "menu", "focus:pane | running", menuIdentity, "docs")
		s.send("docs")
		paint("reopened picker filters again", "menu", "focus:pane | running", "browse project documentation")
		s.send("\x10c")
		s.await("close reopened picker leaves parked input intact", func(f smokeFrame) bool {
			return f.hasInput("prelude $", "echo native-main") && f.footer() == shellFooter
		})
		s.send("\x1bd")
		paint("configured docs shortcut first paint", "docs", "focus:pane | running", "PAGES", "DOCS")
		docsImage := leaderSmokeBody(s.frame(), body)
		s.send("\x10?")
		s.await("unsupported question-mark chord leaves actual docs intact", func(f smokeFrame) bool {
			return f.hasPaneFooter(smokeUnknownChordStatus("docs | floating | focus:pane | running")) && leaderSmokeBody(f, body) == docsImage
		})
		s.send("\x10d")
		s.await("same d chord hides docs independently of x", func(f smokeFrame) bool {
			return f.hasPaneFooter("docs hidden | running") && !strings.Contains(leaderSmokeBody(f, body), "PAGES")
		})
		s.send("\r")
		s.await("main shell executes while configured docs stay hidden", func(f smokeFrame) bool {
			return f.hasLine("native-main") && f.hasInput("prelude $", "") && f.hasPaneFooter("docs hidden | running")
		})
		s.send("\x10d")
		paint("same d chord restores configured docs and focus", "docs", "focus:pane | running", "PAGES", "DOCS")
		if got := leaderSmokeBody(s.frame(), body); got != docsImage {
			t.Fatalf("configured docs changed across hide/show: before=%q after=%q", docsImage, got)
		}
		s.send("\x10c")
		s.await("close configured docs restores base footer", func(f smokeFrame) bool { return f.hasInput("prelude $", "") && f.footer() == shellFooter })
		s.send("\x0c")
		s.await("clear startup banner before manual configured MOTD", func(f smokeFrame) bool {
			return f.footer() == shellFooter && !strings.Contains(strings.Join(f.rows, "\n"), motdIdentity)
		})
		s.send("motd\r")
		s.await("normal manual configured MOTD remains in Bash", func(f smokeFrame) bool {
			return strings.Contains(strings.Join(f.rows, "\n"), motdIdentity) && f.footer() == shellFooter
		})
		s.send("exit 0\r")
		s.expectExit(0)
	})

	t.Run("LeaderRoutingPlacementAndLifecycle", func(t *testing.T) {
		leaderSmokeTools(t)
		s := startSmokeOuterPTY(t, binary)
		s.await("fixture shell ready", func(f smokeFrame) bool { return f.hasInput("prelude $", "") && f.footer() == smokeBaseFooter })
		// Seed history so leaking the prefix into readline would recall a command.
		s.send("echo leader-history\r")
		s.await("Bash history seeded", func(f smokeFrame) bool {
			return f.hasLine("leader-history") && f.hasInput("prelude $", "") && f.footer() == smokeBaseFooter
		})
		shellInput := "echo xmd?vc"
		s.send(shellInput)
		s.await("unprefixed shortcuts belong to Bash", func(f smokeFrame) bool { return f.hasInput("prelude $", shellInput) && f.footer() == smokeBaseFooter })
		s.send("\x10")
		leader := s.await("Ctrl+P unlocks one command with MOTD/menu/docs hints", func(f smokeFrame) bool { return f.hasFooter(smokeLeaderKeys, "") })
		assertSmokeFooterTheme(t, leader, smokeLeaderKeys, "")
		s.send("\x1b")
		s.await("Escape cancels only the prefix", func(f smokeFrame) bool { return f.hasInput("prelude $", shellInput) && f.footer() == smokeBaseFooter })
		s.send("\x10x")
		body := image.Rect(9, 2, 81, 21) // 90x24: full floating Body is 72x19.
		first := s.await("raw menu child first paint", func(f smokeFrame) bool { return strings.Contains(leaderSmokeBody(f, body), "LEADER_MENU:") })
		var menuPID int
		if _, err := fmt.Sscanf(strings.TrimSpace(first.textAt(body.Min.X, body.Min.Y, body.Dx())), "LEADER_MENU:%d", &menuPID); err != nil || menuPID <= 0 {
			t.Fatalf("menu PID marker: %v\n%s", err, s.diagnostics())
		}
		identity, where := fmt.Sprintf("LEADER_MENU:%d", menuPID), "floating"
		var input []byte
		pane := func(what, focus string) {
			check := func(f smokeFrame) bool {
				text := leaderSmokeBody(f, body)
				_, hex, ok := strings.Cut(strings.Join(strings.Fields(text), ""), "BYTES:")
				return ok && hex == fmt.Sprintf("%x", input) &&
					strings.TrimSpace(f.textAt(body.Min.X, body.Min.Y, body.Dx())) == identity &&
					strings.Contains(text, fmt.Sprintf("SIZE:%dx%d", body.Dx(), body.Dy())) &&
					f.hasPaneFooter("menu | "+where+" | focus:"+focus+" | running")
			}
			s.await(what, check)
			// The pane may cover readline. Reveal the actual line to prove keys
			// stayed with the intended child, then restore the same PID and focus.
			s.send("\x10t")
			s.await(what+" preserves parked Bash input", func(f smokeFrame) bool {
				return f.hasInput("prelude $", shellInput) && f.hasPaneFooter("menu hidden | running")
			})
			s.send("\x10t")
			if focus == "shell" {
				s.send("\x10\t")
			}
			s.await(what+" restores pane and focus", check)
		}
		pane("focused native menu with untouched Bash input", "pane")
		keys := "md?x\x03\x10\x10\x16\x10\x16\x07"
		s.send(keys)
		input = append(input, []byte("md?x\x03\x16\x10\x16\x07")...)
		pane("bare keys and quotes reach the pane; double Ctrl+P only locks", "pane")
		s.send("\x07")
		input = append(input, 0x07)
		pane("Ctrl+G reaches the pane without toggling its window", "pane")
		s.send("\x10")
		s.await("pane command mode visibly unlocks", func(f smokeFrame) bool { return f.hasFooter(smokeLeaderKeys, "") && f.panel("menu") })
		s.send("\x07")
		input = append(input, 0x07)
		pane("Ctrl+G reaches the pane and locks an unlocked command", "pane")
		s.send("\x10")
		s.await("second Ctrl+P can lock without child input", func(f smokeFrame) bool { return f.hasFooter(smokeLeaderKeys, "") })
		s.send("\x10")
		pane("second Ctrl+P locks without forwarding a literal prefix", "pane")
		s.send("\x16\x1bx\x16\x1bd\x16\x1bm")
		input = append(input, []byte("\x16\x1bx\x16\x1bd\x16\x1bm")...)
		pane("Ctrl+V bypasses all direct Alt actions", "pane")
		s.send("\x1b")
		input = append(input, 0x1b)
		pane("ordinary Escape reaches the child without changing layout", "pane")
		s.send("\x16\x1d\x16\x1c\x16\x1b[91;5u")
		// The pinned encoder retains distinct Ctrl+[ as CSI-u, not bare Esc.
		input = append(input, []byte("\x16\x1d\x16\x1c\x16\x1b[91;5u")...)
		pane("quoting bypasses next, fallback previous, and enhanced previous keys", "pane")
		paste := "\x1b[200~P\x1b[201~"
		s.send(paste)
		input = append(input, []byte(paste)...)
		pane("outer bracketed paste reaches bracketed-paste child", "pane")
		s.send("\x10t")
		s.await("Ctrl+P t hides native pane without stopping its PID", func(f smokeFrame) bool {
			return f.hasInput("prelude $", shellInput) && f.hasPaneFooter("menu hidden | running") &&
				!strings.Contains(leaderSmokeBody(f, body), identity) && syscall.Kill(menuPID, 0) == nil
		})
		s.send("-g")
		shellInput += "-g"
		s.await("hidden pane returns raw input to Bash", func(f smokeFrame) bool {
			return f.hasInput("prelude $", shellInput) && f.hasPaneFooter("menu hidden | running")
		})
		s.send("\x10t")
		pane("Ctrl+P t restores the same PID, raw input state, and focus", "pane")
		s.send("\x10\t")
		pane("Tab moves focus without replacing the pane", "shell")
		s.send("\x1b[200~-s\x1b[201~")
		shellInput += "-s"
		pane("paste goes to Bash while pane input stays unchanged", "shell")
		s.send("\x10\t")
		pane("Tab restores pane focus", "pane")
		for _, step := range []struct {
			where   string
			body    image.Rectangle
			binding string
			key     string
		}{
			{"left", image.Rect(0, 0, 45, 23), "\x1d", "L"},
			{"right", image.Rect(45, 0, 90, 23), "\x1d", "R"},
			{"top", image.Rect(0, 0, 90, 11), "\x1d", "T"},
			{"bottom", image.Rect(0, 12, 90, 23), "\x1d", "B"},
			{"floating", image.Rect(9, 2, 81, 21), "\x1d", "F"},
			{"bottom", image.Rect(0, 12, 90, 23), "\x1b[91;5u", "b"},
			{"top", image.Rect(0, 0, 90, 11), "\x1b[91;5u", "t"},
			{"right", image.Rect(45, 0, 90, 23), "\x1b[91;5u", "r"},
			{"left", image.Rect(0, 0, 45, 23), "\x1b[91;5u", "l"},
			{"floating", image.Rect(9, 2, 81, 21), "\x1c", "f"},
		} {
			s.send(step.binding + step.key)
			where, body = step.where, step.body
			input = append(input, step.key...)
			pane("direct layout key preserves PID/state and PTY size in "+where, "pane")
		}
		s.resize(120, 2)
		s.await("tiny height suspends pane and returns focus to Bash", func(f smokeFrame) bool {
			return f.cols == 120 && len(f.rows) == 2 && f.hasPaneFooter("menu | floating | focus:shell | running (hidden: resize to restore)")
		})
		s.send("-h")
		shellInput += "-h"
		s.resize(ghosttySmokeCols, ghosttySmokeRows)
		s.await("grow restores retained pane without stealing focus", func(f smokeFrame) bool {
			return strings.Contains(leaderSmokeBody(f, body), identity) && f.hasPaneFooter("menu | floating | focus:shell | running")
		})
		s.send("\x10\tR")
		input = append(input, 'R')
		pane("same child resumes; hidden input went only to Bash", "pane")
		s.send("\x10d")
		docs := s.await("switch safely reaps menu and paints docs child", func(f smokeFrame) bool {
			return strings.Contains(leaderSmokeBody(f, body), "LEADER_DOCS:") &&
				f.hasPaneFooter("docs | floating | focus:pane | running") && errors.Is(syscall.Kill(menuPID, 0), syscall.ESRCH)
		})
		docsIdentity := strings.TrimSpace(docs.textAt(body.Min.X, body.Min.Y, body.Dx()))
		s.send("q")
		finished := s.await("child exit retained while main Bash stays alive", func(f smokeFrame) bool {
			return strings.Contains(leaderSmokeBody(f, body), docsIdentity) && f.hasPaneFooter("docs | floating | focus:shell | exited:7")
		})
		s.send("\x10t")
		s.await("finished docs hides without restart", func(f smokeFrame) bool {
			return f.hasInput("prelude $", shellInput) && f.hasPaneFooter("docs hidden | exited:7")
		})
		s.send("-e")
		shellInput += "-e"
		s.await("Bash remains editable with a hidden final image", func(f smokeFrame) bool {
			return f.hasInput("prelude $", shellInput) && f.hasPaneFooter("docs hidden | exited:7")
		})
		s.send("\x10t")
		s.await("show restores final docs image without focus or restart", func(f smokeFrame) bool {
			return leaderSmokeBody(f, body) == leaderSmokeBody(finished, body) && f.hasPaneFooter("docs | floating | focus:shell | exited:7")
		})
		s.send("\x10d")
		s.await("explicit docs chord restarts a finished surface", func(f smokeFrame) bool {
			return strings.Contains(leaderSmokeBody(f, body), "LEADER_DOCS:") &&
				strings.TrimSpace(f.textAt(body.Min.X, body.Min.Y, body.Dx())) != docsIdentity && f.hasPaneFooter("docs | floating | focus:pane | running")
		})
		s.send("\x10?")
		notice := s.await("unsupported Ctrl+P ? never spawns a MOTD pane", func(f smokeFrame) bool {
			return f.hasPaneFooter(smokeUnknownChordStatus("docs | floating | focus:pane | running")) && strings.Contains(leaderSmokeBody(f, body), "LEADER_DOCS:")
		})
		assertSmokeFooterTheme(t, notice, smokePaneKeys, smokeUnknownChordStatus("docs | floating | focus:pane | running"))
		s.send("\x10c\r")
		s.await("closed panes leave original shell input executable", func(f smokeFrame) bool {
			return f.hasLine(strings.TrimPrefix(shellInput, "echo ")) && f.hasInput("prelude $", "") && f.footer() == smokeBaseFooter
		})
		s.send("\x1bx")
		s.await("live pane reopened", func(f smokeFrame) bool { return strings.Contains(leaderSmokeBody(f, body), "LEADER_MENU:") })
		s.send("\x10cecho leader-alive\r")
		s.await("closing a live child is safe for Bash", func(f smokeFrame) bool {
			return f.hasLine("leader-alive") && f.hasInput("prelude $", "") && f.footer() == smokeBaseFooter
		})
		s.send("echo failed-safe")
		s.await("parked input before failing menu", func(f smokeFrame) bool {
			return f.hasInput("prelude $", "echo failed-safe") && f.footer() == smokeBaseFooter
		})
		s.send("\x10x")
		s.await("menu reopened for failed selection", func(f smokeFrame) bool { return f.panel("menu") })
		s.send("q")
		s.await("nonzero menu exit never executes the source it wrote", func(f smokeFrame) bool {
			return f.hasPaneFooter("menu | floating | focus:shell | exited:7") && !f.hasLine("FAILED_MENU_SOURCE_RAN")
		})
		s.send("\x10c\r")
		s.await("Bash remains usable after the failed selection", func(f smokeFrame) bool {
			return f.hasLine("failed-safe") && f.hasInput("prelude $", "") && f.footer() == smokeBaseFooter
		})
		s.traceMu.Lock()
		failedSourceRan := strings.Contains(string(s.output), "FAILED_MENU_SOURCE_RAN")
		s.traceMu.Unlock()
		if failedSourceRan {
			t.Fatalf("host read and ran a failed menu's output\n%s", s.diagnostics())
		}
		s.send("exit 0\r")
		s.expectExit(0)
	})
}
