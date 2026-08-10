#!/usr/bin/env python3
"""Verify Blesh prompt final rewrite and POSTEXEC spacing before the next prompt."""

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
    print(f"prompt-final PTY smoke: {message}", file=sys.stderr)
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

def blank_rows_between(screen: pyte.Screen, start_row: int, end_row: int) -> int:
    """Count fully blank display rows in (start_row, end_row)."""
    if end_row <= start_row + 1:
        return 0
    return sum(
        1
        for row in range(start_row + 1, end_row)
        if not screen.display[row].strip()
    )


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
        fail("usage: prompt-final-pty-test.py BASH INIT STARSHIP_CONFIG PATH")
    bash, init, starship_config, command_path = sys.argv[1:]

    pid, master = pty.fork()
    if pid == 0:
        environment = os.environ.copy()
        environment.update(
            {
                "BASH_SILENCE_DEPRECATION_WARNING": "1",
                "HOME": os.path.join(os.environ.get("TMPDIR", "/tmp"), "pty-home"),
                "LANG": "C.UTF-8",
                "PATH": command_path,
                "PRELUDE_INIT_QUIET": "1",
                "STARSHIP_CONFIG": starship_config,
                "TERM": "xterm-256color",
                "USER": os.environ.get("USER") or "prelude-test",
                "XDG_CACHE_HOME": os.path.join(
                    os.environ.get("TMPDIR", "/tmp"), "pty-cache"
                ),
            }
        )
        os.makedirs(environment["HOME"], exist_ok=True)
        os.makedirs(environment["XDG_CACHE_HOME"], exist_ok=True)
        os.chdir(environment["HOME"])
        os.execve(bash, [bash, "--noprofile", "--norc", "-i"], environment)

    screen = pyte.Screen(48, 14)
    terminal = pyte.ByteStream(screen)
    try:
        set_size(master, 48, 14)
        transact(
            master,
            f". {shlex.quote(init)}\r".encode(),
            "initial render",
            45.0,
            2.0,
            terminal,
        )
        def assert_prompt_above_status(label: str) -> None:
            context_row = -1
            for row, line in enumerate(screen.display):
                if "╭" in line:
                    context_row = row
            if context_row < 0:
                fail(f"{label}: no framed prompt context row", screen_dump(screen))

            # Live format: context, stem │, ╰─ input (cursor stays here).
            input_row = context_row + 2
            if input_row >= screen.lines or not screen.display[input_row].startswith(
                "╰─ "
            ):
                fail(
                    f"{label}: editable prompt is not on ╰─ two rows below context",
                    screen_dump(screen),
                )
            if screen.cursor.y != input_row:
                fail(
                    f"{label}: cursor row {screen.cursor.y} != ╰─ row {input_row}",
                    screen_dump(screen),
                )

            status_row, _ = locate_text(screen, "Run commands: x <cmd>")
            # Blank spacer panel is docked immediately above status.
            spacer_row = status_row - 1
            if spacer_row <= input_row:
                fail(
                    f"{label}: no room for blank spacer above status "
                    f"(input={input_row}, status={status_row})",
                    screen_dump(screen),
                )
            if screen.display[spacer_row].strip() != "":
                fail(
                    f"{label}: row immediately above status is not blank: "
                    f"{screen.display[spacer_row]!r}",
                    screen_dump(screen),
                )
            if status_row != spacer_row + 1:
                fail(
                    f"{label}: status is not adjacent to blank spacer "
                    f"(spacer={spacer_row}, status={status_row})",
                    screen_dump(screen),
                )

        assert_prompt_above_status("initial render")

        final_command = ": prelude-final-prompt"
        transact(master, final_command.encode(), "final command", terminal=terminal)
        transact(master, b"\r", "final prompt rewrite", 10.0, 1.0, terminal)
        submitted_row, command_start = locate_text(screen, final_command)
        prefix = screen.display[submitted_row][:command_start]
        if not prefix.startswith("╰─ "):
            fail(
                "submitted prompt did not keep the framed ╰─ row "
                f"(prefix={prefix!r})",
                screen_dump(screen),
            )
        if "❯" in prefix:
            fail(
                "submitted prompt still collapsed to the character module",
                screen_dump(screen),
            )
        # Full muted chrome should leave a context row above the command row.
        if submitted_row < 2 or "╭" not in screen.display[submitted_row - 2]:
            fail(
                "submitted prompt lost the muted context row above ╰─",
                screen_dump(screen),
            )

        # After a real command, blank-above-status adjacency must hold.
        marker = "prelude-gap-mark"
        spacing_command = f"printf '%s\\n' {marker}"
        transact(master, spacing_command.encode(), "gap command", terminal=terminal)
        transact(master, b"\r", "gap command exec", 10.0, 1.5, terminal)
        locate_text(screen, marker)
        assert_prompt_above_status("after command")
    finally:
        terminate(pid, master)


if __name__ == "__main__":
    main()
