# shellcheck shell=bash
# Bash's fixed status row. This callback is intentionally pure: it reads the
# current editor buffer and in-memory health snapshot, but never runs a probe.

# Feature gate from _PRELUDE_STARSHIP_STATUS_ENABLED; the render callback and
# the refresh lifecycle both no-op unless this is 1.
_prelude_status_enabled=${_PRELUDE_STARSHIP_STATUS_ENABLED:-0}

# Render output: the middle segment (message plus padding spaces) printed
# after the hint. Plain text painted with the prompt_status_line face; rebuilt
# on every render.
_prelude_status_literal=

# Cached health snapshot, a TSV record (state, last_status, age, message,
# start, revision) read from the status helper's cache. The render callback
# only reads it; prelude/status/refresh is the sole writer.
_prelude_status_health_record=${_PRELUDE_PROMPT_STATUS_RECORD-}

# Revision marker carried with the health record. Folded into the ble.sh
# prompt-unit hash so the row re-renders when a refreshed record arrives.
_prelude_status_revision=${_PRELUDE_PROMPT_STATUS_REVISION-}

# Path to the status helper binary (the probe runner). Invoked detached with
# --cached/--refresh; empty when the consumer configured no status command.
_prelude_status_helper=${_PRELUDE_PROMPT_STATUS-}

# Generated config file passed to the status helper via --config.
_prelude_status_config=${_PRELUDE_PROMPT_STATUS_CONFIG-}

# Plain hint text: the width-measurement twin, and the fallback drawn when no
# rendered twin is provided.
_prelude_status_hint=${_PRELUDE_PROMPT_STATUS_HINT:-'Run commands: x <cmd> or x ⇥'}

# `\g`-markup twin of the hint; drawn when the hint fits. Falls back to the
# plain twin above when unset.
_prelude_status_hint_rendered=${_PRELUDE_PROMPT_STATUS_HINT_RENDERED:-$_prelude_status_hint}

# Discovery message shown when the buffer is empty or not an `x` invocation.
_prelude_status_default=""

# Scratch: whitespace-separated tokens after the `x` prefix of the current
# buffer, produced by _prelude_status_tokenize; empty for non-`x` input.
_prelude_status_words=()

# Discovery message computed for the current buffer each render (catalogue
# hints plus merged health state); becomes the left part of the literal.
_prelude_status_message=

# Render output: the final hint string chosen by the width fit (rendered
# markup), or empty when the hint does not fit.
_prelude_status_hint_line=

# PID of the last detached --refresh helper run; guards against spawning a
# second refresh while one is still in flight.
_prelude_status_refresh_pid=

# PRECMD lifecycle operation: hydrate cached health/revision and schedule due
# refresh without involving Starship's prompt configuration.
function prelude/status/update { prelude/status/refresh; }

