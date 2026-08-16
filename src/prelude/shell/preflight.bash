# shellcheck shell=bash
# shellcheck source-path=SCRIPTDIR
# Loader activation, emitted verbatim by `prelude preflight` and eval'd by the
# consumer from the one place their loader actually runs:
#
#   .envrc      use flake
#               eval "$(prelude-preflight)"
#
#   shellHook   eval "$(prelude-preflight)"
#
# One line covers both because this code branches on the shell it lands in, not
# on the loader that produced it. Like `prelude hook`, it names no build-time
# paths: every project-specific fact — including which MOTD binary to run and
# how the once-per-environment key is computed — belongs to the init file
# PRELUDE_INIT points at. That single owner is what keeps the banner from
# rendering twice when both this and `prelude hook` are in play.

if [ -z "${PRELUDE_INIT-}" ]; then
  printf '%s\n' 'prelude: PRELUDE_INIT is unset — no devshell environment is loaded' >&2
else
  case "$-" in
    *i*)
      # Interactive: the init installs the prompt, completion, and MOTD. It is
      # idempotent and keys the banner on environment identity, so re-sourcing
      # on rebuild or re-entry is both cheap and correct.
      # shellcheck source=./init.bash
      . "$PRELUDE_INIT"
      ;;
    *)
      # Non-interactive. Two very different loaders land here:
      #
      #   direnv  evaluates .envrc non-interactively, but replays the exports it
      #           captures into a terminal-attached shell. The banner belongs
      #           here, and the exported marker suppresses a second render from
      #           a `prelude hook` rc line.
      #   lorri   runs shellHook inside the Nix builder, where output goes to
      #           the build log and nobody sees it (nix-community/lorri#159).
      #           Rendering there would waste the banner and — worse — export a
      #           marker that silences the real shell.
      #
      # DIRENV_IN_ENVRC is set only while direnv executes .envrc, so it is the
      # exact discriminator. Ask the init to render; it owns the MOTD path, the
      # key, and the PRELUDE_INIT_QUIET opt-out.
      if [ -n "${DIRENV_IN_ENVRC-}" ]; then
        # A plain shell variable, not a prefix assignment: `.` is a special
        # builtin, so `VAR=1 . file` can leave VAR in the environment that
        # direnv then captures. Set it, source, unset.
        _PRELUDE_PREFLIGHT_RENDER=1
        # shellcheck source=./init.bash
        . "$PRELUDE_INIT"
        unset _PRELUDE_PREFLIGHT_RENDER
      fi
      ;;
  esac
fi
