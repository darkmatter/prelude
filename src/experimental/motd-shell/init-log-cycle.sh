# PROTOTYPE: cycle captured shell-hook lines through the dedicated status row,
# then leave the final non-empty line visible.
log="${PRELUDE_SHELL_INIT_LOG:-}"
[ -s "$log" ] || exit 0

while IFS= read -r raw || [ -n "$raw" ]; do
  # Status content must be one printable line. Strip ANSI/control sequences and
  # neutralize tmux's format introducer so hook output cannot inject styles.
  line="$(printf '%s' "$raw" | sed -E $'s/\x1B\[[0-9;?]*[ -\/]*[@-~]//g' | tr -d '\000-\010\013\014\016-\037\177')"
  line="${line//#/＃}"
  [ -n "$line" ] || continue
  tmux set-option -t "$PRELUDE_MOTD_TMUX_SESSION" @prelude_init_line "$line"
  tmux refresh-client -S
  sleep 0.9
done <"$log"