_prelude_status_trim() {
  local value=${1-}
  while [[ $value == ' '* || $value == $'\t'* ]]; do
    value=${value#?}
  done
  _prelude_status_trimmed=$value
}

# Tokenize only the public `x` prefix. Quoting, escapes, and control
# characters are deliberately rejected instead of being interpreted.
_prelude_status_tokenize() {
  local input=${1-} rest token
  _prelude_status_words=()
  case $input in
    *\'*|*\"*|*\\*|*$'\n'*|*$'\r'*) return 1 ;;
  esac
  _prelude_status_trim "$input"
  rest=$_prelude_status_trimmed
  [[ $rest == x || $rest == x[[:space:]]* ]] || return 1
  rest=${rest#x}
  _prelude_status_trim "$rest"
  rest=$_prelude_status_trimmed
  while [ -n "$rest" ]; do
    token=$rest
    case $rest in
      *' '*|*$'\t'*) token=${rest%%[[:space:]]*} ;;
      *) rest= ;;
    esac
    [ -n "$token" ] || return 1
    _prelude_status_words[${#_prelude_status_words[@]}]=$token
    [ -n "$rest" ] || break
    rest=${rest#"$token"}
    _prelude_status_trim "$rest"
    rest=$_prelude_status_trimmed
  done
}

_prelude_status_find_command() {
  local name=${1-} i
  _prelude_status_command_index=-1
  for ((i = 0; i < ${#_prelude_catalogue_names[@]}; i++)); do
    if [ "$name" = "${_prelude_catalogue_names[i]}" ]; then
      _prelude_status_command_index=$i
      return 0
    fi
  done
  return 1
}

_prelude_status_candidates() {
  local command_index=$1 argument_index=$2 prefix=$3 i value result=
  for ((i = 0; i < ${#_prelude_catalogue_candidate_values[@]}; i++)); do
    if ((_prelude_catalogue_candidate_commands[i] == command_index)) &&
      ((_prelude_catalogue_candidate_positions[i] == argument_index)); then
      value=${_prelude_catalogue_candidate_values[i]}
      if [[ $value == "$prefix"* ]]; then
        if [ -n "$result" ]; then result+="  "; fi
        result+="$value"
      fi
    fi
  done
  _prelude_status_candidates_result=$result
}

_prelude_status_discovery() {
  local input=${1-} command_index command_name description invocation count position prefix
  _prelude_status_message=$_prelude_status_default
  _prelude_status_tokenize "$input" || return 0
  count=${#_prelude_status_words[@]}
  if ((count == 0)); then
    _prelude_status_message='`x <cmd>` for hints ▶︎  ⇥ cycle  ↑↓ navigate'
    return 0
  fi

  command_name=${_prelude_status_words[0]}
  _prelude_status_find_command "$command_name" || return 0
  command_index=$_prelude_status_command_index
  description=${_prelude_catalogue_descriptions[command_index]}
  if ((count == 1)) && [[ $input != *' ' && $input != *$'\t' ]]; then
    invocation=${_prelude_catalogue_x_invocations[command_index]:-x $command_name}
    _prelude_status_message="$description  ·  $invocation  ·  bare x then Tab for details"
    return 0
  fi

  if [[ $input == *' ' || $input == *$'\t' ]]; then
    position=$((count - 1))
    prefix=
  else
    position=$((count - 2))
    prefix=${_prelude_status_words[$((count - 1))]}
  fi
  local argument_index=-1 i
  for ((i = 0; i < ${#_prelude_catalogue_argument_tokens[@]}; i++)); do
    if ((_prelude_catalogue_argument_commands[i] == command_index)) &&
      ((_prelude_catalogue_argument_positions[i] == position)); then
      argument_index=$i
      break
    fi
  done
  ((argument_index >= 0)) || return 0

  local token required argument_description current
  token=${_prelude_catalogue_argument_tokens[argument_index]}
  required=optional
  if [[ ${_prelude_catalogue_argument_required[argument_index]} == 1 ]]; then
    required=required
  fi
  argument_description=${_prelude_catalogue_argument_descriptions[argument_index]}
  current=${prefix:-<empty>}
  _prelude_status_candidates "$command_index" "$position" "$prefix"
  _prelude_status_message="argument $current ($required, $token): $argument_description"
  if [ -n "$_prelude_status_candidates_result" ]; then
    _prelude_status_message+="  ·  candidates: $_prelude_status_candidates_result"
  fi
}

_prelude_status_health() {
  local state last_status age message start revision discovery health
  [ -n "$_prelude_status_health_record" ] || return 0
  IFS=$'\t' read -r state last_status age message start revision <<EOF
$_prelude_status_health_record
EOF
  : "$revision"
  discovery=$_prelude_status_message
  case $state in
    checking)
      _prelude_status_message="health checking  ·  $discovery"
      ;;
    healthy|running)
      [ -n "$message" ] && _prelude_status_message="$discovery  ·  $message"
      ;;
    stopped)
      health=${message:-unavailable}
      [ -n "$start" ] && health+="  ·  start: $start"
      _prelude_status_message="$health  ·  $discovery"
      ;;
    stale)
      health=stale
      if [ -n "$message" ]; then
        health+=": $message"
      fi
      [ -n "$age" ] && health+=" ($age)"
      if [ "$last_status" = "stopped" ] && [ -n "$start" ]; then
        health="start: $start  ·  $health"
      fi
      _prelude_status_message="$health  ·  $discovery"
      ;;
  esac
}

# `${#value}` counts code points, not terminal cells. Delegate grapheme
# measurement to Blesh so a CJK glyph, emoji, or combining cluster cannot make
# the fixed row overflow or leave a short background fill.
_prelude_status_fit() {
  local value=${1-} limit=${2-} index=0 length width=0 output='' cs w extend
  _prelude_status_fit_text=
  _prelude_status_fit_width=0
  case $limit in
    ''|*[!0-9]*) return 1 ;;
  esac
  length=${#value}
  while ((index < length)); do
    ble/unicode/GraphemeCluster/match "$value" "$index"
    ((width + w <= limit)) || break
    output+=$cs
    ((width += w, index += 1 + extend))
  done
  _prelude_status_fit_text=$output
  _prelude_status_fit_width=$width
  ((index == length))
}

_prelude_status_render() {
  local input=${1-} width left padding
  local hint hint_width available left_width ellipsis_width
  local indent=3 indent_pad
  printf -v indent_pad '%*s' "$indent" ''
  case ${COLUMNS-} in
    ''|*[!0-9]*|0*|????????*) width=80 ;;
    *) width=$((10#$COLUMNS)) ;;
  esac
  _prelude_status_discovery "$input"
  _prelude_status_health
  left=$_prelude_status_message

  available=$((width - 1))
  hint=
  hint_width=0
  if [ -n "$_prelude_status_hint" ] &&
    ((available > 2)) &&
    _prelude_status_fit "$_prelude_status_hint" "$((available - 2))"; then
    hint="$indent_pad$_prelude_status_hint_rendered"
    hint_width=$((_prelude_status_fit_width + indent))
    available=$((available - hint_width))
  fi
  if _prelude_status_fit "$left" "$available"; then
    left=$_prelude_status_fit_text
    left_width=$_prelude_status_fit_width
  else
    _prelude_status_fit '…' "$available" || true
    ellipsis_width=$_prelude_status_fit_width
    if ((ellipsis_width > 0 && ellipsis_width < available)); then
      _prelude_status_fit "$left" "$((available - ellipsis_width))" || true
      left="${_prelude_status_fit_text}…"
      left_width=$((_prelude_status_fit_width + ellipsis_width))
    else
      left=
      left_width=0
    fi
  fi
  padding=$((width - hint_width - left_width))
  printf -v _prelude_status_literal '%s%*s' "$left" "$padding" ''
  _prelude_status_hint_line=$hint
}
_prelude_status_read_health_record() {
  local record=${1-} state last_status age message start revision
  [ -n "$record" ] || return 0
  IFS=$'\t' read -r state last_status age message start revision <<EOF
$record
EOF
  _prelude_status_health_record=$record
  [ -z "$revision" ] || _prelude_status_revision=$revision
}

# The helper owns probing and cache policy. Refresh is detached so prompt
# rendering never waits on a configured local-server check; the callback only
# reads the cached record populated here.
function prelude/status/refresh {
  [ "$_prelude_status_enabled" = 1 ] || return 0
  if [ -n "$_prelude_status_helper" ] && [ -r "$_prelude_status_config" ]; then
    local record
    record=$("$_prelude_status_helper" --cached --config "$_prelude_status_config" 2>/dev/null) || record=
    _prelude_status_read_health_record "$record"
    if [ -z "$_prelude_status_refresh_pid" ] ||
      ! kill -0 "$_prelude_status_refresh_pid" 2>/dev/null; then
      "$_prelude_status_helper" --refresh --config "$_prelude_status_config" \
        >/dev/null 2>&1 &
      _prelude_status_refresh_pid=$!
    fi
  elif [ -n "${_PRELUDE_PROMPT_STATUS_RECORD-}" ]; then
    _prelude_status_read_health_record "$_PRELUDE_PROMPT_STATUS_RECORD"
  fi
}

function ble/prompt/backslash:prelude/status {
  [ "$_prelude_status_enabled" = 1 ] || return 0
  ble/prompt/unit/add-hash '$_ble_edit_str'
  ble/prompt/unit/add-hash '$_prelude_status_revision'
  ble/prompt/unit/add-hash '$_prelude_status_health_record'
  _prelude_status_render "${_ble_edit_str-}"
  [ -z "$_prelude_status_hint_line" ] || ble/prompt/process-prompt-string "$_prelude_status_hint_line"
  ble/prompt/print "$_prelude_status_literal"
}
