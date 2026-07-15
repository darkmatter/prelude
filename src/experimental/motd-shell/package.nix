# PROTOTYPE package: the launcher is the only exposed executable; helper scripts
# are retained in its runtime closure and PATH.
{
  pkgs,
  motd,
  menu,
  docs,
}:
let
  motdPane = pkgs.writeShellApplication {
    name = "prelude-motd-pane";
    runtimeInputs = [
      motd
      pkgs.coreutils
    ];
    text = builtins.readFile ./motd-pane.sh;
  };

  docsPane = pkgs.writeShellApplication {
    name = "prelude-docs-pane";
    runtimeInputs = [
      docs
      pkgs.tmux
      motdPane
    ];
    text = builtins.readFile ./docs-pane.sh;
  };

  childShell = pkgs.writeShellApplication {
    name = "prelude-motd-child-shell";
    runtimeInputs = [
      pkgs.coreutils
      pkgs.gnugrep
      pkgs.tmux
    ];
    text = builtins.readFile ./child-shell.sh;
  };

  workspaceView = pkgs.writeShellApplication {
    name = "prelude-workspace-view";
    runtimeInputs = [
      pkgs.tmux
      motdPane
      docsPane
    ];
    text = builtins.readFile ./workspace-view.sh;
  };

  initLogPane = pkgs.writeShellApplication {
    name = "prelude-init-log-pane";
    runtimeInputs = [ pkgs.coreutils ];
    text = builtins.readFile ./init-log-pane.sh;
  };

  initLogPopup = pkgs.writeShellApplication {
    name = "prelude-init-log-popup";
    runtimeInputs = [ pkgs.less ];
    text = builtins.readFile ./init-log-popup.sh;
  };
in
pkgs.writeShellApplication {
  name = "motd-shell-experiment";
  runtimeInputs = [
    pkgs.coreutils
    pkgs.tmux
    menu
    motdPane
    childShell
    workspaceView
    initLogPane
    initLogPopup
  ];
  text = builtins.readFile ./launcher.sh;
  meta = {
    description = "PROTOTYPE: pinned Prelude MOTD/docs above a real child shell";
    mainProgram = "motd-shell-experiment";
  };
}
