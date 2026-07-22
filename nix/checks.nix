# Flake checks. Checks whose $out is a rendered preview use the `-render(s)`
# suffix — the previews utility (previews.nix) discovers them by name.
{ pkgs
, lib
, config
, demos
, docsAutomation
, previews
, ...
}:
let
  preludeLib = import ./lib.nix { inherit lib; };
  internalLib = import ../src/prelude/lib.nix { inherit lib; };

  # The command-providing packages of the dogfood devshell (shell.nix).
  devshellCommandPackages = [
    config.packages.prelude
    pkgs.nix
    docsAutomation.sync
    docsAutomation.record
    previews
  ]
  # Consume the wrappers exposed by the evaluated Prelude package, just as a
  # downstream module can, instead of rebuilding knowledge from source config.
  ++ config.packages.menu.commandWrappers;

  # Assert that every advertised canonical invocation starts with an executable
  # provided by the devshell. Group selectors (`go:test`) are menu identity and
  # intentionally do not exist on PATH; their invocation (`go test`) does.
  mkRunnableCheck =
    checkName: surface: invocations:
    let
      executableForLine = line:
        builtins.head (lib.filter (token: token != "") (lib.splitString " " line));
      invocationExecutables = invocation:
        map executableForLine (lib.filter (line: line != "") (lib.splitString "\n" invocation));
      executables = lib.unique (lib.concatMap invocationExecutables invocations);
    in
    pkgs.runCommand checkName { nativeBuildInputs = devshellCommandPackages; } ''
      for cmd in ${lib.concatMapStringsSep " " lib.escapeShellArg executables}; do
        command -v "$cmd" >/dev/null 2>&1 || {
          echo "${surface} advertises canonical executable '$cmd' but no devshell package provides it" >&2
          exit 1
        }
      done
      touch "$out"
    '';
