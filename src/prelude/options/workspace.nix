# prelude.workspace.* — default-enabled Bash workspace with guarded interactive activation.
{lib, ...}: {
  options.prelude.workspace.enable =
    (lib.mkEnableOption ''
      the Bash workspace (`packages.prelude-workspace`, bare `prelude`, and `x prelude:workspace`).
      Requires `prelude.menu.enable`, `prelude.motd.enable`, and non-empty
      `prelude.docs.pages`; it does not implicitly enable those components.
      Interactive `nix develop` sources generated project init and enters the
      workspace in the foreground in Bash with a TTY, with the full footer and
      catalogue completion. `.envrc` stays `use flake`: direnv provides the
      lightweight environment, MOTD, and theming for an already-initialized Starship
      prompt. No extra Prelude Bash rc hook is required; the public `prelude hook bash`
      remains optional for already-configured interactive handoffs after direnv.
      Each init is stamped before launch, so exit or failure does not immediately
      reopen it; leaving/reentering the devshell or a changed init permits reentry.
      Noninteractive, envrc, lorri, zsh, and non-TTY contexts never auto-launch it;
      `PRELUDE_WORKSPACE_ACTIVE` prevents child recursion. Workspace mode skips
      legacy BLE initialization. The workspace uses Starship by default; automatic
      entry passes `--starship=false` (the fixed `prelude $` prompt) when
      `prelude.prompt.enable` is off. Manual launches (bare `prelude`,
      `x prelude:workspace`, `prelude-workspace`) use Starship unless given
      `--starship=false`
    '')
    // {default = true;};
}
