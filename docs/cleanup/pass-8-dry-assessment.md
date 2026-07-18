# Pass 8 — DRY / dedup assessment for `prelude`

## Findings

- **Normal-threshold `jscpd` scan:** 8 clone groups across 76 Go files, with 75 duplicated lines (1.07%). The candidates were in `src/experimental/shortcut-keycaps/main.go`, `src/internal/menu/update.go`, the two Markdown renderers, and test helpers/test setup. No production clone was automatically safe to merge without semantic review.
- **Exploratory lower-threshold `jscpd` scan:** 35 clone groups at 3 lines/25 tokens, including the normal-threshold results plus repeated config loading/CLI setup, intentional two-pass lookup logic, status success/failure branches, renderer defaults, and viewport forwarding. The lower threshold exposed parallel code, not a second high-confidence consolidation.
- **Go manual review:** `src/pkg/manual/navigate.go:20-25` and `src/pkg/manual/viewer.go:75-80` perform the same viewport-message forwarding: update `v.viewport`, preserve the returned command, and return the viewer. The operation has an honest name (`updateViewport`), is package-local, has no flags or special cases, and both callsites should evolve together.
- **Nix manual review:** repeated `mkOption`, `mkIf`, `runCommand`, `writeText`, and `concat*` forms are schema declarations, test/config data, or distinct package builders. No repeated semantic algorithm with a stable shared home was found.
- **TypeScript manual review:** the repository has only the one-line `example/index.ts` TypeScript example and no duplicated operation or callsite to consolidate.

## High-confidence recommendations

1. **Extract manual viewport forwarding.**
   - **Files/lines:** `src/pkg/manual/navigate.go:20-25`; `src/pkg/manual/viewer.go:75-80`.
   - **Diff sketch:** add `func (v Viewer) updateViewport(msg tea.Msg) (Viewer, tea.Cmd)` and replace both identical `v.viewport.Update(msg)` blocks with `return v.updateViewport(msg)`.
   - **Why safe:** both callsites already use the same `Viewer` value-receiver semantics and return the viewport's command unchanged; the helper has no parameters beyond the message and no special-case flag. Existing scrolling and mouse-wheel tests cover the two paths.

## Considered but rejected

1. **Share `trimBlankEdges` between MOTD and manual Markdown.** The loops are textually identical (`src/internal/motd/markdown.go:121-130` and `src/pkg/manual/markdown.go:93-102`), but the natural shared home would add a new exported helper to `pkg/ui` or `pkg/shared`. That changes a public package surface for a two-callsite utility; the Markdown renderers also intentionally retain separate style and fallback behavior.
2. **Share the Markdown style builders.** The normal and exploratory scans found many matching style fields (`src/internal/motd/markdown.go:72-116` and `src/pkg/manual/markdown.go:44-90`), but MOTD supports `StyledText` overrides and zero margins while manual uses a fixed background and margin. A helper would need context-specific parameters and would be harder to read.
3. **Unify `GlowRule` and `CenteredGlowRule` default-resolution methods.** `src/pkg/ui/gradient_rules.go:21-33` and `66-78` have matching `background`/`peak` methods, but the rule types have different rendering contracts and may intentionally diverge around labels/transparency. A delegating helper would save only a few lines while adding indirection.
4. **Unify docs/menu/MOTD config loading and CLI bootstrap.** `src/internal/docs/config.go:22-33`, `src/internal/menu/config.go:49-60`, and the three `Run` functions share JSON/config-path scaffolding, but each has different validation, flags, error prefixes, and runtime setup. A shared helper would require special-case callbacks or flags.
5. **Unify status success/failure resolution.** `src/internal/motd/status.go:60-103` repeats output selection, but success and failure deliberately use different levels, fields, and fallback text. Parameterizing those differences would obscure the state machine.
6. **Unify the two task lookup loops.** `src/internal/menu/invocation.go:80-97` is deliberately two passes so exact task names outrank keys regardless of group order. Combining or abstracting it would risk changing the documented precedence.
7. **Unify test-local color and fixture helpers.** `src/internal/motd/styles_test.go:28-35` and `src/pkg/ui/context_test.go:59-66` compare colors similarly, but tests are intentionally self-contained and the helpers have different package-local callers. Moving one would make a test dependency less explicit.
8. **Unify shortcut-keycaps row variants.** `src/experimental/shortcut-keycaps/main.go:129-164` contains deliberately similar row renderers and callsites because the file is a visual comparison gallery. The different separators and key treatments are the subject under evaluation.
9. **Unify Nix builder and shell snippets.** `nix/motd-demo-builder.nix`, `nix/menu-demo-builder.nix`, and `nix/demos.nix` repeat package boilerplate and string concatenation patterns, but they build different artifacts and the repeated shell/config values are intentional data. No honest abstraction reduces complexity.
10. **Introduce a cross-package generic pointer helper.** The `strPtr`/`boolPtr`/`uintPtr` family in `src/internal/motd/markdown.go:133-135` and `manualStringPtr`/etc. in `src/pkg/manual/markdown.go:105-107` looks mechanically similar, but a generic exported pointer utility would be a broad abstraction with no domain name and a public API cost.

## Low-confidence findings (NOT actioning)

The rejected candidates above are genuine textual or near-semantic similarities, but none besides the viewport forwarding has enough evidence that the callsites must always evolve together while preserving readability. In particular, the cross-package Markdown and pointer helpers fail the public-boundary/stable-home test; the renderer, status, config, and Nix candidates fail the same-operation test; and test/gallery repetition is intentional.

## Out-of-scope but noticed

- Type consolidation belongs to pass 6; no type definitions were changed.
- Defensive guards and error-handling policy belong to pass 7; this pass does not alter them.
- Unused files/exports were handled by pass 3; the existing removals remain untouched.
- Existing `docs/media/*` modifications are pre-existing and preserved. Full `nix flake check` remains subject only to the known stale docs-media freshness failure.
- No changes are proposed to the one-line TypeScript example or to generated/reference documentation.

## Pass 8 — implementation report

- **Applied:** 1 high-confidence consolidation (`Viewer.updateViewport`).
- **Reverted:** 0.
- **Test status:** pass — `go test ./...`, `go vet ./...`, `bun x tsc --noEmit -p example/tsconfig.json`, `nix flake check --no-build`, targeted diagnostics for both changed Go files, and `git diff --check`.
- **Net LoC delta:** production code `+8/-6` (net `+2`); assessment artifact `+49` lines, for a pass-8 total of `+57/-6` (net `+51`). No unrelated production files were changed by pass 8.
- **Project diagnostics note:** the project-wide diagnostic summary still lists stale errors for deleted or absent baseline paths; both touched files report zero errors/warnings.