in
{
  # Building the module-produced packages runs shellcheck / go vet on the
  # generated artifacts.
  motd-default = config.packages.motd;
  title-default = config.packages.title;
  menu-default = config.packages.menu;
  prelude-default = pkgs.runCommand "prelude-default"
    { nativeBuildInputs = [ config.packages.prelude ]; }
    ''
      command -v motd >/dev/null
      command -v menu >/dev/null
      command -v docs >/dev/null
      command -v starship >/dev/null
      command -v blesh-share >/dev/null
      test -f ${config.packages.prelude}/share/blesh/ble.sh
      test -f ${config.packages.prelude}/nix-support/setup-hook
      grep -Fq 'source ${pkgs.blesh}/share/blesh/ble.sh' ${config.packages.prelude}/nix-support/setup-hook
      grep -Fq '${lib.getExe pkgs.starship} init bash' ${config.packages.prelude}/nix-support/setup-hook
      touch "$out"
    '';

  title-previews = pkgs.runCommand "title-previews" { } ''
    ${lib.getExe config.packages.title-previews} "choose me" > "$out"
    test "$(grep -c '^===== .* =====$' "$out")" -eq 23
    grep -q '^===== 3d-ascii =====$' "$out"
    grep -q '^===== calvin-s =====$' "$out"
    grep -q '^===== roman =====$' "$out"
    grep -q '^===== univers =====$' "$out"
    test "$(wc -l < "$out")" -gt 50
  '';

  title-generates =
    let
      # JSON, not Nix: nix-instantiate cannot write to /nix/var/nix/profiles
      # inside the build sandbox, so the title tool's Nix-recipe path is
      # unusable here. The tool accepts JSON recipes directly.
      recipe = pkgs.writeText "title.json" ''{"text":"prelude","font":"calvin-s"}'';
    in
    pkgs.runCommand "title-generates" { } ''
      ${lib.getExe config.packages.title} --recipe ${recipe} --output "$out"
      grep -q '┌─┐' "$out"
    '';

  # fromPkg is a small adapter over mkCommand: package selection is positional,
  # while program/arguments and presentation metadata stay composable extras.
  from-pkg =
    let
      command = preludeLib.fromPkg pkgs.nixfmt {
        arguments = [ "." ];
        description = "format Nix sources";
        key = "f";
      };
    in
    assert command.description == "format Nix sources";
    assert command.key == "f";
    assert command.invocation == "nixfmt .";
    assert lib.hasPrefix (lib.getExe pkgs.nixfmt) command.exec;
    assert command.runtimePackages == [ pkgs.nixfmt ];
    pkgs.runCommand "from-pkg" { } "touch $out";

  # Prelude owns navigation commands. `menu` is always advertised on the MOTD
  # (bare, no `x` prefix); docs stays menu-only. Project Getting Started rows
  # remain focused on explicitly selected lifecycle commands.
  prelude-command-defaults =
    assert lib.all (name: lib.elem name config.packages.menu.commandNames) [
      "menu"
      "docs"
    ];
    assert lib.elem "menu" config.packages.motd.commandNames;
    assert lib.elem "menu" config.packages.motd.commandInvocations;
    assert !lib.elem "x menu" config.packages.motd.commandInvocations;
    assert !lib.elem "docs" config.packages.motd.commandNames;
    pkgs.runCommand "prelude-command-defaults" { nativeBuildInputs = [ config.packages.menu ]; } ''
      command -v x >/dev/null
      command -v menu >/dev/null
      command -v docs >/dev/null
      ! command -v help >/dev/null
      touch "$out"
    '';

  # Complete command keys stay public while the first colon derives group/label
  # presentation. Prelude stays first and configured groups follow in order.
  command-ordering =
    let
      plib = import ../src/prelude/lib.nix { inherit lib; };
      evaluated = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          {
            prelude.sort.groups = [
              "docs"
              "develop"
              "demos"
            ];
            prelude.commands = {
              menu = { };
              dev = { };
              "docs:sync" = { };
              "docs:record" = { };
              "demos:menu".exec = "nix run .#example-menu";
            };
          }
          {
            prelude.commands."docs:record".description = "merged";
          }
        ];
      };
      normalized = plib.normalizeCommandGroups evaluated.config.prelude.sort.groups evaluated.config.prelude.commands;
      actual = map
        (group: {
          inherit (group) title;
          commands = map
            (command: {
              inherit (command)
                name
                label
                run
                ;
            })
            group.tasks;
        })
        normalized;
      expected = [
        {
          title = "prelude";
          commands = [
            {
              name = "menu";
              label = "menu";
              run = "menu";
            }
          ];
        }
        {
          title = "docs";
          commands = [
            {
              name = "docs:record";
              label = "record";
              run = "record";
            }
            {
              name = "docs:sync";
              label = "sync";
              run = "sync";
            }
          ];
        }
        {
          title = "develop";
          commands = [
            {
              name = "dev";
              label = "dev";
              run = "dev";
            }
          ];
        }
        {
          title = "demos";
          commands = [
            {
              name = "demos:menu";
              label = "menu";
              run = "nix run .#example-menu";
            }
          ];
        }
      ];
      docsGroup = builtins.elemAt normalized 1;
    in
    assert !(evaluated.options ? sort);
    assert evaluated.options.prelude.sort ? groups;
    assert actual == expected;
    assert (builtins.head docsGroup.tasks).description == "merged";
    pkgs.runCommand "command-ordering" { } "touch $out";

  # Header options share one nested namespace for module and direct consumers.
  motd-header-options =
    let
      evaluated = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          ../src/prelude/options/motd.nix
          {
            prelude.motd = {
              title = {
                text = pkgs.writeText "test-title.txt" "TEST TITLE\n";
                align = "center";
                style = "bracketed";
              };
              padding = {
                x = 2;
                top = 2;
              };
              windowBackground = {
                blend = 0.15;
              };
              header = {
                tagline = {
                  text = "test-tagline";
                  subtitle = "test-subtitle";
                  layout = "inline";
                  align = "center";
                };
                statusHint.layout = "inline";
                status.shell = {
                  order = 100;
                  label = "nix develop";
                  status = "ready";
                };
                status.cache = {
                  order = 200;
                  label = "cache";
                  check = "false";
                  fail = "stale";
                  failLevel = "warning";
                };
              };
              links = [
                {
                  label = "Prelude on GitHub";
                  url = "https://github.com/darkmatter/prelude";
                }
              ];
            };
          }
        ];
      };
      title = evaluated.config.prelude.motd.title;
      header = evaluated.config.prelude.motd.header;
      padding = evaluated.config.prelude.motd.padding;
      links = evaluated.config.prelude.motd.links;
      windowBackground = evaluated.config.prelude.motd.windowBackground;
      shellStatus = header.status.shell;
      exposesShortcutOption = evaluated.options.prelude.motd ? shortcuts;
    in
    assert builtins.readFile title.text == "TEST TITLE\n";
    assert title.align == "center";
    assert title.style == "bracketed";
    assert header.tagline.text == "test-tagline";
    assert header.tagline.subtitle == "test-subtitle";
    assert header.tagline.layout == "inline";
    assert header.tagline.align == "center";
    assert header.statusHint.layout == "inline";
    assert shellStatus.label == "nix develop";
    assert shellStatus.status == "ready";
    assert shellStatus.failLevel == "error";
    assert header.status.cache.failLevel == "warning";
    assert header.status.cache.async;
    assert
    links == [
      {
        label = "Prelude on GitHub";
        url = "https://github.com/darkmatter/prelude";
      }
    ];
    assert padding.x == 2;
    assert padding.y == 0;
    assert padding.top == 2;
    assert padding.left == null;
    assert padding.right == null;
    assert windowBackground.blend == 0.15;
    assert !exposesShortcutOption;

    pkgs.runCommand "motd-header-options" { } "touch $out";

  # Core navigation shortcuts are synthesized from component availability;
  # consumers cannot remove or advertise commands that are disabled.
  component-shortcuts =
    let
      all = internalLib.componentShortcuts {
        motd = true;
        menu = true;
        docs = true;
      };
      menuOnly = internalLib.componentShortcuts {
        motd = false;
        menu = true;
        docs = false;
      };
    in
    assert
    all == [
      {
        command = "motd";
        alias = "?";
      }
      {
        command = "menu";
        alias = "m";
      }
      {
        command = "docs";
        alias = "d";
      }
    ];
    assert
    menuOnly == [
      {
        command = "menu";
        alias = "m";
      }
    ];
    pkgs.runCommand "component-shortcuts" { } "touch $out";

  # prelude.lib.mdSplit → { title = "README"; text; children = [preamble, H2…] }.
  # docs.nix renames first child to project + rootReadme when text matches.
  mdSplit-pages =
    let
      sample = ''
        <div align="center">badge</div>

        # Guide

        intro before any H2

        ## First section

        first body

        ```md
        ## not a real heading
        ```

        ## Second section

        second body

        ## motd options (`prelude.motd.*`)

        punct body
      '';
      node = preludeLib.mdSplit sample;
      children = node.children;
      titles = map (l: l.title) children;
      bodies = map (l: builtins.readFile l.text) children;
      fromPath = preludeLib.mdSplit ../README.md;
      # H1 immediately followed by H2 — preamble body empty but still index 0.
      thin = preludeLib.mdSplit ''
        # Thin

        ## Alpha

        alpha body
      '';
      thinTitles = map (l: l.title) thin.children;
    in
    assert node.title == "README";
    assert node ? text; # always set (toFile for string src)
    assert builtins.length children == 4;
    # Pure mdSplit keeps H1-derived preamble title; docs.nix renames to project.
    assert titles == [
      "Guide"
      "First section"
      "Second section"
      "motd options (`prelude.motd.*`)"
    ];
    assert lib.hasInfix "badge" (builtins.elemAt bodies 0);
    assert lib.hasInfix "intro before any H2" (builtins.elemAt bodies 0);
    assert !(lib.hasInfix "# Guide" (builtins.elemAt bodies 0));
    assert lib.hasInfix "first body" (builtins.elemAt bodies 1);
    assert !(lib.hasInfix "## First section" (builtins.elemAt bodies 1));
    assert lib.hasInfix "## not a real heading" (builtins.elemAt bodies 1);
    assert lib.hasInfix "second body" (builtins.elemAt bodies 2);
    assert lib.hasInfix "punct body" (builtins.elemAt bodies 3);
    assert fromPath.title == "README";
    assert fromPath ? text;
    assert builtins.length fromPath.children > 1;
    # Empty preamble still occupies children[0]; Alpha is not promoted.
    assert thin.title == "README";
    assert thinTitles == [
      "Thin"
      "Alpha"
    ];
    assert lib.hasInfix "alpha body" (builtins.readFile (builtins.elemAt thin.children 1).text);
    pkgs.runCommand "mdSplit-pages" { } "touch $out";

  # docs.nix nav: README → <project> → first original H2 … + FIGlet flag.
  mdSplit-readme-nav =
    let
      docsPkg = import ../src/prelude/docs.nix
        {
          inherit (pkgs)
            lib
            writeText
            buildGoModule
            runCommand
            nixosOptionsDoc
            figlet
            ;
        }
        {
          theme = "phosphor";
          colorProfile = "auto";
          project = "myproj";
          rootReadme = ../README.md;
          pages = [
            (preludeLib.mdSplit ../README.md)
          ];
          nixosOptions = {
            options = { };
          };
        };
      cfg = builtins.fromJSON (builtins.readFile "${docsPkg.passthru.config}/config.json");
      root = builtins.head cfg.nav;
      kids = root.children;
      first = builtins.head kids;
      second = builtins.elemAt kids 1;
    in
    assert root.kind == "group";
    assert root.title == "README";
    assert first.kind == "leaf";
    assert first.title == "myproj";
    assert first.rootReadme == true;
    assert second.kind == "leaf";
    assert second.title == "Quickstart (Setup Wizard)";
    assert (cfg.heroFile or "") != "";
    pkgs.runCommand "mdSplit-readme-nav" { } "touch $out";







  # Dogfood surfaces must render every enable-derived navigation shortcut.
  motd-renders = pkgs.runCommand "motd-renders" { } ''
    NO_COLOR=1 ${lib.getExe config.packages.motd} > "$out"
    grep -F '[?] motd' "$out"
    grep -F '[m] menu' "$out"
    grep -F '[d] docs' "$out"
  '';

  prompt-renders-shortcuts = pkgs.runCommand "prompt-renders-shortcuts" { } ''
    grep -F '[?](bold fg:accent2)' ${config.packages.prompt}
    grep -F '[ motd](fg:muted)' ${config.packages.prompt}
    grep -F '[m](bold fg:accent2)' ${config.packages.prompt}
    grep -F '[ menu](fg:muted)' ${config.packages.prompt}
    grep -F '[d](bold fg:accent2)' ${config.packages.prompt}
    grep -F '[ docs](fg:muted)' ${config.packages.prompt}
    touch "$out"
  '';

  # The MOTD advertises x aliases for project commands (plus bare `menu`);
  # the menu retains canonical underlying invocations for execution and
  # diagnostics.
  motd-commands-runnable =
    mkRunnableCheck "motd-commands-runnable" "motd"
      config.packages.motd.commandInvocations;

  menu-commands-runnable =
    mkRunnableCheck "menu-commands-runnable" "menu"
      config.packages.menu.commandInvocations;

  # Built-in navigation aliases must resolve on the same PATH as their labels.
  motd-shortcuts-runnable =
    assert
    config.packages.motd.shortcutAliases == [
      "?"
      "m"
      "d"
    ];
    mkRunnableCheck "motd-shortcuts-runnable" "built-in shortcuts" config.packages.motd.shortcutAliases;

  titles-command-renders =
    pkgs.runCommand "titles-command-renders"
      {
        nativeBuildInputs = [ config.packages.motd ];
      }
      ''
        prelude-title-previews prelude > "$out"
        test "$(grep -c '^===== .* =====$' "$out")" -eq 23
        grep -q '^===== 3d-ascii =====$' "$out"
        grep -q '^===== calvin-s =====$' "$out"
        test "$(wc -l < "$out")" -gt 50
      '';

  # Package-backed ungrouped aliases carry their runtime package and wrapper.
  package-command-bundled =
    assert lib.elem pkgs.nixfmt config.packages.menu.commandRuntimePackages;
    pkgs.runCommand "package-command-bundled"
      {
        nativeBuildInputs = [ config.packages.menu ];
      }
      ''
        command -v nixfmt >/dev/null
        command -v fmt >/dev/null
        touch "$out"
      '';

  colon-command-names-preserved =
    let
      internalPreludeLib = import ../src/prelude/lib.nix { inherit lib; };
      imported = internalPreludeLib.normalizeCommand "test:unit" {
        exec = "npm run test:unit";
      };
    in
    assert imported.name == "test:unit";
    assert imported.group == "test";
    assert imported.label == "unit";
    pkgs.runCommand "colon-command-names-preserved" { } "touch $out";

  duplicate-canonical-invocations-rejected =
    let
      internalPreludeLib = import ../src/prelude/lib.nix { inherit lib; };
      attempted = builtins.tryEval (
        builtins.deepSeq
          (internalPreludeLib.normalizeCommandEntries {
            "go:test" = {
              exec = "go test";
            };
            "quality:test" = {
              exec = "go test";
            };
          })
          true
      );
    in
    assert !attempted.success;
    pkgs.runCommand "duplicate-canonical-invocations-rejected" { } "touch $out";

  # Group prefixes are parsed into menu metadata and never become PATH names.
  # Canonical package invocations remain the native CLI syntax.
  grouped-commands-use-canonical-invocations =
    assert lib.elem "go:vet" config.packages.menu.commandNames;
    assert lib.elem "go vet -C src ./..." config.packages.menu.commandInvocations;
    assert lib.elem "x go:vet" config.packages.menu.xInvocations;
    assert !lib.elem "go:vet" config.packages.menu.commandWrapperNames;
    assert !lib.elem "go-vet" config.packages.menu.commandWrapperNames;
    pkgs.runCommand "grouped-commands-use-canonical-invocations"
      { nativeBuildInputs = [ config.packages.menu ]; }
      ''
        command -v go >/dev/null
        ! command -v go:vet >/dev/null
        ! command -v go-vet >/dev/null
        touch "$out"
      '';

  # Docs options accept nested nav nodes and full nixosOptionsDoc arg pass-through.
  docs-options =
    let
      tiny = lib.evalModules {
        modules = [
          {
            options.demo = lib.mkOption {
              type = lib.types.str;
              default = "x";
              description = "demo option";
            };
          }
        ];
      };
      evaluated = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          ../src/prelude/options/docs.nix
          {
            prelude.docs.pages = [
              { text = ../docs/welcome.md; }
              {
                title = "Guides";
                children = [
                  { text = ../docs/commands.md; }
                ];
              }
              {
                generate = "nixosOptions";
                title = "Options";
              }
            ];
            # Full nixosOptionsDoc argument set, including a non-transform field.
            prelude.docs.nixosOptions = {
              inherit (tiny) options;
              documentType = "none";
              warningsAreErrors = false;
              revision = "check-rev";
            };
          }
        ];
      };
      pages = evaluated.config.prelude.docs.pages;
      nixos = evaluated.config.prelude.docs.nixosOptions;
      # Exercise pass-through: builder must accept non-transform args unchanged.
      docsPkg = import ../src/prelude/docs.nix
        {
          inherit (pkgs)
            lib
            writeText
            buildGoModule
            runCommand
            nixosOptionsDoc
            figlet
            ;
        }
        {
          theme = "phosphor";
          colorProfile = "auto";
          project = "check";
          pages = [
            {
              generate = "nixosOptions";
              title = "Options";
            }
          ];
          nixosOptions = {
            inherit (tiny) options;
            documentType = "none";
            warningsAreErrors = false;
            revision = "check-rev";
          };
        };
    in
    assert builtins.length pages == 3;
    assert (builtins.head pages).text == ../docs/welcome.md;
    assert (builtins.elemAt pages 1).title == "Guides";
    assert (builtins.elemAt pages 2).generate == "nixosOptions";
    assert nixos.options ? demo;
    assert nixos.documentType == "none";
    assert nixos.warningsAreErrors == false;
    assert nixos.revision == "check-rev";
    # Building the docs package forces nixosOptionsDoc with the pass-through args.
    pkgs.runCommand "docs-options"
      {
        inherit (docsPkg.passthru) config;
      }
      ''
        test -f "$config/config.json"
        test -d "$config/pages"
        # Options leaf must exist and mention the demo option from tiny eval.
        grep -q demo "$config"/pages/*.md
        # Config must not embed option-record material.
        ! grep -q mkOption "$config/config.json"
        touch "$out"
      '';

  # allLeaves must terminate on the real Prelude option tree (pages.children is
  # visible="shallow"), preserve deep option paths, and never emit blank pages
  # for internal/hidden options (nav built from filtered docList).
  docs-allLeaves-prelude =
    let
      preludeEval = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          ../src/prelude/options/motd.nix
          ../src/prelude/options/menu.nix
          ../src/prelude/options/docs.nix
          ../src/prelude/options/prompt.nix
        ];
      };
      docsPkg = import ../src/prelude/docs.nix
        {
          inherit (pkgs)
            lib
            writeText
            buildGoModule
            runCommand
            nixosOptionsDoc
            figlet
            ;
        }
        {
          theme = "phosphor";
          colorProfile = "auto";
          project = "check";
          pages = [
            {
              generate = "nixosOptions";
              title = "Options";
              # default split is allLeaves — omit to exercise the default
            }
          ];
          nixosOptions = {
            options = {
              inherit (preludeEval.options) prelude;
            };
            transformOptions = o: o // { declarations = [ ]; };
            warningsAreErrors = false;
          };
        };
    in
    pkgs.runCommand "docs-allLeaves-prelude"
      {
        inherit (docsPkg.passthru) config;
      }
      ''
        test -f "$config/config.json"
        test -d "$config/pages"
        count=$(find "$config/pages" -name '*.md' | wc -l | tr -d ' ')
        echo "allLeaves page count: $count"
        test "$count" -gt 20

        grep -R -q 'prelude\.motd' "$config/pages"
        grep -R -q 'motd\.env' "$config/pages"

        empty=0
        for f in "$config"/pages/*.md; do
          if ! grep -q '[^[:space:]]' "$f"; then
            echo "empty page: $f" >&2
            empty=$((empty + 1))
          fi
        done
        test "$empty" -eq 0

        if grep -R -q 'pages\.\*\.children\.\*\.children\.\*\.children' "$config/pages"; then
          echo "visible=shallow not honored — recursive children exploded" >&2
          exit 1
        fi
        touch "$out"
      '';

  # internal + transformOptions-hidden options must not leave nav/page entries.
  docs-allLeaves-filters-internal =
    let
      tiny = lib.evalModules {
        modules = [
          {
            options.visibleOpt = lib.mkOption {
              type = lib.types.str;
              default = "ok";
              description = "visible option";
            };
            options.hiddenInternal = lib.mkOption {
              type = lib.types.str;
              default = "nope";
              description = "internal option";
              internal = true;
            };
            options.hiddenByTransform = lib.mkOption {
              type = lib.types.str;
              default = "nope";
              description = "hidden via transformOptions";
            };
          }
        ];
      };
      docsPkg = import ../src/prelude/docs.nix
        {
          inherit (pkgs)
            lib
            writeText
            buildGoModule
            runCommand
            nixosOptionsDoc
            figlet
            ;
        }
        {
          theme = "phosphor";
          colorProfile = "auto";
          project = "check";
          pages = [
            {
              generate = "nixosOptions";
              title = "Options";
            }
          ];
          nixosOptions = {
            inherit (tiny) options;
            transformOptions =
              o:
              if o.name == "hiddenByTransform" then
                o // { visible = false; }
              else
                o;
            warningsAreErrors = false;
          };
        };
    in
    pkgs.runCommand "docs-allLeaves-filters-internal"
      {
        inherit (docsPkg.passthru) config;
      }
      ''
        test -f "$config/config.json"
        grep -q visibleOpt "$config/config.json"
        grep -R -q visibleOpt "$config/pages"
        if grep -q hiddenInternal "$config/config.json"; then
          echo "internal option leaked into nav" >&2
          exit 1
        fi
        if grep -q hiddenByTransform "$config/config.json"; then
          echo "transform-hidden option leaked into nav" >&2
          exit 1
        fi
        ! grep -R -q hiddenInternal "$config/pages"
        ! grep -R -q hiddenByTransform "$config/pages"
        touch "$out"
      '';

  # transformOptions may rewrite name/loc for display; lookup must still use
  # the raw loc so optionAt/nestPath succeed.
  docs-allLeaves-rename-transform =
    let
      tiny = lib.evalModules {
        modules = [
          {
            options.demo = lib.mkOption {
              type = lib.types.str;
              default = "x";
              description = "demo option renamed by transform";
            };
          }
        ];
      };
      docsPkg = import ../src/prelude/docs.nix
        {
          inherit (pkgs)
            lib
            writeText
            buildGoModule
            runCommand
            nixosOptionsDoc
            figlet
            ;
        }
        {
          theme = "phosphor";
          colorProfile = "auto";
          project = "check";
          pages = [
            {
              generate = "nixosOptions";
              title = "Options";
            }
          ];
          nixosOptions = {
            inherit (tiny) options;
            transformOptions = o: o // {
              name = "renamed.demo";
              loc = [ "renamed" "demo" ];
            };
            warningsAreErrors = false;
          };
        };
    in
    pkgs.runCommand "docs-allLeaves-rename-transform"
      {
        inherit (docsPkg.passthru) config;
      }
      ''
        test -f "$config/config.json"
        grep -q 'renamed.demo' "$config/config.json"
        empty=0
        for f in "$config"/pages/*.md; do
          if ! grep -q '[^[:space:]]' "$f"; then
            echo "empty page after rename transform: $f" >&2
            empty=$((empty + 1))
          fi
        done
        test "$empty" -eq 0
        count=$(find "$config/pages" -name '*.md' | wc -l | tr -d ' ')
        test "$count" -ge 1
        touch "$out"
      '';

  # shallow = one pass-through page (full nixosOptionsDoc).
  docs-shallow-passthrough =
    let
      tiny = lib.evalModules {
        modules = [
          {
            options.demo = lib.mkOption {
              type = lib.types.str;
              default = "x";
              description = "demo option";
            };
          }
        ];
      };
      docsPkg = import ../src/prelude/docs.nix
        {
          inherit (pkgs)
            lib
            writeText
            buildGoModule
            runCommand
            nixosOptionsDoc
            figlet
            ;
        }
        {
          theme = "phosphor";
          colorProfile = "auto";
          project = "check";
          pages = [
            {
              generate = "nixosOptions";
              title = "Options";
              split = "shallow";
            }
          ];
          nixosOptions = {
            inherit (tiny) options;
            warningsAreErrors = false;
          };
        };
    in
    pkgs.runCommand "docs-shallow-passthrough"
      {
        inherit (docsPkg.passthru) config;
      }
      ''
        test -f "$config/config.json"
        count=$(find "$config/pages" -name '*.md' | wc -l | tr -d ' ')
        # One options page (+ nothing else in this fixture).
        test "$count" -eq 1
        grep -q demo "$config"/pages/*.md
        touch "$out"
      '';





  # Our own `menu list` renders the grouped command table.
  menu-list-renders = pkgs.runCommand "menu-list-renders" { } ''
    ${lib.getExe config.packages.menu} list > "$out"
    test -s "$out"
    grep -q '^DEMOS$' "$out"
    grep -q "tour every feature demo" "$out"
  '';

  # Every feature demo (motd variants, themes, acme-web motd + menu list)
  # builds (shellcheck) and renders.
  examples-render = pkgs.runCommand "examples-render" { } ''
    CLICOLOR_FORCE=1 ${lib.getExe demos.examplesRunner} > "$out"
    test -s "$out"
    grep -q 'theme amber' "$out"
    grep -q 'theme solarized' "$out"
    grep -q 'Devshell UI for Nix flakes' "$out"
    grep -Fq '38;2;255;199;97' "$out"
    grep -Fq '38;2;119;245;201' "$out"
  '';

  # Generated documentation and its media fingerprints must match the repo.
  docs-generated-fresh = docsAutomation.docsFresh;
  docs-media-fresh = docsAutomation.mediaFresh;
}
