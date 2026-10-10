package main

// Real Tab expands these private bindings only inside Readline, never inside a
// foreground program. Snapshots and edits use the host's private file, never
// terminal paste or accept-line. Wire offsets are bytes, including on Bash 5.3, whose bind -x
// variables use character indexes in the shell's current locale.
const bashCompletionRC = `# shellcheck shell=bash
__prelude_ghostty_completion_query_id=0
__prelude_ghostty_completion_fallback_valid=0
__prelude_ghostty_completion_fallback_pending=0

# Decimal-string ordering here is intentional; arithmetic could overflow.
# shellcheck disable=SC2071
__prelude_ghostty_completion_id() {
    local LC_ALL=C maximum=9223372036854775807
    [[ $1 =~ ^[1-9][0-9]{0,18}$ ]] &&
        { ((${#1} < 19)) || [[ $1 < "$maximum" || $1 == "$maximum" ]]; }
}

__prelude_ghostty_completion_offset() {
    [[ $1 =~ ^(0|[1-9][0-9]{0,6})$ ]] && (( $1 <= $2 ))
}

# Bash 5.3 changed bind -x offsets to characters; keep the host protocol byte-
# based without changing the shell's locale or Readline's editing behavior.
__prelude_ghostty_completion_character_offsets() {
    ((BASH_VERSINFO[0] > 5 || (BASH_VERSINFO[0] == 5 && BASH_VERSINFO[1] >= 3)))
}

__prelude_ghostty_completion_snapshot() {
    line=${READLINE_LINE-}
    point=${READLINE_POINT-}
    mark=${READLINE_MARK-}
    local point_prefix mark_prefix
    if __prelude_ghostty_completion_character_offsets; then
        __prelude_ghostty_completion_offset "$point" "${#line}" &&
            __prelude_ghostty_completion_offset "$mark" "${#line}" || return 1
        point_prefix=${line:0:point}
        mark_prefix=${line:0:mark}
        local LC_ALL=C
        point=${#point_prefix}
        mark=${#mark_prefix}
    else
        local LC_ALL=C
        __prelude_ghostty_completion_offset "$point" "${#line}" &&
            __prelude_ghostty_completion_offset "$mark" "${#line}"
    fi
}

__prelude_ghostty_completion_byte_prefixes() {
    local LC_ALL=C
    point_prefix=${new_line:0:new_point}
    mark_prefix=${new_line:0:new_mark}
}

__prelude_ghostty_completion_to_readline() {
    local point_prefix mark_prefix point_char mark_char
    __prelude_ghostty_completion_byte_prefixes
    point_char=${#point_prefix}
    mark_char=${#mark_prefix}
    # Reject offsets into a multibyte character, not merely out-of-range ones.
    [[ ${new_line:0:point_char} == "$point_prefix" &&
        ${new_line:0:mark_char} == "$mark_prefix" ]] || return 1
    if __prelude_ghostty_completion_character_offsets; then
        new_point=$point_char
        new_mark=$mark_char
    fi
}

__prelude_ghostty_completion_write() {
    local file=$1 id=$2
    shift 2
    [[ -f $file && -w $file ]] || return 1
    # These host-owned files already exist; override noclobber only for this write.
    if ! builtin printf '%s\0' "$@" 2>/dev/null >| "$file"; then
        builtin printf 'error\0%s\0cannot write completion response\0' "$id" 2>/dev/null >| "$file" || :
        return 1
    fi
}

__prelude_ghostty_completion_clear_fallback() {
    __prelude_ghostty_completion_fallback_valid=0
    __prelude_ghostty_completion_fallback_pending=0
    builtin unset __prelude_ghostty_completion_fallback_line __prelude_ghostty_completion_fallback_point \
        __prelude_ghostty_completion_fallback_mark __prelude_ghostty_completion_fallback_generation
}

# The final Tab-macro step observes native completion's edits. Caching before
# complete would miss double-Tab listing when the first Tab inserts a prefix.
__prelude_ghostty_completion_cache_fallback() {
    local status=$?
    local line='' point='' mark=''
    if [[ ${__prelude_ghostty_completion_fallback_pending-0} == 1 &&
        ${__prelude_ghostty_primary_prompt-} == 1 && -n ${__prelude_ghostty_prompt_generation-} ]] &&
        __prelude_ghostty_completion_snapshot; then
        __prelude_ghostty_completion_fallback_line=$line
        __prelude_ghostty_completion_fallback_point=$point
        __prelude_ghostty_completion_fallback_mark=$mark
        __prelude_ghostty_completion_fallback_generation=$__prelude_ghostty_prompt_generation
        __prelude_ghostty_completion_fallback_valid=1
    else
        __prelude_ghostty_completion_clear_fallback
    fi
    __prelude_ghostty_completion_fallback_pending=0
    return "$status"
}

# These helpers use query-local state, including the response array. Bounds are
# checked before appending a row, so an overfull result is an error, not a prefix.
__prelude_ghostty_completion_add() {
    local key=$1 description=$2 quoted insert replacement new_point new_mark delta field row_bytes=5
    [[ -n $key ]] || { response=(error 'empty catalogue key'); return 1; }
    [[ $key == "$prefix"* ]] || return 0
    [[ ${seen["$key"]+present} ]] && return 0
    builtin printf -v quoted '%q' "$key"
    insert=$quoted
    [[ $initial == 1 ]] && insert="x $quoted"
    if [[ $suffix == [[:blank:]]* ]]; then
        new_point=$((start + ${#insert}))
    else
        insert+=' '
        new_point=$((start + ${#insert}))
    fi
    replacement="${line:0:start}$insert$suffix"
    delta=$((${#insert} - (end - start)))
    if ((mark <= start)); then
        new_mark=$mark
    elif ((mark >= end)); then
        new_mark=$((mark + delta))
    else
        new_mark=$((start + ${#insert}))
    fi
    for field in "$key" "$description" "$replacement" "$new_point" "$new_mark"; do
        row_bytes=$((row_bytes + ${#field}))
    done
    if ((count >= 1024 || bytes + row_bytes > 1048576)); then
        response=(error 'completion response exceeds 1 MiB or 1024 candidates')
        return 1
    fi
    response+=("$key" "$description" "$replacement" "$new_point" "$new_mark")
    seen["$key"]=1
    count=$((count + 1))
    bytes=$((bytes + row_bytes))
}

__prelude_ghostty_completion_query_result() {
    local LC_ALL=C
    local start end initial=0 prefix suffix token tail command_end field i bytes=$((${#query_id} + 1)) count=0
    local imports='' record key description import_pid read_status
    local -A seen=()
    # No shell parsing or expansion: quoted words, operators, substitutions and
    # glob syntax belong to native completion. Ordinary suffix arguments survive.
    local syntax=$'\'"\\\x60$;&|<>(){}*?[]~!#\n\r'
    if ! builtin declare -p _prelude_catalogue_names _prelude_catalogue_descriptions >/dev/null 2>&1 ||
        [[ $line == *["$syntax"]* ]] ||
        ! [[ $line =~ ^([[:blank:]]*)x([[:blank:]]|$) ]]; then
        response=(fallback)
        return 0
    fi
    start=${#BASH_REMATCH[1]}
    command_end=$((start + 1))
    if ! __prelude_ghostty_completion_offset "$point" "${#line}" ||
        ! __prelude_ghostty_completion_offset "$mark" "${#line}"; then
        response=(error 'invalid readline byte offsets')
        return 0
    fi
    tail=${line:command_end}
    if ((point == command_end)) && [[ $tail != *[^[:blank:]]* ]]; then
        initial=1
        end=$command_end
        prefix=
    else
        [[ $tail =~ ^([[:blank:]]*) ]] || return 1
        start=$((command_end + ${#BASH_REMATCH[1]}))
        token=${line:start}
        token=${token%%[[:blank:]]*}
        end=$((start + ${#token}))
        if ((point < start || point > end)); then
            response=(fallback)
            return 0
        fi
        prefix=${line:start:point-start}
    fi
    suffix=${line:end}
    response=(candidates "$line" "$point" "$mark")
    for field in "${response[@]}"; do
        bytes=$((bytes + ${#field} + 1))
    done
    if ((bytes > 1048576)); then
        response=(error 'completion snapshot exceeds 1 MiB')
        return 0
    fi
    if ((${#_prelude_catalogue_names[@]} != ${#_prelude_catalogue_descriptions[@]})); then
        response=(error 'catalogue names and descriptions do not match')
        return 0
    fi
    for i in "${!_prelude_catalogue_names[@]}"; do
        if ! __prelude_ghostty_completion_add "${_prelude_catalogue_names[i]}" "${_prelude_catalogue_descriptions[i]-}"; then
            return 0
        fi
    done
    if [[ ${_prelude_catalogue_imports:-0} == 1 ]]; then
        if ! command -v x >/dev/null 2>&1; then
            response=(error 'x is unavailable for runtime imports')
            return 0
        fi
        # Reading to NUL detects unrepresentable import output; -n also bounds a
        # malformed/huge stream. EOF, not a NUL delimiter, is the expected end.
        IFS= builtin read -r -d '' -n 1048577 imports < <(x --imports 2>/dev/null)
        read_status=$?
        import_pid=$!
        if ! builtin wait "$import_pid" 2>/dev/null; then
            response=(error 'x --imports failed')
            return 0
        fi
        if ((${#imports} > 1048576)) || ((read_status == 0)); then
            response=(error 'runtime imports exceed 1 MiB or contain NUL')
            return 0
        fi
        while [[ -n $imports ]]; do
            record=${imports%%$'\n'*}
            if [[ $imports == *$'\n'* ]]; then
                imports=${imports#*$'\n'}
            else
                imports=
            fi
            if [[ $record != *$'\t'* ]]; then
                response=(error 'invalid runtime import record')
                return 0
            fi
            key=${record%%$'\t'*}
            description=${record#*$'\t'}
            if ! __prelude_ghostty_completion_add "$key" "$description"; then
                return 0
            fi
        done
    fi
}

__prelude_ghostty_completion_query() {
    local status=$?
    local line='' point='' mark='' new_line='' new_point='' new_mark='' query_id
    local -a response=()
    __prelude_ghostty_completion_query_id=$((__prelude_ghostty_completion_query_id + 1))
    query_id=$__prelude_ghostty_completion_query_id
    __prelude_ghostty_completion_fallback_pending=0
    # The remainder of the real-Tab macro completes natively only on fallback.
    builtin bind '"\e[996~": complete'
    if [[ ${__prelude_ghostty_primary_prompt-} != 1 ]]; then
        __prelude_ghostty_completion_clear_fallback
        response=(fallback)
    elif ! __prelude_ghostty_completion_snapshot; then
        response=(error 'invalid readline offsets')
    elif ! __prelude_ghostty_completion_query_result; then
        response=(error 'cannot build completion response')
    fi
    if [[ ${response[0]-} == fallback && ${__prelude_ghostty_primary_prompt-} == 1 ]]; then
        if [[ ${__prelude_ghostty_completion_fallback_valid-0} == 1 &&
            $line == "${__prelude_ghostty_completion_fallback_line-}" &&
            $point == "${__prelude_ghostty_completion_fallback_point-}" &&
            $mark == "${__prelude_ghostty_completion_fallback_mark-}" &&
            ${__prelude_ghostty_prompt_generation-} == "${__prelude_ghostty_completion_fallback_generation-}" ]]; then
            builtin bind '"\e[996~": possible-completions'
        fi
        __prelude_ghostty_completion_fallback_pending=1
    elif [[ ${response[0]-} != fallback ]]; then
        __prelude_ghostty_completion_clear_fallback
        builtin bind '"\e[996~": ""'
        if [[ ${response[0]-} == candidates ]] && ((${#response[@]} == 9)); then
            new_line=${response[6]}
            new_point=${response[7]}
            new_mark=${response[8]}
            if __prelude_ghostty_completion_to_readline; then
                response=(applied)
            else
                response=(error 'completion offsets split a multibyte character')
            fi
        fi
    fi
    response=("${response[0]}" "$query_id" "${response[@]:1}")
    # A unique completion is committed in this same Readline callback, before
    # any already-queued Enter can be read. The callback itself never accepts.
    if __prelude_ghostty_completion_write "${__prelude_ghostty_completion_file-}" "$query_id" "${response[@]}" &&
        [[ ${response[0]-} == applied ]]; then
        READLINE_LINE=$new_line
        READLINE_POINT=$new_point
        READLINE_MARK=$new_mark
    fi
    builtin printf '\e]133;Q;%s\a' "$query_id"
    return "$status"
}

__prelude_ghostty_completion_apply_result() {
    local LC_ALL=C
    local file=${__prelude_ghostty_completion_apply_file-} field='' bytes=0
    local -a fields=()
    if [[ ! -f $file || ! -r $file ]]; then
        response=(error 'cannot read completion apply record')
        return 0
    fi
    while IFS= builtin read -r -d '' -n 1048577 field; do
        bytes=$((bytes + ${#field} + 1))
        if ((bytes > 1048576 || ${#fields[@]} >= 9)); then
            response=(error 'completion apply record exceeds bounds')
            return 0
        fi
        fields+=("$field")
        if ((${#fields[@]} == 2)) && __prelude_ghostty_completion_id "$field"; then
            apply_id=$field
        fi
    done 2>/dev/null < "$file"
    if [[ -n $field ]] || ((${#fields[@]} != 9)) || [[ ${fields[0]-} != apply ]]; then
        response=(error 'invalid NUL-delimited completion apply record')
        return 0
    fi
    if ! __prelude_ghostty_completion_id "${fields[1]}" || ! __prelude_ghostty_completion_id "${fields[2]}"; then
        response=(error 'invalid completion operation IDs')
        return 0
    fi
    if ! __prelude_ghostty_completion_offset "${fields[4]}" "${#fields[3]}" ||
        ! __prelude_ghostty_completion_offset "${fields[5]}" "${#fields[3]}" ||
        ! __prelude_ghostty_completion_offset "${fields[7]}" "${#fields[6]}" ||
        ! __prelude_ghostty_completion_offset "${fields[8]}" "${#fields[6]}"; then
        response=(error 'invalid completion apply byte offsets')
        return 0
    fi
    snapshot_id=${fields[2]}
    old_line=${fields[3]}
    old_point=${fields[4]}
    old_mark=${fields[5]}
    new_line=${fields[6]}
    new_point=${fields[7]}
    new_mark=${fields[8]}
    response=(applied)
}

__prelude_ghostty_completion_apply() {
    local status=$?
    local line='' point='' mark='' new_line='' new_point='' new_mark=''
    local old_line='' old_point='' old_mark='' snapshot_id='' apply_id=0
    local -a response=()
    if ! __prelude_ghostty_completion_apply_result; then
        response=(error 'cannot read completion apply record')
    elif [[ ${response[0]-} == applied ]]; then
        if [[ ${__prelude_ghostty_primary_prompt-} != 1 ||
            $snapshot_id != "${__prelude_ghostty_completion_query_id-0}" ]]; then
            response=(stale)
        elif ! __prelude_ghostty_completion_snapshot; then
            response=(error 'invalid readline offsets')
        elif [[ $line != "$old_line" || $point != "$old_point" || $mark != "$old_mark" ]]; then
            response=(stale)
        elif ! __prelude_ghostty_completion_to_readline; then
            response=(error 'completion offsets split a multibyte character')
        fi
    fi
    response=("${response[0]}" "$apply_id" "${response[@]:1}")
    # Apply replies never touch query responses, and are correlated separately.
    if __prelude_ghostty_completion_write "${__prelude_ghostty_completion_apply_file-}" "$apply_id" "${response[@]}" &&
        [[ ${response[0]-} == applied ]]; then
        READLINE_LINE=$new_line
        READLINE_POINT=$new_point
        READLINE_MARK=$new_mark
        __prelude_ghostty_completion_clear_fallback
    fi
    builtin printf '\e]133;R;%s\a' "$apply_id"
    return "$status"
}

builtin bind -x '"\e[994~":__prelude_ghostty_completion_query'
builtin bind -x '"\e[995~":__prelude_ghostty_completion_apply'
builtin bind '"\e[996~": complete'
builtin bind -x '"\e[997~":__prelude_ghostty_completion_cache_fallback'
builtin bind '"\C-i": "\e[994~\e[996~\e[997~"'
`
