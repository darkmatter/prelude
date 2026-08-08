#!/usr/bin/env python3
"""Exercise Prelude's packaged Bash+Blesh textarea on a real PTY."""

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

import pyte

BACKGROUND_SEQUENCES = (b"48;2;32;32;32", b"48:2::32:32:32")
INPUT_MARKER = "Z"

WINDOW_BACKGROUND = "202020"
DEFAULT_BACKGROUND = "default"


def require_default_cells(
    label: str, screen: NonBceScreen, row: int, start: int, end: int
) -> None:
    for column in range(start, min(end, screen.columns)):
        cell = screen.buffer[row][column]
        if cell.bg.lower() not in (DEFAULT_BACKGROUND, ""):
            fail(
                f"{label} cell ({column}, {row}) has background {cell.bg!r}",
                screen_dump(screen),
            )


class NonBceScreen(pyte.Screen):
    """Model Warp: EL and ECH erase cells without carrying the active SGR."""

    def erase_characters(self, count: int | None = None) -> None:
        self.dirty.add(self.cursor.y)
        count = count or 1
        line = self.buffer[self.cursor.y]
        for x in range(self.cursor.x, min(self.cursor.x + count, self.columns)):
            line[x] = self.default_char

    def erase_in_line(self, how: int = 0, private: bool = False) -> None:
        del private
        self.dirty.add(self.cursor.y)
        if how == 0:
            interval = range(self.cursor.x, self.columns)
        elif how == 1:
            interval = range(self.cursor.x + 1)
        elif how == 2:
            interval = range(self.columns)
        else:
            return
        line = self.buffer[self.cursor.y]
        for x in interval:
            line[x] = self.default_char


def fail(message: str, output: bytes = b"") -> None:
    print(f"textarea PTY smoke: {message}", file=sys.stderr)
    if output:
        print(repr(output[-2000:]), file=sys.stderr)
    raise SystemExit(1)


def set_size(fd: int, columns: int, rows: int) -> None:
    size = struct.pack("HHHH", rows, columns, 0, 0)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, size)


def resize_terminal(fd: int, screen: NonBceScreen, columns: int, rows: int) -> None:
    set_size(fd, columns, rows)
    screen.resize(lines=rows, columns=columns)


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
    until_any: tuple[bytes, ...] = (),
    terminal: pyte.ByteStream | None = None,
) -> bytes:
    output = bytearray()
    answered: dict[bytes, int] = {}
    deadline = time.monotonic() + first_timeout
    last_read = 0.0
    required_seen = not until_any
    while True:
        now = time.monotonic()
        if output and required_seen:
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
        if until_any and any(marker in output for marker in until_any):
            required_seen = True
    return bytes(output)


def require_background(label: str, output: bytes) -> None:
    if not any(sequence in output for sequence in BACKGROUND_SEQUENCES):
        fail(f"{label} emitted no #202020 background", output)


def screen_dump(screen: NonBceScreen) -> bytes:
    return "\n".join(
        f"{row:02d}: {text!r}" for row, text in enumerate(screen.display)
    ).encode()


def locate_text(screen: NonBceScreen, text: str) -> tuple[int, int]:
    for row, line in enumerate(screen.display):
        column = line.find(text)
        if column >= 0:
            return row, column
    fail(f"screen contains no {text!r}", screen_dump(screen))
    raise AssertionError("unreachable")


def require_window_cells(
    label: str, screen: NonBceScreen, row: int, start: int, end: int
) -> None:
    for column in range(start, min(end, screen.columns)):
        cell = screen.buffer[row][column]
        if cell.bg.lower() != WINDOW_BACKGROUND:
            fail(
                f"{label} cell ({column}, {row}) has background {cell.bg!r}",
                screen_dump(screen),
            )


def require_wrapped_input(
    label: str, screen: NonBceScreen, expected_characters: int
) -> None:
    rows = [
        (
            row,
            [column for column, cell in line.items() if cell.data == INPUT_MARKER],
        )
        for row, line in screen.buffer.items()
    ]
    rows = [
        (row, columns)
        for row, columns in rows
        if INPUT_MARKER * 3 in screen.display[row]
    ]
    observed = sum(len(columns) for _, columns in rows)
    if observed != expected_characters:
        fail(
            f"{label} retained {observed}/{expected_characters} input cells",
            screen_dump(screen),
        )
    for row, columns in rows:
        require_window_cells(label, screen, row, min(columns), max(columns) + 1)
        last = max(columns) + 1
        if last < screen.columns and screen.buffer[row][last].data == " ":
            require_window_cells(label, screen, row, last, screen.columns)


def transact(
    fd: int,
    payload: bytes,
    label: str,
    timeout: float = 5.0,
    idle: float = 0.35,
    until_any: tuple[bytes, ...] = (),
    terminal: pyte.ByteStream | None = None,
) -> bytes:
    os.write(fd, payload)
    output = read_until_idle(fd, timeout, idle, until_any, terminal)
    if not output:
        fail(f"{label} emitted no terminal output")
    if until_any and not any(marker in output for marker in until_any):
        fail(f"{label} did not reach required terminal output", output)
    return output


