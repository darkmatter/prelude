# PROTOTYPE: full captured shell-hook output in a tmux popup.
if [ ! -s "${PRELUDE_SHELL_INIT_LOG:-}" ]; then
  printf '\n  shell initialization produced no output\n\n  press enter to close\n'
  read -r _
  exit 0
fi
exec less -R +G "$PRELUDE_SHELL_INIT_LOG"
