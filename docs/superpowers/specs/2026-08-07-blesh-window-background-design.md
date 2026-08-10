# Blesh Editable Window-Background Alignment

**Date:** 2026-08-07\
**Status:** Approved design; implemented.

> **Superseded:** [Feathered MOTD Card](./2026-08-08-feathered-motd-card-design.md)
> removes terminal-wide background ownership. This document remains the
> historical record of the previously implemented design.

## Goal

When Prelude's MOTD successfully paints `prelude.motd.windowBackground`, the Bash editable surface must use the same effective static background:

- generated Starship prompt output uses `window` as it does today;
- Blesh's editable command buffer, blank tail, deletion gaps, and wrapped rows use that same `window` color;
- Blesh's fixed status row and cap retain their existing `shadow` hierarchy.

The design preserves a transparent **Blesh editable surface** when the MOTD is disabled, quiet, or fails. It preserves the current Starship runtime contract: Starship's generated wrapper is selected from Nix-time configuration, while Blesh activates only after a successful MOTD paint.

## Decisions and assumptions

1. **Textarea only.** Completion panels, status row, status cap, and unrelated Blesh canvases are out of scope.
1. **Static dynamic-mode fallback.** For `{ relative = ...; }` and `{ blend = ...; }`, Blesh uses the same canonical `palette.bg` fallback already used by Starship. `true` and literal colors therefore match exactly; terminal-relative values remain deterministic rather than introducing a MOTD-to-shell color protocol.
1. **Custom Starship configuration is a hard boundary.** When `prelude.prompt.configFile` is set, do not activate the textarea adapter. This avoids adding a Blesh-owned background that may conflict with an entirely user-owned prompt. The module passes an explicit generated-prompt ownership bit to the shell runtime.
1. **No terminal-default mutation.** OSC 11 or other terminal-default background changes would affect unrelated applications and require fragile restoration. Do not use them.
1. **Pinned Blesh internals, fail closed.** The implementation may use Blesh's function-advice helper and its current textarea render path, but must validate every required function and capability before installing. An unrecognized Blesh layout disables the adapter without changing stock Blesh rendering.
1. **Cache-safe color composition.** Any Blesh color advice must compose the graphics integer before Blesh's SGR cache lookup; it must never rewrite an entry in Blesh's base `g2sgr` cache.

## Current-state facts

- `src/prelude/lib.nix` already resolves a private backdrop envelope containing the Go-safe `palette`, canonical static `window`, `shadow`, and configuration-time `windowBackgroundSet`.
- `src/prelude/prompt.nix` already exposes `window` to generated Starship and conditionally wraps generated prompt output in `bg:window`.
- `src/prelude/shell-init.nix` currently passes `shadow`, but not `window` or generated-prompt ownership, into the Blesh scheme/runtime.
- Blesh owns the editable textarea. Its renderer clears panels and trailing cells with reset-based `EL`, `ECH`, and panel-clear operations; merely styling Starship or `syntax_default` cannot color the blank tail.
- On BCE terminals, erase under an explicitly re-applied background paints that background. On non-BCE terminals such as Warp, erase ignores the active background. Prelude's MOTD already compensates for this by width-filling rows.

## Architecture

```text
Nix backdrop resolver
  └─ { palette, window, shadow, windowBackgroundSet }
       ├─ prompt.nix ──────> Starship generated palette + conditional bg:window
       └─ shell-init.nix ─> Blesh generated scheme: prelude_window face
                                  │
MOTD succeeds ─> _prelude_window_background_set=1
                                  │
                        textarea-background.bash
                        ├─ localized color advice
                        └─ BCE-aware localized clear advice
                                  │
                     Blesh editable textarea only
```

### Nix and scheme boundary

1. Pass `window` and `promptWindowManaged = cfg.prompt.configFile == null` from `backdropPalette`/prompt configuration in `src/prelude/module.nix` into `mkShell`.
1. Add `window ? null` and `promptWindowManaged ? true` to `src/prelude/shell-init.nix`. A missing `window` must fall back to `plib.canonicalColor palette.bg`—not the `shadow` fallback, which intentionally derives a different shade.
1. Add `%prelude_window` to the generated Blesh scheme palette and define one internal, background-only face for textarea paint.
1. Emit `_PRELUDE_PROMPT_WINDOW_MANAGED=0|1` with the existing candidate ownership input. `bash-init.bash` copies it once, before generated inputs are unset, to persistent lowercase `_prelude_prompt_window_managed`; all runtime predicates use only that lowercase state. The generated wrapper then unsets the uppercase input with its other implementation inputs. Keep `window` and `shadow` outside the palette serialized to Go consumers. Do not add a user-configurable palette option.

