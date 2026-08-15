# shellcheck shell=bash
# Prompt trampoline for Bash, emitted by `prelude hook bash`.
#
# Loaders disagree about what they will execute. `nix develop` runs shellHook,
# direnv re-emits setup-hook exports, and lorri applies environment variables
# only — it never runs shellHook (nix-community/lorri#159). PRELUDE_INIT is an
# ordinary exported variable, so every loader carries it, which makes this the
# one activation path that works the same under all three.
#
# Deliberately tiny and version-independent: all behavior lives in the file
# PRELUDE_INIT points at, which each project builds for itself. That keeps this
# snippet stable once it is in an rc file, even as Prelude changes.

_prelude_hook() {
  if [ -z "${PRELUDE_INIT-}" ]; then
    # Left the project. Forget both markers so returning re-renders the MOTD:
    # _PRELUDE_INIT_LOADED gates re-sourcing, and _PRELUDE_MOTD_SHOWN gates the
    # MOTD itself for loaders that have no hook. Leaving the latter set would
    # make re-entry silent, since the environment identity is unchanged.
    unset _PRELUDE_INIT_LOADED _PRELUDE_MOTD_SHOWN
    return 0
  fi

  # PRELUDE_INIT is a store path, so it changes whenever the project's shell
  # configuration is rebuilt. Comparing it keeps re-entry and rebuilds cheap
  # without sourcing anything on every prompt.
  if [ "$PRELUDE_INIT" = "${_PRELUDE_INIT_LOADED-}" ]; then
    return 0
  fi
  _PRELUDE_INIT_LOADED=$PRELUDE_INIT

  # shellcheck source=/dev/null
  . "$PRELUDE_INIT"
}

# Appended, not prepended: the environment loader (lorri, direnv) has to apply
# PRELUDE_INIT before this runs, otherwise activation lags one prompt behind.
# Bash 5.1 allows PROMPT_COMMAND to be an array, so handle both shapes.
if [[ "$(declare -p PROMPT_COMMAND 2>/dev/null)" == "declare -a"* ]]; then
  if [[ " ${PROMPT_COMMAND[*]} " != *" _prelude_hook "* ]]; then
    PROMPT_COMMAND+=(_prelude_hook)
  fi
elif [[ ";${PROMPT_COMMAND-};" != *";_prelude_hook;"* ]]; then
  PROMPT_COMMAND="${PROMPT_COMMAND:+$PROMPT_COMMAND;}_prelude_hook"
fi
