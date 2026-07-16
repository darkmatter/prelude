# Dogfood devshell before Prelude augmentation. wrapPerSystem adds the enabled
# motd/menu/docs packages, prompt config, and final MOTD invocation; this file
# only declares project-specific tools and hooks.
#
#   help / ?   → reprint the welcome banner (motd)
#   menu / m   → interactive command picker
#   docs / d   → hand-authored man-style manual
#   r          → replace this shell with a freshly evaluated devshell
{
  pkgs,

  docsAutomation,
  previews,
  ...
}:
pkgs.mkShell {
  packages = [
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
    # ── footer keybinds (motd.shortcuts) ─────────────────────────────────────
    help() { command motd "$@"; }
    m() { command menu "$@"; }
    # Hand-authored manual: CONTENTS sidebar, 1–N jump, j/k scroll, q quit.
    docs() { command docs "$@"; }
    d() { docs "$@"; }
    r() { exec nix develop "$@"; }

    if [ -n "''${BASH_VERSION-}" ]; then
      export -f help m docs d r 2>/dev/null || true
      alias '?'='help'
      # Under `nix develop` this hook runs inside the interactive bash itself,
      # which has no starship init of its own (rc-based inits are zsh-only on
      # most setups) — wire it here. In direnv's non-interactive eval shell
      # $- has no `i`, so this is skipped and the user's zsh init plus the
      # wrapper-provided STARSHIP_CONFIG export do the theming.
      case "$-" in *i*)
        if command -v starship >/dev/null 2>&1; then
          eval "$(starship init bash)"
        fi
      ;; esac
    fi
  '';
}
