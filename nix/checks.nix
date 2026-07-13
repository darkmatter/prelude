# Flake checks. Checks whose $out is a rendered preview use the `-render(s)`
# suffix — the previews utility (previews.nix) discovers them by name.
{
  pkgs,
  lib,
  config,
  demos,
  ...
}:
{
  # Building the module-produced packages runs shellcheck / go vet on the
  # generated artifacts.
  motd-default = config.packages.motd;
  menu-default = config.packages.menu;

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
              padding.x = 2;
              header = {
                titleStyle = "bracketed";
                tagline = "test-tagline";
                statusLabel = "nix develop";
                statusText = "ready";
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
      header = evaluated.config.prelude.motd.header;
      padding = evaluated.config.prelude.motd.padding;
      shortcuts = evaluated.config.prelude.motd.shortcuts;
    in
    assert header.titleStyle == "bracketed";
    assert header.tagline == "test-tagline";
    assert header.statusLabel == "nix develop";
    assert header.statusText == "ready";
    assert
      shortcuts == [
        {
          command = "menu";
          alias = "m";
        }
      ];
    assert padding.x == 2;
    assert padding.y == 0;
    assert padding.left == null;
    assert padding.right == null;

    pkgs.runCommand "motd-header-options" { } "touch $out";

  # Our own motd renders authored commands, never the menu's task catalog.
  motd-renders = pkgs.runCommand "motd-renders" { } ''
    NO_COLOR=1 ${lib.getExe config.packages.motd} > "$out"
    test -s "$out"
    grep -q "prelude" "$out"
    grep -q "Getting Started" "$out"
    grep -q "nix flake check" "$out"
    grep -q "menu" "$out"
    if grep -q "demo-menu" "$out"; then
      echo "motd must not render menu tasks" >&2
      exit 1
    fi
  '';

  # Command rows contain only an exact runnable command and its description.
  motd-commands-render =
    let
      commandMotd =
        import ../src/prelude/motd.nix
        {
          inherit (pkgs)
            lib
            writeText
            buildGoModule
            ;
        }
        {
          project = "command-test";
          clearScreen = false;
          margin.top = 0;
          git = false;
          groups.hidden.tasks.should-not-render = {
            run = "false";
            description = "menu-only sentinel";
          };
          commands = {
            check = {
              order = 100;
              command = "nix flake check";
              description = "verify the flake";
            };
            browse = {
              command = "menu";
              description = "browse project commands";
            };
          };
        };
    in
    pkgs.runCommand "motd-commands-render" { } ''
      NO_COLOR=1 ${lib.getExe commandMotd} > "$out"
      grep -q "Getting Started" "$out"
      grep -q "commands" "$out"
      grep -q 'nix flake check' "$out"
      grep -q 'verify the flake' "$out"
      grep -q 'browse project commands' "$out"
      if grep -q "should-not-render\|menu-only sentinel" "$out"; then
        echo "motd leaked menu task data" >&2
        exit 1
      fi
      test "$(grep -n 'nix flake check' "$out" | head -n1 | cut -d: -f1)" -lt "$(grep -n '\$ menu' "$out" | head -n1 | cut -d: -f1)"
    '';

  # Narrow command sections stay within the configured width.
  motd-commands-narrow-render =
    let
      narrowMotd =
        import ../src/prelude/motd.nix
        {
          inherit (pkgs)
            lib
            writeText
            buildGoModule
            ;
        }
        {
          project = "narrow";
          width = 20;
          clearScreen = false;
          margin.top = 0;
          git = false;
          commands.check = {
            command = "nix flake check";
            description = "verify";
          };
        };
    in
    pkgs.runCommand "motd-commands-narrow-render" { } ''
      NO_COLOR=1 ${lib.getExe narrowMotd} > "$out"
      grep -q 'nix flake check' "$out"
      grep -q 'verify' "$out"
      if ! awk '{ line = $0; sub(/^ +/, "", line) } length(line) > 20 { print length(line) ": " line > "/dev/stderr"; overflow = 1 } END { exit overflow }' "$out"; then
        echo "narrow motd exceeded configured width" >&2
        exit 1
      fi
    '';

  # Recipes render as fade-rule codeblocks with comments and commands.
  motd-recipes-render =
    let
      recipeMotd =
        import ../src/prelude/motd.nix
        {
          inherit (pkgs)
            lib
            writeText
            buildGoModule
            ;
        }
        {
          project = "recipe-test";
          clearScreen = false;
          margin = {
            top = 0;
          };
          git = false;
          recipes."-clean-stack" = {
            steps = [
              { comment = "Start backing services"; }
              { command = "just db:up"; }
              { command = "just db:migrate && just db:seed"; }
              { command = "-x command"; }
            ];
          };
        };
    in
    pkgs.runCommand "motd-recipes-render" { } ''
      NO_COLOR=1 ${lib.getExe recipeMotd} > "$out.ansi"
      sed $'s/\033\\[[0-9;]*m//g' "$out.ansi" > "$out"
      rm "$out.ansi"
      if grep -q "next steps" "$out"; then
        echo "direct motd without commands must not invent next steps" >&2
        exit 1
      fi
      grep -q "Getting Started" "$out"
      grep -q "examples" "$out"
      grep -q -- "-clean-stack" "$out"
      grep -q '# Start backing services' "$out"
      grep -q "just db:up" "$out"
      grep -q "just db:migrate && just db:seed" "$out"
      grep -q -- "-x command" "$out"
    '';

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
}
