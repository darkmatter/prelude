# shellcheck shell=bash
# Move Starship's native ble.sh right-prompt render into the bottom status line.

_prelude_status_enabled=$_PRELUDE_STARSHIP_STATUS_ENABLED
_prelude_status_line=

# Starship's PRECMD hook runs first and writes a fully rendered right prompt to
# bleopt_prompt_rps1 using the captured status, pipeline, duration, jobs, and
# terminal width. Reuse that result instead of reconstructing Starship's call.
function prelude/status/update {
  [ "$_prelude_status_enabled" = 1 ] || return 0

  local line=${bleopt_prompt_rps1-}
  while [[ $line == $'\n'* ]]; do
    line=${line:1}
  done
  _prelude_status_line=$line
  bleopt prompt_rps1=
}

function ble/prompt/backslash:prelude/status {
  # ble.sh evaluates this dependency reference itself.
  # shellcheck disable=SC2016
  ble/prompt/unit/add-hash '$_prelude_status_line'
  ble/prompt/process-prompt-string "$_prelude_status_line"
}
