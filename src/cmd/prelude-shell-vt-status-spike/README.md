# prelude-shell-vt-status-spike

**PROTOTYPE — delete or absorb after the status-only question is answered.**

## Question

Can Prelude's shell host be useful without a pinned Docs pane: a real shell plus
a two-row, host-owned status surface and scrollback viewport?

The child shell runs behind `github.com/charmbracelet/x/vt`. Its escape
sequences never reach the physical terminal directly; the host composes its
cells into every row except the bottom two status rows.

ble.sh owns the hint surface — completion descriptions, ghost text, sabbrev
snippets, and syntax-checked accept — because it is strictly richer than a
host-side line-edit mirror. The Go host keeps only what ble.sh structurally
cannot do: VT composition, scrollback viewport, and the shared 2-row chrome.

## Run

Enter Prelude's interactive development environment so the child has the same
ble.sh, generated `STARSHIP_CONFIG`, and Starship setup as `nix develop`:

```bash
nix develop
cd src/cmd/prelude-shell-vt-status-spike && go run .
```

ble.sh is bash-only, so the integration requires the `bash` shell that
`nix develop` provides. The host errors early if `$SHELL` is not bash.

| Key | Action |
| --- | --- |
| **Tab** on `x ` | ble.sh descriptive completion menu of project tasks (with descriptions) |
| **Tab** after a command | ble.sh completes that command's declared option candidates |
| **Shift+PgUp** | Scroll one host page back through shell history |
| **Shift+PgDn** | Scroll one host page toward the live shell |
| Any other key or paste | Return to the live tail, then send input to the shell |
| `exit` / **Ctrl+D** | Exit the child shell and restore the outer terminal |

`-scrollback` controls retained history and defaults to 5000 lines. The status
surface reads `PRELUDE_MENU_CONFIG`, Prelude's generated menu JSON, and emits a
shell-sourceable catalog fragment (`prelude-catalog.sh`) alongside the
version-controlled ble.sh integration (`ble/prelude-ble.sh`). The idle first
row shows the same explicitly selected lifecycle commands as the MOTD; the
second row is the `KeyHintsFooter` center row used by menu: scrollback key
hints on the left and live `● ready` / `● error` status on the right.

The Nix package exports the immutable JSON path as `PRELUDE_MENU_CONFIG`; run
inside `nix develop` so the host sees the same catalogue and palette as `menu`.

There is intentionally no `vtpin`, `motd`, `docs`, panel capture, host-side line
mirror, or second child PTY in this module. The experiment is whether the
scrollback surface plus a ble.sh-owned hint surface earns a dedicated host.

## Architecture

```
┌──────────────────────────────────────┐  host-owned (Go)
│  VT-composed shell + scrollback      │   x/vt emulator, pty, key forwarding
├──────────────────────────────────────┤
│  MOTD lifecycle row (idle chrome)     │
├──────────────────────────────────────┤
│  KeyHintsFooter: hints + ● ready      │
└──────────────────────────────────────┘
        │
        │ child shell (bash + ble.sh)
        ▼
┌──────────────────────────────────────┐  ble.sh-owned (in the child PTY)
│  descriptive completion menu (desc)   │   ble/complete/cand/yield + descriptions
│  auto-complete ghost text            │   complete_auto_complete, face auto_complete
│  sabbrev snippets  (\date, \branch)  │   ble-sabbrev -m
│  syntax validation on accept         │   edit_magic_accept=sabbrev:verify-syntax
│  menu filter as you type              │   complete_menu_filter
└──────────────────────────────────────┘
```

The Go host emits `prelude-catalog.sh` (pure data: the `prelude_catalog_*`
arrays and `prelude_palette_*` 256-color indices) into the temp rc directory
and sources it before `prelude-ble.sh`. The integration script is the only
place ble.sh faces are derived from the Prelude palette, so the host's chrome
and ble.sh's menu/ghost-text/markers read the same colors by construction.

## ble.sh opportunity map

Prelude's locked dev shell currently supplies ble.sh
`0.4.0-devel3-unstable-2026-06-27`. The useful next experiments divide cleanly:

| Capability | Supported surface | Status in this spike |
| --- | --- | --- |
| Descriptive completion menu | `ble/complete/cand/yield ACTION CAND DATA`, `complete_menu_style=desc`, `menu_desc_*` faces | Wired: `x ` yields task names with descriptions |
| Auto-complete ghost text | `complete_auto_complete`, `complete_auto_delay`, face `auto_complete` | Enabled |
| Menu filter as you type | `complete_menu_filter`, faces `menu_filter_*` | Enabled |
| sabbrev snippets | `ble-sabbrev -m` (dynamic, shell-function-backed) | Seeded with `\date`, `\pwd`; project targets TBD |
| Syntax validation on accept | `edit_magic_accept=sabbrev:verify-syntax` | Enabled |
| Theming tightness | `ble-face` driven from `prelude_palette_*` (256-color indices) | Wired in `prelude/ble/apply-palette` |
| Native bottom status | `prompt_status_line`, `prompt_status_align` | Unused — the host owns the bottom chrome to stay shared with `menu` |

References: [Recipes](https://github.com/akinomyoga/ble.sh/wiki/Recipes),
[completion developer API](https://github.com/akinomyoga/ble.sh/wiki/Devel-%C2%A72-Completion),
[widget developer API](https://github.com/akinomyoga/ble.sh/wiki/Devel-%C2%A71-Design-widgets),
[editing/status-line options](https://github.com/akinomyoga/ble.sh/wiki/Manual-%C2%A74-Editing),
[completion and auto-completion options](https://github.com/akinomyoga/ble.sh/wiki/Manual-%C2%A77-Completion),
and [public key-binding APIs](https://github.com/akinomyoga/ble.sh/wiki/Manual-%C2%A73-Key-Binding).

## Gates to confirm interactively

These could not be runtime-verified without an interactive ble.sh session and
are wired per the ble.sh wiki but left for `nix develop` testing:

1. **Descriptive menu renders `DATA` as the description** for a single shared
   `prelude_cand` action (modelled on Recipes R3's per-action face +
   `init-menu-item`). If descriptions do not appear, define a per-command action
   instead of the shared one.
2. **Empty-prompt Tab cycles project tasks in catalog order**, not
   alphabetical. `compopt -o nosort` is the lever; confirm `complete_menu_style=desc`
   takes effect per-completion as Devel §2.1.2 promises.
3. **`ble-face` truecolor support** — the palette is passed as 256-color indices
   to stay portable. If Manual §2 Color confirms `#rrggbb`, `rgbaTo256` can be
   dropped in favour of passing hex directly.
