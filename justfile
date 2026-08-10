default: nixsh
  @exec x

# Onboarding helper - Add this to any cmd that should run in the devshell
[private]
nixsh:
    @test -n "${IN_NIX_SHELL:-}" || { \
        echo "error: not in the devshell — one-time setup:" >&2; \
        echo "  1. install Nix:  https://nixos.org/download" >&2; \
        echo "  2. install direnv, then run:  direnv allow" >&2; \
        exit 2; }

# Launches the devshell
shell:
  nix develop

# Launches a setup wizard to generate a new prelude config in your project
wizard: nixsh
  x wizard

# Demo the different views
demos: nixsh
  x demos

# Preview all the themes
themes: nixsh
  x themes


[group('development')]
ci: nixsh
  x build
  x check
