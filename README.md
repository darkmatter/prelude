<div align="center">
  <img width="584" height="110" alt="image" src="https://github.com/user-attachments/assets/95281deb-ca09-4953-8c5d-9a41c4612ba1" />
  <br/><strong>Make your devshell easy to use and nice to look at</strong><br/>
  <br />
</div>

Prelude is a DX-focused utility that provides a consistent, beautiful interface for your devshell. It's built on the idea that programs should be self-documenting, and that the information about _how_ to use a program (docs) should be available close to _where_ you run that program. Flakes help make this true for dependencies as well - use `docs` to learn about the current project, or `nix run github:org/repo#prelude -- docs` to learn about any prelude-enabled dependency.

With prelude, the only command anyone would need to remember is `nix develop`. At shell entry, they are greeted with a nice MOTD:

<br />
<div align="center">
<img align="center" width="800" src="https://github.com/darkmatter/prelude/raw/main/docs/media/shots/motd.png" />
</div>
<br />

## Quickstart (Setup Wizard)

Prelude ships with a setup wizard that generates a basic configuration for you. It
creates `prelude.nix`, a `title.txt` containing your FIGlet title, and—by default—a
project-root `.envrc` containing `use flake` and the preflight line:

```bash
$ nix run github:darkmatter/prelude -- wizard
wrote title.txt
wrote .envrc
wrote prelude.nix

# or specify a path:
$ nix run github:darkmatter/prelude -- wizard -o nix/prelude.nix
```

![demo](docs/recording.gif)

If you want to customize things further, all other options are also included but
commented out in the generated config, along with documentation and the default value.

Import the module with your config as an argument, for example:

```nix
imports = [
  (inputs.prelude.flakeModules.default ./prelude.nix)
];
```

Your MOTD should contain an overview of what your project is, and clear instructions for
the most common tasks. Ideally, the user should be able to go from clone to running
with just the information shown on your MOTD.

### Command Menu

Of course, not all information is going to fit on an MOTD. The command menu which is
automatically generated for you should contain the rest of the relevant commands for your project:

![menu](docs/media/shots/menu.png)

All tasks that one would need to perform should ideally be accessible here. You can
include very detailed documentation, since pressing tab will expand the detail section
which has plenty of room for prose.

To avoid prelude becoming an extra thing you have to maintain, we include utilities such
as `prelude.lib.fromPkg` which adapt existing packages — and the pattern in
`examples/typescript/` reads `package.json` scripts straight into the command
catalogue — so the menu doesn't drift.

### Docs

![docs](docs/media/shots/docs.png)

Docs are incredibly simple to use, since they just parse markdown in your repo:

```nix
{
  prelude.docs = {
    pages = [
      { text = ./README.md; }
      { text = ./docs/foo.md; }
    ];
  };
}
```

## Usage

```nix
{
  inputs.prelude.url = "github:darkmatter/prelude";

  outputs = { prelude, flake-parts, ... }@inputs:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [ prelude.flakeModules.default ];

      systems = [ "x86_64-linux" "aarch64-darwin" ];

      prelude = {
        theme = "phosphor";
        project = "acme-web";
        motd = {
          enable = true;
        };
        menu.enable = true;
        docs.pages = [
          { text = ./docs/getting-started.md; }
          { text = ./docs/commands.md; }
        ];
      };

      perSystem = { pkgs, config, ... }:
        let
          hello = pkgs.writeShellApplication {
            name = "hello";
            text = ''echo "hello from the devshell"'';
          };
          helloApp = {
            type = "app";
            program = pkgs.lib.getExe hello;
          };
          packages = {
            inherit hello;
            default = hello;
          };
          apps = {
            hello = helloApp;
            default = helloApp;
          };
          checks.hello = hello;
        in
        {
          inherit packages apps checks;

          prelude.commands.hello = prelude.lib.fromPkg packages.hello {
            description = "say hello from this project";
            motd = 1;
          };

          devShells.default = pkgs.mkShell {
            packages = [
              config.packages.prelude-shell
              config.packages.prelude-motd
              config.packages.prelude-menu
              config.packages.prelude-docs
            ];
            shellHook = ''eval "$(prelude-preflight)"'';
          };
        };
    };
}
```

