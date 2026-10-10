# prelude.workspace.* — opt-in Bash workspace with guarded interactive activation.
{lib, ...}: {
  options.prelude.workspace.enable = lib.mkEnableOption ''
    the Bash workspace (`packages.prelude-workspace` and `x prelude:workspace`).
    Requires `prelude.menu.enable`, `prelude.motd.enable`, and non-empty
    `prelude.docs.pages`; it does not implicitly enable those components.
    Generated project init enters the workspace in the foreground in interactive
    Bash with a TTY. Keep `.envrc` as `use flake`; the existing Bash rc
    `eval "$(prelude hook bash)"` sources the init after direnv. Each init is
    stamped before launch, so exit or failure does not immediately reopen it;
    leaving/reentering or a changed init permits reentry. Noninteractive, envrc,
    lorri, zsh, and non-TTY contexts never auto-launch it, and
    `PRELUDE_WORKSPACE_ACTIVE` prevents child recursion. Workspace mode skips
    legacy BLE initialization. Automatic Starship follows `prelude.prompt.enable`;
    manual `x prelude:workspace` and `prelude-workspace --starship` remain supported,
    with `--starship` independent of `prelude.prompt.enable`
  '';
}
