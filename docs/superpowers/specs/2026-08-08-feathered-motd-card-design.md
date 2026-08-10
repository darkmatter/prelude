# Feathered MOTD Card

**Date:** 2026-08-08\
**Status:** Superseded 2026-08-09 — the approved bounded-card revision removes the fringe and reclaims its footprint

## Goal

Replace Prelude's terminal-wide background ownership with a bounded MOTD card that remains legible in the selected palette and transitions gently into any terminal background.

The default presentation is:

- an opaque MOTD card using the resolved theme `bg` color;
- a three-cell `▓▒░` fringe whose background remains the terminal default;
- transparent terminal gutters, spacer rows, Starship input chrome, and Blesh textarea;
- no OSC color mutation or dependence on terminal Background Color Erase (BCE).

This design supersedes the full-window ownership introduced by the derived-shadow, Starship-window, and Blesh-window designs. Their component-local palette work remains valid; their window ownership and handoff contracts do not.

## Decisions

1. **Remove `windowBackground` completely.** Delete the public option and every private ownership field, resolver, shell handoff, prompt wrapper, renderer path, test, wizard control, and generated example that exists only for terminal-wide painting. Do not leave a deprecated alias.
1. **Paint only the card.** Change the default `prelude.motd.background` from `false` to `true`; `true` resolves to `palette.bg`. Keep the existing literal, relative, blend, and transparent forms.
1. **Composite without knowing the terminal color.** Draw `░`, `▒`, and `▓` as foreground-colored glyphs using the resolved card background while leaving their background unset. The terminal's actual default shows through each glyph and approximates alpha.
1. **Reset transparency explicitly.** “Transparent” means SGR default background, not merely a style with no background. Emit SGR 49 before screen erasure, fringe runs, transparent spacer/scroll runs, and the final prompt handoff.
1. **Three fixed rings, no new option.** The default fringe is an internal presentation contract. `background = false` remains the escape hatch and renders neither card fill nor fringe.
1. **Always inset.** The complete card-plus-fringe block never touches the left or right terminal edge. Respect configured margins, but impose one terminal-default edge cell when a configured margin is zero.
1. **Preserve component-owned color.** Powerline segments, selection/error faces, and the fixed status component retain their bounded semantic backgrounds. The private `shadow` token remains derived from `palette.bg` for status chrome; it no longer depends on MOTD ownership.
1. **Preserve vertical alignment.** `top`, `center`, and `bottom` continue to place the MOTD while landing the following prompt consistently. Positioning uses background-reset newline runs, not painted full-width rows.
1. **Fail closed on impossible geometry.** If a terminal cannot fit `minimumCardWidth` plus two transparent edge cells, omit the MOTD instead of clipping it or rendering edge-to-edge.

## Public configuration

Remove:

```nix
prelude.motd.windowBackground
```

Change the default:

```nix
prelude.motd.background = true;
```

Retain the existing `background` values:

| Value | Card | Fringe |
| --- | --- | --- |
| `null` or `false` | transparent | absent |
| `true` | resolved `palette.bg` | same resolved color as foreground-only glyphs |
| literal color | literal opaque card | same literal color |
| `{ relative = n; }` | terminal-relative resolved card | same resolved color |
| `{ blend = n; }` | terminal/theme resolved card | same resolved color |

Relative and blend modes may continue using the existing OSC 11 **query** path. The fringe never calculates intermediate RGB colors from that query. Probe failure retains the current deterministic `palette.bg` fallback.

`clearScreen`, `align`, `verticalAlign`, `margin`, `padding`, `width`, `maxWidth`, and `border` remain public and retain their intent. `width = "full"` now means the full card band available after outer insets and the active fringe, not the full terminal width.

## Architecture

```text
resolved palette
  ├─ palette.bg ────────────────> opaque MOTD card
  │                                  │
  │                                  └─ foreground-only ▓▒░ fringe
  └─ lighten(palette.bg) ───────> component-local shell shadow

plain terminal-default window
  ├─ SGR 49 + ED2 + home
  ├─ SGR 49 + newline positioning
  ├─ transparent ui.Window
  │    └─ fringe
  │         └─ opaque ui.Surface
  └─ SGR 49 handoff to transparent Starship/Blesh input
```

### Nix boundary

