# Roots, Launcher, and Cache

**Date:** 2026-10-06\
**Status:** Approved design

## Goal

Each way of starting Prelude shows the right project, and none of them waits on a full flake evaluation after a Nix edit.

- `nix run github:org/repo#prelude-menu`, and the other published packages, show that repo's surfaces wherever they run.
- `prelude`, installed with `nix profile add github:darkmatter/prelude`, opens the surfaces of the repo you are in, without entering its devshell.
- After a Nix edit, the launcher and the devshell's `x` and `docs` show the change within seconds, without a direnv reload.

## Current state

- **Menu.** The menu Config holds the commands a repo declares in Nix. Imports (Justfile recipes, package.json scripts) are read from the caller's working directory, commands run there, and Prelude's own entries (`x`, `docs`, `motd`) are names looked up on the caller's `PATH`. A remote `nix run github:org/repo#prelude-menu` therefore shows the repo's declared commands mixed with the caller's Justfile recipes, and runs everything in the caller's directory.

- **Docs.** Pages are bundled into the docs package when it is built. A remote run already shows that repo's docs. Editing a page shows only after a rebuild, which in a devshell means a direnv reload.

- **MOTD.** The Config is built into the package. Status checks and env probes run in the caller's directory.

- **Default package.** Every prelude-enabled flake's `packages.default` is its own `prelude` dispatcher. Prelude's own flake follows suit, so `nix profile add github:darkmatter/prelude` installs Prelude's dogfood surfaces, not a tool for other repos.

- **Speed.** Measured in this repository with a warm store:

  | Step | Time |
  | --- | --- |
  | Fingerprint of tracked `*.nix` and `flake.lock` (`git ls-files -s`, `git status`) | 5 ms |
  | Evaluate only the menu package, Nix eval cache off | ~1 s |
  | The same, Nix eval cache on | 44 ms |
  | `nix print-dev-env` for the whole devshell | ~4.8 s |

  Consumer repositories report about 30 s for a direnv reload after a Nix edit. Most of that is the devshell, not the menu.

## Decisions

1. **Surfaces have a root.** The root is the directory a surface reads project files from and runs commands in.
   - The menu and docs Configs gain `root`. Empty means the caller's working directory, which is today's behavior.
   - `PRELUDE_ROOT` overrides a bound root; a surface without a root ignores it, so a devshell started from a bound menu's command keeps working in its own checkout. A bound menu hands the root on with its own Config path (`PRELUDE_ROOT_MENU`), and another bound menu ignores a root handed on that way. A bound menu hands its commands `PRELUDE_ROOT` and its Config (`PRELUDE_MENU_CONFIG`), in exec and print mode alike, so a nested `x` or `docs` stays on the same root.
   - Menu: imports resolve from the root, and a task without its own `dir` runs in the root.
   - Docs: with a root, the viewer reads each page from the root (so edits show at once). Without a root, it reads the bundle. The docs Config records each page's path relative to the flake source as well as its bundled copy.
   - MOTD: unchanged. Its checks and probes keep running in the caller's directory.
1. **Published packages are bound to their repo.**
   - The option `prelude.root` names the root that published packages are bound to. `flakeModules.default` defaults it to the flake's source, `self.outPath`; `lib.evalModule` callers set `prelude.root = self`.
   - `prelude` and `prelude-menu` are bound in phase 1, and `prelude-docs` in phase 2. `prelude-motd` keeps the caller's directory (see the MOTD bullet above).
   - In these packages, Prelude's own entries open the package's own programs rather than whatever the caller's `PATH` holds. A bound menu puts a shim for its own `x` first on its commands' `PATH`, so the `x` entry, and any recipe that calls `x`, reach the bound menu. The `prelude` app also puts its own docs, MOTD, and portal first on `PATH`. `prelude-menu` alone must not pull those packages into its closure, so its `docs` and `portal` entries still come from `PATH`.
   - The devshell builds the same surfaces without a root, so the shell behaves as it does today.
   - A remote menu's commands run in a read-only copy of the repo. Commands that only read work, such as `go test` and `nix flake check`. Commands that write into the tree fail, such as `nix build` leaving `./result`.
   - A published package's closure includes the repo's source.
