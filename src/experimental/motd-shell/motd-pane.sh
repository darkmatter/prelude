# PROTOTYPE: render the existing MOTD in a non-interactive tmux pane.
render_motd() {
  printf '\033[2J\033[H'
  motd
}

trap render_motd WINCH
render_motd

# Keep the pane alive and rerender whenever tmux resizes it.
while true; do
  sleep 86400 &
  wait
 done
