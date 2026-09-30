# Prelude

DX interface for Nix devshells: MOTD banner, command menu, docs viewer, and setup wizard.
Configuration is authored in Nix; Go binaries consume normalized JSON and (for MOTD) a live cache.

## Language

### Surfaces

**MOTD**:
Shell-entry welcome banner. One public command (`motd`) that preflights when needed, then pure-renders.
_Avoid_: splash, banner app, session (as the paint model)

**Menu**:
Interactive task picker and non-interactive dispatcher over the command catalogue (`x`).

**Docs**:
Full-screen Markdown viewer over pages embedded at build time.

**Command catalogue**:
Project tasks declared in Nix (`prelude.commands`), projected into menu groups and MOTD next-steps.
_Avoid_: Task list (prefer catalogue for the Nix-side whole; menu still uses Task at its JSON boundary)

### MOTD pipeline

**Config**:
Nix-generated JSON for one surface. Declarative only — no live probe/check results.
_Avoid_: Session, Authored config (as type names)

**Cache**:
Single JSON map of live MOTD facts written by Preflight, read by Render. Each entry has identity, value, checkedAt, and TTL.
_Avoid_: Session, status-only store (old model)

**Preflight**:
Impure phase: due probes, status checks, terminal query; writes Cache. Idempotent enough for last-write-wins concurrency.
_Avoid_: resolve session, StatusResolver (as the public story)

**Render**:
Pure phase: Config + Cache → banner string. Always succeeds (sparse UI when cache is cold/stale). No shell, no OSC in pure mode.
_Avoid_: renderSession orchestration that mutates Config

**RenderInput**:
Single in-memory input to Render: Config plus Cache (and any test-injected layout size).

**Entry key**:
Cache identity for status/env: kind prefix plus check/probe string (normalized). Not list index.
_Avoid_: status index keys

### Cache policy (v1)

**Blocking preflight**:
Runs before paint when due: terminal size, terminal background (when needed), sync status checks, env probes.

**Non-blocking preflight**:
Async status entries never delay paint; after paint, a detached `--preflight-only` refresh may update them.

**TTL defaults** (Go-owned, not Nix):
terminal size/bg and sync status: every non-pure run (TTL 0); async status and env: 5m.

**Static status**:
Header badge with no check — Config only, never cached.

### Library mode

**libprelude**:
The Go surfaces built as a C shared library (`src/cmd/libprelude`) for processes that run them in-process. One JSON request in, one JSON reply out; exports never exit or exec.

**Host**:
A process that runs a surface through libprelude, such as an app using the TypeScript API (`ts/`). The host owns execution: it runs Selections itself and gathers the live facts it evaluates in TypeScript.
_Avoid_: embedder, client

**Selection**:
What the menu picker resolved: the chosen task's name, the argument line entered for it, and the assembled shell command. The CLI execs the command; a host dispatches on the name and parses the line.

### Flags

**`--preflight-only`**:
Run Preflight (write Cache), do not paint. With **`--async`**, only async status entries. Detached post-paint refresh uses both. Replaces the old refresh-status-only path.
_Avoid_: `--refresh-status` as the long-term name

**`--pure`**:
Skip Preflight; Render from files only (Config + Cache). Layout size from input/cache else 80×24.
Also: `PRELUDE_MOTD_PURE=1`.

## Relationships

- **Nix → Config**: `motd.nix` / flake module generate JSON that a thin wrapper passes to the Go binary at run time, so Config edits never recompile Go; Go does not re-default policy owned by Nix except live TTLs.
- **Preflight → Cache → Render**: only direction for live facts; Render never calls Runtime.
- **Menu / Docs**: own Config JSON; no MOTD Cache (unless a future design unifies).
- **TypeScript → Config**: `ts/` authors the same menu and MOTD Config JSON as Nix for Hosts; conformance fixtures hold the two authors to one output.
- **Host → Render**: a Host has no Cache file or detached refresh. The MOTD runs shell checks and probes inline, as blocking Preflight does, and records the Host's own outcomes before Render.
