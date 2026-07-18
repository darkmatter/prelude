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
    config.packages.motd
    config.packages.docs
    docsAutomation.record
    docsAutomation.sync
    previews
  ]
  ++ (with pkgs; [
    shellcheck
    nixfmt
    starship
  ]);
  shellHook = ''
    r() { exec nix develop "$@"; }

    # Starship re-resolves this path on every prompt render.
    export STARSHIP_CONFIG=${config.packages.prompt}

    if [ -n "''${BASH_VERSION-}" ]; then
      export -f r 2>/dev/null || true
      # Under `nix develop` this hook runs inside the interactive bash itself,
      # which has no starship init of its own (rc-based inits are zsh-only on
      # most setups) — wire it here. In direnv's non-interactive eval shell
      # $- has no `i`, so this is skipped and the user's zsh init plus the
      # STARSHIP_CONFIG export above do the theming.
      case "$-" in *i*)
        if command -v starship >/dev/null 2>&1; then
          eval "$(starship init bash)"
        fi
      ;; esac
    fi

    # Keep Nix diagnostics and the welcome card ordered on the same stream.
    motd >&2
  '';
}
