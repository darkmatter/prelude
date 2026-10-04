# `nix run github:darkmatter/prelude#docs`: the docs of the repository you are
# in, from its git root (or the current directory). A flake that exposes
# `prelude-docs`, as flakeModules.default does, opens its own viewer, with its
# own theme, pages and hero. Any other repository gets Prelude's defaults over
# its README.md and docs/ (docs-fallback.nix), built the same way.
{
  pkgs,
  inputs,
  localFlake,
  ...
}:
pkgs.writeShellApplication {
  name = "docs";
  runtimeInputs = [pkgs.git];
  text = ''
    # Name the flake explicitly: a git checkout reads its tracked files only
    # (never node_modules), and a plain directory must not be mistaken for
    # part of some enclosing git repository.
    if root=$(git rev-parse --show-toplevel 2>/dev/null); then
      flake="git+file://$root"
    else
      root=$(pwd)
      flake="path:$root"
    fi
    export PRELUDE_DOCS_ROOT="$root" PRELUDE_DOCS_FLAKE="$flake"

    # Ask the flake only whether it has docs; building them is `nix run`'s job,
    # so a broken configuration reports its own error instead of falling back.
    configured=false
    if [ -f "$root/flake.nix" ]; then
      configured=$(nix eval --no-warn-dirty --impure --expr '
        let
          flake = builtins.getFlake (builtins.getEnv "PRELUDE_DOCS_FLAKE");
          system = builtins.currentSystem;
          forSystem = outputs:
            if builtins.hasAttr system outputs
            then builtins.getAttr system outputs
            else {};
        in
          forSystem (flake.packages or {}) ? prelude-docs
          || forSystem (flake.legacyPackages or {}) ? prelude-docs
      ')
    fi
    if [ "$configured" = true ]; then
      exec nix run --no-warn-dirty "$flake#prelude-docs" -- "$@"
    fi

    if [ ! -f "$root/README.md" ] && [ ! -d "$root/docs" ]; then
      echo "docs: $root has no README.md or docs/ to show" >&2
      exit 1
    fi
    # Built, then run: `nix run --expr` reads its first argument as an
    # attribute path rather than passing it to the viewer.
    viewer=$(nix build --no-link --print-out-paths --impure --expr '
      import ${./docs-fallback.nix} {
        root = builtins.getEnv "PRELUDE_DOCS_ROOT";
        prelude = ${localFlake};
        pkgs = import ${inputs.nixpkgs} {system = builtins.currentSystem;};
      }
    ')
    exec "$viewer/bin/docs" "$@"
  '';
  meta.description = "Show the docs of the repository you are in";
}
