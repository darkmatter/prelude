# PROTOTYPE: compact transient view of captured shell-hook output.
printf '\033[2J\033[H'
if [ -s "${PRELUDE_SHELL_INIT_LOG:-}" ]; then
  tail -n 4 "$PRELUDE_SHELL_INIT_LOG"
else
  printf 'shell initialization produced no output\n'
fi
while true; do
  sleep 86400 &
  wait
done
