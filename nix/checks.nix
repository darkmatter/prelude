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

  # Banner-specific options share one nested namespace for module and direct
  # consumers. Partial border configuration retains the remaining defaults.
  motd-banner-options =
    let
      evaluated = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          ../src/prelude/options/motd.nix
          {
            prelude.motd.banner = {
              badge = "test-badge";
              label = "test-label";
              tagline = "test-tagline";
              border.width = 2;
              statusItems = [
                {
                  text = "ready";
                  status = "success";
                }
              ];
            };
          }
        ];
      };
      banner = evaluated.config.prelude.motd.banner;

    in
    assert banner.badge == "test-badge";
    assert banner.label == "test-label";
    assert banner.tagline == "test-tagline";
    assert banner.border.width == 2;
    assert banner.border.foreground == null;
    assert banner.border.rounded;
    assert
      banner.statusItems == [
        {
          text = "ready";
          status = "success";
        }
      ];

    pkgs.runCommand "motd-banner-options" { } "touch $out";

  # Our own motd renders authored next steps, never the menu's task catalog.
  motd-renders = pkgs.runCommand "motd-renders" { } ''
    NO_COLOR=1 ${lib.getExe config.packages.motd} > "$out"
    test -s "$out"
    grep -q "prelude" "$out"
    grep -q "next steps" "$out"
    grep -q "nix flake check" "$out"
    grep -q "menu list" "$out"
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
              writeShellApplication
              gum
              ncurses
              ;
          }
          {
            project = "command-test";
            clearScreen = false;
            margin.top = 0;
            loadLine = "";
            footer = false;
            git = false;
            banner.border.width = 0;
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
      grep -q "next steps" "$out"
      grep -q '\$ nix flake check.*verify the flake' "$out"
      grep -q '\$ menu.*browse project commands' "$out"
      if grep -q "should-not-render\|menu-only sentinel\|false" "$out"; then
        echo "motd leaked menu task data" >&2
        exit 1
      fi
      test "$(grep -n 'nix flake check' "$out" | cut -d: -f1)" -lt "$(grep -n '\$ menu' "$out" | cut -d: -f1)"
    '';

  # Narrow command sections stay within the configured width, stacking the
  # description below the exact command when two columns no longer fit.
  motd-commands-narrow-render =
    let
      narrowMotd =
        import ../src/prelude/motd.nix
          {
            inherit (pkgs)
              lib
              writeShellApplication
              gum
              ncurses
              ;
          }
          {
            project = "narrow";
            width = 20;
            clearScreen = false;
            margin.top = 0;
            loadLine = "";
            banner.badge = "";
            footer = false;
            git = false;
            banner.border.width = 0;
            commands.check = {
              command = "nix flake check";
              description = "verify the flake";
            };
          };
    in
    pkgs.runCommand "motd-commands-narrow-render" { } ''
      NO_COLOR=1 ${lib.getExe narrowMotd} > "$out"
      grep -q '\$ nix flake check' "$out"
      grep -q 'verify the flake' "$out"
      if ! awk '{ line = $0; sub(/^ +/, "", line) } length(line) > 20 { print length(line) ": " line > "/dev/stderr"; overflow = 1 } END { exit overflow }' "$out"; then
        echo "narrow motd exceeded configured width" >&2
        exit 1
      fi
    '';

  # Recipes render comments, commands, and intentional blank lines as a
  # distinct motd section.
  motd-recipes-render =
    let
      recipeMotd =
        import ../src/prelude/motd.nix
          {
            inherit (pkgs)
              lib
              writeShellApplication
              gum
              ncurses
              ;
          }
          {
            project = "recipe-test";
            clearScreen = false;
            margin = {
              top = 0;
            };
            loadLine = "";
            footer = false;
            git = false;
            banner.border.width = 0;
            recipes."-clean-stack" = {
              lines = [
                "# Start backing services"
                "just db:up"
                ""
                "just db:migrate && just db:seed"
                "-x command"
                ""
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
      grep -q "recipes — common flows that take a few steps" "$out"
      grep -q -- "-clean-stack" "$out"
      grep -q '^│   # Start backing services' "$out"
      grep -q "just db:up" "$out"
      grep -q "just db:migrate && just db:seed" "$out"
      grep -q -- "-x command" "$out"
      grep -q "╭" "$out"
      blank_rows=$(grep -Ec '^│[[:space:]]*│$' "$out")
      test "$blank_rows" -ge 2
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
