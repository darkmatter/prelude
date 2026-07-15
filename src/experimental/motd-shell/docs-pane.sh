# PROTOTYPE: keep docs rendered in the pinned pane. Quitting docs restores the
# MOTD rather than destroying the pane/layout.
docs || true
tmux set-option -t "$PRELUDE_MOTD_TMUX_SESSION" @prelude_view motd
tmux select-pane -t "$PRELUDE_MOTD_TOP_PANE" -T 'MOTD · pinned prototype'
exec prelude-motd-pane
