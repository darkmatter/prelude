# Configuration

Everything is a `prelude.*` option, validated at build time.

- **`prelude.theme`** — palette: `prelude` (default), `phosphor`, `minted`,
  `amber`, `solarized`, `nord`, `gruvbox`, `paper`. Override single tokens with
  `prelude.palette`.
- **`prelude.colorProfile`** — `truecolor` (default), `auto`, or `ansi256`.
  Use `auto` only when supporting terminals where capability detection and
  graceful color-depth fallback matter.
- **`prelude.motd.*`** — title (FIGlet or file), tagline, status probes,
  description, advertised commands, and multi-step recipes.
- **`prelude.commands`** — the shared catalogue keyed by public `x` command.
  A space or `/` makes a subcommand (`"db migrate"` runs as `x db migrate`
  under one `db` row); `:` is an ordinary name character. Groups come only
  from each command's `group`. MOTD rows show a command bare when its
  top-level name is on PATH and in its `x …` dispatch form otherwise.
- **`prelude.docs.pages`** — nav tree of Markdown leaves, groups, and optional
  `{ generate = "nixosOptions"; split?; }` selectors. Generate `split`:
  `allLeaves` (default, nested tree of every terminal option) or `shallow`
  (one full nixosOptionsDoc page). Split one file into H2 leaves with
  `pages = [ (prelude.lib.mdSplit ./README.md) ];` (fence-aware H2 split; keeps path for FIGlet).
- **`prelude.docs.rootReadme`** — exact path to the consumer root `README.md`.
  When a leaf's `text` equals this path, the TUI styles the project title and
  HTML intro (tagline/chips) instead of rendering the center block as raw HTML.
- **`prelude.docs.nixosOptions`** — full `pkgs.nixosOptionsDoc` argument set
  (`{ options = …; … }`, including any of `transformOptions`, `documentType`,
  `warningsAreErrors`, `revision`, …) used when a generate node is present.
- **`prelude.prompt.enable`** — themed Starship config at `packages.prelude-prompt`;
  themes an existing Starship prompt under direnv and selects Starship for
  automatic workspace entry through interactive `nix develop`.
- **`prelude.workspace.enable`** — opt-in foreground workspace entry from generated
  project init in interactive `nix develop` (Bash with a TTY; default: `false`).
  `.envrc` stays `use flake` for the lightweight environment, MOTD, and existing
  Starship theming; the full footer and catalogue completion come with the
  interactive devshell. No extra Prelude Bash rc hook is required. Manual launches
  and optional, already-configured `prelude hook bash` handoffs remain supported;
  see the [workspace guide](guides/workspace.md).

The full option reference is also generated to `docs/reference/options.md`
(refresh with `x sync-docs`) and appears in the docs TUI under **Options**.
