# shellcheck shell=bash
# shellcheck source-path=SCRIPTDIR
# Interactive lifecycle for shells that source Prelude's generated init.

case "$-" in
  *i*) ;;
  *)
    # Non-interactive. Everything below this guard mutates an interactive shell,
    # so the only work available here is the banner — and only when the loader's
    # preflight explicitly asks for it (direnv evaluates .envrc
    # non-interactively but replays its exports into a terminal-attached shell).
    # A bare non-interactive source, including lorri's shellHook run inside the
    # Nix builder, stays silent.
    [ "${_PRELUDE_PREFLIGHT_RENDER-0}" = 1 ] || return 0
    ;;
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
  key="${LORRI_ENV_HASH:-${IN_LORRI_SHELL:-${IN_NIX_SHELL:-}}}|${_PRELUDE_MOTD_ID:-$_PRELUDE_MOTD}"
  [ "$key" != "${_PRELUDE_MOTD_SHOWN-}" ] || return 0

  if "$_PRELUDE_MOTD" >&2; then
    _PRELUDE_MOTD_SHOWN=$key
  fi
}

if [ "${_PRELUDE_PREFLIGHT_RENDER-0}" = 1 ]; then
  # Preflight render: this shell is not the one the developer types into, so
  # install nothing. Export the marker instead — that is the only way the fact
  # "the banner was already shown for this environment" reaches the interactive
  # shell the loader hands off to, and it is what keeps a `prelude hook` rc line
  # from repeating it.
  _prelude_init_show_motd
  [ -z "${_PRELUDE_MOTD_SHOWN-}" ] || export _PRELUDE_MOTD_SHOWN
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

unset -f _prelude_init_show_motd
