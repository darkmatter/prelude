# PROTOTYPE: run a genuine login shell in the focused tmux pane. When it exits,
# tear down the whole ephemeral session so attach returns to the parent shell.
shell_path="${SHELL:-/bin/sh}"
status=0

# Wait until the real client attaches: detached tmux sessions start at a
# temporary 80x24 size, so padding before attachment puts the prompt in the
# wrong place on larger terminals.
while ! tmux list-clients -t "$PRELUDE_MOTD_TMUX_SESSION" -F '#{client_name}' 2>/dev/null | grep -q .; do
  sleep 0.05
done
sleep 0.05

# Fill all but the last row of the final pane. The login shell's first prompt
# lands at the bottom; subsequent output/prompts scroll there naturally, while
# multiline input can grow upward through the half-height pane.
pane_height="$(tmux display-message -p -t "${TMUX_PANE:?}" '#{pane_height}')"
row=1
while [ "$row" -lt "$pane_height" ]; do
  printf '\n'
  row=$((row + 1))
done

case "$(basename "$shell_path")" in
  bash)
    prompt_dir="$(mktemp -d)"
    prompt_rc="$prompt_dir/bashrc"
    if [ -r "${HOME:-}/.bashrc" ]; then
      printf 'source %q\n' "$HOME/.bashrc" >"$prompt_rc"
    fi
    cat >>"$prompt_rc" <<'PRELUDE_PROMPT'
# Preserve shell initialization (including terminal colors), then replace only
# prompt state so the workspace footer remains the sole Powerline surface.
unset PROMPT_COMMAND
PS1='\[\e[38;2;135;135;175m\]\W \[\e[1;38;2;255;151;215m\]❯ \[\e[0m\]'
PS2='\[\e[38;2;74;69;86m\]· \[\e[0m\]'
PRELUDE_PROMPT
    "$shell_path" --noprofile --rcfile "$prompt_rc" -i || status=$?
    rm -rf "$prompt_dir"
    ;;
  zsh)
    prompt_dir="$(mktemp -d)"
    if [ -r "${HOME:-}/.zshrc" ]; then
      printf 'source %q\n' "$HOME/.zshrc" >"$prompt_dir/.zshrc"
    fi
    cat >>"$prompt_dir/.zshrc" <<'PRELUDE_PROMPT'
# Keep user initialization/colors, but detach prompt-plugin redraw hooks before
# installing the compact workspace prompt.
precmd_functions=()
preexec_functions=()
unfunction precmd preexec 2>/dev/null || true
PROMPT='%F{#8787af}%1~ %B%F{#ff97d7}❯%f%b '
PROMPT2='%F{#4a4556}·%f '
RPROMPT=
PRELUDE_PROMPT
    ZDOTDIR="$prompt_dir" "$shell_path" -d || status=$?
    rm -rf "$prompt_dir"
    ;;
  *)
    "$shell_path" -l || status=$?
    ;;
esac

tmux -L "$PRELUDE_MOTD_TMUX_SOCKET" kill-session -t "$PRELUDE_MOTD_TMUX_SESSION" 2>/dev/null || true
exit "$status"
