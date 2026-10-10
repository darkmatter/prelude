# prelude.workspace.* — explicitly launched Bash workspace, separate from activation.
{lib, ...}: {
  options.prelude.workspace.enable = lib.mkEnableOption ''
    explicitly launched Bash workspace (`packages.prelude-workspace` and
    `x prelude:workspace`). Requires `prelude.menu.enable`,
    `prelude.motd.enable`, and non-empty `prelude.docs.pages`; it never enables
    those components or launches during shell activation. `--starship` opts
    in to the consumer's prompt settings independently of `prelude.prompt.enable`
  '';
}
