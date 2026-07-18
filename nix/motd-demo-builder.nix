# Final MOTD demo packages shared by runnable examples and docs captures.
{ pkgs
, lib
, currentMotdConfig
}:
let
  ex = import ../src/prelude/examples.nix;
  plib = import ../src/prelude/lib.nix { inherit lib; };
  mkMotd = import ../src/prelude/motd.nix {
    inherit lib;
    inherit (pkgs)
      writeShellApplication
      writeText
      buildGoModule
      ;
  };
  themePackages = lib.genAttrs plib.themeNames (
    theme:
    mkMotd (
      currentMotdConfig
      // {
        inherit theme;
        # The pager owns clearing between pages; banner content and layout stay
        # identical to the current module-produced MOTD.
        clearScreen = false;
      }
    )
  );
in
{
  examplePackages =
    lib.mapAttrs' (name: config: lib.nameValuePair "example-${name}" (mkMotd config)) ex.motdDemos
    // {
      example-motd = mkMotd ex.motd;
      example-themes = pkgs.writeShellApplication {
        name = "motd-themes";
        text = ''
          themes=(${lib.concatMapStringsSep " " lib.escapeShellArg plib.themeNames})
          commands=(${lib.concatMapStringsSep " " (theme: lib.escapeShellArg (lib.getExe themePackages.${theme})) plib.themeNames})
          n=''${#themes[@]}

          if [ -t 0 ] && [ -t 1 ]; then
            i=0
            while :; do
              printf '\033[2J\033[H'
              PRELUDE_MOTD_CONFIG="" "''${commands[i]}" || true
              printf '\n\033[2mtheme %s · %d/%d · ← → change · q quit\033[0m\n' \
                "''${themes[i]}" "$((i + 1))" "$n"
              IFS= read -rsn1 key || break
              case "$key" in
                q | Q) break ;;
                $'\x1b')
                  rest=""
                  IFS= read -rsn2 -t 1 rest || true
                  case "$rest" in
                    '[C') i=$(((i + 1) % n)) ;;
                    '[D') i=$(((i - 1 + n) % n)) ;;
                    "") break ;;
                    *) : ;;
                  esac
                  ;;
                l | n | ' ') i=$(((i + 1) % n)) ;;
                h | p) i=$(((i - 1 + n) % n)) ;;
                *) : ;;
              esac
            done
          else
            i=0
            while [ "$i" -lt "$n" ]; do
              printf '\n\033[1m── theme %s\033[0m  (%d/%d)\n\n' \
                "''${themes[i]}" "$((i + 1))" "$n"
              PRELUDE_MOTD_CONFIG="" "''${commands[i]}"
              i=$((i + 1))
            done
          fi
        '';
        meta.description = "Page through the current Prelude MOTD in every theme";
      };
    };
}
