# Starship Window-Background Alignment

**Date:** 2026-08-06  
**Status:** Approved design

## Purpose

When Prelude's MOTD owns the terminal window background, the generated Starship prompt must paint the same effective background. The current generated format references `bg:window`, but the generated `prelude` palette does not define a `window` token. This leaves the prompt background unrelated to the MOTD fill.

The fix must preserve transparent prompt behavior when Prelude does not own a window background, while keeping Starship, MOTD, and Blesh on the same Nix-resolved backdrop contract.

## Current-state diagnosis

- `src/prelude/motd.nix` resolves `windowBackground = true` to the resolved `palette.bg` before serializing MOTD configuration.
- `src/prelude/lib.nix` resolves window ownership once in `resolveBackdropPalette`. Its envelope currently contains the Go-safe `palette`, `windowBackgroundSet`, and derived `shadow`. Literal window colors are retained as `base`; `true`, relative, and blend modes have no static literal base.
- `src/prelude/prompt.nix` generates the outer Starship style as `...(bg:window)`, but `palettes.prelude` only adds `shadow` to the resolved palette. `window` is therefore not a generated custom palette entry.
- The outer format wrapper is currently unconditional. If a fallback `window = palette.bg` token is added without gating the wrapper, prompts become opaque even when MOTD is disabled or `windowBackground` is unset.
- A literal window color may be an accepted short hex, six-digit hex, ANSI index, or packed RGB value. A Starship-only token should be normalized to canonical `#RRGGBB`, not emitted as a raw Nix integer.
- `prelude.prompt.configFile` remains a complete user-owned Starship boundary and must not be modified.

## Target behavior

The generated prompt has an effective `window` color and a conditional outer background style:

| MOTD state | `windowBackgroundSet` | Starship outer wrapper | Static `window` token |
| --- | ---: | --- | --- |
| MOTD disabled | false | transparent | canonical `palette.bg` fallback |
| `windowBackground = null` or `false` | false | transparent | canonical `palette.bg` fallback |
| `windowBackground = true` | true | `bg:window` | canonical `palette.bg` |
| Literal color | true | `bg:window` | canonical literal color |
| Relative or blend color | true | `bg:window` | canonical `palette.bg` fallback |

The relative/blend row is deliberately static. MOTD resolves terminal-relative colors at runtime, while generated Starship configuration is evaluated ahead of that query. The prompt uses the same deterministic palette-background fallback already used for derived shadow styling; no runtime MOTD-to-Starship synchronization is introduced.

For the requested `windowBackground = true` case, both sides derive from the same `palette.bg`. Starship receives the canonical truecolor form, while MOTD retains the accepted raw form and Lip Gloss resolves it through the same color contract.

## Architecture

### Backdrop resolver

`src/prelude/lib.nix` remains the single owner of effective backdrop derivation. Extend the returned envelope with a Starship-only `window` value:

```nix
window = canonicalColor (
  if windowBackgroundSet && base != null then base else palette.bg
);
```

`canonicalColor` must be defined in `src/prelude/lib.nix` beside the existing color helpers and exported for focused Nix checks. It must use the accepted-color conversion path (`colorToRGB` followed by `rgbToHex`) so short hex, ANSI-index, and packed-RGB forms are represented as stable truecolor. `shadow` continues to derive from the same selected static base and keeps its current implementation and assertions unchanged (currently a 2.5% lighten-toward-white contract).

The envelope remains separate from `palette`. `window` and `shadow` must not be merged into the Go-safe palette passed to MOTD, menu, or docs JSON decoders.

### Generated Starship config

`src/prelude/prompt.nix` adds the envelope's `window` and `shadow` values to `palettes.prelude`:

```nix
palettes.prelude = pal // {
  inherit (backdrop) window shadow;
};
```

The format keeps its semantic `bg:window` reference only when Prelude owns a window backdrop. The outer style wrapper is generated conditionally:

```nix
format = "[${leftSegments}\n[╰─](fg:accent) ]${
  if backdrop.windowBackgroundSet then "(bg:window)" else "()"
}";
```

The exact expression may be arranged for formatting, but the invariant is mandatory: `bg:window` is present only when `windowBackgroundSet` is true. This prevents a fallback token from changing the existing transparent behavior.

The inner Powerline segment backgrounds remain unchanged. Their separator invariant still applies: each separator's foreground is the previous segment background and its background is the next segment background.

### Shell and Blesh boundary

No Blesh runtime protocol changes. `src/prelude/shell-init.nix` continues to receive the existing `shadow` and `windowBackgroundSet` values. Blesh status-cap behavior remains ownership-gated after MOTD success. The new `window` value is consumed only by generated Starship configuration.

### Custom configuration boundary

When `prelude.prompt.configFile` is set, Prelude continues to return that file verbatim. No `window` token, outer wrapper, or generated prompt setting is injected into user-owned configuration.

## Verification

Extend the existing `prompt-shadow-palette` check in `nix/checks.nix` with focused generated-config assertions:

1. A direct prompt with no window context emits a canonical `window` token derived from `palette.bg` but does not emit the outer `(bg:window)` wrapper.
2. A `windowBackground = true` context emits `window` equal to the canonical resolved `palette.bg` and includes `(bg:window)` in the format.
3. A literal window context emits the canonical literal color and includes `(bg:window)`.
4. Relative and blend contexts retain `windowBackgroundSet = true`, use the canonical `palette.bg` fallback, and include `(bg:window)`.
5. Numeric and short-hex `palette.bg`/literal values are emitted as canonical six-digit truecolor values.
6. The existing `shadow` assertions remain unchanged and continue to prove the selected-base contract.
7. The MOTD, menu, and docs palette JSON objects contain no `window` or `shadow` key; the Starship-only envelope fields remain outside the Go-safe palette.
8. A rendered `starship prompt` smoke test with `TERM=xterm-256color` confirms that the true case emits the expected truecolor background SGR.
9. A supplied `configFile` remains byte-for-byte user-owned and receives no generated backdrop styling.

Run the focused Nix check, then the repository's normal `nix flake check` and Go checks as required by CI. Do not use generated docs or media refreshes; this change does not modify options or documentation source content.

## Non-goals

- No runtime MOTD-cache-to-Starship synchronization.
- No terminal-background query from Starship or Blesh.
- No user-configurable `window` or `shadow` palette options.
- No change to MOTD relative/blend resolution.
- No changes to user-provided Starship config files.
- No unrelated prompt layout or Powerline palette changes.
