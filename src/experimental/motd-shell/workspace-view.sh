# PROTOTYPE: tmux key-binding target for persistent workspace views.
view="${1:-}"
case "$view" in
  docs)
    tmux respawn-pane -k -t "$PRELUDE_MOTD_TOP_PANE" -c "$PWD" prelude-docs-pane
    tmux set-option -t "$PRELUDE_MOTD_TMUX_SESSION" @prelude_view docs
    tmux select-pane -t "$PRELUDE_MOTD_TOP_PANE" -T 'docs · pinned'
    tmux select-pane -t "$PRELUDE_MOTD_TOP_PANE"
    ;;
  motd)
    tmux respawn-pane -k -t "$PRELUDE_MOTD_TOP_PANE" -c "$PWD" prelude-motd-pane
    tmux set-option -t "$PRELUDE_MOTD_TMUX_SESSION" @prelude_view motd
    tmux select-pane -t "$PRELUDE_MOTD_TOP_PANE" -T 'MOTD · pinned prototype'
    tmux select-pane -t "$PRELUDE_MOTD_SHELL_PANE"
    ;;
  shell)
    tmux select-pane -t "$PRELUDE_MOTD_SHELL_PANE"
    ;;
  zoom)
    tmux select-pane -t "$PRELUDE_MOTD_SHELL_PANE"
    tmux resize-pane -Z -t "$PRELUDE_MOTD_SHELL_PANE"
    ;;
  *)
    printf 'unknown workspace view: %s\n' "$view" >&2
    exit 2
    ;;
esac
