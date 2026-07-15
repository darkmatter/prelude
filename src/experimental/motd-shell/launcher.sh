# PROTOTYPE — pinned MOTD/docs + real child shell on an isolated tmux server.
if [ ! -t 0 ] || [ ! -t 1 ]; then
  echo "motd-shell-experiment: requires an interactive terminal" >&2
  exit 1
fi

shell_percent="${PRELUDE_SHELL_PERCENT:-50}"
case "$shell_percent" in
  ''|*[!0-9]*)
    echo "motd-shell-experiment: PRELUDE_SHELL_PERCENT must be an integer (30..70)" >&2
    exit 1
    ;;
esac
if [ "$shell_percent" -lt 30 ] || [ "$shell_percent" -gt 70 ]; then
  echo "motd-shell-experiment: PRELUDE_SHELL_PERCENT must be between 30 and 70" >&2
  exit 1
fi

socket="prelude-motd-shell-$$"
session="prelude-shell"
export PRELUDE_MOTD_TMUX_SOCKET="$socket"
export PRELUDE_MOTD_TMUX_SESSION="$session"

run_tmux() {
  # An isolated socket is safe inside an existing tmux session. Removing the
  # parent TMUX variable avoids tmux's nested-session refusal during attach.
  env -u TMUX tmux -L "$socket" -f /dev/null "$@"
}

cleanup() {
  run_tmux kill-server >/dev/null 2>&1 || true
  if [ "${PRELUDE_SHELL_INIT_LOG_OWNED:-}" = 1 ] && [ -n "${PRELUDE_SHELL_INIT_LOG:-}" ]; then
    rm -f "$PRELUDE_SHELL_INIT_LOG"
  fi
}
trap cleanup EXIT HUP INT TERM

run_tmux new-session -d -s "$session" -c "$PWD" prelude-motd-child-shell
shell_pane="$(run_tmux display-message -p -t "$session:0" '#{pane_id}')"
top_percent=$((100 - shell_percent))
top_pane="$(run_tmux split-window -d -b -v -p "$top_percent" -P -F '#{pane_id}' -t "$session:0" -c "$PWD" prelude-motd-pane)"
export PRELUDE_MOTD_TOP_PANE="$top_pane"
export PRELUDE_MOTD_SHELL_PANE="$shell_pane"

run_tmux set-environment -g PRELUDE_MOTD_TOP_PANE "$top_pane"
run_tmux set-environment -g PRELUDE_MOTD_SHELL_PANE "$shell_pane"
if [ -n "${PRELUDE_SHELL_INIT_LOG:-}" ]; then
  run_tmux set-environment -g PRELUDE_SHELL_INIT_LOG "$PRELUDE_SHELL_INIT_LOG"
fi

# Workspace behavior and pane labels.
run_tmux set-option -t "$session" mouse on
run_tmux set-option -t "$session" pane-border-status top
run_tmux set-option -t "$session" pane-border-format ' #{pane_title} '
run_tmux set-option -t "$session" @prelude_view motd

run_tmux select-pane -t "$top_pane" -T 'MOTD · pinned prototype'
run_tmux select-pane -t "$shell_pane" -T 'shell · exit to return'

# Isolated C-g prefix. C-g C-g forwards a literal prefix to the active program.
run_tmux set-option -t "$session" prefix C-g
run_tmux unbind-key C-b
run_tmux bind-key C-g send-prefix
run_tmux bind-key h run-shell 'prelude-workspace-view motd'
run_tmux bind-key d run-shell 'prelude-workspace-view docs'
run_tmux bind-key s run-shell 'prelude-workspace-view shell'
run_tmux bind-key z run-shell 'prelude-workspace-view zoom'
run_tmux bind-key m display-popup -E -w '90%' -h '90%' -d '#{pane_current_path}' menu
run_tmux bind-key l display-popup -E -w '85%' -h '70%' -d '#{pane_current_path}' prelude-init-log-popup
run_tmux bind-key q confirm-before -p 'exit workspace? (y/n)' "kill-session -t $session"

# Powerline-like persistent footer.
run_tmux set-option -t "$session" status on
run_tmux set-option -t "$session" status-position bottom
run_tmux set-option -t "$session" status-style 'fg=#8787af,bg=#0e0d11'
run_tmux set-option -t "$session" status-left-length 120
run_tmux set-option -t "$session" status-right-length 160
run_tmux set-option -t "$session" status-left '#[fg=#0e0d11,bg=#ff97d7,bold] prelude #[fg=#ff97d7,bg=#211f28]#[fg=#d6d2df,bg=#211f28] #{pane_current_path} #[fg=#211f28,bg=#0e0d11]#[fg=#8787af,bg=#0e0d11] #{@prelude_view} '
run_tmux set-option -t "$session" status-right '#[fg=#8787af]C-g #[fg=#d6d2df]h#[fg=#8787af] motd  #[fg=#d6d2df]d#[fg=#8787af] docs  #[fg=#d6d2df]m#[fg=#8787af] menu  #[fg=#d6d2df]l#[fg=#8787af] logs  #[fg=#d6d2df]s#[fg=#8787af] shell  #[fg=#d6d2df]z#[fg=#8787af] zoom  #[fg=#d6d2df]q#[fg=#8787af] exit '

# Shell-hook output gets a compact temporary pane, then remains available via
# C-g l. Splitting above the shell keeps MOTD/docs pinned at the top.
if [ -s "${PRELUDE_SHELL_INIT_LOG:-}" ]; then
  log_pane="$(run_tmux split-window -d -v -l 5 -P -F '#{pane_id}' -t "$top_pane" -c "$PWD" prelude-init-log-pane)"
  run_tmux select-pane -t "$log_pane" -T 'shell init · closes automatically'
  run_tmux run-shell -b "sleep 4; tmux kill-pane -t '$log_pane' 2>/dev/null || true"
fi

run_tmux select-pane -t "$shell_pane"

# tmux supplies the child with a genuine PTY; arbitrary interactive and
# alternate-screen programs work without Prelude emulating a terminal.
run_tmux attach-session -t "$session" || true
