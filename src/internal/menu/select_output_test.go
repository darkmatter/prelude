package menu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMenuCLISelectOutput(t *testing.T) {
	const helperEnv = "PRELUDE_MENU_TEST_SELECT_OUTPUT_CLI"
	if os.Getenv(helperEnv) == "1" {
		args := flag.Args()
		flag.CommandLine = flag.NewFlagSet("menu", flag.ExitOnError)
		os.Args = append([]string{os.Args[0]}, args...)
		Run()
		os.Exit(0)
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var environ []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "BASH_ENV", "ENV", "SSH_CLIENT", "SSH_CONNECTION", "TERM", "PRELUDE_MENU_CONFIG", "PRELUDE_MENU_DEBUG", helperEnv:
			continue
		}
		environ = append(environ, entry)
	}
	environ = append(environ, helperEnv+"=1", "TERM=xterm-256color")
	command := func(t *testing.T, args ...string) *exec.Cmd {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		t.Cleanup(cancel)
		cmd := exec.CommandContext(ctx, executable, append([]string{"-test.run=^TestMenuCLISelectOutput$", "--"}, args...)...)
		cmd.Env = environ
		return cmd
	}
	run := func(t *testing.T, args ...string) (string, string, error) {
		t.Helper()
		cmd := command(t, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		return stdout.String(), stderr.String(), err
	}
	writeConfig := func(t *testing.T, cfg *Config) string {
		t.Helper()
		raw, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "menu.json")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	readResult := func(t *testing.T, path string) string {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("selection permissions = %o, want 600", info.Mode().Perm())
		}
		return string(raw)
	}

	t.Run("help", func(t *testing.T) {
		stdout, stderr, err := run(t, "--help")
		if err != nil || stdout != "" || !strings.Contains(stderr, "--select-output PATH") ||
			!strings.Contains(stderr, "never execute") || !strings.Contains(stderr, "cancellation") {
			t.Fatalf("help: stdout %q, stderr %q, error %v", stdout, stderr, err)
		}
	})

	t.Run("returns-source-without-executing", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "executed")
		task := Task{Name: "echo", Key: "e", Run: "printf 'ran:%s\\n' > " + shellWord(marker), Args: []Arg{{Token: "VALUE", Required: true}}}
		cfg := testMenuConfig(task)
		cfg.Execute = true
		config := writeConfig(t, cfg)
		for _, tc := range []struct {
			name  string
			flags []string
			args  []string
			line  string
		}{
			{name: "x-key", flags: []string{"--x"}, args: []string{"echo", "prod"}, line: "prod"},
			{name: "embedded-shortcut", flags: []string{"--x", "--embedded"}, args: []string{"e", "--", "'two words'", "--select-output"}, line: "'two words' --select-output"},
			{name: "menu-key", args: []string{"echo", "prod"}, line: "prod"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				output := filepath.Join(t.TempDir(), "selection.sh")
				if err := os.WriteFile(output, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				// Even an existing file with broader permissions must become private.
				if err := os.Chmod(output, 0o644); err != nil {
					t.Fatal(err)
				}
				args := append([]string{"--config", config, "--select-output", output}, tc.flags...)
				stdout, stderr, err := run(t, append(args, tc.args...)...)
				if err != nil || stdout != "" || stderr != "" {
					t.Fatalf("selection: stdout %q, stderr %q, error %v", stdout, stderr, err)
				}
				if got, want := readResult(t, output), task.Run+" "+tc.line; got != want {
					t.Fatalf("selection source = %q, want %q", got, want)
				}
				if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("command ran before host handoff: marker stat = %v", err)
				}
			})
		}
	})

	t.Run("host-source-preserves-execution-context", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "work 'tree")
		first, second := filepath.Join(root, "bin 'one"), filepath.Join(root, "bin two")
		for _, path := range []string{dir, first, second} {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		for path, script := range map[string]string{
			filepath.Join(first, "greet"):  "#!/bin/sh\nprintf 'first:'\npwd\n",
			filepath.Join(second, "greet"): "#!/bin/sh\nprintf 'wrong prefix\\n'\n",
		} {
			if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		task := Task{Name: "greet", Run: "greet\nprintf 'second line\\n' # trailing comment", Dir: dir, PathPrefix: []string{first, second}}
		cfg := testMenuConfig(task)
		cfg.Execute = true
		output := filepath.Join(root, "selection.sh")
		stdout, stderr, err := run(t, "--config", writeConfig(t, cfg), "--x", "--embedded", "--select-output", output, "greet")
		if err != nil || stdout != "" || stderr != "" {
			t.Fatalf("selection: stdout %q, stderr %q, error %v", stdout, stderr, err)
		}
		if source := readResult(t, output); !strings.Contains(source, task.Run) || !strings.HasSuffix(source, "# trailing comment\n)") {
			t.Fatalf("selection lost multiline source or swallowed the subshell close: %q", source)
		}
		bash, err := exec.LookPath("bash")
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		host := exec.CommandContext(ctx, bash, "--noprofile", "--norc", "-c", `source "$1"`, "host", output)
		host.Dir, host.Env = root, environ
		got, err := host.CombinedOutput()
		if want := "first:" + dir + "\nsecond line\n"; err != nil || string(got) != want {
			t.Fatalf("host source: output %q, want %q, error %v", got, want, err)
		}
	})

	t.Run("errors-never-publish-or-execute", func(t *testing.T) {
		cfg := testMenuConfig(Task{Name: "run", Run: "printf 'must not execute\\n'"})
		cfg.Execute = true
		config := writeConfig(t, cfg)
		for _, tc := range []struct {
			name       string
			flags      []string
			key        string
			outputKind string
			wantError  string
		}{
			{name: "missing-directory", key: "run", outputKind: "missing", wantError: "--select-output"},
			{name: "directory", key: "run", outputKind: "directory", wantError: "--select-output"},
			{name: "empty-path", key: "run", outputKind: "empty", wantError: "--select-output"},
			{name: "list", flags: []string{"--x", "--list"}, wantError: "cannot be combined"},
			{name: "imports", flags: []string{"--imports"}, wantError: "cannot be combined"},
			{name: "unknown-command", flags: []string{"--x"}, key: "missing", wantError: `unknown command "missing"`},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root := t.TempDir()
				output := filepath.Join(root, "selection.sh")
				if err := os.WriteFile(output, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				target := output
				switch tc.outputKind {
				case "missing":
					target = filepath.Join(root, "missing", "selection.sh")
				case "directory":
					target = root
				case "empty":
					target = ""
				}
				args := append([]string{"--config", config, "--select-output", target}, tc.flags...)
				if tc.key != "" {
					args = append(args, tc.key)
				}
				stdout, stderr, err := run(t, args...)
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() <= 0 || stdout != "" || !strings.Contains(stderr, tc.wantError) {
					t.Fatalf("failed selection: stdout %q, stderr %q, error %v", stdout, stderr, err)
				}
				if got := readResult(t, output); got != "" {
					t.Fatalf("failed selection published a command: %q", got)
				}
				if tc.outputKind == "missing" {
					if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("failed selection created output: %v", err)
					}
				}
			})
		}
	})

	t.Run("picker-handoff-and-cancellation", func(t *testing.T) {
		python, err := exec.LookPath("python3")
		if err != nil {
			t.Skip("real PTY coverage requires python3")
		}
		marker := filepath.Join(t.TempDir(), "executed")
		runText := "printf '%s\\n' > " + shellWord(marker)
		cfg := testMenuConfig(
			Task{Name: "run", Run: runText},
			Task{Name: "deploy", Run: runText, Args: []Arg{{Token: "ENV", Required: true}}},
			Task{Name: "module", Children: []Task{{Name: "module::child", Label: "child", Run: runText}}},
		)
		cfg.Execute, cfg.ColorProfile = true, "truecolor"
		config := writeConfig(t, cfg)
		for _, tc := range []struct {
			name, keys, ready, want string
			args                    []string
		}{
			{name: "root-selection", keys: "\r", ready: "command menu", want: runText},
			{name: "embedded-root-cancellation", keys: "\x03", ready: "command menu", args: []string{"--x", "--embedded"}},
			{name: "embedded-argument-selection", keys: "prod\r", ready: "enter arguments", args: []string{"--x", "--embedded", "deploy"}, want: runText + " prod"},
			{name: "subcommand-selection", keys: "\r", ready: "command menu", args: []string{"--x", "module"}, want: runText},
		} {
			t.Run(tc.name, func(t *testing.T) {
				output := filepath.Join(t.TempDir(), "selection.sh")
				if err := os.WriteFile(output, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				menu := command(t, append([]string{"--config", config, "--select-output", output}, tc.args...)...)
				ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
				defer cancel()
				args := append([]string{"-c", selectOutputPTY, tc.keys, tc.ready, output}, menu.Args...)
				pty := exec.CommandContext(ctx, python, args...)
				pty.Env = menu.Env
				if out, err := pty.CombinedOutput(); err != nil {
					t.Fatalf("menu PTY: %v (timeout: %v)\n%s", err, ctx.Err(), out)
				}
				if got := readResult(t, output); got != tc.want {
					t.Fatalf("picker source = %q, want %q", got, tc.want)
				}
				if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("picker executed instead of handing off: marker stat = %v", err)
				}
			})
		}
	})
}

