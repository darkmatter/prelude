#!/usr/bin/env bash
set -o pipefail
trap 'status=$?; printf "textarea background assertion failed at line %d\n" "$LINENO" >&2; exit "$status"' ERR

blesh=$1
scheme=$2
adapter=$3
mode=${4:-behavior}

: "${TMPDIR:=/tmp}"
export TERM=xterm-256color
export HOME="$TMPDIR/home"
export XDG_CACHE_HOME="$TMPDIR/cache"
export USER=${USER:-prelude-test}
export LANG=${LANG:-C}
mkdir -p "$HOME" "$XDG_CACHE_HOME"
# Library mode loads the real pinned renderer without requiring a controlling
# TTY. Its command-mode loader returns nonzero after a successful load.
# shellcheck source=/dev/null
source "$blesh" --lib || :
type -t ble/textarea#render >/dev/null
# shellcheck source=/dev/null
source "$scheme" || :
type -t ble/contrib/scheme:prelude/initialize >/dev/null
ble/contrib/scheme:prelude/initialize

if [[ $mode == guard ]]; then
  builtin unset -f ble/color/g#setbg
  diagnostic=$(mktemp)
  # shellcheck source=/dev/null
  source "$adapter" 2>"$diagnostic"
  grep -Fq 'disabled Blesh textarea background adapter' "$diagnostic"
  [[ $(declare -f ble/textarea#render) != *'ble/function#advice/.proc'* ]]
  [[ ${_ble_hook_h_PREEXEC[*]-} != *'prelude/window/background/preexec'* ]]
  [[ ${_ble_hook_h_EXIT[*]-} != *'prelude/window/background/restore'* ]]
  [[ ${_ble_hook_h_DETACH[*]-} != *'prelude/window/background/restore'* ]]
  exit 0
fi

# Pre-existing user hooks are registered before the adapter sources so the
# install-order assertions prove unique-prepend (PREEXEC) and unique-append
# (EXIT/DETACH) preserve relative position.
function prelude/test/preexec { :; }
function prelude/test/exit { :; }
function prelude/test/detach { :; }
blehook PREEXEC+='prelude/test/preexec'
blehook EXIT+='prelude/test/exit'
blehook DETACH+='prelude/test/detach'

# The handoff and teardown emit to Blesh's live TUI stdout FD, never to
# inherited command stdout.  Point that FD at a dedicated capture file and
# keep inherited stdout separate so the test proves the contract.
tui_capture=$(mktemp)
inherited=$(mktemp)
exec {_prelude_tui_fd}>"$tui_capture"
_ble_util_fd_tui_stdout=$_prelude_tui_fd

_prelude_window_background_set=1
_prelude_prompt_window_managed=1
_ble_attached=1
# shellcheck source=/dev/null
source "$adapter"
[[ ${_prelude_textarea_background_installed:-0} == 1 ]]

# Install order: PREEXEC is unique-prepended, EXIT/DETACH unique-appended.
[[ ${_ble_hook_h_PREEXEC[0]} == prelude/window/background/preexec ]]
[[ ${_ble_hook_h_PREEXEC[1]} == prelude/test/preexec ]]
[[ ${_ble_hook_h_EXIT[0]} == prelude/test/exit ]]
[[ ${_ble_hook_h_EXIT[1]} == prelude/window/background/restore ]]
[[ ${_ble_hook_h_DETACH[0]} == prelude/test/detach ]]
[[ ${_ble_hook_h_DETACH[1]} == prelude/window/background/restore ]]

# Exact removal and re-install preserve order.
blehook PREEXEC-='prelude/window/background/preexec'
blehook EXIT-='prelude/window/background/restore'
blehook DETACH-='prelude/window/background/restore'
[[ ${#_ble_hook_h_PREEXEC[@]} == 1 ]]
[[ ${_ble_hook_h_PREEXEC[0]} == prelude/test/preexec ]]
[[ ${#_ble_hook_h_EXIT[@]} == 1 ]]
[[ ${_ble_hook_h_EXIT[0]} == prelude/test/exit ]]
[[ ${#_ble_hook_h_DETACH[@]} == 1 ]]
[[ ${_ble_hook_h_DETACH[0]} == prelude/test/detach ]]
blehook PREEXEC+-='prelude/window/background/preexec'
blehook EXIT-+='prelude/window/background/restore'
blehook DETACH-+='prelude/window/background/restore'
[[ ${_ble_hook_h_PREEXEC[0]} == prelude/window/background/preexec ]]
[[ ${_ble_hook_h_PREEXEC[1]} == prelude/test/preexec ]]
[[ ${_ble_hook_h_EXIT[0]} == prelude/test/exit ]]
[[ ${_ble_hook_h_EXIT[1]} == prelude/window/background/restore ]]
[[ ${_ble_hook_h_DETACH[0]} == prelude/test/detach ]]
[[ ${_ble_hook_h_DETACH[1]} == prelude/window/background/restore ]]

# Active and attached: the handoff emits the resolved window SGR to the TUI
# FD, nothing to inherited stdout, and sets the handoff flag.
: >"$tui_capture"
: >"$inherited"
_prelude_window_background_handed_off=0
prelude/window/background/preexec >"$inherited"
[[ $(<"$tui_capture") == "$_prelude_textarea_window_sgr" ]]
[[ ! -s "$inherited" ]]
[[ ${_prelude_window_background_handed_off:-0} == 1 ]]
[[ ${_prelude_window_background_set:-0} == 1 ]]
[[ ${_prelude_prompt_window_managed:-0} == 1 ]]

# Detached: preexec emits nothing, preserves ownership, is-inactive is false.
# Truncate the capture BEFORE the call so an erroneous emission is caught.
_ble_attached=
_prelude_window_background_handed_off=1
: >"$tui_capture"
: >"$inherited"
prelude/window/background/preexec >"$inherited"
[[ ! -s "$tui_capture" ]]
[[ ! -s "$inherited" ]]
[[ ${_prelude_window_background_set:-0} == 1 ]]
[[ ${_prelude_prompt_window_managed:-0} == 1 ]]
if prelude/textarea/background/is-active; then
  exit 1
fi

# Detached restore with a prior handoff emits the original SGR reset to the
# TUI FD and clears the handoff flag.
: >"$tui_capture"
: >"$inherited"
prelude/window/background/restore >"$inherited"
[[ $(<"$tui_capture") == "$_prelude_textarea_original_sgr0" ]]
[[ ! -s "$inherited" ]]
[[ ${_prelude_window_background_handed_off:-0} == 0 ]]

# A second restore with no active handoff emits nothing.
: >"$tui_capture"
prelude/window/background/restore >"$inherited"
[[ ! -s "$tui_capture" ]]

# Reattach restores styling without repainting the MOTD.
_ble_attached=1
prelude/textarea/background/is-active
: >"$tui_capture"
prelude/window/background/preexec >"$inherited"
[[ -s "$tui_capture" ]]
[[ ${_prelude_window_background_handed_off:-0} == 1 ]]

exec {_prelude_tui_fd}>&-
rm -f "$tui_capture" "$inherited"

ble/color/face2g syntax_command
command_g=$ret
ble/color/g2sgr "$command_g"
command_sgr=$ret
ble/color/g2sgr-ansi "$command_g"
command_ansi=$ret

_prelude_textarea_background_rendering=1
ble/color/g2sgr "$command_g"
owned_command_sgr=$ret
ble/color/g2sgr-ansi "$command_g"
owned_command_ansi=$ret
unset _prelude_textarea_background_rendering

[[ $owned_command_sgr == *'48;2;32;32;32'* ]]
[[ $owned_command_sgr != "$command_sgr" ]]
[[ $owned_command_ansi != "$command_ansi" ]]
ble/color/g2sgr "$command_g"
[[ $ret == "$command_sgr" ]]
ble/color/g2sgr-ansi "$command_g"
[[ $ret == "$command_ansi" ]]

ble/color/face2g syntax_error
error_g=$ret
ble/color/g2sgr "$error_g"
error_sgr=$ret
_prelude_textarea_background_rendering=1
ble/color/g2sgr "$error_g"
[[ $ret == "$error_sgr" ]]
unset _prelude_textarea_background_rendering

original_sgr0=$_ble_term_sgr0
render_output=
render_reset=
eval 'function ble/function#advice/original:ble/textarea#render {
  local cols=10 x=3 render_opts=
  local -a DRAW_BUFF=()
  ble/textarea#render/.erase-forward-line.draw
  local IFS=
  render_output=${DRAW_BUFF[*]}
  render_reset=$_ble_term_sgr0
}'
_ble_term_bce=1
ble/textarea#render ''
[[ $render_reset == "$original_sgr0"* ]]
[[ $render_reset == *'48;2;32;32;32'* ]]
[[ $render_output == *'48;2;32;32;32'* ]]
[[ -z $_ble_term_el || $render_output == *"$_ble_term_el"* ]]
[[ $_ble_term_sgr0 == "$original_sgr0" ]]

function probe_non_bce_erase {
  local cols=10 x=3 render_opts=
  local -a DRAW_BUFF=()
  ble/textarea#render/.erase-forward-line.draw
  local IFS=
  non_bce_output=${DRAW_BUFF[*]}
}
_ble_term_bce=
probe_non_bce_erase
[[ $non_bce_output == *'48;2;32;32;32'* ]]
[[ $non_bce_output == *'       '* ]]
[[ -z $_ble_term_el || $non_bce_output != *"$_ble_term_el"* ]]
ech=${_ble_term_ech//'%d'/7}
[[ -z $ech || $non_bce_output != *"$ech"* ]]

COLUMNS=8
_ble_canvas_panel_height=(2 1)
_ble_canvas_panel_class=('ble/textarea' 'prelude/status')
_ble_canvas_panel_tmargin=0
_ble_textarea_panel=0
_ble_canvas_x=0
_ble_canvas_y=0
DRAW_BUFF=()
ble/canvas/panel#clear.draw 0
IFS= textarea_clear=${DRAW_BUFF[*]}
[[ $textarea_clear == *'48;2;32;32;32'* ]]
[[ $textarea_clear == *'        '* ]]
DRAW_BUFF=()
ble/canvas/panel#clear.draw 1
IFS= status_clear=${DRAW_BUFF[*]}
[[ $status_clear != *'48;2;32;32;32'* ]]

DRAW_BUFF=()
ble/canvas/panel#clear-after.draw 0 3 0
IFS= clear_after=${DRAW_BUFF[*]}
[[ $clear_after == *'48;2;32;32;32'* ]]
[[ $clear_after == *'     '* ]]

_ble_canvas_panel_height=(1 1)
DRAW_BUFF=()
ble/canvas/panel#set-height.draw 0 2 clear
IFS= resized=${DRAW_BUFF[*]}
[[ $resized == *'48;2;32;32;32'* ]]
[[ ${_ble_canvas_panel_height[0]} == 2 ]]

# Model the textarea rows rather than merely searching DRAW_BUFF for an SGR.
# Blesh height changes can preserve live rows; painting those cells is data loss.
_test_default_row=DDDDDDDD
_test_window_row=WWWWWWWW
_test_cursor_x=0
_test_cursor_y=0
function ble/canvas/panel#goto.draw {
  _test_cursor_x=${2:-0}
  _test_cursor_y=${3:-0}
  _ble_canvas_x=$_test_cursor_x
  _ble_canvas_y=$_test_cursor_y
}
function ble/canvas/goto.draw {
  _test_cursor_x=${1:-0}
  _test_cursor_y=${2:-0}
  _ble_canvas_x=$_test_cursor_x
  _ble_canvas_y=$_test_cursor_y
}
function ble/canvas/put.draw {
  local payload=$1 width blanks row
  row=${_test_screen[_test_cursor_y]}
  if [[ $payload == "$_ble_term_el" ]]; then
    width=$((COLUMNS - _test_cursor_x))
    _test_screen[_test_cursor_y]=${row:0:_test_cursor_x}${_test_default_row:0:width}${row:_test_cursor_x+width}
    return 0
  fi
  for ((width = COLUMNS - _test_cursor_x; width > 0; width--)); do
    printf -v blanks '%*s' "$width" ''
    [[ $payload == *"$blanks"* ]] && break
  done
  ((width > 0)) || return 0
  _test_screen[_test_cursor_y]=${row:0:_test_cursor_x}${_test_window_row:0:width}${row:_test_cursor_x+width}
}
function ble/canvas/bflush.draw {
  return 0
}
eval 'function ble/function#advice/original:ble/canvas/panel#set-height.draw {
  local index=$1 new_height=$2 opts=${3-}
  local old_height=${_ble_canvas_panel_height[index]} delta row
  ((delta=new_height-old_height))
  if [[ :$opts: == *:clear:* ]]; then
    for ((row=0;row<new_height;row++)); do
      _test_screen[row]=$_test_default_row
    done
  elif ((delta>0)); then
    if [[ :$opts: == *:shift:* ]]; then
      for ((row=old_height-1;row>=0;row--)); do
        _test_screen[row+delta]=${_test_screen[row]}
      done
      for ((row=0;row<delta;row++)); do
        _test_screen[row]=$_test_default_row
      done
    else
      for ((row=old_height;row<new_height;row++)); do
        _test_screen[row]=$_test_default_row
      done
    fi
  elif ((delta<0)) && [[ :$opts: == *:shift:* ]]; then
    for ((row=0;row<new_height;row++)); do
      _test_screen[row]=${_test_screen[row-delta]}
    done
  fi
  _ble_canvas_panel_height[index]=$new_height
  return 0
}'

_test_screen=(PPPPPPPP)
_ble_canvas_panel_height[0]=1
ble/canvas/panel#set-height.draw 0 2
[[ ${_test_screen[0]} == PPPPPPPP ]]
[[ ${_test_screen[1]} == WWWWWWWW ]]

_test_screen=(PPPPPPPP)
_ble_canvas_panel_height[0]=1
ble/canvas/panel#set-height.draw 0 2 shift
[[ ${_test_screen[0]} == WWWWWWWW ]]
[[ ${_test_screen[1]} == PPPPPPPP ]]

_test_screen=(PPPPPPPP)
_ble_canvas_panel_height[0]=1
ble/canvas/panel#set-height.draw 0 2 clear
[[ ${_test_screen[0]} == WWWWWWWW ]]
[[ ${_test_screen[1]} == WWWWWWWW ]]

_test_screen=(PPPPPPPP QQQQQQQQ)
_ble_canvas_panel_height[0]=2
ble/canvas/panel#set-height.draw 0 1
[[ ${_test_screen[0]} == PPPPPPPP ]]

_test_screen=(RRRRRRRR)
_ble_prompt_rps1_shown=1
_ble_prompt_rps1_gbox[3]=1
cols=4
ble/textarea#render/.erase-rprompt
[[ ${_test_screen[0]} == RRRRRWWW ]]

_test_screen=(TTTTTTTT)
_ble_prompt_rps1_shown=1
text=x
cols=8
_ble_textmap_pos[1]='2 0'
ble/textarea#render/.cleanup-trailing-spaces-after-newline
[[ ${_test_screen[0]} == TTWWWWWW ]]

eval 'function ble/function#advice/original:ble/textarea#render {
  observed_ech=$_ble_term_ech
  _ble_textarea_cache=(rendered)
  return 0
}'
_ble_term_ech='ECH%d'
_ble_term_bce=
ble/textarea#render ''
[[ -z $observed_ech ]]
[[ $_ble_term_ech == 'ECH%d' ]]

replayed=0
redrawn=0
eval 'function ble/function#advice/original:ble/textarea#redraw-cache { replayed=1; }'
function ble/textarea#redraw { redrawn=1; }
_ble_textarea_cache=(cached)
_prelude_textarea_background_cache_epoch=1
_prelude_window_background_set=0
ble/textarea#redraw-cache
[[ $redrawn == 1 ]]
[[ $replayed == 0 ]]

replayed=0
redrawn=0
_ble_textarea_cache=(cached)
_prelude_textarea_background_cache_epoch=1
_prelude_window_background_set=1
ble/textarea#redraw-cache
[[ $replayed == 1 ]]
[[ $redrawn == 0 ]]

_prelude_prompt_window_managed=0
_prelude_textarea_background_rendering=1
ble/color/g2sgr "$command_g"
[[ $ret == "$command_sgr" ]]


# Source-idempotency: re-running the installer preserves the confirmed
# install, does not emit the diagnostic, and never duplicates hooks.
_prelude_window_background_handed_off=1
re_source_diagnostic=$(mktemp)
# shellcheck source=/dev/null
source "$adapter" 2>"$re_source_diagnostic"
[[ ${_prelude_textarea_background_installed:-0} == 1 ]]
[[ ! -s "$re_source_diagnostic" ]]
[[ ${_prelude_window_background_handed_off:-0} == 1 ]]
preexec_hooks=0
exit_hooks=0
detach_hooks=0
user_preexec_hooks=0
for hook in "${_ble_hook_h_PREEXEC[@]}"; do
  [[ $hook != prelude/window/background/preexec ]] || ((preexec_hooks += 1))
  [[ $hook != prelude/test/preexec ]] || ((user_preexec_hooks += 1))
done
for hook in "${_ble_hook_h_EXIT[@]}"; do
  [[ $hook != prelude/window/background/restore ]] || ((exit_hooks += 1))
done
for hook in "${_ble_hook_h_DETACH[@]}"; do
  [[ $hook != prelude/window/background/restore ]] || ((detach_hooks += 1))
done
[[ $preexec_hooks == 1 ]]
[[ $exit_hooks == 1 ]]
[[ $detach_hooks == 1 ]]
[[ $user_preexec_hooks == 1 ]]
rm -f "$re_source_diagnostic"