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
    config.packages.motd
    config.packages.menu
    config.packages.docs
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
      executables = lib.unique (
        map (invocation: builtins.head (lib.splitString " " invocation)) invocations
      );
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
      recipe = pkgs.writeText "title.nix" ''
        {
          text = "prelude";
          font = "calvin-s";
        }
      '';
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

  # Prelude owns navigation commands; project Getting Started rows remain
  # focused on the explicitly selected lifecycle commands.
  prelude-command-defaults =
    assert lib.all (name: lib.elem name config.packages.menu.commandNames) [
      "menu"
      "help"
      "docs"
    ];
    assert lib.all (name: !lib.elem name config.packages.motd.commandNames) [
      "menu"
      "help"
      "docs"
    ];
    pkgs.runCommand "prelude-command-defaults" { nativeBuildInputs = [ config.packages.menu ]; } ''
      command -v x >/dev/null
      command -v menu >/dev/null
      command -v help >/dev/null
      command -v docs >/dev/null
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
            sort.groups = [
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
      normalized = plib.normalizeCommandGroups evaluated.config.sort.groups evaluated.config.prelude.commands;
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

  # The MOTD advertises x aliases; the menu retains canonical underlying
  # invocations for execution and diagnostics.
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
    assert lib.elem "go:test" config.packages.menu.commandNames;
    assert lib.elem "go test -C src ./..." config.packages.menu.commandInvocations;
    assert lib.elem "x go:test" config.packages.menu.xInvocations;
    assert lib.elem "x go:test" config.packages.motd.commandInvocations;
    assert !lib.elem "go:test" config.packages.menu.commandWrapperNames;
    assert !lib.elem "go-test" config.packages.menu.commandWrapperNames;
    pkgs.runCommand "grouped-commands-use-canonical-invocations"
      { nativeBuildInputs = [ config.packages.menu ]; }
      ''
        command -v go >/dev/null
        ! command -v go:test >/dev/null
        ! command -v go-test >/dev/null
        touch "$out"
      '';

  # Docs options accept Markdown page paths in declaration order.
  docs-options =
    let
      evaluated = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          ../src/prelude/options/docs.nix
          {
            prelude.docs.pages = [
              { text = ../examples/default/docs/name.md; }
              { text = ../examples/default/docs/synopsis.md; }
            ];
          }
        ];
      };
      pages = evaluated.config.prelude.docs.pages;
    in
    assert builtins.length pages == 2;
    assert (builtins.head pages).text == ../examples/default/docs/name.md;
    pkgs.runCommand "docs-options" { } "touch $out";

  # Our own `menu list` renders the grouped command table.
  menu-list-renders = pkgs.runCommand "menu-list-renders" { } ''
    ${lib.getExe config.packages.menu} list > "$out"
    test -s "$out"
    grep -q '^DEMOS$' "$out"
    grep -q "acme-web command menu demo" "$out"
  '';

  # Every feature demo (motd variants, themes, acme-web motd + menu list)
  # builds (shellcheck) and renders.
  examples-render = pkgs.runCommand "examples-render" { } ''
    ${lib.getExe demos.examplesRunner} > "$out"
    test -s "$out"
  '';

  # Generated documentation and its media fingerprints must match the repo.
  docs-generated-fresh = docsAutomation.docsFresh;
  docs-media-fresh = docsAutomation.mediaFresh;
}
