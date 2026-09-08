#!/usr/bin/env python3
"""Verify MOTD leaves every cell outside its opaque card at terminal default.

Pyte's Screen models Background Color Erase (BCE): erase and scroll operations
fill cells with the cursor's current background attributes. A non-BCE model is
simulated by overriding erase to use the terminal default.

The test seeds an inherited non-default background before the MOTD render,
then verifies both terminal models agree that the card is the only painted
horizontal footprint. The renderer must reset to SGR 49 before positioning,
screen erasure, spacer rows, and the final prompt handoff.
"""

import errno
import fcntl
import os
import pty
import select
import signal
import struct
import sys
import termios
import time

import pyte


def fail(message: str, output: bytes = b"") -> None:
    print(f"motd-bg PTY smoke: {message}", file=sys.stderr)
    if output:
        print(repr(output[-2000:]), file=sys.stderr)
    raise SystemExit(1)


def set_size(fd: int, columns: int, rows: int) -> None:
    size = struct.pack("HHHH", rows, columns, 0, 0)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, size)


def read_until_idle(fd: int, timeout: float, idle: float = 0.35) -> bytes:
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


class NonBCEScreen(pyte.Screen):
    """A screen where erases always fill with terminal-default, not cursor bg.

    Non-BCE terminals (e.g. some macOS Terminal.app configurations) never
    inherit the cursor's background when erasing cells. This subclass overrides
    erase_in_line and erase_in_display to always use default_char, matching
    that behaviour.
    """

    def erase_in_line(self, how: int = 0, private: bool = False) -> None:
        saved = self.cursor.attrs
        self.cursor.attrs = self.default_char
        super().erase_in_line(how, private)
        self.cursor.attrs = saved

    def erase_in_display(self, how: int = 0, *args, **kwargs) -> None:
        saved = self.cursor.attrs
        self.cursor.attrs = self.default_char
        super().erase_in_display(how, *args, **kwargs)
        self.cursor.attrs = saved


def run_motd(motd_bin: str, command_path: str) -> bytes:
    """Invoke the motd binary via PTY and return its raw byte output."""
    cols, rows = 80, 24

    pid, master = pty.fork()
    if pid == 0:
        env = os.environ.copy()
        env.update(
            {
                "HOME": os.path.join(os.environ.get("TMPDIR", "/tmp"), "motd-bg-home"),
                "LANG": "C.UTF-8",
                "PATH": command_path,
                "TERM": "xterm-256color",
            }
        )
        os.makedirs(env["HOME"], exist_ok=True)
        os.execve(motd_bin, [motd_bin, "--pure"], env)

    try:
        set_size(master, cols, rows)
        return read_until_idle(master, 10.0, 0.5)
    finally:
        terminate(pid, master)


def find_card_footprint(
    screen: pyte.Screen, seeded_bg: str
) -> tuple[int, int, str]:
    """Find the dominant painted card background and its horizontal bounds."""
    counts: dict[str, int] = {}
    for row in range(screen.lines):
        for col in range(screen.columns):
            background = screen.buffer[row][col].bg
            if background not in {"default", seeded_bg}:
                counts[background] = counts.get(background, 0) + 1
    if not counts:
        fail("could not find an opaque card background in screen output")

    card_bg = max(counts, key=counts.get)
    columns = [
        col
        for row in range(screen.lines)
        for col in range(screen.columns)
        if screen.buffer[row][col].bg == card_bg
    ]
    return min(columns), max(columns) + 1, card_bg


def check_screen(
    screen: pyte.Screen, raw_output: bytes, model_name: str, seeded_bg: str
) -> None:
    foot_left, foot_right, card_bg = find_card_footprint(screen, seeded_bg)

    violations: list[str] = []
    for row in range(screen.lines):
        for col in range(screen.columns):
            cell = screen.buffer[row][col]
            if cell.bg == seeded_bg:
                violations.append(
                    f"  ({row},{col}): inherited background leaked after render"
                )
            elif cell.data in "░▒▓":
                violations.append(
                    f"  ({row},{col}): obsolete fringe glyph {cell.data!r}"
                )
            elif not (foot_left <= col < foot_right) and cell.bg != "default":
                violations.append(
                    f"  ({row},{col}): gutter cell bg={cell.bg!r}, want default"
                )
    if violations:
        dump = "\n".join(
            f"{row:02d}: {screen.display[row]}" for row in range(screen.lines)
        )
        fail(
            f"[{model_name}] {len(violations)} cells violate the bounded card:\n"
            + "\n".join(violations[:20])
            + f"\nCard bg: {card_bg}; cols {foot_left}-{foot_right-1}\nScreen:\n{dump}",
            raw_output,
        )


def main() -> None:
    if len(sys.argv) != 3:
        fail("usage: motd-bg-pty-test.py MOTD_BIN COMMAND_PATH")
    motd_bin, command_path = sys.argv[1:]

    # Seed a non-default background before rendering. Fresh screens begin at
    # bg="default", which cannot prove the renderer actively clears inherited
    # background state before positioning and handing off to the prompt.
    seeded_bg = "#010203"
    seed_sgr = b"\x1b[48;2;1;2;3m"

    raw_output = run_motd(motd_bin, command_path)
    if not raw_output.strip():
        fail("motd produced no output")

    if b"\x1b[49m" not in raw_output:
        fail("motd output has no SGR 49 default-background reset", raw_output)

    # Re-feed into both models with the inherited non-default background active.
    # The seed writes one cell to set the cursor bg, then the MOTD output runs
    # under that inherited state — proving SGR 49 resets are necessary.
    bce_screen = pyte.Screen(80, 24)
    bce_stream = pyte.ByteStream(bce_screen)
    bce_stream.feed(seed_sgr + b" ")
    if bce_screen.buffer[0][0].bg != seeded_bg:
        fail(f"BCE seed failed: cell bg={bce_screen.buffer[0][0].bg!r}, want {seeded_bg!r}")
    bce_stream.feed(raw_output)
    check_screen(bce_screen, raw_output, "BCE", seeded_bg)

    non_bce_screen = NonBCEScreen(80, 24)
    non_bce_stream = pyte.ByteStream(non_bce_screen)
    non_bce_stream.feed(seed_sgr + b" ")
    if non_bce_screen.buffer[0][0].bg != seeded_bg:
        fail(f"non-BCE seed failed: cell bg={non_bce_screen.buffer[0][0].bg!r}, want {seeded_bg!r}")
    non_bce_stream.feed(raw_output)
    check_screen(non_bce_screen, raw_output, "non-BCE", seeded_bg)
