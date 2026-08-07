# MOTD, Setup, and Docs Module Deepening

**Date:** 2026-08-06\
**Status:** Approved design

## Purpose

Three Prelude modules — MOTD, Setup, and Docs — have accumulated internal sequencing that is shallower than its public contract warrants. Each preserves a deliberately stable public surface (flags, JSON, output, file materialization) while delegating runtime/layout/terminal work to local adapters. This design specifies how each module is deepened internally without altering its public invariants, so that three disjoint implementation slices can proceed in parallel after this specification is reviewed.

A fourth candidate — the Command catalogue — was considered and is explicitly dropped from this round. Source evidence is below.

## Current-state diagnosis

### MOTD

`src/prelude/motd.nix` resolves configuration into a JSON boundary baked into a Go binary at link time (`buildGoModule`, `ldflags = [ "-X main.defaultConfigPath=${configFile}" ]`). Runtime terminal layout, probes, Git state, and styling live in `src/internal/motd` — never in generated shell source.

The Go flow (`src/internal/motd/run.go`) is: load Config → open Cache → Preflight-if-needed → pure Resolve/Render → write output → detached async Preflight. Key invariants:

- `Config` (`src/internal/motd/config.go`) is the normalized JSON boundary from Nix; Go owns probes, layout, and rendering.
- `Cache` (`src/internal/motd/cache.go`) is the single JSON map of live facts written by Preflight; entries carry identity, payload, and TTL.
- `Preflight` (`src/internal/motd/preflight.go`) runs impure work for due Cache entries in three modes: `PreflightBlocking` (terminal size/bg, sync statuses, env), `PreflightAsync` (only async status), `PreflightAll` (both). Blocking and async are asymmetric: a blocking failure in the normal foreground run is logged as `motd: preflight:` and rendering continues with whatever Cache is available; only `--preflight-only` exits non-zero on a blocking Preflight failure. Async failures write stale/pending and never abort.
- `Resolve` (`src/internal/motd/paintmodel.go`) builds a pure `PaintModel` from `Config` and `Cache` — no Runtime, no Cache I/O.
- `Render` (`src/internal/motd/render.go`, `render_input.go`) produces the banner purely from `RenderInput{Config, Cache, TerminalWidth, TerminalHeight}`. Missing/stale cache yields sparse UI; never fails for live data absence.
- Post-output: after stdout is fully written, if due async entries exist and the run is not pure, a detached async Preflight refreshes the cache (`startAsyncPreflight`). The banner has already been emitted; the refresh is for the *next* invocation.
- Legacy `--refresh-status` alias maps to `--preflight-only --async` for one release (`run.go:30-37`).

`Runtime` (`src/internal/motd/runtime.go`) isolates shell effects used only by Preflight (Probe, Check). Pure Render never uses Runtime.

### Setup

Setup is not a standalone Nix module; it is a `writeShellApplication` in `src/prelude/module.nix:163-183` named `setup`, which delegates to `prelude-title --wizard "$@"`. The wizard lives in `src/internal/wizard/` and `src/cmd/title/main.go`.

The wizard (`src/internal/wizard/wizard.go`, `run.go`) is an iteration of the title form extended with `prelude.*` options. On finish (`finishWizard`, `run.go:201-260`) it materializes:

1. Renders the FIGlet title and writes it atomically to `title.txt` beside the config (`titlePathBesideConfig`).
1. If docs is enabled and `docs/getting-started.md` does not already exist, writes the starter page (`starterDocsPath`, `starterDocsPage`) — non-clobbering.
1. Renders the Nix config template (`renderWizardConfig`) and writes it atomically to the config path (default `prelude.nix`).

Key invariants:

- The config path is a sidecar module — never `flake.nix`. `isFlakeNixPath` refuses to write `flake.nix` so existing flakes are not overwritten (`run.go:156-158`); the emitted template header says "intentionally separate from flake.nix".
- `title.txt` is always a sibling of the config, so the emitted module references `./title.txt`.
- Starter docs non-clobbering: `os.Stat` checks existence before writing (`run.go:218-222`).
- Write order: title → starter docs → config. Stderr emits `wrote title.txt`, `wrote <config>`, and import next steps.
- The generated template (`src/internal/wizard/templates/flake_parts.nix.tmpl`) lists every supported option as a commented default from `defaults.nix`; only wizard choices are active.
- FIGlet normalization (`src/internal/wizard/render.go`): right-trim every row, drop fully-blank edge rows, keep interior blank rows.
- `--recipe` prefilles title text and font from a JSON or Nix recipe; JSON is preferred inside the build sandbox (`src/internal/wizard/recipe.go`).
- Terminal gates: `--wizard` and `--generate` are mutually exclusive; `--interactive` forces the chooser even without a TTY.