// Use the same Python PTY facilities as the flake smoke checks, without adding
// a Go dependency or replacing the real picker input/rendering lifecycle.
const selectOutputPTY = `
import errno, fcntl, os, pty, select, signal, struct, sys, termios, time

pid, master = pty.fork()
if pid == 0:
    os.execv(sys.argv[4], sys.argv[4:])
output = bytearray()
sent = waited = False
try:
    fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", 34, 100, 0, 0))
    deadline = time.monotonic() + 8
    while time.monotonic() < deadline:
        if select.select([master], [], [], 0.1)[0]:
            try:
                output.extend(os.read(master, 65536))
            except OSError as exc:
                if exc.errno != errno.EIO:
                    raise
        if not sent and sys.argv[2].encode() in output:
            with open(sys.argv[3], "rb") as result:
                if result.read():
                    raise RuntimeError("command published before picker quit")
            os.write(master, sys.argv[1].encode())
            sent = True
        done, status = os.waitpid(pid, os.WNOHANG)
        if done:
            waited = True
            while select.select([master], [], [], 0)[0]:
                try:
                    chunk = os.read(master, 65536)
                except OSError as exc:
                    if exc.errno == errno.EIO:
                        break
                    raise
                if not chunk:
                    break
                output.extend(chunk)
            if not sent:
                raise RuntimeError("picker exited before accepting input: " + repr(output))
            if b"\x1b[?1049h" not in output or b"\x1b[?1049l" not in output:
                raise RuntimeError("picker did not restore its alt screen: " + repr(output))
            sys.exit(os.waitstatus_to_exitcode(status))
    raise RuntimeError("picker timed out: " + repr(output))
finally:
    if not waited:
        os.kill(pid, signal.SIGKILL)
        os.waitpid(pid, 0)
    os.close(master)
`
