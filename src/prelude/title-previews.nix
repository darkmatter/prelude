{
  lib,
  writeShellApplication,
  figlet,
  ...
}:
let
  fonts = import ./fonts.nix;
  renderers = lib.concatStringsSep "\n" (
    lib.mapAttrsToList (name: path: ''
      printf '\n===== %s =====\n\n' ${lib.escapeShellArg name}
      figlet -f '${path}' -- "$text"
    '') fonts
  );
in
writeShellApplication {
  name = "prelude-title-previews";
  runtimeInputs = [ figlet ];
  text = ''
    if [ "$#" -eq 0 ]; then
      echo "usage: prelude-title-previews TEXT..." >&2
      exit 2
    fi
    text="$*"
    ${renderers}
  '';
  meta.description = "Preview text in every bundled Prelude FIGlet font";
}