def terminate(pid: int, fd: int) -> None:
    # Closing the PTY sends SIGHUP to the shell session and, critically, closes
    # the output inherited by any helper processes. Kill the whole session if
    # Bash does not retire promptly; a leaked child would stall the Nix builder.
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
        fail("usage: textarea-background-pty-test.py BASH INIT STARSHIP_CONFIG PATH")
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
        os.execve(bash, [bash, "--noprofile", "--norc", "-i"], environment)

    screen = NonBceScreen(48, 14)
    terminal = pyte.ByteStream(screen)
    try:
        resize_terminal(master, screen, 48, 14)
        startup = transact(
            master,
            f". {shlex.quote(init)}\r".encode(),
            "initial render",
            45.0,
            2.0,
            BACKGROUND_SEQUENCES + (b"disabled Blesh textarea background adapter",),
            terminal,
        )
        if b"disabled Blesh textarea background adapter" in startup:
            fail("packaged runtime rejected the pinned Blesh source", startup)
        require_background("initial render", startup)

        # Warp behaves as a non-BCE terminal. Force the same Blesh capability
        # state before exercising deletion and blank-cell creation on the PTY.
        transact(master, b"_ble_term_bce=\r", "non-BCE setup", terminal=terminal)

        typed = transact(master, b"printf prelude", "typing", terminal=terminal)
        require_background("typing", typed)

        deleted = transact(master, b"\x7f", "deletion", terminal=terminal)
        require_background("deletion", deleted)
        if b" " not in deleted:
            fail("deletion emitted no replacement space", deleted)
        command_row, command_start = locate_text(screen, "printf prelud")
        deleted_column = command_start + len("printf prelud")
        if screen.buffer[command_row][deleted_column].data != " ":
            fail("deletion left a non-blank cell", screen_dump(screen))
        require_window_cells(
            "deletion", screen, command_row, deleted_column, deleted_column + 1
        )

        transact(master, b"\x15", "line clear", terminal=terminal)
        if "printf prelud" in screen.display[command_row]:
            fail("line clear retained deleted input", screen_dump(screen))
        require_window_cells(
            "line clear",
            screen,
            command_row,
            command_start,
            command_start + len("printf prelud"),
        )

        resize_terminal(master, screen, 22, 14)
        try:
            os.killpg(pid, signal.SIGWINCH)
        except ProcessLookupError:
            fail("shell exited before wrapped-input check")
        read_until_idle(master, 3.0, terminal=terminal)
        wrapped = transact(
            master,
            b"printf " + INPUT_MARKER.encode() * 41,
            "wrapped input",
            8.0,
            terminal=terminal,
        )
        require_background("wrapped input", wrapped)
        require_wrapped_input("wrapped input", screen, 41)

        resize_terminal(master, screen, 36, 16)
        os.killpg(pid, signal.SIGWINCH)
        resized = read_until_idle(master, 6.0, terminal=terminal)
        if not resized:
            fail("resize emitted no terminal output")
        require_background("resize", resized)
        require_wrapped_input("resize", screen, 41)
        # Clear any leftover input from the wrapped-input test before
        # running ordinary command output.  \x15 on a non-empty line redraws;
        # on an empty line it emits nothing, so drain without requiring output.
        transact(master, b"\x15", "pre-echo clear", 3.0, terminal=terminal)
        # Ordinary command output: `echo hi` glyphs carry the window background.
        # Use octal-escaped output absent from the command bytes so
        # locate_text finds the output row, not the editable command.
        echo_output = transact(
            master,
            b"printf '\\150\\151\\n'\r",
            "echo handoff",
            until_any=b"hi".split(),
            terminal=terminal,
        )
        echo_row, echo_col = locate_text(screen, "hi")
        require_window_cells("echo handoff", screen, echo_row, echo_col, echo_col + 2)

        # Child reset boundary: an explicit SGR 0 returns subsequent glyphs
        # to the terminal default, proving Prelude does not rewrite child output.
        # Octal escapes keep the marker out of the command bytes.
        read_until_idle(master, 1.0, terminal=terminal)
        transact(
            master,
            b"printf '\\033[0m\\101\\102\\n'\r",
            "child reset",
            until_any=b"AB".split(),
            terminal=terminal,
        )
        reset_row, reset_col = locate_text(screen, "AB")
        require_default_cells("child reset", screen, reset_row, reset_col, reset_col + 2)

        # Redirected exit: `exit >file` must not contaminate the file with
        # any Prelude SGR bytes (including the reset sequence itself), and
        # the terminal must receive the cleanup reset on the TUI FD.
        read_until_idle(master, 1.0, terminal=terminal)
        exit_output = transact(
            master,
            b"exit >/tmp/prelude-exit-test\r",
            "redirected exit",
            8.0,
            terminal=terminal,
        )
        try:
            with open("/tmp/prelude-exit-test", "rb") as handle:
                exit_file = handle.read()
        except FileNotFoundError:
            exit_file = b""
        if b"\x1b" in exit_file:
            fail("redirected exit file contains escape bytes", exit_file)
        if b"\x1b[0m" not in exit_output and b"\x1b[m" not in exit_output:
            fail("redirected exit emitted no SGR reset on the TUI", exit_output)
    finally:
        terminate(pid, master)


if __name__ == "__main__":
    main()
