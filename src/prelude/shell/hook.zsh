# Prompt trampoline for zsh, emitted by `prelude hook zsh`.
#
# Loaders disagree about *where* they run things. `nix develop` runs shellHook
# in your shell; direnv re-emits setup-hook exports; lorri runs shellHook inside
# the Nix builder — non-interactive, in the build directory — and replays only
# the variables it exported (nix-community/lorri#159). PRELUDE_INIT is an
# ordinary exported variable, so every loader carries it, which makes this the
# one activation path that behaves the same under all three.
#
# Deliberately tiny and version-independent: all behavior lives in the file
# PRELUDE_INIT points at, which each project builds for itself. That keeps this
# snippet stable once it is in an rc file, even as Prelude changes.

_prelude_hook() {
  if [ -z "${PRELUDE_INIT-}" ]; then
    # Left the project. Forget the loaded init path so returning activates the
    # environment again.
    unset _PRELUDE_INIT_LOADED
    return 0
  fi

  # PRELUDE_INIT is a store path, so it changes whenever the project's shell
  # configuration is rebuilt. Comparing it keeps re-entry and rebuilds cheap
  # without sourcing anything on every prompt.
  if [ "$PRELUDE_INIT" = "${_PRELUDE_INIT_LOADED-}" ]; then
    return 0
  fi

  # The init records its path only after it has run, so a source failure is
  # retried on the next prompt.
  . "$PRELUDE_INIT"
}

# Appended, not prepended: the environment loader (lorri, direnv) has to apply
# PRELUDE_INIT before this runs, otherwise activation lags one prompt behind.
typeset -ag precmd_functions
if (( ! ${precmd_functions[(I)_prelude_hook]} )); then
  precmd_functions+=(_prelude_hook)
fi