### Docs

`src/prelude/docs.nix` builds a config bundle (`runCommand`) from a nav tree, page materialization, a FIGlet hero, and a `config.json`, then bakes the config path into a Go viewer (`buildGoModule`, `subPackages = [ "cmd/docs" ]`).

Key invariants:

- Config bundle contract (`config.json`): `{ project, colorProfile, palette, nav, heroFile }`. `nav` is the expanded tree (generate nodes already resolved into groups/leaves). `heroFile` is a relative filename resolved by the Go viewer against the config dir; empty when no hero.
- Page materialization: `collect` assigns `pages/NNN.md` slots; root README leaves keep their body as-authored (no forced H1 header); other leaves get `# ${title}\n\n` prepended. `copyLines` emits `cp` or `printf+cat` per entry.
- Nix materialization rules: `writeText` embeds path strings only; `runCommand` creates real store edges via `cp`/`cat`. FIGlet hero is a separate `runCommand` with `figlet` as `nativeBuildInput` — pure derivation, never read by Nix eval (no IFD).
- Legacy inline pages: the Go viewer (`src/internal/docs/config.go`) back-compat-parses an older `pages` shape (array of `{text}`) into `NavNode` leaves when the current `nav` field is absent.
- Config-relative assets: `heroFile` is resolved against the config dir (`filepath.Join(base, heroFile)`), same convention as `NavNode.markdownFile`.
- Title precedence: explicit `NavNode.Title` wins; empty title falls back to the first H1 in the Markdown body (`markdownTitle`); if still empty, `page N` (`src/internal/docs/doc.go`). `markdownTitle` always supplies `page N`, so the `RootReadme` "README" fallback in `convertNode` is currently unreachable.
- Group/leaf normalization: `nodeShapeOK` enforces that each node is exactly one of generate/group/leaf; groups require `title`; leaves require `text`. `expandGenerate` handles `split = "allLeaves"` (nested sidebar) and `split = "shallow"` (single full page).
- README presentation: root README leaves (`rootReadme = true`) keep their body as-authored and are marked in Nix by path equality with `prelude.docs.rootReadme`.
- Viewer behavior: `src/internal/docs/run.go` loads config, converts nav (`convertNav` → `manual.NavNode`), and runs a `tea.Program`. The viewer is presentation-side; `pkg/manual` stays the presentation adapter.

### Command catalogue (dropped)

The Command catalogue (`src/prelude/command-catalogue.nix`) owns identity, normalization, grouping, selection, and surface projections for `prelude.commands`. Generators (`menu.nix`, `motd.nix`, `module.nix`) consume the domain and its projections rather than re-implementing catalogue rules.

Source evidence establishes an intentional, tested Nix→Go seam:

- `src/prelude/command-catalogue.nix` exports `projectMenuGroups` (menu TUI JSON), `projectMotdCatalog` (flat catalogue), and `projectMotdRows` (reduced MOTD rows).
- `src/prelude/menu.nix` consumes `projectMenuGroups` for the Go menu TUI JSON boundary.
- `src/internal/menu/invocation.go` resolves the public `x <command-key>` contract through `resolveXInvocation`; the complete key is globally unique and remains public.
- `nix/checks.nix:437-532` tests command ordering, defaults, and key uniqueness; `nix/checks.nix:1260-1353` tests MOTD/menu command runnability and shortcut resolution.

This seam is intentional and tested. The Command catalogue refactor is dropped from this round because the Nix→Go seam is intentional and covered by the checks above, and palette ownership is a separate out-of-scope concern.

## Target behavior

Each module preserves its public surface exactly. Deepening is internal only.

### MOTD

Preserve:

- Public flags: `--config`, `--preflight-only`, `--async`, `--pure`, `--refresh-status` (legacy alias).
- Config precedence: `PRELUDE_MOTD_CONFIG` env → Nix-injected `defaultConfigPath` fallback.
- Cache identity: the on-disk JSON map written by Preflight, last-write-wins, atomic rename.
- The Config → Preflight → Cache → Resolve/Render sequence.
- Pure Resolve/Render: no Runtime, no Cache I/O after Preflight.
- Preflight ordering: blocking before async; `PreflightBlocking` refreshes terminal size/bg, sync statuses, env; `PreflightAsync` refreshes only async status.
- Asymmetric cache/preflight failures: a blocking failure in the normal foreground run is logged as `motd: preflight:` and rendering continues with available Cache; only `--preflight-only` exits non-zero on a blocking Preflight failure. Async failures write stale/pending and never abort; cache read failure yields sparse UI.
- Post-output async refresh: the banner is written to stdout before the detached async Preflight begins.

Deepen internally using an orchestration plan/effect seam: the Config → Preflight → Cache → Resolve/Render sequence is expressed as a named internal orchestration that composes discrete effects (terminal probe, status check, env probe, cache read/write, render). Runtime, terminal, and Cache stay local adapters to that orchestration. No new exported interface is added — the binary entry point, JSON boundary, and `Render`/`RenderInput` public surface are unchanged.

### Setup

Preserve:

- flake-parts sidecar output (the generated `prelude.nix` is a flake-parts module, not standalone).
- `flake.nix` refusal (`isFlakeNixPath` rejects `flake.nix`).
- Sibling `title.txt` beside the config.
- Starter Docs non-clobbering (`os.Stat` before write).
- Write order: title → starter docs → config.
- Flags: `--recipe`, `-o`/`--output`, `--wizard`, `--generate`, `--interactive`.
- Terminal gates: `--wizard` and `--generate` mutually exclusive; `--interactive` forces chooser.
- FIGlet normalization (right-trim, drop blank edge rows, keep interior blanks).
- Recipe behavior: JSON recipes parsed directly; Nix recipes fall back to `nix-instantiate`; JSON preferred in the build sandbox.

Deepen internal mode/output sequencing: the title → starter docs → config materialization is expressed as a named internal sequence with discrete mode transitions (title render, docs materialization, config emission). The TUI, render, filesystem, and recipe helpers stay local adapters. The public `setup` wrapper and `prelude-title --wizard` entry point are unchanged.

### Docs

Preserve:

- `config.json`/pages/hero bundle contract.
- Nix materialization rules (`writeText` for strings, `runCommand` for store edges, FIGlet hero as separate pure derivation).
- Legacy inline pages (`pages` array of `{text}`) parsed by the Go viewer when `nav` is absent.
- Config-relative assets (`heroFile` resolved against the config dir).
- Title precedence (explicit → first H1 → `page N`; the `RootReadme` "README" fallback is currently unreachable because `markdownTitle` always supplies `page N`).
- README presentation (root README body as-authored, no forced H1).
- Viewer behavior (`tea.Program` over `manualDocument`).

Normalize current and legacy inputs through one internal Docs bundle-interpretation seam: the Go viewer's config loader currently handles legacy `pages` as a back-compat fallback beside the current `nav` tree. Deepen this into a single internal interpretation that accepts both shapes and produces one canonical `[]NavNode` with the same group/leaf/hero/title invariants. Nix stays the source-side adapter (it owns the bundle, materialization, and JSON); `pkg/manual` stays the presentation adapter (it owns the TUI document). No removal of legacy handling and no movement of README HTML semantics.

## Architecture

### MOTD orchestration seam

The module is `src/internal/motd`. The current entry point (`Run`) interleaves config load, cache open, preflight mode selection, render, and post-output async refresh in one function. Deepen by introducing an internal orchestration that names the sequence:

1. **Config load** — load JSON boundary (unchanged `loadConfig`).
1. **Preflight** — compose blocking and/or async effects against the Cache, per mode. The Runtime and terminal adapters remain local; the orchestration names which effects run in which mode and how failures map (blocking in the foreground run logs `motd: preflight:` and renders on with available Cache; `--preflight-only` exits non-zero on blocking failure; async writes-through).
1. **Cache read** — load the post-preflight cache (or sparse empty on cache failure).
1. **Resolve/Render** — pure, from `RenderInput`.
1. **Output** — write banner to stdout.
1. **Post-output async** — if due async entries exist and the run is not pure, start detached preflight.

