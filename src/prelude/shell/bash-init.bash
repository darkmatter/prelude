# shellcheck shell=bash
# Bash-specific bootstrap. Runtime behavior is split into completion and status.

# Load programmable completion before ble.sh so ble can integrate it.
if [ -z "${BASH_COMPLETION_VERSINFO-}" ] && [ -r "$_PRELUDE_BASH_COMPLETION" ]; then
  # shellcheck source=/dev/null
  source "$_PRELUDE_BASH_COMPLETION"
fi

# Delayed attach ensures Starship and status widgets exist before the first
# prompt is painted.
if [ -z "${BLE_VERSION-}" ]; then
  # shellcheck source=/dev/null
  source "$_PRELUDE_BLESH" --attach=none
fi

if [ -n "${_PRELUDE_DARWIN-}" ]; then
  # Prefer BSD stty on Darwin; Nix coreutils stty breaks termios here.
  ble/bin/stty() { command /bin/stty "$@"; }
  ble/bin/stty/.instantiate() { return 0; }
fi

# Prelude is a regular ble.sh contrib color scheme. Prepending the generated
# runtime keeps the user's existing import path available for other contribs.
bleopt import_path="$_PRELUDE_SHELL_RUNTIME/contrib${bleopt_import_path:+:$bleopt_import_path}"
bleopt color_scheme=prelude

# shellcheck source=./completion.bash
. "$_PRELUDE_SHELL_RUNTIME/completion.bash"
# shellcheck source=./status.bash
. "$_PRELUDE_SHELL_RUNTIME/status.bash"

complete -F _prelude_complete_x x
if ((${#_prelude_catalogue_direct_names[@]})); then
  complete -F _prelude_complete_direct "${_prelude_catalogue_direct_names[@]}"
fi

eval "$("$_PRELUDE_STARSHIP" init bash)"
blehook PRECMD-='prelude/status/update' 2>/dev/null || true
if [ "$_prelude_status_enabled" = 1 ]; then
  # Starship installs its native ble.sh PRECMD hook during initialization. Keep
  # Prelude after it so the status line reuses Starship's rendered right prompt.
  blehook PRECMD!='prelude/status/update'
  bleopt prompt_status_align=left
  bleopt prompt_status_line='\q{prelude/status}'
else
  bleopt prompt_status_line=
fi

_prelude_init_show_motd

if [ -z "${BLE_ATTACHED-}" ]; then
  ble-attach
fi