Entering the devshell prints the MOTD; `x` opens the interactive picker and
`docs` opens the configured Markdown pages.

### Package-backed commands

Prelude keeps ordinary flake-parts outputs as the source of truth. Bind
`packages`, `apps`, and `checks` with normal Nix values, inherit them into the
per-system result, and adapt only the packages you want in the command catalogue:

```nix
prelude.commands.dev = prelude.lib.fromPkg packages.dev {
  arguments = [ "serve" ];
  description = "start the development server";
};
```

`fromPkg package extras` resolves `meta.mainProgram` (or an explicit `program`),
shell-escapes `arguments`, and carries the package into the generated menu
closure. The returned value is an ordinary Prelude command, so it composes with
literal command definitions and future collection adapters without introducing
a second output schema.

### Running commands

The public catalogue entrypoint is **`x`**.

```
x                 # open the interactive menu
x dev             # run a command by catalogue key
x d               # …or by its single-key accelerator
x dev --port 80   # extra CLI args skip argument entry
x --list          # print the command table (non-interactive)
```

The interactive picker is a Go/bubbletea TUI (config baked to JSON at build
time, one shared binary per system):

| Keys | Action |
| ------------------- | -------------------------------------------------------- |
| type | filter commands (name, usage, description, group) |
| `↑` `↓` / `⌃n` `⌃p` | move selection |
| `⇥` | list: expand details · args: cycle suggested-value chips |
| `↵` | run selection / append focused chip / submit args |
| `esc` | collapse → clear query → quit; args: back to list |
| backspace (empty) | args: back to the list |

Selecting a command with declared `args` opens argument entry: every argument
is listed with its token, a required/flag/optional tag, description, and
suggested values as chips; a live `$ command` preview updates as you type.
Required arguments are validated on submit. The assembled command is
`exec`'d via `bash -c` — set `prelude.menu.execute = false` to print it
instead. Set `PRELUDE_MENU_DEBUG=<path>` to log TUI events for debugging.

### The docs viewer

Each Markdown file is one navigable page. Declaring pages enables the docs
package automatically; the first `# Heading` becomes the sidebar label.

```nix
prelude.docs = {
  pages = [
    { text = ./docs/getting-started.md; }
    { text = ./docs/commands.md; }
  ];
};
```

Pages support ordinary Markdown such as headings, lists, emphasis, links, and
fenced code blocks. Use digits to jump between pages, `Tab`/`Shift-Tab` to
step through them, `j`/`k` to scroll, and `q` to quit.

## Themes

`prelude.theme` selects a palette: `prelude` (default brand palette),
`phosphor`, `minted` (indigo-black with sage + rose), `amber`, `solarized`,
`nord`, `gruvbox`, and `paper` (light) — most ported from the cli-menu-design
demo — plus `mono` (strict dark grayscale) and `apathy` (ported from
czxtm/apathy-theme: purple-tinted darks, lavender + butterscotch accents).
Page through the current MOTD in every theme with `nix run .#example-themes`.

Tokens: `fg`, `muted`, `dim`, `border`, `accentBorder`, `accent`,
`accent2`, `success`, `warning`, `info`, `error`, `selectionFg`, `bg`,
`surface`, and `secondary`. Semantic status colors are defined independently
for every theme; other values are converted from the design's oklch definitions
with CSS Color 4 gamut mapping.
Override any of them via `prelude.palette`:

```nix
prelude = {
  theme = "nord";
  palette.accent = "#88c0d0";
};
```

Explicit colors on text items always beat the theme; `foreground = null`
(the default) means "use the theme's role color".

### Color depth

The palettes are truecolor hex. The Go renderers use the terminal environment
and output capabilities to select the effective color depth:

