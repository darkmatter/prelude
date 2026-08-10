# Derived Shadow Palette Token

**Date:** 2026-08-05\
**Status:** Approved design

> **Superseded:** [Feathered MOTD Card](./2026-08-08-feathered-motd-card-design.md)
> removes terminal-wide background ownership. This document remains the
> historical record of the previously implemented design.

## Purpose

Prelude needs one low-contrast color below its effective terminal backdrop. The
color must track palette overrides and an explicitly configured MOTD window
background rather than becoming a set of stale, theme-specific hex literals.

The new semantic token is named `shadow`.

## Color contract

`shadow` is derived, never independently configured:

```text
shadow = Darken(base, 0.05)
```

`Darken` uses the same contract as Lip Gloss: multiply each 8-bit RGB channel
by `0.95` and truncate toward zero. For example, `#0e0b13` becomes `#0d0a12`.

The resolver operates after `prelude.palette.bg` overrides have merged with the
selected theme. It canonicalizes the existing accepted color forms exactly as
Lip Gloss does: `#RGB`, `#RRGGBB`, decimal ANSI indexes `0..255`, and decimal
packed 24-bit RGB values above that range. ANSI `0..15` use Lip Gloss's fixed
ANSI table; `16..255` use its xterm cube/grayscale mapping. Unsupported strings
fail configuration evaluation rather than silently deriving an unrelated color.
The derived output is always truecolor `#RRGGBB`, so Starship and ble.sh receive
the same deterministic color.

The exact five-percent rule is authoritative; there is no artificial minimum
contrast. A near-black base can therefore collapse to an equal or one-step-darker
shadow (for example, `#000000` remains `#000000`).

`shadow` is deliberately absent from `prelude.palette` options. Letting users
override it would sever the invariant that it is exactly five percent darker
than its selected base.

## Base selection

The base belongs to the effective window backdrop, not blindly to the selected
theme. `windowBackgroundSet` means Prelude actually paints that backdrop, so it
is false whenever `prelude.motd.enable = false`, regardless of the configured
window value. When MOTD is enabled, selection follows these rules:

| `windowBackground` value | `windowBackgroundSet` | Static shadow base |
| --- | ---: | --- |
| `null` or `false` | false | resolved `palette.bg` |
| `true` | true | resolved `palette.bg` |
| literal hex or ANSI-256 color | true | that literal color |
| `{ relative = …; }` or `{ blend = …; }` | true | resolved `palette.bg` fallback |

The final case is intentionally static. MOTD learns its actual terminal-relative
window color only at runtime after querying the terminal; generated Starship and
ble.sh configuration cannot safely depend on a MOTD-to-shell synchronization
protocol. It retains the ownership signal but uses the deterministic
palette-background fallback selected during design.

## Ownership and wiring

- `src/prelude/lib.nix` owns pure color canonicalization and darkening. Its
  result is an envelope containing the unchanged Go-safe base palette, `shadow`,
  and the window-background ownership fact. `shadow` must **not** be merged into
  the palette attrset serialized by MOTD, menu, or docs: those Go JSON decoders
  reject unknown fields.
- `src/prelude/module.nix` computes the effective window-background context once,
  gates ownership on `cfg.motd.enable`, and passes the envelope into both prompt
  and shell generation.
- Direct `mkPrompt` consumers receive a private
  `windowBackgroundContext = { set = false; base = null; };` default in
  `defaults.nix`. Their deterministic fallback is the resolved `palette.bg`.
- `src/prelude/prompt.nix` adds only the envelope's `shadow` to Starship's
  `palettes.prelude`, making `bg:shadow` and `fg:shadow` available to generated
  prompt settings without changing the palette passed to Go tools.
- `src/prelude/shell-init.nix` adds `shadow` to the generated ble.sh scheme
  palette and emits `_PRELUDE_WINDOW_BACKGROUND_SET=0|1`. Before bootstrap
  cleanup, `src/prelude/shell/bash-init.bash` initializes the persistent
  `_prelude_window_background_set` to false; `src/prelude/shell/init.bash`
  promotes it from the generated candidate only when MOTD successfully paints
  the window. The generated temporary variable remains in the existing unset
  sweep.
- `src/prelude/shell/scheme.bash` receives `%prelude_shadow` for
  backdrop-dependent shell chrome. Blesh initializes that chrome from the
  persisted ownership state; if already initialized, `init.bash` refreshes it
  after MOTD. In either path, it uses `shadow` only when MOTD painted the window.
- A user-provided `prelude.prompt.configFile` remains a complete Starship
  ownership boundary. Prelude does not modify that file; the generated Blesh
  scheme still receives its resolved palette contract when the shell package is
  enabled.

## Non-goals

- No runtime MOTD-cache-to-Blesh synchronization.
- No terminal-background query from ble.sh.
- No hard-coded shadow value per theme.
- No separate user override for `shadow`.
- No changes to MOTD's existing relative/blend resolution semantics.

## Verification

Focused Nix checks must prove:

1. Every built-in theme produces an exact, valid derived `shadow` token.
1. A hex `prelude.palette.bg` override changes `shadow` according to the 5% rule;
   a black override proves that equality is permitted when RGB precision requires
   it.
1. Short hex and numeric color forms match Lip Gloss's canonical RGB conversion
   before darkening.
1. A literal `motd.windowBackground` becomes the `shadow` base; `true`, `null`,
   and `false` select the documented base/ownership states.
1. Relative and blend window backgrounds retain ownership while falling back to
   the resolved palette background. Disabled MOTD always clears ownership.
1. Generated Starship TOML and the generated ble.sh scheme agree on `shadow`.
   Generated `_PRELUDE_WINDOW_BACKGROUND_SET` must survive bootstrap cleanup:
   successful owned startup selects `shadow`; successful fallback, quiet, and
   failed-MOTD startup select `palette.bg`.
1. Direct `mkPrompt` generation without module-injected context deterministically
   derives `shadow` from resolved `palette.bg`.
