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

# Cursor: blinking vertical bar (DECSCUSR 5) in the emacs keymap, and the same
# shape whenever ble.sh yields the terminal to external commands.
ble-bind -m emacs --cursor 5
bleopt term_cursor_external=5

# shellcheck source=./completion.bash
. "$_PRELUDE_SHELL_RUNTIME/completion.bash"
# shellcheck source=./status.bash
. "$_PRELUDE_SHELL_RUNTIME/status.bash"
# shellcheck source=./status-cap.bash
. "$_PRELUDE_SHELL_RUNTIME/status-cap.bash"

complete -F _prelude_complete_x x
if ((${#_prelude_catalogue_direct_names[@]})); then
  complete -F _prelude_complete_direct "${_prelude_catalogue_direct_names[@]}"
fi

eval "$("$_PRELUDE_STARSHIP" init bash)"
# Submitted history rewrites only the left PS1 chrome (muted palette). The
# typed command text and command stdout/stderr are not restyled. Bake the
# config path into the leave-rewrite string so it survives init env cleanup.
# Falls back to the character module when no final config is generated
# (user-owned configFile).
if [ -n "${_PRELUDE_STARSHIP_FINAL_CONFIG-}" ]; then
  # STARSHIP_SHELL=plain omits bash \[\] non-print markers; ble parses raw ANSI.
  # shellcheck disable=SC2016,SC2089,SC2090
  bleopt prompt_ps1_final='$(STARSHIP_CONFIG='"$(printf '%q' "$_PRELUDE_STARSHIP_FINAL_CONFIG")"' STARSHIP_SHELL=plain starship prompt --terminal-width="$COLUMNS")'
else
  # shellcheck disable=SC2016
  bleopt prompt_ps1_final='$(starship module character)'
fi

if [ "$_prelude_status_enabled" = 1 ]; then
  # Blesh's status panel is one row. A blank sibling panel sits immediately
  # above it so the gap is docked to status, not glued under ╰─ (cursor stays
  # on the framed input row). configFile keeps statusEnabled=0 and skips this.
  prelude/status/cap/install || :
  blehook PRECMD!='prelude/status/update'
  prelude/status/update
  bleopt prompt_status_align=left
  bleopt prompt_status_line='\q{prelude/status}'
fi


_prelude_init_show_motd

if [ -z "${BLE_ATTACHED-}" ]; then
  ble-attach
fi
