#!/usr/bin/env python3
"""Public `eval "$(prelude hook bash)"` workspace activation contract.

The sentinel substitutes only the native executable at public consumer evaluation:
PRELUDE_INIT, the CLI hook printer, and the configured workspace wrapper stay real.
A second stage uses the unmodified packaged workspace and real Menu/Docs. Pyte is
only an observer of the outer terminal; no workspace protocol/input code is copied.

usage: workspace-hook-pty-test.py BASH ZSH FIXTURES_JSON
"""

import errno
import fcntl
import json
import os
import pty
import select
import shlex
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time
import unicodedata
from pathlib import Path


PRIMARY = "public-primary> "
CHILD = "active-child> "
READY = b"WORKSPACE-SENTINEL-READY"
COLS, ROWS = 140, 40


def sentinel() -> None:
    """One foreground launch, an optional active child, then a nonzero exit."""
    signal.alarm(20)
    tty = [os.isatty(fd) for fd in (0, 1, 2)]
    event = {
        "kind": "launch",
        "argv": sys.argv[2:],
        "init": os.environ.get("PRELUDE_INIT"),
        "active": os.environ.get("PRELUDE_WORKSPACE_ACTIVE"),
        "tty": tty,
        "foreground": tty[0] and os.tcgetpgrp(0) == os.getpgrp(),
        "terminal": os.ttyname(0) if tty[0] else None,
        "parent": os.getppid(),
    }
    with open(os.environ["PRELUDE_TEST_LAUNCH_LOG"], "a", encoding="utf-8") as log:
        log.write(json.dumps(event) + "\n")
    print(READY.decode(), flush=True)
    for line in sys.stdin:
        if line.strip() == "child":
            result = subprocess.run(
                [os.environ["PRELUDE_TEST_BASH"], "--noprofile", "--rcfile",
                 os.environ["PRELUDE_TEST_CHILD_RC"], "-i"],
                check=False, timeout=10,
            )
            if result.returncode != 0:
                raise SystemExit(result.returncode)
            print("WORKSPACE-SENTINEL-CHILD-DONE", flush=True)
        else:
            break
    print("WORKSPACE-SENTINEL-EXIT:37", flush=True)
    raise SystemExit(37)


def events(log: Path) -> list[dict]:
    if not log.exists():
        return []
    return [json.loads(line) for line in log.read_text().splitlines() if line]


def launches(log: Path) -> list[dict]:
    return [event for event in events(log) if event["kind"] == "launch"]


def environment(fixture: dict, directory: Path, bash: str) -> dict[str, str]:
    home = directory / "home"
    home.mkdir()
    return {
        "HOME": str(home),
        "XDG_CACHE_HOME": str(directory / "cache"),
        "TMPDIR": str(directory),
        "PATH": fixture["commandPath"],
        "LANG": "C.UTF-8",
        "LC_ALL": "C.UTF-8",
        "TERM": "xterm-256color",
        "SHELL": bash,
        "HISTFILE": "/dev/null",
        "INPUTRC": "/dev/null",
        "PRELUDE_INIT_QUIET": "1",
        "PRELUDE_MOTD_PURE": "1",
        "PRELUDE_TEST_BASH": bash,
        "PRELUDE_TEST_LAUNCH_LOG": str(directory / "launches.jsonl"),
    }


