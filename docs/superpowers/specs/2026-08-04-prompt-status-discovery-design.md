# Prelude Prompt, Status, and Discovery Redesign

**Date:** 2026-08-04\
**Status:** Implemented

## Purpose

Prelude currently uses Starship for the prompt and borrows Starship's right prompt for
Bash's fixed ble.sh status line. That arrangement makes the primary prompt visually
sparse, couples status rendering to Starship's right-prompt implementation, and leaves
command discovery dependent on entering the picker. This design makes the prompt teach
Prelude's command model without evaluating live input, while keeping health checks
cached and keeping shell/configuration ownership explicit.

## Current-state diagnosis

- `src/prelude/prompt.nix` generates two leading blank rows and a `$character`
  prompt, then places the Powerline/status string in Starship `right_format`.
  The Powerline currently contains the project, directory, Git branch/status, and
  command-duration segments; navigation chips are appended to that status format.
- `src/prelude/shell/bash-init.bash` initializes Starship, installs the status
  hook, and moves Starship's rendered right prompt into the ble.sh status line.
  `src/prelude/shell/status.bash` reads `bleopt_prompt_rps1`, clears it, and
  renders the captured value through a ble.sh `\q` prompt callback.
- `src/prelude/options/prompt.nix` defines `settings` and `configFile`. In
  `src/prelude/module.nix`, generated status behavior is enabled only when
  `prelude.prompt.configFile == null`; a supplied config file is therefore a
  user-owned Starship configuration boundary.
- `src/prelude/command-catalogue.nix` is the source of command identity and
  metadata. It normalizes keys, descriptions, usage/details/examples, arguments,
  bounded options, and canonical `x` invocations. Its projections already carry
  the metadata needed by the menu and shell consumers. `src/internal/menu/run.go`
  defines bare `x` as the picker, `x --list` as the noninteractive listing, and
  `x <key>` as dispatch; `src/internal/menu/listview.go` owns filtering and
  expanded selection presentation.
- `src/internal/motd/cache.go`, `preflight.go`, and `run.go` provide the existing
  persisted cache, TTL freshness checks, asynchronous status refresh, and pure
  rendering path. Cache entries carry `CheckedAt`, `TTL`, status text, and level;
  rendering can proceed with the last cache when preflight fails.

The redesign removes the Starship-right-prompt dependency rather than recreating it
under another name.

## Target rendering

The generated Starship configuration has this visual order:

```text
(blank breathing row)
(blank breathing row)
<existing Powerline segments, one left-aligned Starship row>
<muted ~/<configured project>> <bold accent2 ❯> <editable input>

<full-width `▄` cap: top half context background, bottom half status surface>
<fixed bottom ble.sh status row>                         [? motd] [m menu] [d docs]
```

1. Preserve exactly the existing two blank breathing rows.
1. Remove `right_format`; the existing Powerline must not be rendered on the
   right. Render the current Powerline segments as one additional left-aligned
   Starship row above the input row.
1. Make the input row follow the menu visual convention: muted
   `~/<configured project>`, bold accent2 `❯`, then the user's editable input.
   This is styling only. Do not place a fake `Type a command…` placeholder in
   the input buffer.
1. Bash places a full-width one-cell `▄` cap immediately above the fixed status
   row. Its top half blends with the terminal background and its bottom half
   blends with the status surface, matching the menu footer treatment.
1. The fixed bottom ble.sh status row retains the exact navigation chips
   `? motd`, `m menu`, and `d docs`. Contextual discovery and local-server health
   occupy the status row's owned dynamic area without changing those chips.
1. Bash receives the ble.sh status behavior. Zsh retains native Starship and
   does not receive the fixed ble.sh status row.

The dynamic Bash area must be invalidated through a ble.sh `\q` callback whose
prompt unit hash includes the live input buffer. This permits discovery to change
as the user edits without causing a probe or shell evaluation. The callback is a
pure projection of input plus generated catalogue metadata and cached health data.
The relevant callback/status-line contract is the official ble.sh prompt and
status-line documentation, alongside the existing hook in
`src/prelude/shell/status.bash`.

## Contextual-discovery state machine

The status projection classifies input conservatively. Classification is metadata
lookup and safe tokenization only; it never invokes a shell parser by evaluating the
buffer and never executes the buffer.

| Input state | Discovery content |
| --- | --- |
| Empty/default input | Welcome the configured project. Teach bare `x` for the interactive picker, `x --list` for a noninteractive listing, and Tab for inline completion. |
| `x` or `x ` | Teach selecting a command key and show that the inline chooser/completion is available. |
| Known `x <key>` | Show the catalogue description and canonical invocation. Direct users to bare `x` then Tab for expanded usage, details, and examples. |
| Current argument for a known command | Show the current argument token, whether it is required or optional, its description, and bounded candidate values when the catalogue supplies them. |
| Unknown, quoted, or incomplete input | Show safe generic discovery hints only. Never evaluate, execute, or partially interpret it as a command. |

The generated command catalogue is the only metadata source. The implementation must
not invent a second registry, infer command-specific documentation from display
labels, or claim that `d` supplies dynamic command metadata. `d` remains the general
Markdown documentation entrypoint. The canonical execution form remains `x <key>`;
any start instruction derived from a command key uses that canonical form.

