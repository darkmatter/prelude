# shellcheck shell=bash
# Catalogue-aware completion for `x` commands, their subcommands, and direct
# commands. A command's name is its words joined by one space (`db migrate`),
# so completion walks the words already typed: the next word offers the
# subcommands below them, and the arguments of the longest command they name.

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

# Yield the word after the given words for every name that continues them:
# the top-level commands for no words, a parent's subcommands below it.
_prelude_complete_next_words() {
  local IFS=' ' i depth=$# name prefix=$*
  local -a parts
  for ((i = 0; i < ${#_prelude_catalogue_names[@]}; i++)); do
    name=${_prelude_catalogue_names[i]}
    read -r -a parts <<<"$name"
    ((${#parts[@]} == depth + 1)) || continue
    if ((depth > 0)); then
      [[ "$name" == "$prefix "* ]] || continue
    fi
    _prelude_complete_yield "${parts[depth]}" "${_prelude_catalogue_descriptions[i]}"
  done
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

# Runtime imports (Justfile recipes, package.json scripts): `x --imports`
# prints each command the menu imported, at every depth, as its words and a
# description, so `x <TAB>` offers exactly what `x` dispatches, even when the
# Nix-declared catalogue is empty. Yields the word after the given words.
_prelude_complete_imports() {
  [ "${_prelude_catalogue_imports:-0}" = 1 ] || return 0
  local IFS=' ' depth=$# prefix=$* route description
  local -a parts
  while IFS=$'\t' read -r route description; do
    read -r -a parts <<<"$route"
    ((${#parts[@]} == depth + 1)) || continue
    if ((depth > 0)); then
      [[ "$route" == "$prefix "* ]] || continue
    fi
    _prelude_complete_yield "${parts[depth]}" "$description"
  done < <(x --imports 2>/dev/null)
  return 0
}

# Complete the words from COMP_WORDS[first] up to the cursor as a command
# path: the next word of a command name, then the arguments of the longest
# command those words name. With imports=1 the menu's runtime imports count too.
_prelude_complete_path() {
  local first=$1 imports=$2
  local -a typed=("${COMP_WORDS[@]:first:COMP_CWORD-first}")
  _prelude_complete_next_words "${typed[@]}"
  ((imports == 0)) || _prelude_complete_imports "${typed[@]}"
  ((${#typed[@]} > 0)) || return 0
  _prelude_catalogue_match "${typed[@]}" || return 0
  _prelude_catalogue_completion_candidates \
    "$_prelude_catalogue_match_index" \
    "$((${#typed[@]} - _prelude_catalogue_match_words))"
}

_prelude_complete_x() {
  local _prelude_complete_prefix=${COMP_WORDS[COMP_CWORD]-}
  local _prelude_complete_used_ble
  COMPREPLY=()
  _prelude_complete_path 1 1
  [ -z "$_prelude_complete_used_ble" ] || bleopt complete_menu_style=desc
  compopt -o nosort 2>/dev/null || true
}

# Initial-word completion (`complete -I`): when the cursor is still on the `x`
# command word with no trailing space, show the top-level commands as
# `x <name>` candidates so a single Tab reveals the chooser instead of falling
# through to PATH command-name completion. `noquote` keeps the intentional
# space literal so a selected entry inserts as `x <name>` (two words), not a
# single quoted token. Any other command word is left untouched by returning
# an empty list, which lets bash/ble.sh fall back to default command completion.
_prelude_complete_initial() {
  COMPREPLY=()
  [ "${COMP_WORDS[0]-}" = "x" ] || return 0
  local i candidate route description
  for ((i = 0; i < ${#_prelude_catalogue_names[@]}; i++)); do
    [[ "${_prelude_catalogue_names[i]}" != *' '* ]] || continue
    candidate="x ${_prelude_catalogue_names[i]}"
    [[ "$candidate" == "${COMP_WORDS[0]}"* ]] || continue
    COMPREPLY+=("$candidate")
  done
  if [ "${_prelude_catalogue_imports:-0}" = 1 ]; then
    while IFS=$'\t' read -r route description; do
      [[ "$route" != *' '* ]] || continue
      candidate="x $route"
      [[ "$candidate" == "${COMP_WORDS[0]}"* ]] || continue
      COMPREPLY+=("$candidate")
    done < <(x --imports 2>/dev/null)
  fi
  compopt -o nosort 2>/dev/null || true
  compopt -o noquote 2>/dev/null || true
}

# A direct command (`db`) completes like `x db`: its own name is the first
# word of the path.
_prelude_complete_direct() {
  local _prelude_complete_prefix=${COMP_WORDS[COMP_CWORD]-}
  local _prelude_complete_used_ble
  COMPREPLY=()
  _prelude_complete_path 0 0
  [ -z "$_prelude_complete_used_ble" ] || bleopt complete_menu_style=desc
  compopt -o nosort 2>/dev/null || true
}
