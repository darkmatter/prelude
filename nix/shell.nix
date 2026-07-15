# Dogfood devshell: greeted by our own motd; footer shortcuts are real shell
# entrypoints that match motd.shortcuts in dogfood.nix.
#
#   help / ?   → reprint the welcome banner (motd)
#   menu / m   → interactive command picker
#   docs / d   → hand-authored man-style manual
{
  pkgs,
  config,
  docsAutomation,
  previews,
  ...
}:
pkgs.mkShell {
  packages = [
    config.packages.motd
    config.packages.menu
    config.packages.docs
    docsAutomation.record
    docsAutomation.sync
    previews
  ]
  ++ (with pkgs; [
    shellcheck
    nixfmt
  ]);
  shellHook = ''
    # ── footer keybinds (motd.shortcuts) ─────────────────────────────────────
    help() { command motd "$@"; }
    m() { command menu "$@"; }
    # Hand-authored manual: CONTENTS sidebar, 1–N jump, j/k scroll, q quit.
    docs() { command docs "$@"; }
    d() { docs "$@"; }

    if [ -n "''${BASH_VERSION-}" ]; then
      export -f help m docs d 2>/dev/null || true
      alias '?'='help'
    fi

    # Nix writes evaluation/build diagnostics to stderr. Render the shell-hook
    # MOTD on the same stream so those diagnostics are ordered before the card
    # instead of racing stdout and appearing over its header.
    motd >&2
  '';
}
