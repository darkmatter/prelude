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
  ++ config.packages.menu.taskWrappers;

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

  # Keyed groups/tasks normalize into deterministic display order. Explicit
  # order wins; otherwise keys break ties and become the default labels.
  task-ordering =
    let
      plib = import ../src/prelude/lib.nix { inherit lib; };
      evaluated = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          {
            prelude.groups = {
              z-last = {
                order = 100;
                tasks.z-task = { };
              };
              a-first = {
                order = 100;
                title = "First";
                tasks = {
                  z-default = { };
                  m-default = { };
                  a-explicit.order = 100;
                };
              };
            };
          }
          {
            prelude.groups.a-first.tasks.m-default.description = "merged";
          }
        ];
      };
      normalized = plib.normalizeGroups evaluated.config.prelude.groups;
      actual = map (group: {
        inherit (group) title;
        tasks = map (task: task.name) group.tasks;
      }) normalized;
      firstTasks = builtins.head normalized;
      defaultRuns = map (task: task.run) firstTasks.tasks;
      mergedDescription = (builtins.elemAt firstTasks.tasks 1).description;
      expected = [
        {
          title = "First";
          tasks = [
            "a-explicit"
            "m-default"
            "z-default"
          ];
        }
        {
          title = "z-last";
          tasks = [ "z-task" ];
        }
      ];
    in
    assert actual == expected;
    assert
      defaultRuns == [
        "a-explicit"
        "m-default"
        "z-default"
      ];
    assert mergedDescription == "merged";
    pkgs.runCommand "task-ordering" { } "touch $out";

  # Header options share one nested namespace for module and direct consumers.
  motd-header-options =
    let
      evaluated = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          ../src/prelude/options/motd.nix
          {
            prelude.motd = {
              title = pkgs.writeText "test-title.txt" "TEST TITLE\n";
              padding = {
                x = 2;
                top = 2;
              };
              windowBackground = {
                blend = 0.15;
              };
              header = {
                titleStyle = "bracketed";
                tagline = "test-tagline";
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
    assert builtins.readFile title == "TEST TITLE\n";
    assert header.titleStyle == "bracketed";
    assert header.tagline == "test-tagline";
    assert shellStatus.label == "nix develop";
    assert shellStatus.status == "ready";
    assert shellStatus.failLevel == "error";
    assert header.status.cache.failLevel == "warning";
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

  # MOTD commands are selected from the menu task catalogue, whose generated
  # wrappers are bundled with packages.motd when the menu is enabled.
  motd-commands-runnable =
    mkRunnableCheck "motd-commands-runnable" "motd"
      config.packages.motd.commandNames;

  menu-tasks-runnable = mkRunnableCheck "menu-tasks-runnable" "menu" config.packages.menu.taskNames;

  # Package-backed tasks carry their runtime package into the evaluated menu
  # package and still receive a directly invocable task wrapper.
  package-task-bundled =
    assert lib.elem pkgs.nixfmt config.packages.menu.taskRuntimePackages;
    pkgs.runCommand "package-task-bundled"
      {
        nativeBuildInputs = [ config.packages.menu ];
      }
      ''
        command -v nixfmt >/dev/null
        command -v fmt >/dev/null
        touch "$out"
      '';

  # Docs options accept hand-authored sections; nothing is required beyond blocks.
  docs-options =
    let
      evaluated = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          ../src/prelude/options/docs.nix
          {
            prelude.docs = {
              enable = true;
              sections.name = {
                order = 100;
                blocks = [
                  {
                    type = "lead";
                    term = "acme";
                    text = "demo";
                  }
                ];
              };
            };
          }
        ];
      };
      secs = evaluated.config.prelude.docs.sections;
    in
    assert secs.name.blocks != [ ];
    assert (builtins.head secs.name.blocks).type == "lead";
    assert (builtins.head secs.name.blocks).term == "acme";
    pkgs.runCommand "docs-options" { } "touch $out";

  # Our own `menu list` renders the grouped task table.
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