- Remove `resolveWindowBackgroundContext`.
- Simplify or replace `resolveBackdropPalette` so it resolves only the Go-safe palette and a `shadow` derived from `palette.bg`; remove `window` and `windowBackgroundSet`.
- `module.nix` passes the resolved palette to MOTD, menu, docs, and prompt, and passes only the derived `shadow` needed by shell status chrome.
- Remove `windowBackgroundContext`, `window`, `windowBackgroundSet`, and `promptWindowManaged` from prompt and shell builders.
- Keep `shadow` outside Go palette JSON. Remove it from Starship's palette if Starship no longer references it.

### Prompt and shell boundary

Generated Starship configuration removes every `bg:window` style and the `window` palette entry. Existing Powerline segment backgrounds and separator transitions remain unchanged; fill, right-format caps, the character, and the editable-line framing become transparent.

Delete the MOTD-to-shell ownership lifecycle:

- generated `_PRELUDE_WINDOW_BACKGROUND_SET` and `_PRELUDE_PROMPT_WINDOW_MANAGED` inputs;
- lowercase ownership state and MOTD-success promotion;
- `prelude_textarea_window` face;
- `textarea-background.bash` and its guarded Blesh advice;
- command-output `PREEXEC`/`PRECMD` background hooks;
- status-cap ownership refresh.

Blesh uses its stock transparent textarea. The fixed status row remains a bounded component using the palette-derived `shadow`; its optional cap uses the same static component treatment rather than MOTD ownership.

### Go rendering boundary

Remove `WindowBackground` and its relative/blend fields from `motd.Config` and `PaintModel`. Remove the `window` style/context map and all window-fill fallbacks. Header gradients and section-gap fills derive from the resolved block/card background.

Keep `ui.Window` as the transparent outer placement component. Keep `ui.Surface` as the opaque card painter. Implement the domain-specific fringe in `src/internal/motd` rather than expanding the shared UI API for a single consumer.

## Fringe rendering

For depth three, the completed card is wrapped as uniform light-shade rings:

```text
░░░░░░░░░░░░░░
░░░░░░░░░░░░░░
░░░░░░░░░░░░░░
░░████████░░░░
░░████████░░░░
░░░░░░░░░░░░░░
░░░░░░░░░░░░░░
░░░░░░░░░░░░░░
```

`█` denotes opaque card cells and is not emitted literally.

Every ring uses the same lightest shade (`░`, U+2591) — no gradient nesting.
A single uniform overlay produces a subtle foreground-paint dither whose density
varies by terminal/font; it is the closest portable approximation of a compositing
overlay without alpha-blending semantics.

Every fringe run must begin in an explicit default-background state:

```text
SGR 49
SGR foreground = resolved card background
fringe glyphs
style reset
```

An unset Lip Gloss background alone is insufficient because rendering may begin under an inherited non-default SGR background.

When responsive layout reduces depth, each ring still uses `░`:

| Depth | Card-to-terminal sequence |
| ---: | --- |
| 3 | `░░░` |
| 2 | `░░` |
| 1 | `░` |
| 0 | at least one terminal-default edge cell |

A bordered MOTD renders its rounded border inside the opaque card; the fringe wraps the completed bordered body.

## Responsive geometry

Let:

```text
leftInset  = max(configured left margin, 1)
rightInset = max(configured right margin, 1)
available  = terminalWidth - leftInset - rightInset
```

Choose the largest fringe depth from zero through three for which `available - 2*depth >= minimumCardWidth`. Resolve the requested card width and `maxWidth` inside that remaining band. Alignment places the complete `cardWidth + 2*depth` block within `available`; the final horizontal offset includes `leftInset`.

If `available < minimumCardWidth`, return no MOTD output and do not clear the screen.

The card body is rendered before vertical placement so the fringe's added rows are part of its measured height. Existing collapsed vertical margins remain. `verticalAlign` computes the unused rows from the complete outer height.

## Clear-screen and cursor contract

For `clearScreen = true`:

1. Emit SGR 49.
1. Emit ED2 and home.
1. Emit SGR 49 followed by newline-only `aboveRows` positioning.
1. Emit the transparent outer window containing fringe and card.
1. Emit SGR 49 followed by newline-only `belowRows` positioning.
1. Emit SGR 49 before returning control to the shell prompt.

The total emitted row count remains `terminalHeight - 1` when the complete MOTD fits, so the next prompt lands on the same row for top, center, and bottom alignment. No spacer requires width-filled spaces.

For `clearScreen = false`, place the card-plus-fringe block using transparent margins and finish with SGR 49. Do not query or mutate the terminal default color.

## Wizard, examples, and documentation

