# Dogfood devshell: greeted by our own motd; footer shortcuts are real shell
# entrypoints that match motd.shortcuts in dogfood.nix.
#
#   help / ?   → reprint the welcome banner (motd)
#   menu / m   → interactive command picker
#   docs / d   → docs page (stub until built)
{
  pkgs,
  config,
  previews,
  ...
}:
pkgs.mkShell {
  packages = [
    config.packages.motd
    config.packages.menu
    previews
  ]
  ++ (with pkgs; [
    shellcheck
    nixfmt
  ]);
  shellHook = ''
    # ── footer keybinds (motd.shortcuts) ─────────────────────────────────────
    # Prefer functions over aliases so args pass through and special names work.
    help() { command motd "$@"; }
    '?'() { command motd "$@"; }
    m() { command menu "$@"; }
    docs() {
      # Placeholder until the docs surface exists.
      printf '%s\n' "docs: not built yet — check README.md for now"
      return 1
    }
    d() { docs "$@"; }

    # direnv / nested bash: make the short names available to child shells.
    if [ -n "''${BASH_VERSION-}" ]; then
      export -f help m docs d 2>/dev/null || true
      # '?' is a valid function name but awkward to export; alias covers it.
      alias '?'='help'
    fi

    motd
  '';
}