## Opt-in local-server health

Add an explicit opt-in `prelude.prompt.localServer` descriptor. It is absent/disabled
by default and has the following conceptual shape:

```nix
{
  command = "dev";
  check = "curl -fsS http://127.0.0.1:3000/health >/dev/null";
  ttl = "5m";
}
```

- `command` is a canonical command-catalogue key, not arbitrary display text. Nix
  normalization/projection must validate that the key exists and must retain the
  metadata needed to construct the canonical start instruction (`x <key>`).
- `check` is the explicit local health check passed to the existing status-probe
  runtime. It is not derived from user input or from the command's display label.
- `ttl` is explicit descriptor configuration and controls cache freshness. The
  normalized runtime representation must preserve the duration and reject invalid
  values rather than silently choosing a different lifetime.

Health refresh reuses the existing persisted, asynchronous status-probe lifecycle in
`src/internal/motd/cache.go`, `src/internal/motd/preflight.go`, and
`src/internal/motd/run.go` (including its pure rendering/cache-read boundary). A due
or missing entry may be scheduled for asynchronous preflight by the shell/runtime
lifecycle. Prompt rendering and the `\q` callback read cache only. No process or
network probe runs per keystroke, per edit, or while merely classifying input.

The dynamic health projection exposes these states:

- **Checking:** no usable result is cached yet and a refresh is pending or due.
- **Fresh healthy:** the latest check succeeded and is within the configured TTL.
- **Stopped/unavailable:** the latest check failed or reports the local server as
  unavailable. This state takes priority over generic discovery and includes a
  concrete start instruction using the canonical command, for example `run x dev`.
- **Stale:** the cached result is past TTL. Preserve the last known result while
  labeling it stale and show a compact age such as `17m ago`; if the last result
  was stopped, keep the concrete start instruction primary.

A failed refresh is non-fatal to prompt painting: retain the previous cache when one
exists, otherwise show checking/safe discovery rather than blocking the input. Cache
identity and atomic persistence follow the existing project/config scoping and
last-write-wins behavior; a prompt render must remain usable when the cache is
missing or unreadable.

## Ownership boundaries

- **Generated prompt config:** owns the redesigned Starship rows, palette/style
  defaults, and generated Bash status integration when `configFile` is unset.
- **`prelude.prompt.configFile`:** remains fully user-owned. When supplied, Prelude
  must not merge generated rows, install generated status behavior, or synthesize
  local-server status into that configuration.
- **Starship:** owns the native prompt visual rows and zsh prompt behavior.
- **ble.sh:** owns Bash line editing, inline completion, the fixed bottom status
  placement, and `\q` invalidation. Its status callback consumes only a pure,
  precomputed projection.
- **Command catalogue:** owns canonical command keys and generated metadata
  (description, invocation, arguments, options, usage, details, examples).
- **MOTD/preflight cache:** owns impure status checks, TTLs, asynchronous refresh,
  and persisted cached facts. Prompt rendering never owns probe execution.
- **Docs:** `d` opens general Markdown docs; it is not a dynamic command-help API.
- **Shell variants:** Bash gets Prelude's ble.sh status behavior; zsh keeps native
  Starship without the status row.

## Non-goals

- No CI/build status in the prompt or local-server descriptor.
- No per-keystroke process, network, or shell evaluation.
- No evaluation of quoted, incomplete, unknown, or otherwise unsafe input.
- No second command registry and no command metadata fabricated from labels.
- No change to the canonical `x` execution/listing/picker contract.
- No generated status behavior inside a user-supplied `configFile`.
- No claim that `d` provides command-specific dynamic usage, details, or examples.

## Verifiable acceptance criteria

1. **Nix option and projection validation:** the default has no local-server probe;
   a valid descriptor projects exactly its canonical command key, explicit check,
   and TTL; an unknown command key or invalid TTL fails configuration validation;
   generated status remains disabled when `configFile` is supplied.
1. **Status callback behavior:** focused tests cover empty/default, `x`, `x `,
   known key, current argument, and safe fallback states. The callback includes
   the live input buffer in its ble.sh hash and never evaluates the buffer.
1. **Health cache lifecycle:** focused tests cover missing/checking, fresh healthy,
   failed/stopped, stale, compact ages, configured TTL expiry, and the canonical
   start instruction for stopped/unavailable results.
1. **No synchronous probe per edit:** focused tests or instrumentation prove that
   editing the input causes projection/invalidation only; no process or network
   check is run synchronously per edit or keystroke.
1. **Normal visual output:** rendering checks prove two blank rows, a left-aligned
   Powerline row above the input, muted `~/<configured project>`, bold accent2
   `❯`, no Starship `right_format`, and no input placeholder. Bash retains the
   fixed status row and a full-width cap immediately above it with exact `? motd`,
   `m menu`, and `d docs` chips; zsh has no generated status row.
1. **Custom configuration ownership:** a user-owned `configFile` is rendered
   without generated Powerline/discovery/health/status behavior, while normal
   generated configuration receives the redesign.
1. **Boundary regressions:** tests confirm catalogue metadata comes from the
   generated command projection, `x --list` remains noninteractive, `d` remains
   general Markdown docs, and no CI status is introduced.
