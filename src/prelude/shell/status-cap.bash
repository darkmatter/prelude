# shellcheck shell=bash
# A one-cell cap immediately above Prelude's fixed status row.
#
# Blesh owns the bottom status panel and pins it to one row. The menu's footer
# gets its soft edge from a separate `▄` row, so replicate that geometry as an
# adjacent bottom-docked panel instead of putting a newline in
# `prompt_status_line`. The panel registry is Blesh-private; install validates
# the pinned layout before changing it so an upstream change cannot silently
# corrupt the terminal canvas.

_prelude_status_cap_panel=
_prelude_status_cap_dirty=

# Blesh panel callback names include `#`, which Bash accepts but ShellCheck
# cannot parse. This is a fixed program literal: no runtime data is evaluated.
# The status callback is populated after the first panel layout, so reserve the
# cap from its configured line instead. Command layout collapses both rows.
eval '
function prelude/status/cap#panel::getHeight {
  if [[ ${bleopt_prompt_status_line-} ]] && ! ble/edit/is-command-layout; then
    height=0:1
  else
    height=0:0
  fi
}

function prelude/status/cap#panel::invalidate {
  _prelude_status_cap_dirty=1
}

function prelude/status/cap#panel::onHeightChange {
  _prelude_status_cap_dirty=1
}

# Blesh collapses its status panel synchronously while command output owns the
# terminal. The cap is a sibling, so it must join that transition rather than
# waiting for a later prompt render.
function prelude/status/cap#collapse {
  local -a DRAW_BUFF=()
  ble/canvas/panel#set-height.draw "$_prelude_status_cap_panel" 0
  ble/canvas/panel#set-height.draw "$_ble_prompt_status_panel" 0
  ble/canvas/bflush.draw
}


function prelude/status/cap#panel::render {
  local index=$1 panel_height=$3 height
  prelude/status/cap#panel::getHeight "$index"
  [[ ${height#*:} == 1 ]] || return 0

  local -a DRAW_BUFF=()
  if ((panel_height != 1)); then
    # Cap renders before the Blesh status panel. Resize here too, otherwise the
    # status panel adds both rows after this callback has already returned.
    ble/canvas/panel/reallocate-height.draw
    ble/canvas/bflush.draw
    panel_height=${_ble_canvas_panel_height[index]}
    ((panel_height == 1)) || return 0
  fi

  [[ $_prelude_status_cap_dirty ]] || return 0
  _prelude_status_cap_dirty=

  local cols=${COLUMNS-} cap
  ((cols > 0)) || return 0
  ((_ble_term_xenl || cols--))
  ((cols > 0)) || return 0

  local ret
  ble/color/face2g prelude_status_cap
  ble/color/g2sgr "$ret"
  local sgr=$ret
  ble/string#repeat ▄ "$cols"
  cap=$ret
  ble/canvas/panel#goto.draw "$index"
  # The final two arguments are the Blesh-tracked cursor position after draw.
  # A full-width cap leaves the terminal at its right margin. Preserving that
  # nonzero position makes Blesh emit the CR before following status paint.
  ble/canvas/panel#put.draw "$index" "$sgr$cap$_ble_term_sgr0" "$cols" 0
  ble/canvas/bflush.draw
}
'

function prelude/status/cap/install {
  [[ $_prelude_status_cap_panel ]] && return 0

  local status_panel=$_ble_prompt_status_panel
  if [[ ${_ble_canvas_panel_class[status_panel]-} != ble/prompt/status ]]; then
    printf '%s\n' 'prelude: unsupported ble.sh status-panel layout' >&2
    return 1
  fi

  _ble_canvas_panel_class=(
    "${_ble_canvas_panel_class[@]:0:status_panel}"
    prelude/status/cap
    "${_ble_canvas_panel_class[@]:status_panel}"
  )
  _ble_canvas_panel_height=(
    "${_ble_canvas_panel_height[@]:0:status_panel}"
    0
    "${_ble_canvas_panel_height[@]:status_panel}"
  )
  _prelude_status_cap_panel=$status_panel
  _ble_prompt_status_panel=$((status_panel + 1))
  _ble_canvas_panel_vfill=$status_panel
  _prelude_status_cap_dirty=1

  # This is Blesh's pinned internal transition point. Install the wrapper only
  # after the cap exists, so custom prompt configurations retain stock Blesh.
  eval '
function ble/prompt/status#collapse {
  prelude/status/cap#collapse
}
'
}
