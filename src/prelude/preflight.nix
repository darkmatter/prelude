# `prelude-preflight` prints the loader activation snippet — the line consumers
# put in `.envrc` or a devshell `shellHook`:
#
#   eval "$(prelude-preflight)"
#
# The snippet itself is checked in at ./shell/preflight.bash and carries no
# build-time paths, exactly like `prelude hook`. It decides what to do from the
# shell it is evaluated in and delegates every project-specific fact — the MOTD
# binary and PRELUDE_INIT_QUIET opt-out — to the init file `$PRELUDE_INIT`
# names. Render requests do not exchange state between loaders.
{writeShellApplication}: {
  # Prelude's generated shell runtime directory (shell-init.nix `runtime`),
  # which carries preflight.bash next to the hook snippets.
  shellRuntime,
}:
writeShellApplication {
  name = "prelude-preflight";
  text = ''
    if [ "''${1:-}" = "--help" ] || [ "''${1:-}" = "-h" ]; then
      cat <<'EOF'
    usage: eval "$(prelude-preflight)"

    Print the shell code that activates this project's Prelude environment.
    Put it in .envrc (after `use flake`) or in a devshell shellHook; the
    printed code decides what to do from the shell it is evaluated in.
    EOF
      exit 0
    fi
    if [ "$#" -gt 0 ]; then
      echo "prelude-preflight: unexpected argument '$1'" >&2
      # The hint is the literal line a user types; nothing here expands.
      # shellcheck disable=SC2016
      echo 'hint: eval "$(prelude-preflight)"' >&2
      exit 2
    fi
    exec cat ${shellRuntime}/preflight.bash
  '';
  meta.description = "Print the shell code that activates a Prelude devshell environment";
}