| Environment | Result |
| -------------------------------------- | --------------------------- |
| `COLORTERM=truecolor` + 256-color TERM | 24-bit color (`38;2;r;g;b`) |
| no `COLORTERM` (e.g. Apple Terminal) | quantized to 256 (`38;5;n`) |
| `TERM=screen` (default tmux) | no color at all |
| piped / non-tty | no color at all |

If colors look flat, set `prelude.colorProfile = "truecolor"` to force 24-bit
output. `"ansi256"` forces quantization instead; `"auto"` (default) detects the
terminal profile. For tmux, the cleaner global fix is advertising truecolor:
`set -ga terminal-features ',*:RGB'` (or `terminal-overrides ',*:Tc'`).

## Command schema

`prelude.commands` is keyed by the public command name used by `x`. Prelude adds
`x` whenever the menu is enabled, plus `docs` whenever documentation pages
exist.

The first colon derives menu presentation while the complete key remains public:
`go:test` appears as `test` under `go` and runs as `x go:test`. Additional colons
remain in the displayed suffix (`test:unit:watch` → group `test`, name
`unit:watch`). Ungrouped commands appear under `develop`.

Commands feed the interactive menu; only deliberately ungrouped commands become
convenience executables:

| Field | Type | Default | Description |
| ------------- | ----------- | ---------- | ------------------------------------------------------------------------ |
| `description` | str | `""` | One-line description. |
| `exec` | str / null | key suffix | Shell command executed by the menu. |
| `invocation` | str / null | `exec` | Canonical underlying command metadata; exact duplicates fail evaluation. |
| `key` | str / null | `null` | Single-key accelerator (`x <key>` fast path). |
| `usage` | str / null | `null` | Usage form shown in the menu details. |
| `details` | str / null | `null` | Extended description shown before argument entry. |
| `examples` | list of str | `[ ]` | Worked example invocations. |
| `args` | list of arg | `[ ]` | Arguments; presence triggers arg-entry mode in the menu. |

Package-backed commands belong under `perSystem` and use `prelude.lib.fromPkg`:

```nix
prelude.commands."quality:lint" = prelude.lib.fromPkg pkgs.eslint {
  arguments = [ "." ];
  description = "lint the project";
};

prelude.sort.groups = [
  "develop"
  "quality"
];
```

`meta.mainProgram` selects the binary; use `program = "name"` for another
binary. `arguments` are shell-escaped and appended. The package is bundled into
`packages.prelude-menu` automatically. `fromPkg` derives a clean canonical invocation
from the executable basename plus arguments (`go test …`, never a Nix store
path). Grouped entries do not receive wrappers. `prelude.lib.mkCommand` remains
the lower-level constructor for callers that need to choose between `package`,
`executable`, or raw `command` sources.

Argument: `{ token, description ? "", required ? false, boolean ? false, options ? [ ] }`. Tokens starting with `--` insert as `--flag value`;
anything else (e.g. `<remote>`, `name`) inserts positionally; `boolean`
tokens insert as-is when confirmed.

`x` and the menu share one catalogue and execution path. Every menu entry runs
through its complete, globally unique key (`x test`, `x go:test`,
`x test:unit:watch`), so no source or discriminator model is needed. See the
[command conventions](docs/guides/command-conventions.md).

Evaluated packages expose this result for downstream composition and checks:
`config.packages.prelude-menu.commandNames` lists menu selectors,
`commandInvocations` lists canonical shell forms, `commandWrapperNames` lists
only deliberate ungrouped aliases, and `commandWrappers` contains the wrapper
derivations actually built.

### MOTD commands

A command appears on the MOTD Getting Started list when its `motd` field is
set to an integer sort order. Commands with `motd = null` (the default) are
hidden, except `x`: when the menu is enabled it is always listed first
(override with an explicit `motd` order) as the bare command `x` so newcomers
can open the command palette. `docs` stays off this list.
Displayed commands and descriptions are
derived from `prelude.commands`, so they cannot drift from the runnable menu
commands.

