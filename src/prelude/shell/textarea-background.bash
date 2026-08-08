# shellcheck shell=bash
# Blesh owns these renderer globals; the guarded installer validates them first.
# shellcheck disable=SC2154
# Paint only Blesh's editable textarea with Prelude's MOTD window color.
#
# The pinned renderer resets SGR before both text and erase operations. This
# adapter composes the window background at those two seams and never changes
# the terminal's default background or unrelated Blesh panels.

# State reset is unconditional for the initial source but must not wipe a
# confirmed install on re-source: the adapter short-circuits installation and
# preserves its renderer state.
if [[ ${_prelude_textarea_background_installed:-0} != 1 ]]; then
  _prelude_textarea_background_installed=0
  _prelude_textarea_background_epoch=0
  _prelude_textarea_background_cache_epoch=0
fi

function prelude/textarea/background/is-active {
  [[ ${_prelude_window_background_set:-0} == 1 &&
    ${_prelude_prompt_window_managed:-0} == 1 &&
    ${_ble_attached:-0} == 1 ]]
}

function prelude/textarea/background/validate {
  local name definition
  for name in \
    ble/function#advice \
    ble/function#advice/do \
    ble/textarea#render \
    ble/textarea#redraw \
    ble/textarea#redraw-cache \
    ble/textarea#invalidate \
    ble/textarea#render/.erase-forward-line.draw \
    ble/textarea#render/.erase-rprompt \
    ble/textarea#render/.cleanup-trailing-spaces-after-newline \
    ble/textarea#slice-text-buffer \
    ble/canvas/panel#clear.draw \
    ble/canvas/panel#clear-after.draw \
    ble/canvas/panel#set-height.draw \
    ble/canvas/panel#goto.draw \
    ble/canvas/goto.draw \
    ble/canvas/put.draw \
    ble/color/face2g \
    ble/color/face2sgr \
    ble/color/g2sgr \
    ble/color/g2sgr-ansi \
    ble/color/g#getbg \
    ble/color/g#setbg \
    ble/string#reserve-prototype \
    ble/string#split-words; do
    ble/is-function "$name" || return 1
    definition=$(declare -f "$name")
    case $name in
      "ble/function#advice" | "ble/function#advice/do") ;;
      *)
        [[ $definition != *'ble/function#advice/.proc'* ]] || return 1
        ;;
    esac
  done

  [[ ${_ble_textarea_panel+x} && ${_ble_term_bce+x} &&
    ${_ble_term_sgr0+x} && ${_ble_term_el+x} && ${_ble_term_ech+x} &&
    ${_ble_term_cub+x} && ${_ble_canvas_x+x} && ${_ble_canvas_y+x} ]] || return 1

  definition=$(declare -f ble/textarea#render/.erase-forward-line.draw)
  [[ $definition == *'_ble_term_sgr0'* && $definition == *'_ble_term_el'* ]] || return 1
  definition=$(declare -f ble/textarea#render/.erase-rprompt)
  [[ $definition == *'_ble_term_el'* && $definition == *'ble/canvas/bflush.draw'* ]] || return 1
  definition=$(declare -f ble/textarea#render/.cleanup-trailing-spaces-after-newline)
  [[ $definition == *'_ble_term_el'* && $definition == *'_ble_textmap_pos'* ]] || return 1
  definition=$(declare -f ble/textarea#slice-text-buffer)
  [[ $definition == *'_ble_term_ech'* && $definition == *'_ble_term_cr'* ]] || return 1
  definition=$(declare -f ble/textarea#redraw-cache)
  [[ $definition == *'_ble_textarea_cache'* && $definition == *'ble/canvas/panel#clear.draw'* ]] || return 1
  definition=$(declare -f ble/canvas/panel#set-height.draw)
  [[ $definition == *'_ble_canvas_panel_height'* && $definition == *'ble/canvas/panel#clear.draw'* ]] || return 1
}

function prelude/textarea/background/fill-width.draw {
  local width=$1
  ((width > 0)) || return 0
  ble/string#reserve-prototype "$width"
  ble/canvas/put.draw "$_prelude_textarea_window_sgr${_ble_string_prototype::width}$_prelude_textarea_original_sgr0${_ble_term_cub//'%d'/$width}"
}

function prelude/textarea/background/fill-rows.draw {
  local index=$1 start=$2 count=$3 cols=${COLUMNS:-80} row
  ((count > 0)) || return 0
  for ((row = start; row < start + count; row++)); do
    ble/canvas/panel#goto.draw "$index" 0 "$row" sgr0
    prelude/textarea/background/fill-width.draw "$cols"
  done
}

function prelude/textarea/background/fill-panel.draw {
  local index=$1 height=$2
  prelude/textarea/background/fill-rows.draw "$index" 0 "$height"
  ble/canvas/panel#goto.draw "$index" 0 0 sgr0
}

function prelude/textarea/background/fill-after.draw {
  local index=$1 x=$2 y=$3
  local height=${_ble_canvas_panel_height[index]} cols=${COLUMNS:-80} row start width
  ((y < height)) || return 1
  for ((row = y; row < height; row++)); do
    if ((row == y)); then
      start=$x
    else
      start=0
    fi
    width=$((cols - start))
    ble/canvas/panel#goto.draw "$index" "$start" "$row" sgr0
    prelude/textarea/background/fill-width.draw "$width"
  done
  ble/canvas/panel#goto.draw "$index" "$x" "$y" sgr0
}

function prelude/textarea/background/render {
  local active=0
  if prelude/textarea/background/is-active; then
    active=1
    local _prelude_textarea_background_rendering=1
    local _ble_term_sgr0=$_prelude_textarea_original_sgr0$_prelude_textarea_window_sgr
    # Every ECH inside the pinned renderer must take its literal-space path
    # on non-BCE terminals: they erase with their default background.
    ((_ble_term_bce)) || local _ble_term_ech=
  fi
  ble/function#advice/do || :

  local render_exit=$ADVICE_EXIT
  # Blesh populates the cache inside render; tag those bytes with the ownership
  # state under which their reset sequences were generated.
  if [[ ${_ble_textarea_cache[0]+set} ]]; then
    _prelude_textarea_background_cache_epoch=$active
  fi
  ADVICE_EXIT=$render_exit
}

function prelude/textarea/background/color {
  if ! prelude/textarea/background/is-active ||
    [[ ${_prelude_textarea_background_rendering:-0} != 1 ]]; then
    ble/function#advice/do || :
    return
  fi

  local g=${ADVICE_WORDS[1]:-0} background
  ble/color/g#getbg "$g"
  background=$ret
  if ((background < 0)); then
    ble/color/g#setbg g "$_prelude_textarea_window_bg"
    ADVICE_WORDS[1]=$g
  fi
  ble/function#advice/do || :
}

function prelude/textarea/background/erase-forward-line {
  if ! prelude/textarea/background/is-active || ((_ble_term_bce)); then
    ble/function#advice/do || :
    return
  fi

  local width=$((cols - x))
  prelude/textarea/background/fill-width.draw "$width"
  ADVICE_EXIT=0
}

function prelude/textarea/background/erase-rprompt {
  if ! prelude/textarea/background/is-active || ((_ble_term_bce)); then
    ble/function#advice/do || :
    return
  fi
  if [[ ! $_ble_prompt_rps1_shown ]]; then
    ADVICE_EXIT=0
    return
  fi

  _ble_prompt_rps1_shown=
  local height=${_ble_prompt_rps1_gbox[3]} terminal_cols=${COLUMNS:-80} y
  local start=$((cols + 1))
  local width=$((terminal_cols - start))
  # Blesh's dynamic scope makes this buffer visible to the canvas draw calls.
  # shellcheck disable=SC2034
  local -a DRAW_BUFF=()
  for ((y = 0; y < height; y++)); do
    ble/canvas/panel#goto.draw "$_ble_textarea_panel" "$start" "$y" sgr0
    prelude/textarea/background/fill-width.draw "$width"
  done
  if ble/canvas/bflush.draw; then
    ADVICE_EXIT=0
  else
    ADVICE_EXIT=$?
  fi
}

function prelude/textarea/background/cleanup-trailing-spaces {
  if ! prelude/textarea/background/is-active || ((_ble_term_bce)); then
    ble/function#advice/do || :
    return
  fi

  local -a buffer
  ble/string#split-lines buffer "$text"
  local line index=0 pos terminal_cols=${COLUMNS:-80}
  for line in "${buffer[@]}"; do
    ((index += ${#line}))
    ble/string#split-words pos "${_ble_textmap_pos[index]}"
    ble/canvas/panel#goto.draw "$_ble_textarea_panel" "${pos[0]}" "${pos[1]}" sgr0
    prelude/textarea/background/fill-width.draw "$((terminal_cols - pos[0]))"
    ((index++))
  done
  _ble_prompt_rps1_shown=
  ADVICE_EXIT=0
}

function prelude/textarea/background/clear-panel {
  local index=${ADVICE_WORDS[1]:-0}
  if ! prelude/textarea/background/is-active || ((index != _ble_textarea_panel)); then
    ble/function#advice/do || :
  elif ((_ble_term_bce)); then
    local _ble_term_sgr0=$_prelude_textarea_original_sgr0$_prelude_textarea_window_sgr
    ble/function#advice/do || :
  else
    prelude/textarea/background/fill-panel.draw "$index" "${_ble_canvas_panel_height[index]}"
    ADVICE_EXIT=0
  fi
}

function prelude/textarea/background/clear-after {
  local index=${ADVICE_WORDS[1]:-0} x=${ADVICE_WORDS[2]:-0} y=${ADVICE_WORDS[3]:-0}
  if ! prelude/textarea/background/is-active || ((index != _ble_textarea_panel)); then
    ble/function#advice/do || :
  elif ((_ble_term_bce)); then
    local _ble_term_sgr0=$_prelude_textarea_original_sgr0$_prelude_textarea_window_sgr
    ble/function#advice/do || :
  elif prelude/textarea/background/fill-after.draw "$index" "$x" "$y"; then
    ADVICE_EXIT=0
  else
    ADVICE_EXIT=1
  fi
}

function prelude/textarea/background/set-height {
  local index=${ADVICE_WORDS[1]:-0}
  if ! prelude/textarea/background/is-active || ((index != _ble_textarea_panel)); then
    ble/function#advice/do || :
    return
  fi

  local old_height=${_ble_canvas_panel_height[index]}
  local options=${ADVICE_WORDS[3]-}
  if ((_ble_term_bce)); then
    local _ble_term_sgr0=$_prelude_textarea_original_sgr0$_prelude_textarea_window_sgr
    ble/function#advice/do || :
    return
  fi

  ble/function#advice/do || :
  local resize_exit=$ADVICE_EXIT
  local new_height=${_ble_canvas_panel_height[index]}
  local start=0 count=0
  if ((resize_exit == 0 && old_height != new_height && new_height > 0)); then
    if [[ :$options: == *:clear:* ]]; then
      count=$new_height
    elif ((new_height > old_height)); then
      count=$((new_height - old_height))
      [[ :$options: == *:shift:* ]] || start=$old_height
    fi
    if ((count > 0)); then
      local restore_x=$_ble_canvas_x restore_y=$_ble_canvas_y
      prelude/textarea/background/fill-rows.draw "$index" "$start" "$count"
      ble/canvas/goto.draw "$restore_x" "$restore_y" sgr0
    fi
  fi
  ADVICE_EXIT=$resize_exit
}

function prelude/textarea/background/redraw-cache {
  local active=0
  prelude/textarea/background/is-active && active=1
  if [[ ${_prelude_textarea_background_cache_epoch:-0} != "$active" ]]; then
    _ble_textarea_cache=()
    if ble/textarea#redraw; then
      ADVICE_EXIT=0
    else
      ADVICE_EXIT=$?
    fi
  else
    ble/function#advice/do || :
  fi
}

function prelude/textarea/background/sync-ownership {
  local active=0
  prelude/textarea/background/is-active && active=1
  if [[ ${_prelude_textarea_background_epoch:-0} != "$active" ]]; then
    _prelude_textarea_background_epoch=$active
    _prelude_textarea_background_cache_epoch=$active
    _ble_textarea_cache=()
    ble/function#try ble/textarea#invalidate || :
  fi
}

function prelude/textarea/background/install-advice {
  local target=$1 procedure=$2
  ble/function#advice around "$target" "$procedure" || return 1
  _prelude_textarea_background_advised+=("$target")
}

function prelude/textarea/background/disable {
  local target
  for target in "${_prelude_textarea_background_advised[@]}"; do
    ble/function#advice remove "$target" || :
  done
  _prelude_textarea_background_advised=()
  printf '%s\n' 'prelude: disabled Blesh textarea background adapter (unsupported Blesh internals)' >&2
}

function prelude/textarea/background/install {
  # Short-circuit a confirmed existing install on re-source. Re-advising
  # already-advised targets fails validation, so preserve the living install.
  [[ ${_prelude_textarea_background_installed:-0} == 1 ]] && return 0
  # Bash dynamic scope exposes this transaction-local rollback list to disable
  # without retaining installation bookkeeping after a successful install.
  local -a _prelude_textarea_background_advised=()
  if ! prelude/textarea/background/validate; then
    prelude/textarea/background/disable
    return 0
  fi

  local ret window_g
  ble/color/face2g prelude_textarea_window
  window_g=$ret
  ble/color/g#getbg "$window_g"
  if ((ret < 0)); then
    prelude/textarea/background/disable
    return 0
  fi
  _prelude_textarea_window_bg=$ret
  ble/color/face2sgr prelude_textarea_window
  _prelude_textarea_window_sgr=$ret
  _prelude_textarea_original_sgr0=$_ble_term_sgr0

  if ! prelude/textarea/background/install-advice ble/textarea#render \
    prelude/textarea/background/render ||
    ! prelude/textarea/background/install-advice ble/textarea#redraw-cache \
      prelude/textarea/background/redraw-cache ||
    ! prelude/textarea/background/install-advice ble/textarea#render/.erase-forward-line.draw \
      prelude/textarea/background/erase-forward-line ||
    ! prelude/textarea/background/install-advice ble/color/g2sgr \
      prelude/textarea/background/color ||
    ! prelude/textarea/background/install-advice ble/color/g2sgr-ansi \
      prelude/textarea/background/color ||
    ! prelude/textarea/background/install-advice ble/canvas/panel#clear.draw \
      prelude/textarea/background/clear-panel ||
    ! prelude/textarea/background/install-advice ble/canvas/panel#clear-after.draw \
      prelude/textarea/background/clear-after ||
    ! prelude/textarea/background/install-advice ble/textarea#render/.erase-rprompt \
      prelude/textarea/background/erase-rprompt ||
    ! prelude/textarea/background/install-advice ble/textarea#render/.cleanup-trailing-spaces-after-newline \
      prelude/textarea/background/cleanup-trailing-spaces ||
    ! prelude/textarea/background/install-advice ble/canvas/panel#set-height.draw \
      prelude/textarea/background/set-height; then
    prelude/textarea/background/disable
    return 0
  fi
  _prelude_textarea_background_installed=1
  prelude/textarea/background/sync-ownership
}

prelude/textarea/background/install