This makes Starship and Blesh consume the same canonical color without duplicating color conversion or adding a second resolver.

### Localized Blesh adapter

Add `src/prelude/shell/textarea-background.bash`, install it in the explicit runtime-module list in `src/prelude/shell-init.nix`, and source it in `src/prelude/shell/bash-init.bash` after `bleopt color_scheme=prelude` and the persistent ownership inputs exist, but before `ble-attach`.

The adapter installs only when its expected Blesh functions are present. It uses Blesh's function-advice facility rather than copying `ble/textarea#render`. It has two bounded advice layers:

1. **Render layer.** An around-advice on `ble/textarea#render` activates only when both `_prelude_window_background_set=1` and `_prelude_prompt_window_managed=1`. It reads the window face, establishes a dynamic render marker, and locally shadows `_ble_term_sgr0` to `original reset + bg:window`. This is a local Bash binding, not a one-time ambient SGR emission, so Blesh's existing inner `$_ble_term_sgr0` expansions resolve to the replacement during that render. The broader effect is deliberate: every reset terminator reached in the textarea render leaves the editable panel on `window`, not merely `EL`/`ECH`.
1. **Panel-clear layer.** Advice on the current panel clear/height primitives checks `index == _ble_textarea_panel` plus the same lowercase ownership predicate. It handles panel and resize clears that occur outside `ble/textarea#render`; status and cap indices always delegate to stock Blesh behavior.

Within the render marker, advice around `ble/color/g2sgr` and its direct ANSI entry point must call `ble/color/g#setbg` on an input graphics integer only when it has no explicit background, then invoke the original converter with that new integer **before** its cache lookup. `ble/color/g#setbg` uses Blesh's `ret` convention, so the advice must not depend on an earlier `ret` value surviving that call. It must never mutate `_ble_color_g2sgr`, `_ble_color_g2sgr_ansi`, or an existing cache entry. The composed integer is a distinct key in each cache, so both cold and warm base-cache cases produce the right SGR without leaking window styling into status/cap or completion rendering.

The reset shadow must not silently become reusable Blesh state outside an owned textarea render. Treat the ownership predicate as an adapter-cache epoch: invalidate or rebuild any textarea cache written under the shadow when that predicate changes, and ensure cache replay follows the same predicate. A replay after the predicate is false must contain no adapter-added window reset.

The adapter reads `_prelude_window_background_set`, `_prelude_prompt_window_managed`, and `_ble_term_bce` at render/clear time. It needs no second lifecycle state or runtime color handoff. Before a successful MOTD paint, in a custom-config shell, or after a failed/quiet MOTD launch, it takes the stock Blesh path.

### BCE-aware clearing

The adapter must not assume Background Color Erase. `_ble_term_bce` is empty when disabled and `1` when enabled, so every branch tests it arithmetically rather than comparing it with `0` or caching its installation-time value.

| `_ble_term_bce` | Textarea clear behavior |
| --- | --- |
| enabled | Use Blesh's existing `EL`/`ECH` operations only after the dynamic `_ble_term_sgr0` replacement has explicitly made their reset sequence `reset + bg:window`. Do not rely on a background SGR emitted before Blesh runs. |
| disabled | Use a textarea-local helper that writes background-styled spaces to the exact erased width and restores Blesh's expected cursor position. Do not emit `EL` or `ECH` as the mechanism that paints those cells. |

The non-BCE helper and the index-scoped panel layer must cover every erase branch reachable for the textarea, including:

- full textarea-panel clears and panel-height clears;
- clear-after and tail deletion;
- ECH-style partial erases;
- right-prompt cleanup and multiline-wrap cleanup;
- resize/height changes that create blank textarea cells.

It must use Blesh's existing prototype-space facility rather than per-cell shell loops. Each wrapper must maintain Blesh's canvas coordinate and cursor contract exactly.

## Error handling and compatibility

- Validate the required Blesh advice API, textarea renderer, panel identity, terminal capability variables, and clear helpers before installation.
- If validation fails, print one concise diagnostic to stderr and leave stock Blesh behavior intact. Never install a partial adapter.
- Pair that runtime guard with a CI assertion against the actual `${pkgs.blesh}` source: every required function, variable, and source-shape sentinel must be present. A Blesh upgrade must fail CI rather than silently disabling the feature.
- A custom `configFile` remains byte-for-byte user-owned and receives no textarea adaptation. Do not alter MOTD relative/blend resolution, status-cap behavior, or Starship's existing quiet/failed-MOTD behavior.