```nix
prelude.commands.check = {
  description = "verify the flake";
  exec = "nix flake check";
  motd = 1;  # shown on the MOTD after x, sorts first among project rows
};
```

Rows sort ascending by `motd`, ties broken by command name. Ungrouped rows
render bare — each one is on PATH through a generated wrapper or a first-class
entrypoint — while grouped keys (`go:test`) keep the `x <key>` dispatch form
because only `x` can call them. A dim note under the list points at `x` as the
fallback when another command shadows a bare name
(`prelude.motd.gettingStarted.commandNote`; empty string hides it). When the
menu is enabled, `packages.prelude-motd` carries the menu, runtime packages, and
deliberate ungrouped wrappers; grouped commands remain available through their
canonical executable.

`config.packages.prelude-motd.commandNames` exposes selected command names,
`commandInvocations` exposes the canonical strings rendered by the MOTD, and
`commandWrappers` exposes only the deliberate ungrouped aliases bundled through
the menu. Direct `mkMotd` consumers and configurations without the menu start
with no command catalogue.

### MOTD recipes

`prelude.motd.recipes` describes polished, project-specific workflows separately
from single-command next steps. Recipes should cover setup, build, test, deploy,
and similar work rather than Prelude navigation. They are keyed by name and sort by `order`,
then key; `title` defaults to the key. Prefer structured `steps`.

```nix
prelude.motd.recipes.clean-local-stack = {
  title = "spin up a clean local stack";
  steps = [
    { comment = "start postgres + redis first"; }
    { command = "just db:up"; }
    { command = "just db:migrate && just db:seed"; }
    { command = "just dev"; }
  ];
};
```

Each step is either a `{ command }` (bold shell line) or a `{ comment }`
(dim `#` caption). Legacy `lines` (`#…` / free-form commands) still normalize
into steps at the Nix boundary.

### Generated FIGlet title

See the [title rendering guide](docs/guides/title-rendering.md) for the complete
interactive, stdout, recipe, and MOTD integration workflow. For a complete new
project configuration, run the wizard:

```console
nix run github:darkmatter/prelude -- wizard
```

That writes `prelude.nix` and a sibling `title.txt` (override the config path
with `-o`; the title always lands next to it). The generated config is a full
options template: wizard choices are active, and every other supported option
is present as a commented default so the file doubles as documentation. See
[options reference](docs/reference/options.md) for the prose docs.

Prelude ships 23 selectable fonts: `3d-ascii`, `ansi-shadow`, `calvin-s`,
`computer`, `cricket`, `cybermedium`, `dos-rebel`, `dr-pepper`, `fender`,
`georgia11`, `halfiwi`, `kban`, `kompaktblk`, `larry3d`, `mini`, `roman`,
`slant`, `small-slant`, `speed`, `standard`, `thin`, `tubes-regular`, and
`univers`. Open the interactive title chooser:

```console
nix run .#prelude -- title
```

The first screen is prefilled with the current directory name. Continue to a
one-style-per-page live preview, use arrows or `j`/`k` to move through the
bundled FIGlet fonts, and press enter to choose. The selected title is rendered
to stdout; pass `-o` to write it directly:

```console
nix run .#prelude -- title -o title.txt
```

An explicit recipe can prefill both text and font, but is never discovered or
rewritten implicitly:

```nix
# title.nix
{
  text = "acme-web";
  font = "calvin-s";
}
```

```console
nix run .#prelude -- title --recipe title.nix
nix run .#prelude -- title --generate --recipe title.nix -o title.txt
```

Without `--recipe`, non-interactive `--generate` renders the current directory
name with the default font.

The original all-font stream remains available when a printable overview is
more useful than the chooser:

```console
nix run .#prelude -- title-previews "acme-web"
```

Redirect stdout or check in an explicitly written file, then point the MOTD at it:

```nix
prelude.motd.title = {
  text = ./title.txt;
  align = "center"; # left, center, or right within the card
  style = "spine";  # project-name fallback when text is null
};
```