- Remove the wizard's window-background model field, selector row, summary text, preview fill, template field, and tests.
- Make the wizard preview show the resolved card and three-ring fringe against its transparent preview surface.
- Update generated templates to emit `background = true` and no `windowBackground` line.
- Replace the full-window example and descriptions with a feathered-card example.
- Regenerate the options reference and media required by repository checks.
- Mark the previous full-window design documents as superseded by this design; retain them as history rather than rewriting their implemented-state record.

## Compatibility and failure behavior

- The fringe uses conventional one-cell Unicode shade glyphs. Tests must assert their measured width under the pinned width library.
- `NO_COLOR` and existing color-disable behavior remain governed by the current MOTD renderer; do not add a second fringe-specific environment policy.
- A transparent card (`background = false`) skips the fringe because no safe card color/contrast surface exists.
- A failed terminal-background query affects only explicitly configured relative/blend card modes and follows the existing fallback. Default rendering performs no terminal-color query solely for the fringe.
- Narrow terminals omit the MOTD only at the impossible minimum-width boundary. This is normal successful behavior and must not print a diagnostic.
- Explicit SGR 49 protects BCE erases, scroll-created rows, and fringe gaps from inherited/card backgrounds. Do not use OSC 11/111.

## Verification

### Go unit coverage

1. Default rendering resolves an opaque `palette.bg` card and a three-ring fringe.
1. Exact stripped patterns match the nested ring geometry; each shade glyph has display width one.
1. Fringe glyphs carry the card color as foreground and no explicit non-default background.
1. Rendering started under an injected non-default background emits SGR 49 before ED2, every fringe run, every newline spacer batch, and final prompt handoff.
1. A terminal-state emulator confirms fringe gaps and spacer/scroll-created cells use the terminal default on both BCE and non-BCE models.
1. Literal, relative, and blend backgrounds drive both card and fringe from one resolved color; transparent background produces neither fill nor fringe.
1. Default card/fringe rendering does not request a terminal-background probe. Relative/blend configurations retain query and fallback behavior.
1. Full, fixed, and max widths cover depth 3/2/1/0, left/center/right alignment, configured zero margins, and the impossible-width omission path.
1. Top/center/bottom vertical alignment preserve body placement and final prompt row with fringe height included.
1. Border and unbordered card paths keep the fringe outside the completed card.

### Generated and shell coverage

1. Generated option/config JSON contains no window-background fields.
1. Generated Starship TOML contains no `window` token or `bg:window`, while existing Powerline transition assertions still pass.
1. Generated Blesh runtime contains no window ownership variables, textarea face/advice, or command-background hooks.
1. Retain the submitted-prompt `prompt_ps1_final` regression in a generically named PTY harness; remove ownership-only PTY assertions and fixtures.
1. Wizard output, templates, and examples contain no `windowBackground` key and preview the fringe.

### Repository verification

Run focused Go tests and the relevant Nix check first. Then run `go test ./...`, `go vet ./...`, and `nix flake check`. Run the repository's documentation synchronization and media-fingerprint commands required by the changed public option and wizard preview. Do not use `nix fmt`.

## Implementation sequence

1. Remove the public option and simplify Nix palette/prompt/shell generation through every caller.
1. Remove shell ownership/advice modules and convert Starship/Blesh to transparent input surfaces.
1. Replace Go window-background state with the opaque card plus explicit-default-background fringe renderer.
1. Make card/fringe geometry responsive and preserve vertical alignment/prompt landing.
1. Update wizard, templates, examples, documentation, and historical spec statuses.
1. Replace window-ownership tests with fringe, transparency-state, prompt, geometry, and generated-contract regressions.
1. Run focused smoke verification, then repository gates and required generated-doc/media checks.

## Definition of done

- Prelude never paints or mutates the full terminal background.
- The default MOTD is an opaque `palette.bg` card surrounded on all sides by a terminal-transparent `▓▒░` fade.
- The card remains inset on supported widths; responsive depth reduction and impossible-width omission are deterministic.
- Fringe gaps, erased cells, spacer rows, and the post-MOTD prompt use the terminal default even when rendering begins under a non-default background.
- Top, center, and bottom alignment still place the MOTD and following prompt correctly.
- Starship and Blesh input surfaces are transparent outside bounded semantic components.
- No `windowBackground` option, generated field, ownership state, shell adapter, command hook, wizard control, or stale current documentation remains.
- Existing custom card backgrounds, borders, prompt Powerline transitions, status behavior, and custom Starship configuration boundaries remain valid.
- Focused checks and repository gates pass.
