# Agent Skill App

**Date:** 2026-08-08  
**Status:** Approved design

## Goal

Expose a standalone flake app that lets an agent discover and read Prelude's
installation, configuration, command, option, and guide material without
cloning the repository or driving the interactive docs TUI:

```sh
nix run github:darkmatter/prelude#skill
nix run github:darkmatter/prelude#skill -- options
```

The app's detailed content remains ordinary checked-in Markdown under `docs/`.
The executable owns routing only; it must not duplicate or render documentation
into a separate format.

## Current state

- `nix/apps.nix` owns this repository's standalone app outputs. It already
  exposes utility apps such as `docs-sync` and `docs-record`.
- `src/prelude/module.nix` exposes `docs` only when a consumer configures
  `prelude.docs.pages`; that app is an interactive Bubble Tea viewer for a
  configured project's documentation.
- The upstream Markdown sources already cover the needed material:
  - `docs/your-own-repo.md` — setup and module wiring.
  - `docs/reference/options.md` — generated full option reference.
  - `docs/commands.md` — public command conventions.
  - `docs/configuration.md` — configuration overview.
  - `docs/guides/*.md` — focused guides.

An agent-facing static reader must therefore be a separate app rather than a
mode of the consumer-configured docs viewer.

## Public interface

`nix run github:darkmatter/prelude#skill` invokes an executable named `skill`.
Arguments after `--` select material:

| Invocation | Result |
| --- | --- |
| `skill` | Prints the concise Markdown introduction in `docs/skill.md`. |
| `skill list` | Lists every supported stable selector and its purpose. |
| `skill install` | Prints `docs/your-own-repo.md`. |
| `skill options` | Prints `docs/reference/options.md`. |
| `skill commands` | Prints `docs/commands.md`. |
| `skill configuration` | Prints `docs/configuration.md`. |
| `skill guide command-conventions` | Prints `docs/guides/command-conventions.md`. |
| `skill guide title-rendering` | Prints `docs/guides/title-rendering.md`. |

The introduction must say that the detailed material is Markdown, show the
published-style command form, and direct agents to `list` before guessing a
selector. It should fit a single concise terminal screen.

Selectors are deliberately finite and semantic. A generic filesystem-path
reader is out of scope: it would make agents learn repository layout and could
expose implementation design notes rather than public documentation.

Unknown selectors, missing guide names, and extra arguments fail nonzero, write
a concise diagnostic to stderr, and point to `skill list`. Markdown is written
unchanged to stdout so an agent can capture or parse it.

## Architecture and ownership

Add `nix/skill.nix`, a `pkgs.writeShellApplication` whose only runtime work is
argument validation and routing to immutable Markdown paths. Its closure must
contain each selected document, so the command works from a remote flake
reference and does not depend on the caller's checkout.

`nix/per-system.nix` imports the package once. `nix/packages.nix` exports it as
`packages.skill`, and `nix/apps.nix` exposes the corresponding `apps.skill`
entry with the repository's existing `mkApp` convention. This makes both
`nix build .#skill` and `nix run …#skill` resolve to the same program.

`docs/skill.md` is the sole authored intro. Existing installation, options,
commands, configuration, and guide files remain authoritative; no content is
copied into Nix strings. The existing interactive `docs` app and its
consumer-configured navigation remain unchanged.

## Error handling

The wrapper must distinguish supported selectors from malformed input before
printing any document. It must never fall back to a local relative path, invoke
a shell parser on user input, or silently redirect an unknown topic. Error
messages go to stderr so valid Markdown output is cleanly machine-readable.

## Verification

Add a focused Nix check that runs the built executable from an empty temporary
directory and proves:

1. Bare `skill` contains the published `nix run github:darkmatter/prelude#skill`
   instruction and directs callers to `list`.
2. `list` advertises the complete selector contract: `install`, `options`,
   `commands`, `configuration`, `guide command-conventions`, and
   `guide title-rendering`.
3. Each primary selector emits identifying text from its authoritative Markdown
   source.
4. An unknown selector, `skill install extra`, and `skill guide` each exit
   nonzero, emit no Markdown on stdout, and direct the caller to `list` on
   stderr.
5. `nix build .#skill` and `nix run .#skill` resolve to the same executable,
   whose Markdown paths work without a caller checkout.

Exercise the same remote-style argument shape locally with `nix run .#skill`
and `nix run .#skill -- <selector>`, then run the focused check and the
repository's relevant Nix verification.

## Non-goals

- No interactive terminal UI or ANSI formatting.
- No runtime network access, source checkout requirement, or docs generation.
- No generic file browser.
- No change to the existing `docs` viewer, `prelude.docs.*` options, or
  generated options-reference pipeline.