`--output` is the long form of `-o`; `--interactive` forces the chooser when
terminal detection is unavailable. Without either output flag, rendered FIGlet
text is the only stdout content. FIGlet is only used by the renderer; the MOTD
embeds and displays the resulting text. `motd.title.style` remains the
project-name fallback used only when `motd.title.text` is null.

## motd options (`prelude.motd.*`)

`description` is a styled text item (`{ text, foreground, background, bold, italic, faint }`; null foreground uses the theme fg role).

See the [options reference](docs/reference/options.md) for the complete list of `prelude.motd.*` fields, types, and defaults.

Navigation shortcuts are internal: enabled MOTD, menu, and docs components add
`[?] motd`, `[x] menu`, and `[d] docs` respectively. Their aliases are installed
on `PATH`; consumers cannot hide a shortcut while its component is enabled.

## menu options (`prelude.menu.*`)

The filter and argument-entry prompt uses a blinking terminal bar cursor.

See the [options reference](docs/reference/options.md) for the complete list of `prelude.menu.*` fields, types, and defaults.

Group order is configured with `prelude.sort.groups` (default:
`[ "develop" ]`). Prelude's own navigation group remains first.

> MOTD guidance is authored independently with exact runnable `commands` and
> multi-step `recipes`; command groups never render in the MOTD.

## prompt options (`prelude.prompt.*`)

