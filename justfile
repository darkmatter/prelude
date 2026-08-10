# Onboarding helper - Add this to any cmd that should run in the devshell
[private]
nixsh:
    @test -n "${IN_NIX_SHELL:-}" || { \
        echo "error: not in the devshell — one-time setup:" >&2; \
        echo "  1. install Nix:  https://nixos.org/download" >&2; \
        echo "  2. install direnv, then run:  direnv allow" >&2; \
        exit 2; }
default: nixsh
  x

wizard: nixsh
  x wizard

demos: nixsh
  x demos

themes: nixsh
  x themes
