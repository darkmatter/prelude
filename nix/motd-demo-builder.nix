# Final MOTD demo packages shared by runnable examples and docs captures.
{
  pkgs,
  lib,
  currentMotdConfig,
  titlePkg,
}:
let
  ex = import ../src/prelude/examples.nix;
  plib = import ../src/prelude/lib.nix { inherit lib; };
  presets = import ../src/prelude/wizard-presets.nix;
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

  # FIGlet wordmark the wizard would write next to prelude.nix.
  wizardTitle =
    pkgs.runCommand "wizard-preset-title.txt"
      {
        nativeBuildInputs = [
          titlePkg
          pkgs.figlet
        ];
      }
      ''
        cat > recipe.nix <<EOF
        {
          text = ${builtins.toJSON presets.project};
          font = ${builtins.toJSON presets.font};
        }
        EOF
        prelude-title --generate --recipe ./recipe.nix -o "$out"
      '';

  # Active fields only — same surface the wizard sets. Everything else falls
  # through to defaults.nix (as if left commented in the generated module).
  wizardCommandCatalog = lib.listToAttrs (
    lib.imap1 (i: command: {
      name = command.name;
      value = {
        description = command.description or "";
        motd = if presets.motd then i else null;
      }
      // lib.optionalAttrs (command ? exec && command.exec != null && command.exec != "") {
        exec = command.exec;
      };
    }) presets.commands
  );

  wizardMotd = mkMotd {
    inherit (presets) theme colorProfile project;
    commandCatalog = wizardCommandCatalog;
    title = {
      text = wizardTitle;
    };
    # Stack-friendly when paged via .#examples.
    clearScreen = false;
    margin.top = 0;
  };

  # Checked-in full options template (wizard emission with these presets).
  wizardConfigFile = ../nix/internal/example.nix;

  example-default = pkgs.writeShellApplication {
    name = "prelude-example-default";
    text = ''
      usage() {
        cat <<'EOF'
      usage: prelude-example-default [--config]

        Preview what setup produces with stock wizard presets:
          theme/project/commands active, every other option commented at its
          default (see src/prelude/wizard-presets.nix + nix/internal/example.nix).

          (default)   render the MOTD for that config
          --config    print the generated prelude.nix instead
      EOF
      }

      case "''${1:-}" in
        -h | --help)
          usage
          exit 0
          ;;
        --config | -c)
          cat ${wizardConfigFile}
          exit 0
          ;;
        "")
          ;;
        *)
          echo "prelude-example-default: unknown argument: $1" >&2
          usage >&2
          exit 2
          ;;
      esac

      if [ -t 1 ]; then
        printf '\033[2m── setup wizard presets ──\033[0m\n'
        printf '\033[2m   %s · %s · %d commands · config: nix run .#example-default -- --config\033[0m\n\n' \
          ${lib.escapeShellArg presets.project} \
          ${lib.escapeShellArg presets.theme} \
          ${toString (builtins.length presets.commands)}
      fi
      exec ${lib.getExe wizardMotd}
    '';
    meta.description = "Preview MOTD (or config) from stock setup wizard presets";
  };
in
{
  examplePackages =
    lib.mapAttrs' (name: config: lib.nameValuePair "example-${name}" (mkMotd config)) ex.motdDemos
    // {
      inherit example-default;
      example-motd = mkMotd ex.motd;
      example-themes = pkgs.writeShellApplication {
        name = "motd-themes";
        text = ''
          themes=(${lib.concatMapStringsSep " " lib.escapeShellArg plib.themeNames})
          commands=(${
            lib.concatMapStringsSep " " (
              theme: lib.escapeShellArg (lib.getExe themePackages.${theme})
            ) plib.themeNames
          })
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
