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
- **`prelude.docs.pages`** — ordinary Markdown files; the first heading
  becomes the sidebar label.
- **`prelude.prompt.enable`** — themed Starship config at `packages.prompt`.

The full option reference is generated to `docs/reference/options.md`
(refresh with `docs-sync`).