The orchestration seam is internal. `Runtime` (`src/internal/motd/runtime.go`), the terminal adapter (`systemRenderTerminal`), and `Cache`/`cacheStore` remain local adapters. No new exported interface; `Render`, `RenderInput`, `Config`, `Cache`, `Preflight`, and `Resolve` keep their current signatures and JSON contract. The `--refresh-status` legacy alias and `PRELUDE_MOTD_PURE` env gate stay.

Locality: all deepening is within `src/internal/motd`. `src/prelude/motd.nix` is not modified — the Nix JSON boundary and `buildGoModule` call are unchanged.

### Setup mode/output sequencing

The module spans `src/internal/wizard` (Go) and `src/prelude/module.nix` (Nix wrapper). Deepen the `finishWizard` materialization into a named internal sequence with discrete mode transitions:

1. **Title render** — FIGlet render + normalization, write `title.txt` atomically.
1. **Docs materialization** — if docs enabled and starter page absent, write it (non-clobbering).
1. **Config emission** — render template, write config atomically.
1. **Stderr reporting** — write notices and import next steps.

The TUI (`wizardModel`, `View`, `Update`), render (`renderFIGlet`, `normalizeFIGletOutput`), filesystem (`writeAtomic`), and recipe (`loadRecipe`, `initialRecipe`) helpers stay local adapters. The `setup` wrapper in `module.nix` and the `prelude-title --wizard` entry point are unchanged. The `flake.nix` refusal, sibling `title.txt`, starter docs non-clobbering, write order, and template semantics are unchanged.

Locality: all deepening is within `src/internal/wizard`. `src/prelude/module.nix` is not modified — the `setupPkg` wrapper is unchanged.

### Docs bundle-interpretation seam

The module spans `src/prelude/docs.nix` (Nix source-side) and `src/internal/docs` + `pkg/manual` (Go presentation-side). Deepen the Go viewer's config loader into a single internal interpretation:

- Current input: `{ nav: [NavNode], heroFile: string, ... }`.
- Legacy input: `{ pages: [{ text: string }], ... }`.
- Both normalize to `[]NavNode` with the same group/leaf/hero/title invariants. The legacy path already does this (`src/internal/docs/config.go:57-68`); deepen it into a named interpretation seam so both shapes flow through one normalization, not a bolt-on fallback.

Nix (`src/prelude/docs.nix`) stays the source-side adapter: it owns the bundle, materialization, JSON, and FIGlet hero. `pkg/manual` stays the presentation adapter: it owns the TUI document and rendering. No removal of legacy handling — the `pages` shape continues to parse. No movement of README HTML semantics — root README leaves keep their body as-authored. The viewer's title resolution is explicit `NavNode.Title` → first H1 (`markdownTitle`) → `page N`; because `markdownTitle` always supplies `page N`, the `RootReadme` "README" fallback in `convertNode` is currently unreachable, and this deepening preserves that behavior without claiming "README" wins.

Locality: deepening is within `src/internal/docs` (config loader, `convertNav`, `doc.go`). `src/prelude/docs.nix` and `pkg/manual` are not modified.

### Parallel and commit strategy

Three disjoint slices, implemented in parallel:

- **MOTD** touches only `src/internal/motd/` (Go).
- **Setup** touches only `src/internal/wizard/` (Go).
- **Docs** touches only `src/internal/docs/` (Go).

No slice touches a file owned by another slice. The Nix source-side (`src/prelude/*.nix`) and `pkg/manual` are not modified by any slice. After targeted verification (below), each slice lands as a separate commit. If a slice's touched files stay disjoint from the others, the commits are independent.

## Verification

### MOTD

Extend the existing Go tests in `src/internal/motd/`:

1. **Mode/legacy alias**: `--refresh-status` maps to `--preflight-only --async`; `PRELUDE_MOTD_PURE=1` sets pure; `--pure` skips Preflight. Assert the orchestration produces the same observable output as the current flow for each mode.
1. **Config source**: `PRELUDE_MOTD_CONFIG` env overrides the Nix-injected default; missing config is a fatal error with the current message shape.
1. **Cache failure policy**: cache read failure yields a sparse `Cache{}` and the banner still renders (sparse UI); blocking Preflight failure in the foreground run is logged as `motd: preflight:` and rendering continues with available Cache (non-zero exit only under `--preflight-only`); async Preflight failure writes stale/pending status and does not abort.
1. **Foreground preflight warning behavior**: blocking Preflight with `spinner=true` on a TTY emits the MiniDot spinner on stderr; `--async` suppresses the spinner. Assert the spinner is emitted for blocking and suppressed for async.
1. **Output-before-async behavior**: the banner is fully written to stdout before the detached async Preflight begins. Assert that stdout contains the banner and the async preflight is started (or not, when no due async entries) after the output write.

