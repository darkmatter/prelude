#!/usr/bin/env python3
"""Verify independent MOTD activation paths without exported coordination state.

Two activation paths can run in one prompt-enabled `nix develop`: the consumer
shellHook evaluates preflight, then Prelude's setup hook sources the same init.
That same-shell re-entry must not render a second banner. Direnv evaluates
`.envrc` non-interactively and later hands its exports to an interactive shell;
those separate shells deliberately do not coordinate through exported state.

Stage 1 emulates prompt-enabled `nix develop`: an interactive Bash evaluates
preflight, skips the guarded setup-hook source, then evaluates preflight again.
The two explicit invocations must each render, while the automatic source stays
quiet.
Stage 2 emulates direnv: a non-interactive shell with DIRENV_IN_ENVRC set evals
the preflight snippet. It must render once without exporting private Prelude
coordination variables.
Stage 3 emulates the shell direnv hands off to: an interactive Bash on a PTY with
that captured environment runs the `prelude hook` trampoline. It must render the
banner once when the init loads, then stay quiet on the next ordinary prompt
because the hook tracks the loaded PRELUDE_INIT path.

usage: preflight-hook-pty-test.py BASH PRELUDE_INIT PREFLIGHT_SNIPPET HOOK PATH SENTINEL_TEXT
"""

import errno
import fcntl
import os
import pty
import select
import signal
import struct
import subprocess
import sys
import termios
import time


def fail(message: str, output: bytes = b"") -> None:
    print(f"preflight-hook PTY smoke: {message}", file=sys.stderr)
    if output:
        sys.stderr.buffer.write(output + b"\n")
    raise SystemExit(1)


def set_size(fd: int, columns: int, rows: int) -> None:
    size = struct.pack("HHHH", rows, columns, 0, 0)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, size)


def read_until_idle(fd: int, timeout: float, idle: float = 0.5) -> bytes:
    output = bytearray()
    deadline = time.monotonic() + timeout
    last = time.monotonic()
    while True:
        remaining = max(deadline - time.monotonic(), 0.0)
        wait = min(max(idle - (time.monotonic() - last), 0.0), remaining or 0.0)
        ready, _, _ = select.select([fd], [], [], wait)
        if not ready:
            if time.monotonic() >= deadline:
                break
            continue
        try:
            chunk = os.read(fd, 65536)
        except OSError as exc:
            if exc.errno == errno.EIO:
                break
            raise
        if not chunk:
            break
        output.extend(chunk)
        last = time.monotonic()
    return bytes(output)


def terminate(pid: int, fd: int) -> None:
    try:
        os.close(fd)
    except OSError:
        pass
    try:
        os.kill(pid, signal.SIGTERM)
        os.waitpid(pid, 0)
    except (OSError, ChildProcessError):
        pass


def base_env(command_path: str, home: str) -> dict[str, str]:
    os.makedirs(home, exist_ok=True)
    return {
        "HOME": home,
        "LANG": "C.UTF-8",
        "PATH": command_path,
        "TERM": "xterm-256color",
        "TMPDIR": os.environ.get("TMPDIR", "/tmp"),
    }


def same_shell_stage(
    bash: str,
    snippet: str,
    init: str,
    env: dict[str, str],
    sentinel_text: str,
) -> None:
    """Verify setup-hook dedupe without suppressing explicit preflight."""
    stage_env = dict(env, PRELUDE_INIT=init)
    command = f"""
. "{snippet}"
[ "$PRELUDE_INIT" = "${{_PRELUDE_INIT_LOADED-}}" ] || exit 91
if [ "$PRELUDE_INIT" != "${{_PRELUDE_INIT_LOADED-}}" ]; then
    . "{init}"
fi
. "{snippet}"
"""
    result = subprocess.run(
        [bash, "--norc", "--noprofile", "-i", "-c", command],
        env=stage_env,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        fail("same-shell activation exited non-zero", result.stderr)
    render_count = result.stderr.count(sentinel_text.encode())
    if render_count != 2:
        fail(
            f"two explicit preflights rendered the MOTD {render_count} times; expected twice",
            result.stderr,
        )


def envrc_stage(
    bash: str,
    snippet: str,
    init: str,
    env: dict[str, str],
    sentinel_text: str,
) -> dict[str, str]:
    """Run the preflight snippet the way direnv runs .envrc; return its exports."""
    stage_env = dict(env, PRELUDE_INIT=init, DIRENV_IN_ENVRC="1")
    result = subprocess.run(
        [bash, "--norc", "--noprofile", "-c", f'. "{snippet}"\nexec env -0'],
        env=stage_env,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        fail("envrc stage exited non-zero", result.stderr)
    if result.stderr.count(sentinel_text.encode()) != 1:
        fail("envrc stage did not render exactly one banner", result.stderr)
    exported = {}
    for entry in result.stdout.split(b"\0"):
        if not entry or b"=" not in entry:
            continue
        name, _, value = entry.partition(b"=")
        exported[name.decode()] = value.decode()
    private_exports = sorted(name for name in exported if name.startswith("_PRELUDE_"))
    if private_exports:
        fail(f"envrc stage exported private coordination state: {', '.join(private_exports)}")
    return exported


def interactive_stage(bash: str, hook: str, init: str, env: dict[str, str]) -> bytes:
    """Run an interactive Bash with the `prelude hook` trampoline installed."""
    stage_env = dict(env, PRELUDE_INIT=init, PS1="pflt$ ")

    pid, master = pty.fork()
    if pid == 0:
        os.execve(bash, [bash, "--norc", "--noprofile", "-i"], stage_env)

    try:
        set_size(master, 100, 40)
        # Drain the shell's first prompt, printed before the hook exists.
        read_until_idle(master, 6.0)
        # Sourcing the hook appends _prelude_hook to PROMPT_COMMAND, and Bash
        # prints the next prompt as soon as the source returns. Capture that
        # activation plus one ordinary prompt to prove the hook does not source
        # the same PRELUDE_INIT path repeatedly.
        os.write(master, f". {hook}\n".encode())
        output = bytearray(read_until_idle(master, 15.0))
        os.write(master, b"\n")
        output += read_until_idle(master, 15.0)
        os.write(master, b"exit\n")
        read_until_idle(master, 6.0)
        return bytes(output)
    finally:
        terminate(pid, master)




def main() -> None:
    if len(sys.argv) != 7:
        fail("usage: BASH PRELUDE_INIT SNIPPET HOOK PATH SENTINEL_TEXT")
    bash, init, snippet, hook, command_path, sentinel_text = sys.argv[1:]

    env = base_env(command_path, os.path.join(os.environ.get("TMPDIR", "/tmp"), "preflight-home"))
    same_shell_stage(bash, snippet, init, env, sentinel_text)
    exported = envrc_stage(bash, snippet, init, env, sentinel_text)
    interactive = interactive_stage(bash, hook, init, exported)
    render_count = interactive.count(sentinel_text.encode())
    if render_count != 1:
        fail(
            f"interactive hook rendered the MOTD {render_count} times; expected exactly once",
            interactive,
        )


if __name__ == "__main__":
    main()