A [starship](https://starship.rs/) config themed from the active palette.
`packages.prelude-prompt` is the generated `starship.toml`. The default layout starts
with two blank lines for breathing room and a marker prompt. The existing
Powerline is the generated Starship `right_format`; in Bash, Starship's native
ble.sh integration renders it and Prelude moves that rendered value into
ble.sh's bottom status line:

```
❯
░▒▓ prelude  …/prelude   main  ✘»+⇡   ···  [?] motd  [x] menu  [d] docs
```

Status: ramp + project pill on `secondary`, then continuous Powerline
transitions through a `bg` directory segment, an inverted `fg` branch
segment, and a `surface` status segment before returning to the terminal
background. Command duration and shortcuts remain transparent; shortcuts are
right-aligned via `$fill`. Other shells that support Starship `right_format`
retain it as a native right prompt. Marker: bold `success`, `error` on failure.
Styles reference palette tokens by name — the config carries
`palettes.prelude` mapping `bg`, `surface`, `secondary`, `fg`, `muted`, `dim`,
`accent`, `accent2`, `success`, `warning`, `info`, `error`, … to the resolved
theme, so `settings` overrides can use the same names (e.g.
`style = "fg:success"`).

See the [options reference](docs/reference/options.md) for the complete list of `prelude.prompt.*` fields, types, and defaults.

Add the shell core and each enabled component package to your devshell:

```nix
devShells.default = pkgs.mkShell {
  packages = [
    config.packages.prelude-shell
    config.packages.prelude-motd
    config.packages.prelude-menu
    config.packages.prelude-docs
  ];
};
```

`packages.prelude-shell`'s setup-hook exports `STARSHIP_CONFIG` to the generated
`starship.toml` path. Because the export lives in the setup-hook (not
`shellHook`), it is picked up by **direnv `use flake`** — direnv re-emits
setup-hook exports on every reload and unloads them on exit, so the prompt
re-themes on entry and reverts on leave. `shellHook` only fires under
`nix develop`, so putting the export there alone would lose the prompt under
direnv.

The setup-hook also exports `PRELUDE_INIT`: the path to an idempotent init file
that renders the MOTD and — when the prompt is enabled — wires Starship
(`starship init`), ble.sh, completion, and the status row into the current
shell. It guards on interactivity, so it stays inert during non-interactive
evaluation. Every project exports it, including `prelude.prompt.enable = false`
ones: a MOTD-only build names no Starship or ble.sh paths, so those consumers
carry none of that closure.

### Activation

Activation comes from files in the repository, so onboarding a developer never
requires touching their shell rc. One command covers every loader:
`prelude-preflight` prints shell code, and the consumer evals it.

```bash
eval "$(prelude-preflight)"
```

The printed code branches on the shell it is evaluated in, not on the loader that
produced it, and it names no store paths — the MOTD binary and its
once-per-environment key belong to `$PRELUDE_INIT` alone, so the banner has
exactly one owner and cannot render twice.

| shell it lands in | what the snippet does |
|---|---|
| interactive | sources `$PRELUDE_INIT` (prompt, ble.sh, completion, MOTD) |
| non-interactive, `DIRENV_IN_ENVRC` set | asks the init to render the MOTD and export its once-marker |
| non-interactive, otherwise | nothing |

That third row is deliberate. `DIRENV_IN_ENVRC` is set only while direnv
evaluates `.envrc` — the one non-interactive context whose exports are replayed
into a terminal-attached shell. lorri's `shellHook` is also non-interactive, but
it runs inside the Nix builder: a banner there goes to a build log nobody reads,
and an exported marker would silence the banner in the shell that *is* attached
to a terminal.

**`nix develop`** runs `shellHook`, so put the line there:

```nix
shellHook = ''eval "$(prelude-preflight)"'';
```

With the prompt enabled Prelude also appends the init to `shellHook` itself, and
the init is idempotent, so the two paths cannot double up.

**direnv** runs `.envrc`. `prelude wizard` writes exactly this:

```bash
use flake
if has prelude-preflight; then
  eval "$(prelude-preflight)"
fi
```

**lorri** *does* run `shellHook` — but inside the Nix builder: non-interactively,
with `$PWD` set to the build directory and output going to the build log
([lorri#159](https://github.com/nix-community/lorri/issues/159)). Variables it
exports are captured and replayed into your shell; anything needing a terminal,
such as the MOTD, is not. So the banner has to come from a context lorri does not
own — the same `.envrc`, via the direnv adapter:

```bash
eval "$(lorri export direnv-adapter)"
if has prelude-preflight; then
  eval "$(prelude-preflight)"
fi
```

Running lorri without direnv leaves `shellHook` as the only activation path, and
that path cannot show a banner from the builder; use `prelude hook` in your rc
file for that setup.

Prelude's setup-hook exports — `PRELUDE_MENU_CONFIG`, `PRELUDE_INIT`, and
`STARSHIP_CONFIG` — survive lorri's environment capture because they are
variables. A shell *function* such as `prelude-init` does not; it remains
available inside `nix develop` for manual re-runs.

#### `shellHook` is itself an exported variable

`shellHook` is captured the same way, by both direnv and lorri, and
`nix print-dev-env --json` exposes it at `variables.shellHook.value`. A shell
that has the environment applied can therefore run the project's hook directly,
without anything from Prelude:

```bash
eval "$shellHook"
```

Under lorri that renders the MOTD, defines the devshell's functions, and applies
the prompt. For a shell with no loader at all, `. <(nix print-dev-env)` does the
whole job — the script it emits ends with `eval "${shellHook:-}"`, so it runs
the hook itself.

Prefer this when you control the devshell's `shellHook`. Prefer `$PRELUDE_INIT`
when you do not: `eval "$shellHook"` re-runs the project's *entire* hook on every
invocation, which is only safe if that hook is idempotent — the reason lorri
confines its own run to the builder instead of replaying it in your shell
([lorri#159](https://github.com/nix-community/lorri/issues/159)). Prelude's own
init is idempotent by construction; an arbitrary consumer's hook may not be.

#### Optional: `prelude hook`

If you already run lorri's native prompt hook (`eval "$(lorri hook zsh)"` in
your rc file), `prelude hook` is the matching line that renders the MOTD on
directory entry without `.envrc`. Append it from inside a project — do not
`eval` it, because `prelude` is a project package and is not on `PATH` when your
rc file runs:

```bash
prelude hook zsh >> ~/.zshrc     # or: prelude hook bash >> ~/.bashrc
```

The snippet is static and project-independent — it reads only `$PRELUDE_INIT` —
so one line covers every Prelude project. With no argument the dialect comes
from `$SHELL`. It coexists with `eval "$(prelude-preflight)"`: under direnv the
init exports its own once-marker from `.envrc`, so whichever fires first renders
the banner and the other stays quiet.

> **Never `export -f` in a devshell `shellHook`.** Bash stores an exported
> function as the environment variable `BASH_FUNC_<name>%%`. Loaders that
> capture the environment replay that name into whatever shell the user runs,
> and zsh rejects `%` in a variable name, erroring on every prompt. Guarding on
> `BASH_VERSION` does not help: that guard is evaluated inside the Nix builder,
> which is always Bash. Put shell-specific setup in `prelude hook`, which runs
> where `$SHELL` is meaningful.

Users who already run Starship globally need nothing else: `starship init` in
their existing shell re-resolves `$STARSHIP_CONFIG` on every prompt render, so
the export alone re-themes their prompt. The `prelude-init` call is only needed
to install Starship/ble.sh/completion into a shell that doesn't already have them.

`packages.prelude-shell` contains the PATH-resolving namespaced CLI, activation
files, and — when enabled — Starship, ble.sh, and bash-completion. MOTD, menu,
and docs remain separate packages, so consumers add exactly the enabled
components without installing the self-contained app, wizard, title
generators, repository examples, or preview utilities. `packages.prelude`
backs `apps.prelude` and the upstream default package; do not add it to a
consumer devshell. Its setup hook sources a small, idempotent `prelude-init`
entrypoint in the current interactive shell.
The component packages do not retain one another. Built-in shortcut wrappers
(`?`, `x`, and `d`) resolve cross-component targets through `PATH`, so add every
enabled target component rather than relying on one package to pull in another.
That entrypoint composes checked-in lifecycle, completion, and status modules
with a Nix-generated command catalogue. Starship's documented ble.sh PRECMD
hook captures command state and renders `right_format`; Prelude's following
hook moves that already-rendered value into the status line. There is no extra
Starship process per keystroke, nested shell, or reconstructed rcfile.

The resolved Prelude palette is also compiled into a native ble.sh
`contrib/scheme/prelude.bash` color scheme. It maps ble.sh's editing, syntax,
command, filename, variable, validation, and completion faces to the same
semantic theme roles used by the MOTD, menu, docs, status line, and Starship.

The same palette is compiled into a vim-airline theme at
`contrib/airline/prelude.bash`, discoverable through the runtime's
`import_path`, which Prelude seeds before ble.sh loads so `~/.blerc` can
select it too. With ble.sh's `lib/vim-airline` status line enabled, `bleopt vim_airline_theme=prelude` paints the mode, git, and path segments from the
active theme: `accent` for normal mode, `info` for insert, `error` for
replace, `warning` for visual, and `accent2` for command-line, over
`secondary`/`surface` chrome. Importing `lib/vim-airline` yields Prelude's
Starship status row to the airline bar.

**Tab** remains owned by ble.sh's native completion menu. Prelude contributes
catalogue-aware candidates and descriptions for `x` commands and their
arguments without replacing ble.sh's navigation or rendering. The status line
contains only Starship's Powerline—no duplicate command browser or hints.

When `prelude.prompt.configFile` is set, that file remains verbatim and fully
user-owned; Prelude does not move its right prompt into the Bash status line.

Starship re-resolves `$STARSHIP_CONFIG` on every prompt render, and direnv
propagates env vars (only `PS1` itself is stripped). Entering the project
therefore re-themes the prompt and leaving it reverts to the user's own config.
`prelude-init` is inert during non-interactive direnv evaluation (it guards on
`$-` containing `i`); it activates only when sourced from an interactive shell.

## Without flake-parts

```nix
prelude.lib.mkMotd
  { inherit (pkgs) lib writeText; buildGoModule = pkgs.buildGo126Module; }
  {
    theme = "gruvbox";
    project = "acme-web";
    header.tagline.text = "everything you need to build, test & ship";
    commandCatalog.dev = {
      description = "start local development";
      exec = "pnpm dev";
      motd = 1;
    };
  }
```

`lib.mkMotd` and `lib.mkMenu` take the component-specific configs the module
assembles; `lib.themes` exposes the palettes. Pass
`buildGoModule = pkgs.buildGo126Module` with the other component-specific
builder dependencies. The overlay (`overlays.default`) provides
`pkgs.prelude.mkMotd` / `mkMenu` with the pinned builder and other dependencies
pre-applied.

## Demos

This flake dogfoods like a consumer: it imports [`prelude.nix`](prelude.nix)
next to `flakeModules.default`, with docs pages under [`docs/`](docs/).
`nix develop` greets you with Prelude's own MOTD, and `x` drives the project
from inside the shell. The module has one app, `prelude`; this repository also
keeps `examples` and `previews` as development-only apps. Neither is contributed
by the module or installed into consumer devshells.

```sh
nix develop                   # our own motd + x, built by our own module
nix run . -- motd             # this repo's welcome banner
nix run . -- menu             # this repo's command catalogue
nix run .#previews            # build the render checks and show their output
nix run .#previews -- motd-renders   # …or just specific checks
nix run .#example-default     # MOTD from stock wizard presets
# nix run .#example-default -- --config   # print the generated prelude.nix
nix run .#example-motd        # acme-web welcome banner demo
nix run .#example-menu        # acme-web command menu demo (arg entry)
nix run .#examples            # render every demo in sequence
nix run .#example-themes      # page through the current motd in every theme
nix run .#example-minimal     # standalone banner + styled description (the old card look)
nix run .#example-surface     # background fill, status chips, thick border
```

## Generated documentation

The [terminal showcase](docs/generated/showcases.md) is rendered from the real
example packages with [VHS](https://github.com/charmbracelet/vhs). It includes
animated MOTD/menu recordings, still PNGs, and the exact Nix configuration used
for each recording. The [options reference](docs/reference/options.md) is built
from the module declarations with `nixosOptionsDoc`.

```sh
nix run .#docs-sync           # regenerate deterministic Markdown
nix run .#docs-record         # record stale GIF/PNG media, then sync Markdown
```

`nix flake check` compares committed Markdown and media fingerprints with their
generator inputs. CI runs `docs-record` after every writable branch push and
auto-commits changes under `docs/` when regeneration is needed.

## Layout

```
flake.nix              # canonical outputs + flakeModules.default + templates.default
nix/internal/prelude.nix # dogfood config (same shape as wizard output)
nix/*.nix              # per-system composition, demos, checks, apps, overlay, lib
nix/docs-automation.nix # VHS tapes, fingerprints, docs apps + freshness checks
nix/*-demo-builder.nix  # final demo packages shared by apps and recordings
examples/reference/    # standalone downstream consumer example
examples/typescript/   # package.json-script menu pattern
templates/default/     # `nix flake init -t github:darkmatter/prelude#default` scaffold
docs/                  # docs-viewer pages, guides, generated references, media
src/cmd/               # Go entrypoints (motd, menu, docs, title)
src/internal/          # Go renderers: motd (Lip Gloss), menu (Bubble Tea),
                       #   docs, wizard (config wizard + title chooser)
src/pkg/               # shared Go packages (manual, shared, ui)
src/prelude/
  themes.nix           # palettes (oklch → hex, CSS gamut-mapped)
  defaults.nix         # shared defaults for module + direct consumers
  lib.nix              # shared palette/config normalization helpers
  option-types.nix     # option types/builders shared by options/*.nix
  options/             # prelude.* option declarations
    shared.nix         #   theme, palette, colorProfile, project, groups
    motd.nix menu.nix docs.nix prompt.nix
  motd.nix             # mkMotd — Go renderer + normalized JSON config
  menu.nix             # mkMenu — Bubble Tea renderer + normalized JSON config
  module.nix           # flake-parts module: imports options/, wires perSystem
                       #   (wrapped for importApply — consume it via
                       #    flakeModules.default, not by importing the file)
  examples.nix         # runnable demos (nix run .#examples)
```