1. **Prelude's own flake gets a launcher.**
   - Its `packages.default` becomes a small `prelude` launcher. Prelude's dogfood dispatcher stays at `#prelude`.
   - For `menu`, `x`, `docs`, and `motd`, the launcher walks up from the working directory to the nearest `flake.nix`. It runs that flake's own published package for the surface, with `PRELUDE_ROOT` set to the checkout.
   - Running the repo's pinned build keeps the Config schema and the binaries at the version the repo locks.
   - Prelude's own commands stay on the launcher, so `nix run github:darkmatter/prelude -- wizard` keeps working.
   - A directory with no prelude-enabled flake above it gets a one-line error naming the missing output.
1. **A cache, refreshed in the background.** The launcher follows the MOTD's Preflight → Cache → Render pattern.
   - **Key:** the checkout root, plus a fingerprint of the tracked `*.nix` files and `flake.lock` (index blob hashes, plus hashes of modified files). Other edits never invalidate it. Docs pages are read live (decision 1), so they need no place in the key.
   - **Entry:** the store paths of the surface's built package. A GC root under the cache directory keeps them alive.
   - **Hit:** the surface runs at once.
   - **Stale entry** (the fingerprint changed): the previous entry runs at once, with a dim "refreshing after a Nix change" note. A detached build of only that surface's package writes the new entry, last write wins by atomic rename, and the next start is current.
   - **No entry:** the first build runs in the foreground, once per repo.
   - **Missing store paths** (collected anyway) count as no entry.
   - **Age:** an entry older than an hour also refreshes in the background, which covers Nix files that read non-Nix files.
1. **The devshell uses the same cache.**
   - The devshell's `x`, `menu`, and `docs` check the cache for the checkout before using the Config the shell was built with. They use the newer of the two.
   - An edit to `prelude.commands` then shows up in `x` within seconds, without waiting for the shell to reload.
   - Projects can stop nix-direnv from reloading on every Nix edit and reload only when they need new packages in the shell.

## Phases

Each phase ships on its own and leaves `x check` green.

1. **Root and repo-bound packages.**
   - Add the menu Config `root` and `PRELUDE_ROOT`.
   - Imports resolve from the root, and tasks run there.
   - Published packages bind to `prelude.root` (by default the flake's `self.outPath`); a bound menu brings `just` and its commands' packages on PATH and puts its own `x` first on its commands' PATH.
   - Add a check that runs a built `prelude-menu` from another directory. It must list the repo's own Justfile recipes and run a command in the repo's source.
1. **Docs read pages from the root.**
   - Add the docs Config `root` and the pages' relative paths.
   - Add a check that a page edit shows without a rebuild when a root is set.
1. **The launcher.**
   - `packages.default` becomes the launcher. It builds in the foreground on every start, which is correct but slow.
   - Add a check that runs the launcher in a fixture repo.
1. **The cache.**
   - Add the fingerprint, entries, GC roots, and background refresh.
   - Measure the launcher's start before and after this phase.
1. **The devshell on the cache.**
   - The devshell's `x`, `menu`, and `docs` prefer a newer cache entry.
   - The setup guide shows how to keep nix-direnv from reloading on every Nix edit.

## Out of scope

- MOTD status checks and env probes in a remote run. They keep running in the caller's directory.
- Writable remote runs, such as cloning the repo first.
- The launcher for projects without a flake.

## Docs to update

- CONTEXT.md: **Root**, the directory a surface reads project files from and runs commands in; **Launcher**, the profile-installed `prelude` that runs the current repo's pinned surfaces.
- README and `docs/your-own-repo.md`: the remote and installed ways to run Prelude.

## Related

[Command Sources](2026-10-01-command-sources-design.md) keeps its own cache phase, for runtime imports such as Justfile recipes, package.json scripts, and Bun apps. The cache here holds Nix-built surfaces, keyed by Nix inputs. The two are separate.

## Verification

Per phase: `x go:test` and `x go:vet`; `x ts:test` when `ts/` changes; `x fmt` for Nix changes; `x sync-docs` when options change; and `x check`.
