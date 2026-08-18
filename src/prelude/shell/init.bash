# shellcheck shell=bash
# shellcheck source-path=SCRIPTDIR
# Interactive lifecycle for shells that source Prelude's generated init.

case "$-" in
  *i*) ;;
  *)
    # Non-interactive. Everything below this guard mutates an interactive shell,
    # so the only work available here is the banner, and only when preflight
    # explicitly asks for it. Direnv evaluates .envrc non-interactively with
    # terminal-visible stderr; a bare non-interactive source, including lorri's
    # shellHook run inside the Nix builder, stays silent.
    [ "${_PRELUDE_PREFLIGHT_RENDER-0}" = 1 ] || return 0
    ;;
esac

# Render the MOTD whenever an activation path asks. Prompt hooks separately
# avoid sourcing an unchanged PRELUDE_INIT on every ordinary prompt.
_prelude_init_show_motd() {
  [ -n "${_PRELUDE_MOTD-}" ] || return 0
  [ -z "${PRELUDE_INIT_QUIET:-}" ] || return 0

  "$_PRELUDE_MOTD" >&2 || return 0
}

if [ "${_PRELUDE_PREFLIGHT_RENDER-0}" = 1 ]; then
  # This shell is not the one the developer types into, so render only; install
  # no prompt, completion, or shell hooks, and carry no render state forward.
  _prelude_init_show_motd
  unset -f _prelude_init_show_motd
  return 0
fi

# Everything below mutates the shell irreversibly (ble.sh attaches, Starship
# installs its hooks, completion registers). Those must happen at most once per
# shell; the MOTD above is deliberately outside this guard so it can re-render.
# Not exported: child shells initialize themselves.
if [ -z "${_PRELUDE_INIT_DONE-}" ]; then
  _PRELUDE_INIT_DONE=1

  if [ "${_PRELUDE_PROMPT_ENABLED-0}" != 1 ]; then
    # MOTD-only build: prelude.prompt is off, so Starship, ble.sh, and
    # completion are deliberately absent from the closure and there is no
    # prompt runtime to install. The MOTD is the entire surface.
    _prelude_init_show_motd
  elif [ -n "${BASH_VERSION-}" ]; then
    # shellcheck source=./catalogue.bash
    . "$_PRELUDE_SHELL_RUNTIME/catalogue.bash"
    # bash-init.bash renders the MOTD itself, after ble.sh has attached.
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
if [ -n "${PRELUDE_INIT-}" ]; then
  _PRELUDE_INIT_LOADED=$PRELUDE_INIT
fi

unset -f _prelude_init_show_motd
