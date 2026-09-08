# `prelude-preflight` prints the activation snippet for custom shellHooks:
#
#   eval "$(prelude-preflight)"
#
# The snippet itself is checked in at ./shell/preflight.bash and carries no
# build-time paths, exactly like `prelude hook`. It decides what to do from the
# shell it is evaluated in and delegates every project-specific fact — the MOTD
# binary and PRELUDE_INIT_QUIET opt-out — to the init file `$PRELUDE_INIT`
# names. Conventional nix-direnv consumers need only `use flake`; Prelude's
# generated init recognizes direnv while the cached shellHook is evaluated.
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

    Print shell code that activates this project's Prelude environment from a
    custom shellHook. Conventional nix-direnv .envrc files need only
    `use flake`.
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
