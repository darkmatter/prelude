# Title rendering guide

Use `prelude-title` to create a FIGlet wordmark for `prelude.motd.title.text`.
The chooser previews the title with the same divider treatment used by the MOTD,
then renders the selected result to stdout or an explicit file.

## Interactive workflow

Start the chooser from the project whose title you want to render:

```sh
nix run .#title
```

The first screen is a prefilled text field. Without an explicit recipe, Prelude
uses the current directory name. Continue to the style pager, review the live
FIGlet previews, and press Enter to select one.

Pager keys:

| Key                                | Action                     |
| ---------------------------------- | -------------------------- |
| `←`, `↑`, `h`, `k`, `Shift-Tab`    | Previous style             |
| `→`, `↓`, `j`, `l`, `Tab`, `Space` | Next style                 |
| `Home` / `End`                     | First / last style         |
| `Enter`                            | Confirm the selected style |
| `Esc` / `Backspace`                | Return to the title field  |
| `q` / `Ctrl-C`                     | Cancel                     |

## Setup wizard

`--wizard` extends the chooser into a project setup wizard: after the title
and style pages it collects the project name, theme (with live palette
preview), color depth, component toggles (each previewed in your theme),
initial project commands (name, exec, description), and the config shape,
then prints a ready-to-use config to stdout:

```sh
nix run .#title -- --wizard > prelude.nix
```

The wizard UI renders on stderr, so redirecting stdout captures only the
config. The rendered wordmark is written to `docs/title.txt` (override with
`-o`; an existing file at that path is replaced). Enabling the docs viewer
also writes a starter `docs/getting-started.md`, but an existing page is
kept untouched.

The final step chooses between two config shapes:

- **flake-parts** (default) — a module to import next to Prelude's:

  ```nix
  imports = [ prelude.flakeModules.default ./prelude.nix ];
  ```

- **standalone** — `prelude.lib.mkMotd`/`mkMenu`/`mkDocs` builder calls for
  flakes that do not use flake-parts; call the file with
  `{ inherit pkgs prelude; }` and put the resulting packages on your shell.

## Choose where the result goes

The rendered title is stdout by default. In non-interactive generation this
makes the command composable:

```sh
nix run .#title -- --generate --recipe config/title.nix > title.txt
```

Redirecting stdout disables terminal detection, so use `-o` when you want to
make an interactive selection and save it atomically:

```sh
nix run .#title -- -o title.txt
```

`--output` is the long form of `-o`. With either output option, stdout stays
empty and the `wrote …` status is sent to stderr.

Prelude never searches for, creates, or rewrites `title.nix` implicitly.

## Optional recipe input

A recipe is an explicit input preset for title text and font:

```nix
{
  text = "acme-web";
  font = "calvin-s";
}
```

Pass it deliberately:

```sh
nix run .#title -- --recipe config/title.nix
nix run .#title -- --generate --recipe config/title.nix -o title.txt
```

`--generate` skips the chooser. Without `--recipe`, it renders the current
directory name with the default font.

## Configure the MOTD

Point the Prelude module at the rendered file:

```nix
prelude.motd.title = {
  text = ./title.txt;
  align = "center";
};
```

`title.align` controls placement of the complete rectangular wordmark inside
the MOTD. Do not use FIGlet justification to bake left or right padding into the
artifact.

`title.style` is unrelated to the FIGlet font. It only controls the fallback
project-name treatment when `title.text` is `null`.

## Rendering controls

The title workflow treats FIGlet rendering as three user-facing choices:

1. **Font** — glyph design, height, and overall geometry. The current pager
   exposes Prelude's 23 bundled fonts.
2. **Spacing** — how adjacent FIGcharacters combine. The agreed chooser shape is
   `Font default`, `Smushed`, `Kerned`, or `Full width`.
3. **Width** — the fixed FIGlet output width used for wrapping. A fixed value is
   reproducible; terminal-derived width is not.

Spacing and width are the next chooser controls. Until they are exposed, Prelude
uses each font's default spacing behavior and FIGlet's default 80-column width.

## Reproducible output

For checked-in titles and CI regeneration:

- use an explicit recipe when text and font must be stable;
- use a fixed output path with `-o`;
- avoid terminal-width rendering;
- commit the rendered text file consumed by the Nix module;
- run `git diff --check` to catch accidental whitespace changes.

The renderer removes line-ending whitespace while preserving the rectangular
geometry of the FIGlet block during preview and MOTD placement.
