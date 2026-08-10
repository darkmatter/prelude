# Dogfood devshell: explicitly compose Prelude's generated packages with the
# project-specific tools and hooks.
#
# Prelude supplies the `x` and `docs` commands. The project only adds `r`
# as a local convenience for replacing the current shell.
{
  pkgs,
  config,
  docsAutomation,
  previews,
  ...
}:
pkgs.mkShell {
  packages =
    [
      config.packages.prelude
      docsAutomation.record
      docsAutomation.sync
      previews
    ]
    ++ (with pkgs; [
      shellcheck
      nixfmt
    ]);
  DIRENV_LOG_FORMAT = "";
  shellHook = ''
    r() { exec nix develop "$@"; }

    # Starship re-resolves this path on every prompt render.
    export STARSHIP_CONFIG=${config.packages.prompt}
    # `config.packages.prelude` appends its idempotent, current-shell init after
    # this hook. It composes MOTD, ble.sh, completion, Starship, and the native
    # status line without starting another shell.
    if [ -n "''${BASH_VERSION-}" ]; then
      export -f r 2>/dev/null || true
    fi
  '';
}
