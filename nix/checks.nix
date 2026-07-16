# Flake checks. Checks whose $out is a rendered preview use the `-render(s)`
# suffix — the previews utility (previews.nix) discovers them by name.
{
  pkgs,
  lib,
  config,
  demos,
  docsAutomation,
  previews,
  ...
}:
let
  preludeLib = import ./lib.nix { inherit lib; };
  system = pkgs.stdenv.hostPlatform.system;

  # The command-providing packages of the dogfood devshell (shell.nix).
  devshellCommandPackages = [
    config.packages.motd
    config.packages.menu
    config.packages.docs
    docsAutomation.sync
    docsAutomation.record
    previews
  ]
  # Consume the wrappers exposed by the evaluated Prelude package, just as a
  # downstream module can, instead of rebuilding knowledge from source config.
  ++ config.packages.menu.commandWrappers;

  # Assert that every advertised command name resolves on a PATH built from
  # the devshell's packages.
  mkRunnableCheck =
    checkName: surface: names:
    pkgs.runCommand checkName { nativeBuildInputs = devshellCommandPackages; } ''
      for cmd in ${lib.concatMapStringsSep " " lib.escapeShellArg names}; do
        command -v "$cmd" >/dev/null 2>&1 || {
          echo "${surface} advertises '$cmd' but no devshell package provides it" >&2
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

  # Canonical output families become commands by name without forcing output
  # values. Explicit output paths preserve names that need quoted attr segments.
  output-commands =
    let
      commands = preludeLib.commandsFromOutputs {
        inherit pkgs system;
        outputs = {
          packages = {
            default = throw "commandsFromOutputs forced packages.default";
            api = throw "commandsFromOutputs forced packages.api";
          };
          apps."api.admin" = throw "commandsFromOutputs forced apps.api.admin";
          checks.unit = throw "commandsFromOutputs forced checks.unit";
        };
      };
    in
    assert
      builtins.attrNames commands == [
        "build:api"
        "check:unit"
        "run:api.admin"
      ];
    assert commands."run:api.admin".group == "run";
    assert commands."build:api".group == "build";
    assert commands."check:unit".group == "check";
    assert lib.hasSuffix " run '.#apps.${system}.\"api.admin\"'" commands."run:api.admin".exec;
    assert lib.hasSuffix " build '.#packages.${system}.\"api\"'" commands."build:api".exec;
    assert lib.hasSuffix " build '.#checks.${system}.\"unit\"'" commands."check:unit".exec;
    pkgs.runCommand "output-commands" { } "touch $out";

  # wrapPerSystem preserves canonical outputs, lets explicit command fields win,
  # and augments rather than replaces the caller's devshell inputs and hook.
  wrap-per-system =
    let
      motd = pkgs.writeShellApplication {
        name = "motd";
        text = "true";
      };
      docs = pkgs.writeShellApplication {
        name = "docs";
        text = "true";
      };
      prompt = pkgs.writeText "starship.toml" "";
      wrapped =
        preludeLib.wrapPerSystem
          (
            { pkgs, ... }:
            {
              packages = {
                default = throw "wrapPerSystem forced packages.default";
                api = throw "wrapPerSystem forced packages.api";
              };
              apps.api = throw "wrapPerSystem forced apps.api";
              checks.unit = throw "wrapPerSystem forced checks.unit";
              prelude.commands."run:api" = {
                description = "custom run description";
                key = "a";
              };
              devShells.default = pkgs.mkShell {
                packages = [ pkgs.hello ];
                shellHook = "export BEFORE_PRELUDE=1";
              };
            }
          )
          {
            inherit pkgs system;
            config.packages = {
              inherit motd docs prompt;
            };
          };
      shell = wrapped.devShells.default;
      runCommand = wrapped.prelude.commands."run:api";
    in
    assert wrapped.packages ? api;
    assert !(wrapped.prelude.commands ? "build:default");
    assert runCommand.description == "custom run description";
    assert runCommand.key == "a";
    assert lib.hasSuffix " run '.#apps.${system}.\"api\"'" runCommand.exec;
    assert lib.elem pkgs.nix runCommand.runtimePackages;
    assert lib.elem motd shell.nativeBuildInputs;
    assert lib.elem docs shell.nativeBuildInputs;
    assert lib.elem pkgs.hello shell.nativeBuildInputs;
    assert shell.shellHook == "export BEFORE_PRELUDE=1\nexport STARSHIP_CONFIG=${prompt}\nmotd >&2";
    pkgs.runCommand "wrap-per-system" { } "touch $out";

  # Flat commands normalize into deterministic groups. Explicit order wins;
  # names break ties, and each group appears at its first command's position.
  command-ordering =
    let
      plib = import ../src/prelude/lib.nix { inherit lib; };
      evaluated = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          {
            prelude.commands = {
              z-task = {
                group = "z-last";
                order = 100;
              };
              z-default.group = "a-first";
              m-default.group = "a-first";
              a-explicit = {
                group = "a-first";
                order = 100;
              };
            };
          }
          {
            prelude.commands.m-default.description = "merged";
          }
        ];
      };
      normalized = plib.normalizeCommandGroups evaluated.config.prelude.commands;
      actual = map (group: {
        inherit (group) title;
        commands = map (command: command.name) group.tasks;
      }) normalized;
      firstGroup = builtins.head normalized;
      defaultExecs = map (command: command.run) firstGroup.tasks;
      mergedDescription = (builtins.elemAt firstGroup.tasks 1).description;
      expected = [
        {
          title = "a-first";
          commands = [
            "a-explicit"
            "m-default"
            "z-default"
          ];
        }
        {
          title = "z-last";
          commands = [ "z-task" ];
        }
      ];
    in
    assert actual == expected;
    assert
      defaultExecs == [
        "a-explicit"
        "m-default"
        "z-default"
      ];
    assert mergedDescription == "merged";
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
              shortcuts = [
                {
                  command = "menu";
                  alias = "m";
                }
              ];
            };
          }
        ];
      };
      title = evaluated.config.prelude.motd.title;
      header = evaluated.config.prelude.motd.header;
      padding = evaluated.config.prelude.motd.padding;
      shortcuts = evaluated.config.prelude.motd.shortcuts;
      windowBackground = evaluated.config.prelude.motd.windowBackground;
      shellStatus = header.status.shell;
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
      shortcuts == [
        {
          command = "menu";
          alias = "m";
        }
      ];
    assert padding.x == 2;
    assert padding.y == 0;
    assert padding.top == 2;
    assert padding.left == null;
    assert padding.right == null;
    assert windowBackground.blend == 0.15;

    pkgs.runCommand "motd-header-options" { } "touch $out";

  # Smoke: dogfood motd runs and prints something. Content is not asserted
  # while the banner layout is still in flux.
  motd-renders = pkgs.runCommand "motd-renders" { } ''
    NO_COLOR=1 ${lib.getExe config.packages.motd} > "$out"
    test -s "$out"
  '';

  # MOTD commands are selected from the command catalogue, whose generated
  # wrappers are bundled with packages.motd when the menu is enabled.
  motd-commands-runnable =
    mkRunnableCheck "motd-commands-runnable" "motd"
      config.packages.motd.commandNames;

  menu-commands-runnable =
    mkRunnableCheck "menu-commands-runnable" "menu"
      config.packages.menu.commandNames;

  titles-wrapper-renders =
    pkgs.runCommand "titles-wrapper-renders"
      {
        nativeBuildInputs = [ config.packages.motd ] ++ config.packages.motd.commandWrappers;
      }
      ''
        titles > "$out"
        test "$(grep -c '^===== .* =====$' "$out")" -eq 23
        grep -q '^===== 3d-ascii =====$' "$out"
        grep -q '^===== calvin-s =====$' "$out"
        test "$(wc -l < "$out")" -gt 50
      '';

  # Package-backed commands carry their runtime package into the evaluated menu
  # package and still receive a directly invocable wrapper.
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

  # Docs options accept Markdown page paths in declaration order.
  docs-options =
    let
      evaluated = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          ../src/prelude/options/docs.nix
          {
            prelude.docs.pages = [
              { text = ./dogfood/docs/name.md; }
              { text = ./dogfood/docs/synopsis.md; }
            ];
          }
        ];
      };
      pages = evaluated.config.prelude.docs.pages;
    in
    assert builtins.length pages == 2;
    assert (builtins.head pages).text == ./dogfood/docs/name.md;
    pkgs.runCommand "docs-options" { } "touch $out";

  # Our own `menu list` renders the grouped command table.
  menu-list-renders = pkgs.runCommand "menu-list-renders" { } ''
    ${lib.getExe config.packages.menu} list > "$out"
    test -s "$out"
    grep -q "demo-menu" "$out"
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
