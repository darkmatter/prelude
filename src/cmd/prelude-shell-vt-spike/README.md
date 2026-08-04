# prelude-shell-vt-spike

**PROTOTYPE — delete or absorb after the architecture question is answered.**

## Question

Can Prelude keep a real interactive shell in a short pane while MOTD, Docs,
and status chrome remain pinned, without rewriting the child shell's VT
escape sequences into outer-terminal coordinates?

This sibling spike puts the child PTY behind `github.com/charmbracelet/x/vt`.
Only a composed cell frame reaches the physical terminal; the child never
writes to it directly.

## Run

Enter Prelude's interactive development environment so the child receives its
`bash-interactive`, ble.sh, generated `STARSHIP_CONFIG`, and Starship binary:

```bash
nix develop
cd src/cmd/prelude-shell-vt-spike && go run .
```

Use `-shell-rows 1` to make the isolation especially obvious:

```bash
cd src/cmd/prelude-shell-vt-spike && go run . -shell-rows 1
```

| Shell command | Action |
| --- | --- |
| `vtpin motd` | Pin a fresh MOTD capture above the shell |
| `vtpin docs` | Start a live, navigable Docs surface above the shell |
| `vtpin off` | Remove the pinned surface and expand the shell |
| `exit` / **Ctrl+D** | Exit the child shell and restore the outer terminal |

With live Docs pinned, press **Ctrl+G** to toggle input between the shell and
Docs. The `input:shell` / `input:docs` field in the bottom status row shows the
current owner. Docs redraws in place as you navigate; press **Ctrl+G** again to
return to the shell. Pressing `q` while Docs owns input exits Docs and unpins it.

`vtpin` is a shell function installed only in the hosted child. Its distinct
name leaves Prelude's production Zellij-backed `pin` command unshadowed. It
sends an allowlisted target over a private control pipe, so the host does not parse
terminal output. Ctrl+G is the host's explicit input-owner toggle when live
Docs is ready; otherwise it and all other keys are forwarded to the shell.
Interactive commands such as `menu` are deliberately not pin targets; run them
in the shell pane where they retain input ownership.

The status row exposes the relevant in-memory state after every event: pin
mode, input owner, shell geometry, child main/alternate screen, panel
generation, and panel load phase.

## What this intentionally proves

- Child CUP, ED, DECSTBM, saved-cursor, and alternate-screen state stays inside
  the virtual terminal.
- Resize changes both the virtual screen and PTY window; no Ctrl+L is injected.
- Static panel commands load asynchronously and stale resize results are
  discarded.
- Panel commands render into their own off-screen VT, including terminal-query
  replies, so their TTY colors, cursor addressing, and erase behavior survive
  composition.
- Docs remains a live PTY and virtual terminal. Its redraws update only the pin
  band, and resize delivers the new geometry to both Docs and the shell.
- Ctrl+G switches keyboard, paste, focus, cursor, and mouse ownership between
  the live Docs PTY and the shell PTY without changing either process's screen.
- Pin selection is explicit (`vtpin motd`, `vtpin docs`, `vtpin off`) and travels over
  a dedicated child-to-host control channel rather than a captured hotkey.
- The child initializes the same Bash ble.sh + Starship stack (or native Zsh
  Starship integration) and consumes the inherited `STARSHIP_CONFIG`
  unchanged, matching the prompt from `nix develop`.
- Bubble Tea owns raw mode, the outer alternate screen, rendering, and cleanup.

It is not production hardening: panel capture is best-effort, there are no
end-to-end tests, and `x/vt` is pinned experimental code.

## Observed result — 2026-08-02

The architecture question passed its first live PTY smoke:

- A child `ED + CUP` full-screen clear changed only the shell cells while the
  pinned rows remained intact.
- Child alternate-screen entry changed the exposed state from `child:main` to
  `child:alt` without disturbing the Docs panel.
- Resizing the outer PTY from 80×24 to 100×30 recomputed the pinned layout as
  `shell:100x3@26`, resized the child PTY, and discarded the old panel
  generation.
- Printable Bubble Tea input must use `Key.Text`; forwarding only `Key.Code`
  loses shifted characters such as uppercase letters and punctuation.
- Normal child `exit` restored the outer terminal screen and modes.
- A regression probe confirmed panel commands receive TTY stdin, stdout, and
  stderr; the original flattening happened after capture in `ansi.Strip`.
- A live run against the repository's real MOTD renderer preserved its palette,
  bold/faint attributes, full-screen erase, cursor-addressed layout, and OSC 11
  background query across an 80×24 → 100×30 resize.
- A live run from interactive `nix develop` reused its package-owned prompt
  setup hook: the child reported the same generated `STARSHIP_CONFIG`, attached
  the same ble.sh version, and rendered the Prelude Powerline prompt without
  leaking Bash non-printing markers into the VT cells.
- The explicit control path selected MOTD and Docs directly, rejected
  `vtpin menu` without changing the active panel, and expanded the shell with
  `vtpin off`.
- `vtpin docs` rendered the repository's real Docs viewer, including its
  sidebar, selected page, status bar, colors, and alternate-screen layout.
- Ctrl+G changed the reported owner from `input:shell` to `input:docs`; `j`
  moved the selected page from `prelude` to `Quickstart (Setup Wizard)` and
  redrew the live Docs body. A second Ctrl+G returned input to the styled shell.
- `q` while Docs owned input exited the Docs child and unpinned the surface.
  `vtpin off`, replacing the panel, and exiting the hosted shell also reaped the
  live Docs process without leaving a child behind.

The remaining decision is whether to absorb the cell-compositor approach into
a real Prelude shell host or delete this spike after further hands-on use.
