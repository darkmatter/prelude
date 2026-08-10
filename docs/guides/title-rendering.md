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

| Key | Action |
| ---------------------------------- | -------------------------- |
| `←`, `↑`, `h`, `k`, `Shift-Tab` | Previous style |
| `→`, `↓`, `j`, `l`, `Tab`, `Space` | Next style |
| `Home` / `End` | First / last style |
| `Enter` | Confirm the selected style |
| `Esc` / `Backspace` | Return to the title field |
| `q` / `Ctrl-C` | Cancel |

## Setup

`setup` opens the project setup flow. After the title and style pages it
collects the project name, theme (with live palette preview), color depth, and
initial project commands (name, exec, description) before component toggles.
That ordering lets the first three commands appear in every component and MOTD
preview. When MOTD is enabled, setup then asks for a one-line project tagline
and a multiline welcome message before the status page. The asynchronous
`nix flake check` item is enabled by default. A separate dev-server toggle asks
only for its health URL, prefilled as
`${APP_HOST:-http://127.0.0.1:3000}/health`; setup builds the `curl -fsS` check
for the user. The layout, spacing, and surface pages then cover
horizontal and vertical placement, title alignment, margin and padding presets,
width, backgrounds, an optional border, and clear-screen behavior. Setup writes
a ready-to-use config next to a sibling title file:

```sh
nix run .#setup
# equivalent: nix run .#setup -- -o prelude.nix
```

That writes a **sidecar** `prelude.nix` and `title.txt` in the current
directory — never `flake.nix` (refused if you pass `-o flake.nix`). The
`.envrc` setup toggle is on by default and writes `use flake` to `.envrc` in
the directory where setup runs. Turn it off to skip that file; an existing
`.envrc` is kept unchanged.

Point `-o` at another config path to relocate the config and wordmark — the
wordmark is always `title.txt` beside the config (e.g. `-o nix/prelude.nix` →
`nix/prelude.nix` and `nix/title.txt`). `.envrc` remains in the setup working
directory. An existing config or wordmark at either output path is replaced.

The generated file is an options template as well as a working module: every
supported Prelude option appears once with a short comment. Wizard choices
are active; everything else is commented out at its default so you can see
defaults and enable knobs without leaving the file. Full prose docs live in
`docs/reference/options.md`.

Import it from your existing flake-parts flake (do not replace `flake.nix`):

```nix
imports = [
  inputs.prelude.flakeModules.default
  ./prelude.nix
];
```

The setup UI renders on stderr; status lines (`wrote …` or `kept existing …`)
and a short import hint go there too. The previous
`nix run .#title -- --wizard` invocation remains available for compatibility.
Enabling the docs viewer also writes a starter `docs/getting-started.md`, but
an existing page is kept untouched.

Commented defaults in the generated file mirror `src/prelude/defaults.nix` so
the config doubles as the option surface.

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
the MOTD. Prelude pads each FIGlet row to the wordmark's bounding width before
moving the whole block, so left/right alignment preserves the original art.
Do not use FIGlet justification to bake padding into the artifact.

`title.style` is unrelated to the FIGlet font. It only controls the fallback
project-name treatment when `title.text` is `null`.

## Rendering controls

The title workflow treats FIGlet rendering as three user-facing choices:

1. **Font** — glyph design, height, and overall geometry. The current pager
   exposes Prelude's 23 bundled fonts.
1. **Spacing** — how adjacent FIGcharacters combine. The agreed chooser shape is
   `Font default`, `Smushed`, `Kerned`, or `Full width`.
1. **Width** — the fixed FIGlet output width used for wrapping. A fixed value is
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