class HookPTY:
    """A bounded outer-shell loop, synchronized by prompts/files, not sleeps."""

    def __init__(self, command: list[str], env: dict[str, str], directory: Path,
                 stdin: Path | None = None, stdout: Path | None = None):
        import pyte

        self.raw = bytearray()
        self.screen = pyte.Screen(COLS, ROWS)
        self.stream = pyte.ByteStream(self.screen)
        self.status = None
        self.eof = False
        self.pid, self.fd = pty.fork()
        if self.pid == 0:
            fcntl.ioctl(0, termios.TIOCSWINSZ, struct.pack("HHHH", ROWS, COLS, 0, 0))
            os.chdir(directory)
            if stdin is not None:
                fd = os.open(stdin, os.O_RDONLY)
                os.dup2(fd, 0)
                os.close(fd)
            if stdout is not None:
                fd = os.open(stdout, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
                os.dup2(fd, 1)
                os.close(fd)
            os.execve(command[0], command, env)

    def __enter__(self):
        return self

    def __exit__(self, *_):
        # Closing the controlling PTY also hangs up a still-running workspace.
        # Kill the foreground group and outer shell on failure; never block in
        # waitpid without a deadline (especially after a failed autoentry).
        if self.status is None:
            try:
                foreground = os.tcgetpgrp(self.fd)
                if foreground > 0 and foreground != os.getpgrp():
                    os.killpg(foreground, signal.SIGKILL)
            except OSError:
                pass
            try:
                os.kill(self.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            deadline = time.monotonic() + 2
            while self.status is None and time.monotonic() < deadline:
                self.poll()
                time.sleep(0.01)
        os.close(self.fd)

    def poll(self):
        if self.status is None:
            pid, status = os.waitpid(self.pid, os.WNOHANG)
            if pid:
                self.status = os.waitstatus_to_exitcode(status)

    def read(self):
        if not self.eof and select.select([self.fd], [], [], 0.05)[0]:
            try:
                data = os.read(self.fd, 65536)
            except OSError as exc:
                if exc.errno != errno.EIO:
                    raise
                data = b""
            self.eof = not data
            self.raw.extend(data)
            self.stream.feed(data)
        self.poll()

    def diagnostic(self) -> str:
        return "\n".join(self.screen.display) + f"\nraw tail: {bytes(self.raw[-4000:])!r}"

    def require(self, condition: bool, description: str):
        if not condition:
            raise AssertionError(description + "\n" + self.diagnostic())

    def wait(self, predicate, description: str, timeout: float = 8):
        deadline = time.monotonic() + timeout
        while not predicate():
            self.read()
            if predicate():
                return
            if time.monotonic() >= deadline or (self.eof and self.status is not None):
                self.require(False, f"timed out waiting for {description}; exit={self.status}")

    def send(self, text: str) -> int:
        mark = len(self.raw)
        os.write(self.fd, text.encode())
        return mark

    def prompt(self, mark: int = 0, text: str = PRIMARY):
        self.wait(lambda: text.encode() in self.raw[mark:], f"fresh {text!r} prompt")

    def command(self, text: str):
        self.prompt(self.send(text + "\n"))

    def finish(self):
        self.send("exit\n")
        self.wait(lambda: self.status is not None, "outer shell exit")
        self.require(self.status == 0, f"outer shell exited {self.status}")


def bash_command(bash: str, directory: Path, extra: str = "") -> list[str]:
    rc = directory / "outer.bashrc"
    rc.write_text(f"PS1={shlex.quote(PRIMARY)}\nPS2='public-secondary> '\n"
                  f"tty > {shlex.quote(str(directory / 'outer-terminal'))}\n{extra}\n")
    return [bash, "--noprofile", "--rcfile", str(rc), "-i"]


def install_loader(session: HookPTY, fixture: dict, log: Path, shape: str):
    # Only the preceding loader, not the eval command, applies PRELUDE_INIT.
    # Installing the public hook twice also covers its string/array dedupe.
    command = (
        '_test_loader() { '
        'if [ -n "${_test_next_init-}" ]; then '
        'export PRELUDE_INIT="$_test_next_init" PATH="$_test_next_path"; '
        'else unset PRELUDE_INIT; fi; '
        f'printf \'{{"kind":"loader","init":"%s"}}\\n\' "${{PRELUDE_INIT-}}" >> {shlex.quote(str(log))}; '
        '}; '
        + ("PROMPT_COMMAND=(_test_loader); " if shape == "array"
           else "PROMPT_COMMAND=_test_loader; ")
        + f'_test_next_init={shlex.quote(fixture["init"])}; '
        + f'_test_next_path={shlex.quote(fixture["commandPath"])}; '
        + 'unset PRELUDE_INIT; eval "$(prelude hook bash)"; eval "$(prelude hook bash)"'
    )
    return session.send(command + "\n")


def enter(session: HookPTY, fixture: dict, log: Path, count: int, mark: int):
    session.wait(lambda: len(launches(log)) >= count and READY in session.raw[mark:],
                 f"{fixture['name']} automatic foreground launch #{count}")
    entries = events(log)
    calls = launches(log)
    session.require(len(calls) == count, f"workspace launched more than {count} times: {entries}")
    call = calls[-1]
    expected = ["--starship"] if fixture["promptEnabled"] else []
    session.require(call["argv"] == expected, f"automatic Starship arguments: {call}")
    session.require(call["init"] == fixture["init"], f"loader applied the wrong init: {call}")
    session.require(call["active"] == "1", f"public wrapper did not mark its child active: {call}")
    session.require(call["tty"] == [True, True, True] and call["foreground"],
                    f"workspace is not on the foreground terminal: {call}")
    terminal = Path(log.parent / "outer-terminal").read_text().strip()
    session.require(call["terminal"] == terminal, f"workspace did not inherit the outer terminal {terminal}: {call}")
    session.require(call["parent"] == session.pid, f"workspace detached from the outer Bash: {call}")
    position = len(entries) - 1
    session.require(position > 0 and entries[position - 1] ==
                    {"kind": "loader", "init": fixture["init"]},
                    f"Prelude ran before the environment loader: {entries}")
    session.require(PRIMARY.encode() not in session.raw[mark:],
                    "outer prompt returned while workspace was still running")


def snapshot_command(path: Path) -> str:
    return ("printf '%s\\n' \"$PS1\" \"${PRELUDE_WORKSPACE_ACTIVE-}\" "
            "\"${BLE_VERSION-}\" \"$(declare -F ble-attach)\" "
            f"> {shlex.quote(str(path))}")


def assert_outer(session: HookPTY, directory: Path, active: str = ""):
    snapshot = directory / "outer-state"
    session.command(snapshot_command(snapshot))
    session.require(snapshot.read_text().splitlines() == [PRIMARY, active, "", ""],
                    f"workspace activation mutated the primary prompt, leaked its active marker, or attached BLE: {snapshot.read_text()!r}")


def lifecycle(bash: str, fixture: dict, other: dict, shape: str):
    with tempfile.TemporaryDirectory(prefix=f"workspace-hook-{fixture['name']}-{shape}-") as temp:
        directory = Path(temp)
        env = environment(fixture, directory, bash)
        log = Path(env["PRELUDE_TEST_LAUNCH_LOG"])
        child_rc = directory / "active-child.bashrc"
        child_rc.write_text(f'PS1={shlex.quote(CHILD)}\neval "$(prelude hook bash)"\n')
        env["PRELUDE_TEST_CHILD_RC"] = str(child_rc)
        with HookPTY(bash_command(bash, directory), env, directory) as session:
            session.prompt()
            enter(session, fixture, log, 1, install_loader(session, fixture, log, shape))

            # The wrapper's marker must survive a real interactive child shell
            # installing the public hook and reaching repeated prompts.
            session.prompt(session.send("child\n"), CHILD)
            state = directory / "child-state"
            mark = session.send(snapshot_command(state) + "\n")
            session.prompt(mark, CHILD)
            session.require(state.read_text().splitlines() == [CHILD, "1", "", ""],
                            "active child changed its prompt/attached BLE/lost the active marker")
            session.prompt(session.send("\n"), CHILD)
            session.require(len(launches(log)) == 1, "active child recursively launched a workspace")
            mark = session.send("exit\n")
            session.wait(lambda: b"WORKSPACE-SENTINEL-CHILD-DONE" in session.raw[mark:], "active child exit")

            session.prompt(session.send("exit-sentinel\n"))
            assert_outer(session, directory)
            for command in ("", "", 'eval "$(prelude hook bash)"'):
                session.command(command)
                session.require(len(launches(log)) == 1, "workspace relaunched after exit on an unchanged init")

            # A changed generated init must activate on that very next prompt,
            # with the new consumer's automatic Starship policy.
            mark = session.send(f'_test_next_init={shlex.quote(other["init"])}; '
                                f'_test_next_path={shlex.quote(other["commandPath"])}\n')
            enter(session, other, log, 2, mark)
            session.prompt(session.send("exit-sentinel\n"))
            session.command("")
            session.require(len(launches(log)) == 2, "changed-init workspace relaunched after exit")

            session.command("_test_next_init=")
            session.command("")
            session.require(len(launches(log)) == 2, "leaving the environment launched a workspace")
            mark = session.send(f'_test_next_init={shlex.quote(fixture["init"])}; '
                                f'_test_next_path={shlex.quote(fixture["commandPath"])}\n')
            enter(session, fixture, log, 3, mark)
            session.prompt(session.send("exit-sentinel\n"))
            assert_outer(session, directory)
            session.command("")
            session.require(len(launches(log)) == 3, "reentered workspace relaunched on the following prompt")
            session.finish()
    print(f"PASS sentinel {fixture['name']} ({shape} PROMPT_COMMAND): loader ordering, foreground/TTY/active, child recursion, exit/dedupe, changed init, leave/reentry", flush=True)


def guards(bash: str, zsh: str, fixture: dict):
    with tempfile.TemporaryDirectory(prefix=f"workspace-hook-guards-{fixture['name']}-") as temp:
        directory = Path(temp)
        env = environment(fixture, directory, bash)
        env["PRELUDE_INIT"] = fixture["init"]
        log = Path(env["PRELUDE_TEST_LAUNCH_LOG"])
        # Exercise the generated public init as a loader does, including .envrc
        # export capture. A hook alone in `bash -c` would never reach activation.
        for context in ({}, {"DIRENV_IN_ENVRC": "1"}):
            result = subprocess.run(
                [bash, "--noprofile", "--norc", "-c",
                 'eval "$(prelude hook bash)"; . "$PRELUDE_INIT"; env -0'],
                env=dict(env, **context), capture_output=True, timeout=10, check=False,
            )
            assert result.returncode == 0, (fixture["name"], context, result.stderr)
            exported = {entry.partition(b"=")[0] for entry in result.stdout.split(b"\0") if b"=" in entry}
            assert not any(name.startswith(b"_PRELUDE_") for name in exported), exported
            assert b"PRELUDE_WORKSPACE_ACTIVE" not in exported, exported
            assert not launches(log), (context, events(log), result.stderr)

        # Noninteractive with both descriptors on a controlling TTY is still
        # not a request to take over the terminal (lorri's builder lifecycle).
        with HookPTY([bash, "--noprofile", "--norc", "-c", '. "$PRELUDE_INIT"'], env, directory) as session:
            session.wait(lambda: session.status is not None, "noninteractive TTY evaluation")
            session.require(session.status == 0 and not launches(log), "noninteractive TTY launched workspace")

        for context in ({"DIRENV_IN_ENVRC": "1"}, {"PRELUDE_WORKSPACE_ACTIVE": "1"}):
            rc = bash_command(bash, directory, 'eval "$(prelude hook bash)"')
            with HookPTY(rc, dict(env, **context), directory) as session:
                session.prompt()
                session.command("")
                assert_outer(session, directory, context.get("PRELUDE_WORKSPACE_ACTIVE", ""))
                session.require(not launches(log), f"suppressed interactive context launched: {context}")
                session.finish()

        # Test each descriptor separately: `-t 0 || -t 1` must not pass.
        input_file = directory / "piped-input"
        input_file.write_text("\nexit\n")
        rc = bash_command(bash, directory, 'eval "$(prelude hook bash)"')
        with HookPTY(rc, env, directory, stdin=input_file) as session:
            session.wait(lambda: session.status is not None, "non-TTY stdin shell exit")
            session.require(session.status == 0 and not launches(log), "non-TTY stdin launched workspace")
        with HookPTY(rc, env, directory, stdout=directory / "piped-output") as session:
            session.prompt()
            session.command("")
            session.require(not launches(log), "non-TTY stdout launched workspace")
            session.finish()

        # A real zsh hook/init evaluation is supported, but must not start this
        # Bash-only TUI. Use -f to avoid inherited user/global interactive rc.
        with HookPTY([zsh, "-d", "-f", "-i", "-c",
                      'eval "$(prelude hook zsh)"; . "$PRELUDE_INIT"'], env, directory) as session:
            session.wait(lambda: session.status is not None, "zsh evaluation")
            session.require(session.status == 0 and not launches(log), "zsh launched a Bash workspace")
    print(f"PASS guards {fixture['name']}: noninteractive, lorri/TTY, direnv export + interactive, active child, stdin-only/stdout-only TTY, zsh", flush=True)


def zsh_transition(bash: str, zsh: str, workspace: dict, standalone: dict):
    # Unsupported workspace entry must not consume later standalone prompt init.
    with tempfile.TemporaryDirectory(prefix="workspace-hook-zsh-transition-") as temp:
        directory = Path(temp)
        env = environment(workspace, directory, bash)
        env.update(SHELL=zsh, PS1=PRIMARY, PRELUDE_INIT=workspace["init"])
        log = Path(env["PRELUDE_TEST_LAUNCH_LOG"])
        with HookPTY([zsh, "-d", "-f", "-i"], env, directory) as session:
            session.prompt()
            session.command('eval "$(prelude hook zsh)"')
            session.command("")
            session.command("unset PRELUDE_INIT")
            enter = (f'export PRELUDE_INIT={shlex.quote(standalone["init"])} '
                     f'PATH={shlex.quote(standalone["commandPath"])} '
                     f'STARSHIP_CONFIG={shlex.quote(standalone["promptConfig"])}')
            # This marker exists only in the real consumer's Starship Config,
            # never in the typed command, so an echoed command cannot pass.
            prompt = "PUBLIC-ZSH-STARSHIP> "
            session.prompt(session.send(enter + "\n"), prompt)
            for command in ("", "unset PRELUDE_INIT", enter):
                session.prompt(session.send(command + "\n"), prompt)
            session.require(not launches(log), "zsh workspace/standalone transition launched a workspace")
            session.require(b"\x1b[?1049h" not in session.raw, "zsh transition entered a workspace alternate screen")
            session.finish()
    print("PASS zsh transition: workspace-enabled public hook then standalone real Starship activation and leave/reentry; no workspace", flush=True)


def disabled_hook(bash: str, fixture: dict):
    with tempfile.TemporaryDirectory(prefix="workspace-hook-disabled-") as temp:
        directory = Path(temp)
        env = environment(fixture, directory, bash)
        env["PRELUDE_INIT"] = fixture["init"]
        with HookPTY(bash_command(bash, directory, 'eval "$(prelude hook bash)"'), env, directory) as session:
            session.prompt()
            session.command("")
            assert_outer(session, directory)
            session.command("unset PRELUDE_INIT")
            session.command(f'export PRELUDE_INIT={shlex.quote(fixture["init"])}')
            assert_outer(session, directory)
            session.require(not launches(Path(env["PRELUDE_TEST_LAUNCH_LOG"])), "disabled workspace launched")
            session.require(b"\x1b[?1049h" not in session.raw, "disabled workspace entered alternate screen")
            session.finish()
    print("PASS disabled workspace: public hook keeps primary prompt, including leave/reentry", flush=True)


def manual_flags(bash: str, fixtures: list[dict]):
    for fixture in fixtures:
        with tempfile.TemporaryDirectory(prefix="workspace-hook-manual-") as temp:
            directory = Path(temp)
            env = environment(fixture, directory, bash)
            log = Path(env["PRELUDE_TEST_LAUNCH_LOG"])
            for arguments in ([], ["--starship"]):
                result = subprocess.run([fixture["launcher"], *arguments], input="exit-sentinel\n",
                                        text=True, capture_output=True, env=env, timeout=5, check=False)
                assert result.returncode == 37, (arguments, result)
                call = launches(log)[-1]
                assert call["argv"] == arguments and call["active"] == "1", call
    print("PASS manual workspace flags: bare launch stays bare; --starship is forwarded in both consumers", flush=True)


def real_workspace(bash: str, fixture: dict):
    with tempfile.TemporaryDirectory(prefix="workspace-hook-real-") as temp:
        directory = Path(temp)
        env = environment(fixture, directory, bash)
        log = Path(env["PRELUDE_TEST_LAUNCH_LOG"])
        with HookPTY(bash_command(bash, directory), env, directory) as session:
            session.prompt()
            baseline = termios.tcgetattr(session.fd)
            mark = install_loader(session, fixture, log, "array")
            legend = "Alt + [m]─motd──[x]─menu──[d]─docs"

            def primary_prompt(input_text=""):
                rows = session.screen.display
                row = next((y for y in range(ROWS - 2, 1, -1) if rows[y].startswith("╰─")), None)
                # Pyte stores combining characters as NFC. Match its cells,
                # but still require every character at the newest full prompt.
                expected = unicodedata.normalize("NFC", "╰─ " + input_text).rstrip()
                return (row is not None and "LOCKED" in rows[-1] and "Ctrl+P" in rows[-1]
                        and legend in rows[row - 2] and rows[row - 1].rstrip() == "│"
                        and rows[row].rstrip() == expected)

            session.wait(primary_prompt, "real packaged workspace + generated Starship primary prompt")
            session.require(b"\x1b[?1049h" in session.raw[mark:], "real workspace never entered alternate screen")
            session.require(PRIMARY.encode() not in session.raw[mark:], "autoentry returned instead of running foreground")
            header = next(y for y, row in enumerate(session.screen.display) if legend in row)
            key_x = session.screen.display[header].index(legend) + len("Alt + [")
            key = session.screen.buffer[header][key_x]
            label = session.screen.buffer[header][key_x - 1]
            session.require(key.data == "m" and key.bold and key.bg == "default" and key.fg != label.fg,
                            "generated Starship shortcut lost its original bold/accent/transparent styling")

            state = directory / "real-child-state"
            session.send(snapshot_command(state) + "\n")
            session.wait(state.exists, "real workspace child environment")
            session.require(state.read_text().splitlines()[-3:] == ["1", "", ""],
                            "real workspace child lost the public active marker or attached BLE")
            session.wait(primary_prompt, "primary prompt after real child probe")

            # Park actual Unicode readline input, open the real configured
            # Menu and Docs, then hide the pane and recover the primary prompt.
            parked = "echo PUBLIC-PARKED-界é"
            session.send(parked)
            session.wait(lambda: primary_prompt(parked), "parked Unicode input at primary prompt")
            session.send("\x10x")
            session.wait(lambda: f"{fixture['project']} — command menu" in "\n".join(session.screen.display)
                         and f"{fixture['name']}-task" in "\n".join(session.screen.display)
                         and "menu | floating | focus:pane | running" in session.screen.display[-1],
                         "actual consumer Menu on automatic workspace")
            session.send("\x10d")
            session.wait(lambda: f"{fixture['name']} documentation sentinel" in "\n".join(session.screen.display)
                         and "docs | floating | focus:pane | running" in session.screen.display[-1],
                         "actual consumer Docs on automatic workspace")
            session.send("\x10t")
            session.wait(lambda: primary_prompt(parked), "same primary prompt/input after hiding Docs")
            session.require("documentation sentinel" not in "\n".join(session.screen.display),
                            "hidden Docs still paints over primary prompt")
            session.prompt(session.send("\x15exit\n"))
            session.require(b"\x1b[?1049l" in session.raw[mark:], "workspace did not leave alternate screen on exit")
            session.require(termios.tcgetattr(session.fd) == baseline, "workspace did not restore outer readline terminal attributes")
            session.require(os.tcgetpgrp(session.fd) == session.pid, "workspace did not restore outer foreground Bash")
            assert_outer(session, directory)
            entry_count = session.raw.count(b"\x1b[?1049h")
            session.command("")
            session.command("")
            session.require(entry_count == 1 and session.raw.count(b"\x1b[?1049h") == 1,
                            "real workspace relaunched after exit at the next prompt")
            session.finish()
    print("PASS real packaged workspace: public hook autoentry, generated Starship, Unicode primary input, actual Menu/Docs, outer prompt/TTY/foreground restoration, no relaunch", flush=True)


def main():
    if len(sys.argv) != 4:
        raise SystemExit("usage: workspace-hook-pty-test.py BASH ZSH FIXTURES_JSON")
    bash, zsh, manifest = sys.argv[1:]
    fixtures = json.loads(Path(manifest).read_text())
    def expired(*_):
        raise AssertionError("workspace-hook suite exceeded 120s")

    signal.signal(signal.SIGALRM, expired)
    signal.alarm(120)
    a, b = fixtures["sentinel"]
    for fixture, other in ((a, b), (b, a)):
        for shape in ("string", "array"):
            lifecycle(bash, fixture, other, shape)
        guards(bash, zsh, fixture)
    zsh_transition(bash, zsh, a, fixtures["standalone"])
    disabled_hook(bash, fixtures["disabled"])
    manual_flags(bash, fixtures["sentinel"])
    real_workspace(bash, fixtures["real"])
    signal.alarm(0)


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--sentinel":
        sentinel()
    else:
        try:
            main()
        except (AssertionError, subprocess.TimeoutExpired) as error:
            print(f"workspace-hook PTY regression: {error}", file=sys.stderr)
            raise SystemExit(1) from error
