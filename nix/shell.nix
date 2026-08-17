# Dogfood devshell: explicitly compose Prelude's generated packages with the
# project-specific tools and hooks.
#
# Prelude supplies the `x` and `docs` commands. The project only adds `r`
# as a local convenience for replacing the current shell.
{
  pkgs,
  config,
  docsAutomation,
  previews,
  ...
}:
pkgs.mkShell {
  packages =
    [
      config.packages.prelude
      config.packages.prelude-portal
      docsAutomation.record
      docsAutomation.sync
      previews
    ]
    ++ (with pkgs; [
      shellcheck
      nixfmt
    ]);
  DIRENV_LOG_FORMAT = "";
  shellHook = ''
    r() { exec nix develop "$@"; }

    # Starship re-resolves this path on every prompt render.
    export STARSHIP_CONFIG=${config.packages.prompt}
    # `config.packages.prelude` appends its idempotent, current-shell init after
    # this hook. It composes MOTD, ble.sh, completion, Starship, and the native
    # status line without starting another shell.
    #
    # `r` is deliberately a plain function and never `export -f`ed. Exporting a
    # function stores it as the environment variable `BASH_FUNC_r%%`, and a
    # `BASH_VERSION` guard cannot prevent that: the guard runs inside the Nix
    # builder, which is always Bash, never the user's shell. Loaders that
    # capture the environment (lorri, direnv) then replay that name into
    # whatever shell the user runs, and zsh rejects `%` in a variable name.
    # Shell-specific setup belongs in `prelude hook`, which runs where $SHELL
    # is actually meaningful.
  '';
}
