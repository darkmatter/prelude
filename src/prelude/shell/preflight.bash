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
# paths: every project-specific fact, including which MOTD binary to run,
# belongs to the init file PRELUDE_INIT points at. Render requests are
# independent and carry no coordination state between loaders.

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
    *)
      # Non-interactive. Two very different loaders land here:
      #
      #   direnv  evaluates .envrc non-interactively with terminal-visible
      #           stderr, so the banner belongs here.
      #   lorri   runs shellHook inside the Nix builder, where output goes to
      #           the build log and nobody sees it (nix-community/lorri#159),
      #           so rendering there would waste the banner.
      #
      # DIRENV_IN_ENVRC is set only while direnv executes .envrc, so it is the
      # exact discriminator. Ask the init to render; it owns the MOTD path and
      # the PRELUDE_INIT_QUIET opt-out.
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
