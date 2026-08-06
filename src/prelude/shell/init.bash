# shellcheck shell=bash
# shellcheck source-path=SCRIPTDIR
# Interactive lifecycle shared by nix develop and Prelude's Zellij shell.

case "$-" in
  *i*) ;;
  *) return 0 ;;
esac

# Deliberately not exported: child shells initialize themselves, while sourcing
# prelude-init repeatedly in the same shell is harmless.
if [ -n "${_PRELUDE_INIT_DONE-}" ]; then
  return 0
fi
_PRELUDE_INIT_DONE=1

_prelude_init_show_motd() {
  # A configured window fill is only a capability until the MOTD succeeds.
  # Quiet pin panes intentionally skip it and must keep Blesh on its static bg.
  _prelude_window_background_set=0
  if [ -n "${_PRELUDE_MOTD-}" ] &&
    [ -z "${PRELUDE_INIT_QUIET:-}" ] &&
    [ -z "${_PRELUDE_MOTD_SHOWN-}" ] &&
    "$_PRELUDE_MOTD" >&2; then
    _PRELUDE_MOTD_SHOWN=1
    _prelude_window_background_set=${_PRELUDE_WINDOW_BACKGROUND_SET:-0}
  fi

  if [ -n "${BASH_VERSION-}" ] &&
    type prelude/status/cap/refresh-face >/dev/null 2>&1; then
    prelude/status/cap/refresh-face "$_prelude_window_background_set"
  fi
}

if [ -n "${BASH_VERSION-}" ]; then
  # shellcheck source=./catalogue.bash
  . "$_PRELUDE_SHELL_RUNTIME/catalogue.bash"
  # shellcheck source=./bash-init.bash
  . "$_PRELUDE_SHELL_RUNTIME/bash-init.bash"
elif [ -n "${ZSH_VERSION-}" ]; then
  eval "$("$_PRELUDE_STARSHIP" init zsh)"
  _prelude_init_show_motd
fi

unset -f _prelude_init_show_motd
