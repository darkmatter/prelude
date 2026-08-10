# Agent-facing Markdown router. The detailed material stays in docs/ so it can
# be reviewed, linked, and consumed without a terminal UI or a source checkout.
{pkgs, ...}: let
  docs = {
    intro = ../docs/skill.md;
    install = ../docs/your-own-repo.md;
    options = ../docs/reference/options.md;
    commands = ../docs/commands.md;
    configuration = ../docs/configuration.md;
    commandConventions = ../docs/guides/command-conventions.md;
    titleRendering = ../docs/guides/title-rendering.md;
  };
in
  pkgs.writeShellApplication {
    name = "skill";
    runtimeInputs = [pkgs.coreutils];
    text = ''
      reject() {
        printf 'skill: %s\n' "$1" >&2
        printf "Run 'skill list' to see supported documentation topics.\n" >&2
        exit 64
      }

      case "$#" in
        0)
          cat ${docs.intro}
          ;;
        1)
          case "$1" in
            list)
              cat ${docs.intro}
              ;;
            install)
              cat ${docs.install}
              ;;
            options)
              cat ${docs.options}
              ;;
            commands)
              cat ${docs.commands}
              ;;
            configuration)
              cat ${docs.configuration}
              ;;
            guide)
              reject "guide requires a name"
              ;;
            *)
              reject "unknown topic '$1'"
              ;;
          esac
          ;;
        2)
          if [ "$1" != guide ]; then
            reject "unexpected arguments after '$1'"
          fi

          case "$2" in
            command-conventions)
              cat ${docs.commandConventions}
              ;;
            title-rendering)
              cat ${docs.titleRendering}
              ;;
            *)
              reject "unknown guide '$2'"
              ;;
          esac
          ;;
        *)
          reject "unexpected arguments"
          ;;
      esac
    '';
    meta.description = "Print agent-oriented Prelude documentation as Markdown";
  }
