# Dogfood devshell: explicitly compose Prelude's generated packages with the
# project-specific tools and hooks.
#
# Prelude supplies the `menu`, `help`, and `docs` commands. The project only
# adds `r` as a local convenience for replacing the current shell.
{ pkgs
, config
, docsAutomation
, previews
, ...
}:
pkgs.mkShell {
  packages = [
    config.packages.prelude
    docsAutomation.record
    docsAutomation.sync
    previews
  ]
  ++ (with pkgs; [
    shellcheck
    nixfmt
  ]);
  shellHook = ''
    r() { exec nix develop "$@"; }

    # Starship re-resolves this path on every prompt render.
    export STARSHIP_CONFIG=${config.packages.prompt}

    # `config.packages.prelude` initializes ble.sh and Starship for interactive
    # Bash through its setup hook. Keeping that logic out of this shellHook
    # ensures this repository exercises the same package contract as consumers.
    if [ -n "''${BASH_VERSION-}" ]; then
      export -f r 2>/dev/null || true
    fi


    # Keep Nix diagnostics and the welcome card ordered on the same stream.
    motd >&2
  '';
}
