{
  lib,
  writeShellApplication,
  figlet,
  jq,
  nix,
  ...
}:
let
  fonts = import ./fonts.nix;
  fontNames = lib.attrNames fonts;
  fontCases = lib.concatStringsSep "\n" (
    lib.mapAttrsToList (name: path: "${lib.escapeShellArg name}) font_path='${path}' ;;") fonts
  );
in
writeShellApplication {
  name = "prelude-title";
  runtimeInputs = [
    figlet
    jq
    nix
  ];
  text = ''
    recipe="title.nix"
    output=""

    while [ "$#" -gt 0 ]; do
      case "$1" in
        --recipe)
          recipe="$2"
          shift 2
          ;;
        --output)
          output="$2"
          shift 2
          ;;
        -h|--help)
          echo "usage: prelude-title [--recipe title.nix] [--output title.txt]"
          exit 0
          ;;
        *)
          echo "prelude-title: unknown argument: $1" >&2
          exit 2
          ;;
      esac
    done

    if [ ! -f "$recipe" ]; then
      echo "prelude-title: recipe not found: $recipe" >&2
      exit 1
    fi
    if [ -z "$output" ]; then
      output="$(dirname "$recipe")/title.txt"
    fi

    config="$(nix-instantiate --eval --strict --json "$recipe")"
    text="$(printf '%s' "$config" | jq -er '.text | select(type == "string" and length > 0)')" || {
      echo "prelude-title: title.nix must define a non-empty text string" >&2
      exit 1
    }
    font="$(printf '%s' "$config" | jq -er '.font | select(type == "string" and length > 0)')" || {
      echo "prelude-title: title.nix must define a font string" >&2
      exit 1
    }

    case "$font" in
      ${fontCases}
      *)
        echo "prelude-title: unknown font '$font' (expected one of: ${lib.concatStringsSep ", " fontNames})" >&2
        exit 1
        ;;
    esac

    mkdir -p "$(dirname "$output")"
    figlet -f "$font_path" -- "$text" > "$output"
    echo "wrote $output"
  '';
  meta.description = "Generate a checked-in Prelude MOTD title from title.nix";
}