### Setup

Extend the existing Go tests in `src/internal/wizard/`:

1. **Public policy matrix**: assert the generated `prelude.nix` contains the expected active choices (theme, project, title text path, docs pages when enabled) and commented defaults for every other option, matching the current template fragments.
1. **Materialization outcomes**:
   - `title.txt` is written beside the config with the rendered FIGlet output and trailing newline.
   - Starter docs page is written only when docs is enabled and the file does not already exist (non-clobbering).
   - Config is written after title and starter docs (write order).
   - `flake.nix` is refused; no file is written.
   - Stderr contains write notices and import next steps.
1. **Recipe behavior**: JSON recipe prefils text and font; unknown font is rejected; Nix recipe falls back to `nix-instantiate` outside the sandbox.
1. **FIGlet normalization**: blank edge rows are dropped; interior blank rows survive; right-trim per row.
1. **Terminal gates**: `--wizard` and `--generate` are mutually exclusive (non-zero exit); `--interactive` opens the chooser without a TTY.

### Docs

Extend the existing Go tests in `src/internal/docs/`:

1. **Current bundle input**: `config.json` with `nav` tree (groups + leaves + generate-expanded) loads into the expected `[]NavNode` with correct kinds, titles, markdown files, gap-before, and root-readme flags.
1. **Legacy bundle input**: `config.json` with `pages` array (no `nav`) loads into `[]NavNode` leaves with `text` as Markdown, one leaf per page. Both shapes produce the same canonical structure through the interpretation seam.
1. **Assets/errors**: `heroFile` relative to the config dir loads the hero; a non-empty `heroFile` that cannot be read is a fatal `loadConfig` error; an empty/absent `heroFile` skips the load and yields an empty `Hero`, allowing the manual viewer's project-name fallback.
1. **Group/leaf normalization**: `convertNav` maps groups to `manual.NavNode` with children and `gapBefore`; leaves map with Markdown and resolved title. Group title is preserved; leaf title follows the precedence chain.
1. **Hero**: the FIGlet hero is baked into the bundle as `hero.txt` and referenced by `heroFile`; the viewer loads it at config load time and stores it in `Hero` (not JSON-serialized).
1. **Titles**: explicit title wins; empty title falls back to first H1 (stripped of inline markup); no H1 falls back to `page N` (supplied by `markdownTitle`, always non-empty). Because `markdownTitle` never returns empty, the `RootReadme` "README" fallback in `convertNode` is currently unreachable; assert current behavior (`page N`) for root README with empty title, and do not assert that "README" wins.

### Shared

Run the focused Go tests for each slice (`go test ./internal/motd/...`, `go test ./internal/wizard/...`, `go test ./internal/docs/...`). Do not run formatters, linters, builds, or project-wide tests. Do not refresh generated docs or media — this design does not modify options or documentation source content.

## Non-goals

- No Command catalogue refactor. The Nix→Go seam in `src/prelude/command-catalogue.nix`, `src/prelude/menu.nix`, and `src/internal/menu/invocation.go` is intentional and tested; replacing it with palette ownership work is out of scope.
- No new exported interface on any module. MOTD's `Render`/`RenderInput`/`Config`/`Cache`, Setup's `setup` wrapper and `prelude-title --wizard` entry point, and Docs's `config.json` contract and `pkg/manual` document are all unchanged.
- No modification to Nix source-side (`src/prelude/motd.nix`, `src/prelude/module.nix`, `src/prelude/docs.nix`) or `pkg/manual`.
- No removal of legacy Docs `pages` handling or movement of README HTML semantics.
- No change to MOTD cache identity, preflight ordering, or the asymmetric cache/preflight failure policy.
- No change to Setup's `flake.nix` refusal, sibling `title.txt`, starter docs non-clobbering, write order, or template semantics.
- No change to Docs's Nix materialization rules, config-relative assets, title precedence, or viewer behavior.
- No generated docs or media refreshes.
