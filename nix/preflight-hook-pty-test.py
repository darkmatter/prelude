#!/usr/bin/env python3
"""Verify loader-aware MOTD activation without duplicate renders.

Two activation paths can run in one prompt-enabled `nix develop`: the consumer
shellHook evaluates preflight, then Prelude's setup hook sources the same init.
That same-shell re-entry must not render a second banner. Direnv evaluates
`.envrc` non-interactively and later hands its exports to an interactive shell;
the loader shell's render is the one the developer sees, so the init records
it as PRELUDE_MOTD_SHOWN and the interactive handoff loads the runtime
without painting the banner again.

Stage 1 emulates prompt-enabled `nix develop`: an interactive Bash evaluates
preflight, skips the guarded setup-hook source, then evaluates preflight again.
The two explicit invocations must each render, while the automatic source stays
quiet.
Stage 2 emulates nix-direnv: a non-interactive shell with DIRENV_IN_ENVRC set
sources the init from the cached shellHook. It must render once, export no
private Prelude coordination variables, and record the render as
PRELUDE_MOTD_SHOWN keyed by the init path.
Stage 3 emulates the shell direnv hands off to: an interactive Bash on a PTY with
that captured environment runs the `prelude hook` trampoline. The envrc render
already greeted the developer, so hook activation and the next ordinary prompt
must stay quiet — while a deliberate re-source of the init still renders.
Stage 4 emulates a prompt runtime attaching while init is still running. An
already-installed trampoline must recognize that init and avoid a second render.

usage: preflight-hook-pty-test.py BASH PRELUDE_INIT PREFLIGHT_SNIPPET HOOK PATH SENTINEL_TEXT RUNTIME_INIT
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
import tempfile
import time
from pathlib import Path


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
    init: str,
    env: dict[str, str],
    sentinel_text: str,
) -> dict[str, str]:
    """Run automatic direnv activation; return captured exports."""
    stage_env = dict(env, PRELUDE_INIT=init, DIRENV_IN_ENVRC="1")
    result = subprocess.run(
        [
            bash,
            "--norc",
            "--noprofile",
            "-c",
            f'. "{init}"\nexec env -0',
        ],
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
    if exported.get("PRELUDE_MOTD_SHOWN") != init:
        fail(
            "envrc stage did not record its render as PRELUDE_MOTD_SHOWN=<init>: "
            f"{exported.get('PRELUDE_MOTD_SHOWN')!r}"
        )
    return exported


def interactive_stage(
    bash: str, hook: str, init: str, env: dict[str, str]
) -> tuple[bytes, bytes]:
    """Run an interactive Bash with the `prelude hook` trampoline installed.

    Returns (automatic, deliberate): output from hook activation plus one
    ordinary prompt, then output from deliberately re-sourcing the init.
    """
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
        # activation plus one ordinary prompt: the envrc shell already
        # rendered, so neither may paint the banner again.
        os.write(master, f". {hook}\n".encode())
        automatic = bytearray(read_until_idle(master, 15.0))
        os.write(master, b"\n")
        automatic += read_until_idle(master, 15.0)
        # The suppression is a one-shot handoff, not a mute: deliberately
        # re-sourcing the init is an explicit reprint and must render.
        os.write(master, f". {init}\n".encode())
        deliberate = read_until_idle(master, 15.0)
        os.write(master, b"exit\n")
        read_until_idle(master, 6.0)
        return bytes(automatic), deliberate
    finally:
        terminate(pid, master)


def reentrant_init_stage(
    bash: str,
    runtime_init: str,
    hook: str,
    env: dict[str, str],
    sentinel_text: str,
) -> None:
    """Ensure prompt attachment cannot source an init that is already running."""
    with tempfile.TemporaryDirectory(prefix="prelude-reentrant-") as runtime:
        runtime_path = Path(runtime)
        (runtime_path / "catalogue.bash").write_text("", encoding="utf-8")
        (runtime_path / "bash-init.bash").write_text(
            '_prelude_init_show_motd\neval "${PROMPT_COMMAND-}"\n',
            encoding="utf-8",
        )
        stage_env = dict(
            env,
            PRELUDE_INIT=runtime_init,
            _PRELUDE_SHELL_RUNTIME=runtime,
            _PRELUDE_PROMPT_ENABLED="1",
            _PRELUDE_MOTD="motd",
        )
        result = subprocess.run(
            [
                bash,
                "--norc",
                "--noprofile",
                "-i",
                "-c",
                f'. "{hook}"\n. "{runtime_init}"',
            ],
            env=stage_env,
            capture_output=True,
            check=False,
        )
    if result.returncode != 0:
        fail("re-entrant init stage exited non-zero", result.stderr)
    render_count = result.stderr.count(sentinel_text.encode())
    if render_count != 1:
        fail(
            f"re-entrant prompt setup rendered {render_count} banners; expected one",
            result.stderr,
        )


def main() -> None:
    if len(sys.argv) != 8:
        fail("usage: BASH PRELUDE_INIT SNIPPET HOOK PATH SENTINEL_TEXT RUNTIME_INIT")
    bash, init, snippet, hook, command_path, sentinel_text, runtime_init = sys.argv[1:]

    env = base_env(command_path, os.path.join(os.environ.get("TMPDIR", "/tmp"), "preflight-home"))
    same_shell_stage(bash, snippet, init, env, sentinel_text)
    exported = envrc_stage(bash, init, env, sentinel_text)
    automatic, deliberate = interactive_stage(bash, hook, init, exported)
    render_count = automatic.count(sentinel_text.encode())
    if render_count != 0:
        fail(
            f"interactive hook re-rendered the MOTD {render_count} times after the "
            "envrc render; expected the handoff to stay quiet",
            automatic,
        )
    render_count = deliberate.count(sentinel_text.encode())
    if render_count != 1:
        fail(
            f"deliberate init re-source rendered the MOTD {render_count} times; "
            "expected exactly once",
            deliberate,
        )
    reentrant_init_stage(bash, runtime_init, hook, env, sentinel_text)


if __name__ == "__main__":
    main()
