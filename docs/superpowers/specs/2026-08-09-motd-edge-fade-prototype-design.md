# MOTD Horizontal Edge-Fade Prototype

**Date:** 2026-08-09\
**Status:** Approved prototype design

## Question

Does a narrow horizontal color gradient let the bounded opaque MOTD card meet the surrounding terminal background more gently without changing the card's central reading surface?

This is throwaway prototype code. Its only durable output is the visual decision to keep or reject the edge treatment.

## Visual contract

- The middle 75% of the card remains exactly the resolved card background.
- The outer 12.5% on each side blends between the surrounding terminal background and the resolved card background.
- The left edge blends `terminal → card`; the right edge mirrors it as `card → terminal`.
- The gradient is horizontal only. Every card row uses the same column colors.
- Colors use `lipgloss.Blend1D`, matching the existing fading-rule interpolation method.
- Foreground colors and text attributes remain unchanged.
- Component-owned backgrounds, including code blocks, raised headers, description fills, and status chips, remain unchanged.
- The optional rounded border remains unchanged.
- Transparent cards, margins, spacer rows, and cells outside the card remain transparent.

For a card width $w$, each fade has $e = \\max(1, \\lceil w / 8 \\rceil)$ cells. Columns `[e, w-e)` retain the exact card background. The endpoint color is `PaintModel.TerminalBackground`; when terminal detection is unavailable, the existing deterministic palette-background fallback is used.

## Prototype boundary

The effect is gated by a clearly named private environment variable and has no public Nix option. Disabled is byte-for-byte the existing render path. The prototype does not change generated configuration, option documentation, wizard controls, examples, or screenshots.

The implementation lives beside the MOTD renderer in a file whose name identifies it as a prototype. It runs after the unbordered card body is composed and before optional border wrapping and transparent window placement.

## Rendering approach

An ANSI-aware row transform tracks the active SGR background while walking printable graphemes and their terminal-cell widths. For each cell whose effective background is the base card background, it emits the gradient background for that column immediately before the grapheme. Other backgrounds pass through unchanged. Applying the transform before border wrapping keeps border cells outside the gradient.

A post-render transform is preferred over threading column state through every MOTD component. It isolates the experiment, preserves the current component APIs, and can be deleted without migration if the visual result is rejected.

## Failure behavior

- Transparent card: skip the transform.
- Missing or invalid prototype selector: use the unchanged renderer.
- Missing terminal-background detection: use the same palette-background fallback already used by relative and blended MOTD backgrounds.
- Narrow cards: clamp each fade to at least one cell while preserving a non-negative center band.
- Wide graphemes: select the gradient from the grapheme's starting column and advance by its measured cell width.

## Evaluation

Run the real MOTD in a PTY with the prototype enabled. Inspect an opaque card with and without the optional rounded border, confirming:

1. the center background equals the configured card color;
1. both edges converge on the resolved terminal background;
1. left and right transitions mirror each other;
1. code blocks and other component backgrounds remain flat;
1. terminal gutters and vertical spacer rows remain unpainted;
1. disabling the prototype restores the existing output.

After evaluation, either delete the prototype or rewrite the accepted treatment as a supported renderer contract with focused tests and documentation.
