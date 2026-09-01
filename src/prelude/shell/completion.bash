# shellcheck shell=bash
# Catalogue-aware completion for grouped `x` commands and direct commands.

_prelude_complete_yield() {
  local candidate=$1 description=$2
  [[ "$candidate" == "$_prelude_complete_prefix"* ]] || return 0
  if declare -F ble/complete/cand/yield >/dev/null 2>&1; then
    ble/complete/cand/yield word "$candidate" "$description"
    _prelude_complete_used_ble=1
  else
    COMPREPLY+=("$candidate")
  fi
}

_prelude_catalogue_completion_candidates() {
  local command_index=$1 argument_index=$2 i
  for ((i = 0; i < ${#_prelude_catalogue_candidate_values[@]}; i++)); do
    if ((_prelude_catalogue_candidate_commands[i] == command_index)) &&
      ((_prelude_catalogue_candidate_positions[i] == argument_index)); then
      _prelude_complete_yield \
        "${_prelude_catalogue_candidate_values[i]}" \
        "${_prelude_catalogue_candidate_descriptions[i]}"
    fi
  done
}

# Runtime Justfile import: the same recipes the menu imports. `just --summary`
# lists public recipe names (underscore-prefixed and private recipes are
# already excluded), so `x <TAB>` can offer the complete dispatchable catalogue
# even when the Nix-declared one is empty.
_prelude_complete_just_recipes() {
  local _prelude_complete_prefix=$1
  [ "${_prelude_catalogue_just_import:-0}" = 1 ] || return 0
  command -v just >/dev/null 2>&1 || return 0
  local recipe
  for recipe in $(just --summary 2>/dev/null); do
    _prelude_complete_yield "$recipe" ""
  done
  return 0
}

_prelude_complete_x() {
  local _prelude_complete_prefix=${COMP_WORDS[COMP_CWORD]-}
  local _prelude_complete_used_ble task_index=-1 i
  COMPREPLY=()
  if ((COMP_CWORD == 1)); then
    for ((i = 0; i < ${#_prelude_catalogue_names[@]}; i++)); do
      _prelude_complete_yield \
        "${_prelude_catalogue_names[i]}" \
        "${_prelude_catalogue_descriptions[i]}"
    done
    _prelude_complete_just_recipes "$_prelude_complete_prefix"
  else
    for ((i = 0; i < ${#_prelude_catalogue_names[@]}; i++)); do
      if [ "${COMP_WORDS[1]}" = "${_prelude_catalogue_names[i]}" ]; then
        task_index=$i
        break
      fi
    done
    ((task_index < 0)) ||
      _prelude_catalogue_completion_candidates "$task_index" "$((COMP_CWORD - 2))"
  fi
  [ -z "$_prelude_complete_used_ble" ] || bleopt complete_menu_style=desc
  compopt -o nosort 2>/dev/null || true
}

# Initial-word completion (`complete -I`): when the cursor is still on the `x`
# command word with no trailing space, show the catalogue as `x <key>` candidates
# so a single Tab reveals the chooser instead of falling through to PATH
# command-name completion. `noquote` keeps the intentional space literal so a
# selected entry inserts as `x <key>` (two words), not a single quoted token.
# Any other command word is left untouched by returning an empty list, which
# lets bash/ble.sh fall back to default command completion.
_prelude_complete_initial() {
  COMPREPLY=()
  [ "${COMP_WORDS[0]-}" = "x" ] || return 0
  local i candidate
  for ((i = 0; i < ${#_prelude_catalogue_names[@]}; i++)); do
    candidate="x ${_prelude_catalogue_names[i]}"
    [[ "$candidate" == "${COMP_WORDS[0]}"* ]] || continue
    COMPREPLY+=("$candidate")
  done
  if [ "${_prelude_catalogue_just_import:-0}" = 1 ] && command -v just >/dev/null 2>&1; then
    local recipe
    for recipe in $(just --summary 2>/dev/null); do
      candidate="x $recipe"
      [[ "$candidate" == "${COMP_WORDS[0]}"* ]] || continue
      COMPREPLY+=("$candidate")
    done
  fi
  compopt -o nosort 2>/dev/null || true
  compopt -o noquote 2>/dev/null || true
}

_prelude_complete_direct() {
  local _prelude_complete_prefix=${COMP_WORDS[COMP_CWORD]-}
  local _prelude_complete_used_ble task_index=-1 i
  COMPREPLY=()
  for ((i = 0; i < ${#_prelude_catalogue_direct_names[@]}; i++)); do
    if [ "${COMP_WORDS[0]}" = "${_prelude_catalogue_direct_names[i]}" ]; then
      task_index=${_prelude_catalogue_direct_indexes[i]}
      break
    fi
  done
  ((task_index < 0)) ||
    _prelude_catalogue_completion_candidates "$task_index" "$((COMP_CWORD - 1))"
  [ -z "$_prelude_complete_used_ble" ] || bleopt complete_menu_style=desc
  compopt -o nosort 2>/dev/null || true
}
