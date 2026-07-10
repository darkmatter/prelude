# `nix run .#previews` — build the render checks (their outputs ARE the
# previews) and display each one. Render checks are discovered from the
# checks attrset by their `-render(s)` suffix, so the two never drift.
{ pkgs, lib, checks, ... }:
let
  renderNames = lib.filter
    (n: lib.hasSuffix "-renders" n || lib.hasSuffix "-render" n)
    (lib.attrNames checks);

  # Biggest output last.
  previewCheckNames =
    lib.filter (n: n != "examples-render") renderNames
    ++ lib.optional (checks ? examples-render) "examples-render";
in
pkgs.writeShellApplication {
  name = "previews";
  text = ''
    # Check names may be passed as args; defaults to every render check.
    checks=(${lib.concatMapStringsSep " " lib.escapeShellArg previewCheckNames})
    if [ "$#" -gt 0 ]; then
      checks=("$@")
    fi
    for chk in "''${checks[@]}"; do
      printf '\n\033[1m── %s\033[0m  (nix build .#checks.${pkgs.system}.%s)\n\n' "$chk" "$chk"
      out=$(nix build --no-link --print-out-paths ".#checks.${pkgs.system}.$chk")
      cat "$out"
      echo ""
    done
  '';
}
