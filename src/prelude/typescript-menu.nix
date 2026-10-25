# Bun owns both the picker and execution; functions never cross the JSON boundary.
{
  lib,
  pkgs,
}: {
  entrypoint,
  project,
  palette,
  colorProfile,
  root ? null,
  runtimePackages ? [],
}: let
  x = pkgs.writeShellApplication {
    name = "x";
    runtimeInputs =
      [
        pkgs.bun
        pkgs.coreutils
      ]
      ++ runtimePackages;
    text = ''
      entrypoint=${lib.escapeShellArg entrypoint}
      ${
        if root != null
        then ''
          project_root=${lib.escapeShellArg (toString root)}
        ''
        else ''
          project_root="$PWD"
          while [ ! -f "$project_root/$entrypoint" ]; do
            if [ "$project_root" = / ]; then
              printf 'x: cannot find %s in this directory or its ancestors\n' "$entrypoint" >&2
              exit 1
            fi
            project_root=$(dirname "$project_root")
          done
        ''
      }
      if [ ! -f "$project_root/$entrypoint" ]; then
        printf 'x: cannot find %s in %s\n' "$entrypoint" "$project_root" >&2
        exit 1
      fi
      cd "$project_root"
      # Keep shell commands that call x bound to this same catalogue.
      launcher_bin=$(dirname "$0")
      export PATH="$launcher_bin:$PATH"
      exec ${lib.getExe pkgs.bun} "$project_root/$entrypoint" "$@"
    '';
  };
  menu = pkgs.writeShellApplication {
    name = "menu";
    text = ''
      if [ "$#" -gt 0 ]; then
        echo 'menu: opens the interactive picker only' >&2
        echo "hint: run commands with \`x <key>\`; list with \`x --list\`" >&2
        exit 1
      fi
      exec ${lib.getExe x}
    '';
  };
  # Shell/workspace hosts still need the palette boundary, but the runtime
  # TypeScript catalogue is not known to Nix. Do not publish stale Nix tasks.
  configFile = pkgs.writeText "prelude-typescript-menu.json" (
    builtins.toJSON {
      inherit project palette colorProfile;
      groups = [];
      motdCommands = [];
      just.enable = false;
      scripts.enable = false;
    }
  );
in
  pkgs.symlinkJoin {
    name = "prelude-typescript-menu";
    paths = [
      x
      menu
    ];
    passthru = {inherit configFile;};
    meta = {
      description = "TypeScript-owned Prelude picker and x dispatcher";
      mainProgram = "menu";
    };
  }