## Verification plan

Extend `nix/checks.nix` rather than creating a second test framework.

1. **Generated contract.** Assert that generated Starship and Blesh receive the same canonical `window` value, that direct shell generation falls back to `canonicalColor palette.bg`, that `bash-init.bash` persists and the wrapper unsets generated-prompt ownership correctly, and that Go-facing palette JSON still contains neither `window` nor `shadow`.
1. **Real-source drift guard.** Inspect the actual `${pkgs.blesh}` derivation in `nix/checks.nix`; assert the required advice APIs, textarea/panel clear functions, capability variables, and source-shape sentinels remain present.
1. **Installer guard.** Mock required Blesh APIs and prove the installer installs only when every required symbol/layout predicate is valid; invalid layouts leave original behavior untouched.
1. **Color composition and cache safety.** With both cold and warm base `g2sgr` and `g2sgr-ansi` caches, assert an active foreground-only graphics value is passed to the original converter as a distinct window-background graphics key, while an explicitly backgrounded value remains unchanged. Assert neither base cache is mutated and inactive, quiet, and custom-config states add nothing.
1. **BCE path.** Mock `_ble_term_bce=1`; assert Blesh's erased cells receive the dynamically shadowed `reset + bg:window` sequence, preserve Blesh cursor coordinates, and do not affect status/cap paths.
1. **Non-BCE path.** Mock `_ble_term_bce=`; assert every covered clear branch emits width-correct background-styled spaces, restores the expected cursor position, and does not rely on `EL` or `ECH` to paint the erased cells.
1. **Panel-scope and replay path.** Exercise textarea resize/height clears outside the render advice and prove the index guard leaves Prelude's status/cap panels unchanged. Then force a textarea cache replay after ownership becomes inactive and prove it contains no adapter-added window reset.
1. **Interactive smoke.** Run the packaged Bash+Blesh runtime on a PTY with a literal window color. Exercise initial render, typing, deletion, wrapped input, and resize; inspect emitted terminal state or ANSI output for the same truecolor background across the editable surface.
1. **Regression matrix.** Cover `windowBackground = true`, a literal color, a relative/blend mode, disabled MOTD, quiet initialization, MOTD failure, and `configFile`. Confirm static fallback and ownership behavior match the table above.
1. Run the focused Nix check, then CI-equivalent `go test ./...`, `go vet ./...`, and `nix flake check` after implementation.

## Proposed implementation sequence

1. Thread canonical `window` and generated-prompt ownership through `module.nix` and `shell-init.nix`; make `bash-init.bash` persist the latter before the generated wrapper unsets it; add the scheme token and internal textarea background face.
1. Add the guarded Blesh adapter: render-scoped reset/color advice with pre-cache graphics composition and ownership-epoch cache handling, index-scoped panel-clear advice, and the BCE/non-BCE erase split.
1. Register the new module in the runtime derivation and source it after scheme setup but before `ble-attach`.
1. Add generated-contract, real-source drift, mock-canvas, cold/warm cache, replay, BCE/non-BCE, panel-scope, ownership, and PTY tests.
1. Run focused verification and the repository gates. Refresh generated options or recording assets only if a public option or rendered documentation changes; neither should be necessary for this internal behavior change.

## Definition of done

- In a normal generated-prompt, MOTD-owned Bash session, typed, deleted, blank, wrapped, and resize-created textarea cells visually use the same canonical static color as Starship's generated `window` background.
- On Warp/non-BCE behavior, erased textarea cells remain correctly painted rather than reverting to the terminal default.
- Explicit syntax/selection/error backgrounds retain their semantic colors, including with a warm Blesh SGR cache.
- Status/cap retain their existing shadow treatment, including when textarea panel height changes.
- Quiet, failed, disabled-MOTD, custom-config, and unsupported-Blesh paths leave no partial styling or altered terminal defaults.
- Generated Starship configuration and user-supplied `configFile` boundaries remain intact.
- All listed checks pass.

## Alternatives rejected

- **Starship/face markup alone:** cannot paint Blesh-owned blank tails and clear-created cells.
- **Always set the terminal default background:** leaks Prelude styling into unrelated programs and requires unreliable restoration.
- **Fork/copy Blesh's textarea renderer:** creates a large, version-sensitive maintenance fork where bounded, guarded advice is sufficient.
- **Runtime MOTD-to-Starship color synchronization:** intentionally out of scope; it adds a second config lifecycle and is unnecessary for the selected static fallback contract.
- **Applying textarea adaptation to a custom Starship config:** would add observable Blesh styling outside the user-owned `configFile` boundary.
