# shellcheck shell=bash
# shellcheck source-path=SCRIPTDIR
# Loader activation, emitted verbatim by `prelude preflight` and eval'd by a
# custom shellHook:
#
#   shellHook   eval "$(prelude-preflight)"
#
# Conventional nix-direnv consumers need only `use flake`: the generated init
# recognizes DIRENV_IN_ENVRC when the cached shellHook sources it. Like
# `prelude hook`, this snippet names no build-time paths: every project-specific
# fact, including which MOTD binary to run, belongs to the init file
# PRELUDE_INIT points at.

if [ -z "${PRELUDE_INIT-}" ]; then
  printf '%s\n' 'prelude: PRELUDE_INIT is unset — no devshell environment is loaded' >&2
else
  case "$-" in
    *i*)
      # Interactive: the init installs the prompt and completion, and this
      # explicit activation renders the MOTD.
      # shellcheck source=./init.bash
      . "$PRELUDE_INIT"
      ;;
    # Non-interactive shellHooks, including lorri's builder context, stay
    # silent. direnv activation is owned directly by the generated init.
    *) ;;
  esac
fi
