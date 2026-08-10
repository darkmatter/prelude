#!/usr/bin/env python3
"""Verify ~/.blerc can enable the Prelude vim-airline theme.

Sources the generated Prelude init in a shell whose ~/.blerc imports
lib/vim-airline and selects the Prelude theme, then asserts the airline
status row rendered instead of failing with "theme 'prelude' not found".
"""

import errno
import fcntl
import os
import pty
import select
import shlex
import signal
import struct
import sys
import termios
import time
from typing import NoReturn

import pyte


def fail(message: str, output: bytes = b"") -> NoReturn:
    print(f"airline-theme PTY smoke: {message}", file=sys.stderr)
    if output:
        print(repr(output[-2000:]), file=sys.stderr)
    raise SystemExit(1)


def set_size(fd: int, columns: int, rows: int) -> None:
    size = struct.pack("HHHH", rows, columns, 0, 0)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, size)


def answer_terminal_queries(
    fd: int, output: bytearray, answered: dict[bytes, int]
) -> None:
    replies = {
        b"\x1b[5n": b"\x1b[0n",
        b"\x1b[6n": b"\x1b[1;1R",
        b"\x1b[?6n": b"\x1b[?1;1R",
        b"\x1b[c": b"\x1b[?1;2c",
        b"\x1b[0c": b"\x1b[?1;2c",
        b"\x1b[>c": b"\x1b[>0;0;0c",
        b"\x1b[>0c": b"\x1b[>0;0;0c",
        b"\x1b]10;?\x07": b"\x1b]10;rgb:ffff/ffff/ffff\x1b\\",
        b"\x1b]11;?\x07": b"\x1b]11;rgb:0000/0000/0000\x1b\\",
        b"\x1b]10;?\x1b\\": b"\x1b]10;rgb:ffff/ffff/ffff\x1b\\",
        b"\x1b]11;?\x1b\\": b"\x1b]11;rgb:0000/0000/0000\x1b\\",
    }
    for query, reply in replies.items():
        observed = output.count(query)
        for _ in range(observed - answered.get(query, 0)):
            os.write(fd, reply)
        answered[query] = observed


def read_until_idle(
    fd: int,
    first_timeout: float = 8.0,
    idle: float = 0.35,
    terminal: pyte.ByteStream | None = None,
) -> bytes:
    output = bytearray()
    answered: dict[bytes, int] = {}
    deadline = time.monotonic() + first_timeout
    last_read = 0.0
    while True:
        now = time.monotonic()
        if output:
            timeout = max(0.0, min(idle - (now - last_read), deadline - now))
        else:
            timeout = max(0.0, deadline - now)
        if timeout == 0.0:
            break
        readable, _, _ = select.select([fd], [], [], timeout)
        if not readable:
            break
        try:
            chunk = os.read(fd, 65536)
        except OSError as error:
            if error.errno == errno.EIO:
                break
            raise
        if not chunk:
            break
        output.extend(chunk)
        if terminal is not None:
            terminal.feed(chunk)
        answer_terminal_queries(fd, output, answered)
        last_read = time.monotonic()
    return bytes(output)


def transact(
    fd: int,
    payload: bytes,
    label: str,
    timeout: float = 5.0,
    idle: float = 0.35,
    terminal: pyte.ByteStream | None = None,
) -> bytes:
    os.write(fd, payload)
    output = read_until_idle(fd, timeout, idle, terminal)
    if not output:
        fail(f"{label} emitted no terminal output")
    return output


def screen_dump(screen: pyte.Screen) -> bytes:
    return "\n".join(
        f"{row:02d}: {text!r}" for row, text in enumerate(screen.display)
    ).encode()


def locate_text(screen: pyte.Screen, text: str) -> tuple[int, int]:
    for row, line in enumerate(screen.display):
        column = line.find(text)
        if column >= 0:
            return row, column
    fail(f"screen contains no {text!r}", screen_dump(screen))


def terminate(pid: int, fd: int) -> None:
    try:
        os.close(fd)
    except OSError:
        pass
    for _ in range(20):
        try:
            waited, _ = os.waitpid(pid, os.WNOHANG)
        except ChildProcessError:
            return
        if waited == pid:
            return
        time.sleep(0.05)
    try:
        os.killpg(pid, signal.SIGKILL)
    except ProcessLookupError:
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            return
    for _ in range(20):
        try:
            waited, _ = os.waitpid(pid, os.WNOHANG)
        except ChildProcessError:
            return
        if waited == pid:
            return
        time.sleep(0.05)


def main() -> None:
    if len(sys.argv) != 5:
        fail("usage: airline-theme-pty-test.py BASH INIT STARSHIP_CONFIG PATH")
    bash, init, starship_config, command_path = sys.argv[1:]

    pid, master = pty.fork()
    if pid == 0:
        environment = os.environ.copy()
        environment.update(
            {
                "BASH_SILENCE_DEPRECATION_WARNING": "1",
                "HOME": os.path.join(os.environ.get("TMPDIR", "/tmp"), "airline-home"),
                "LANG": "C.UTF-8",
                "PATH": command_path,
                "PRELUDE_INIT_QUIET": "1",
                "STARSHIP_CONFIG": starship_config,
                "TERM": "xterm-256color",
                "USER": os.environ.get("USER") or "prelude-test",
                "XDG_CACHE_HOME": os.path.join(
                    os.environ.get("TMPDIR", "/tmp"), "airline-cache"
                ),
            }
        )
        os.makedirs(environment["HOME"], exist_ok=True)
        os.makedirs(environment["XDG_CACHE_HOME"], exist_ok=True)
        # blerc runs while ble.sh loads inside Prelude's init, before any
        # bleopt call in bash-init.bash; the runtime must already be on
        # import_path for the theme to resolve.
        with open(os.path.join(environment["HOME"], ".blerc"), "w") as blerc:
            blerc.write("ble-import lib/vim-airline\n")
            blerc.write("bleopt vim_airline_theme=prelude\n")
        os.chdir(environment["HOME"])
        os.execve(bash, [bash, "--noprofile", "--norc", "-i"], environment)

    screen = pyte.Screen(80, 16)
    terminal = pyte.ByteStream(screen)
    try:
        set_size(master, 80, 16)
        output = transact(
            master,
            f". {shlex.quote(init)}\r".encode(),
            "initial render",
            45.0,
            2.0,
            terminal,
        )
        if b"not found" in output:
            fail("vim-airline theme lookup failed during init", output)
        # The airline bar took over the status row: its encoding and history
        # segments render on the bottom row (one typed command precedes the
        # prompt, hence !2/2).
        locate_text(screen, "UTF-8[unix]")
        locate_text(screen, "!2/2")
        if any("Run commands:" in row for row in screen.display):
            fail(
                "Prelude status row is still active despite lib/vim-airline",
                screen_dump(screen),
            )
        if any("not found" in row for row in screen.display):
            fail("vim-airline theme lookup error on screen", screen_dump(screen))
    finally:
        terminate(pid, master)


if __name__ == "__main__":
    main()
