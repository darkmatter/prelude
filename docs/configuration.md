# Configuration

Everything is a `prelude.*` option, validated at build time.

- **`prelude.theme`** — palette: `phosphor` (default), `minted`, `amber`,
  `solarized`, `nord`, `gruvbox`, `paper`. Override single tokens with
  `prelude.palette`.
- **`prelude.colorProfile`** — `auto`, `truecolor`, or `ansi256`; force
  truecolor when terminal detection guesses wrong.
- **`prelude.motd.*`** — title (FIGlet or file), tagline, status probes,
  description, advertised commands, and multi-step recipes.
- **`prelude.commands`** — the shared catalogue keyed by public `x` command.
  The first colon derives menu group/name while the complete key remains
  callable; MOTD rows use that same `x <key>` form.
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
- **`prelude.prompt.enable`** — themed Starship config at `packages.prompt`.

The full option reference is also generated to `docs/reference/options.md`
(refresh with `docs-sync`) and appears in the docs TUI under **Options**.
