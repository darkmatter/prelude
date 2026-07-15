# PROTOTYPE — standalone flake-parts module for evaluating a pinned MOTD/docs
# workspace above a real child shell. Delete this directory and its flake.nix
# references if the experiment is rejected. It does not extend prelude.*.
{ localFlake }:
{ ... }:
{
  perSystem =
    {
      pkgs,
      lib,
      config,
      ...
    }:
    let
      _unusedLocalFlake = localFlake;
      package = import ./package.nix {
        inherit pkgs;
        inherit (config.packages) motd menu docs;
      };
      baseShell = config.devShells.default;
      # Dogfood currently prints its MOTD at the end of shellHook. The workspace
      # owns that view, so suppress only that exact line in this parallel shell.
      capturedHook = lib.replaceStrings [ "motd >&2" ] [ ": # MOTD is pinned by workspace" ] (
        baseShell.shellHook or ""
      );
    in
    {
      packages.motd-shell-experiment = package;
      apps.motd-shell-experiment = {
        type = "app";
        program = pkgs.lib.getExe package;
      };

      # Parallel opt-in shell; the ordinary `nix develop` remains unchanged.
      devShells.motd-shell-experiment = baseShell.overrideAttrs (old: {
        propagatedBuildInputs = (old.propagatedBuildInputs or [ ]) ++ [ package ];
        shellHook = ''
          export PRELUDE_SHELL_INIT_LOG="''${TMPDIR:-/tmp}/prelude-shell-init-$$.log"
          export PRELUDE_SHELL_INIT_LOG_OWNED=1
          {
            ${capturedHook}
          } >"$PRELUDE_SHELL_INIT_LOG" 2>&1

          if [ -t 0 ] && [ -t 1 ]; then
            exec ${pkgs.lib.getExe package}
          fi

          cat "$PRELUDE_SHELL_INIT_LOG" >&2
          rm -f "$PRELUDE_SHELL_INIT_LOG"
        '';
      });
    };
}
