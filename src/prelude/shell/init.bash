# shellcheck shell=bash
# shellcheck source-path=SCRIPTDIR
# Interactive lifecycle for shells that source Prelude's generated init.

case "$-" in
  *i*) ;;
  *) return 0 ;;
esac

# Render the MOTD once per active environment. `prelude hook` re-sources this
# file from the prompt, so the guard keys on environment identity rather than
# "once per shell": entering a project, rebuilding it, or leaving and coming
# back all produce a new key, while ordinary prompts do not.
_prelude_init_show_motd() {
  [ -n "${_PRELUDE_MOTD-}" ] || return 0
  [ -z "${PRELUDE_INIT_QUIET:-}" ] || return 0

  # LORRI_ENV_HASH changes on every rebuild. direnv and `nix develop` do not
  # set it, so fall back to the shell-file markers they do export.
  local key
  key="${LORRI_ENV_HASH:-${IN_LORRI_SHELL:-${IN_NIX_SHELL:-}}}|$_PRELUDE_MOTD"
  [ "$key" != "${_PRELUDE_MOTD_SHOWN-}" ] || return 0

  if "$_PRELUDE_MOTD" >&2; then
    _PRELUDE_MOTD_SHOWN=$key
  fi
}

# Everything below mutates the shell irreversibly (ble.sh attaches, Starship
# installs its hooks, completion registers). Those must happen at most once per
# shell; the MOTD above is deliberately outside this guard so it can re-render.
# Not exported: child shells initialize themselves.
if [ -z "${_PRELUDE_INIT_DONE-}" ]; then
  _PRELUDE_INIT_DONE=1

  if [ -n "${BASH_VERSION-}" ]; then
    # shellcheck source=./catalogue.bash
    . "$_PRELUDE_SHELL_RUNTIME/catalogue.bash"
    # shellcheck source=./bash-init.bash
    . "$_PRELUDE_SHELL_RUNTIME/bash-init.bash"
  elif [ -n "${ZSH_VERSION-}" ]; then
    eval "$("$_PRELUDE_STARSHIP" init zsh)"
    _prelude_init_show_motd
  fi
else
  # Re-entry: the shell is already set up, so only the MOTD can change.
  _prelude_init_show_motd
fi

unset -f _prelude_init_show_motd
