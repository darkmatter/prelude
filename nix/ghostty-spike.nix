# Repository-only experiment: neither the native library nor the spike enters
# a consumer's closure or the default devshell. Nixpkgs' lock pins the C API.
{
  pkgs,
  lib,
  config,
  ...
}: let
  normalPromptConfig = config.packages.prelude-prompt;
  prompt = config.packages.prelude-shell.promptPreset {
    prefix = "Alt +";
    shortcuts = [
      {
        alias = "m";
        command = "motd";
      }
      {
        alias = "x";
        command = "menu";
      }
      {
        alias = "d";
        command = "docs";
      }
    ];
  };
  promptConfig = prompt.live;

  # An inherited Prelude default is not a user-owned Starship override.
  # Keep this selection identical in the wrapper and the dedicated devshell.
  promptInit = ''
    if [ -z "''${STARSHIP_CONFIG-}" ] || [ "$STARSHIP_CONFIG" = ${lib.escapeShellArg (toString normalPromptConfig)} ]; then
      export STARSHIP_CONFIG=${lib.escapeShellArg (toString promptConfig)}
    fi

  '';
  completionInit = config.packages.prelude-shell.completionInit;
  menuConfig = config.packages.prelude-menu.menuConfig;
  runtime = [
    pkgs.bashInteractive
    pkgs.starship
  ];
  surfaces = [
    config.packages.prelude-menu
    config.packages.prelude-docs
    config.packages.prelude-motd
  ];
  native = pkgs.buildGo126Module {
    pname = "prelude-ghostty-spike";
    version = "0.1.0";
    src = lib.fileset.toSource {
      root = ../prototypes/ghostty;
      fileset =
        lib.fileset.fileFilter (
          file:
            file.hasExt "go"
            || file.hasExt "c"
            || file.hasExt "h"
            || lib.elem file.name [
              "go.mod"
              "go.sum"
            ]
        )
        ../prototypes/ghostty;
    };
    vendorHash = "sha256-dRR8YIxXmK6wT9S1R9uL2aW6Q3yb3t7SMLe/x/LrM/U=";
    nativeBuildInputs = [pkgs.pkg-config];
    buildInputs = [pkgs.libghostty-vt];
    nativeCheckInputs = runtime ++ surfaces;
    env = {
      CGO_ENABLED = "1";
      STARSHIP_CONFIG = promptConfig;
      PRELUDE_GHOSTTY_TEST_STARSHIP_CONFIG = promptConfig;

      PRELUDE_COMPLETION_INIT = completionInit;
      PRELUDE_MENU_CONFIG = menuConfig;
    };
    doCheck = true;
    checkFlags = ["-count=1"];
    meta.mainProgram = "prelude-ghostty-spike";
  };
  package = pkgs.symlinkJoin {
    name = "prelude-ghostty-spike";
    paths = [native];
    nativeBuildInputs = [pkgs.makeWrapper];
    postBuild = ''
      wrapProgram "$out/bin/prelude-ghostty-spike" \
        --prefix PATH : ${lib.makeBinPath (runtime ++ surfaces)} \
        --run ${lib.escapeShellArg promptInit} \
        --set-default PRELUDE_COMPLETION_INIT ${completionInit} \
        --set-default PRELUDE_MENU_CONFIG ${menuConfig}
    '';
    passthru = {inherit promptConfig promptInit;};
    meta = {
      description = "libghostty-vt shell spike with Prelude leader chords and movable surface panes";
      mainProgram = "prelude-ghostty-spike";
    };
  };
in {
  inherit package;
  check = native;
  shell = pkgs.mkShell {
    packages =
      [
        pkgs.go
        pkgs.pkg-config
        pkgs.libghostty-vt
      ]
      ++ runtime
      ++ surfaces;
    PRELUDE_GHOSTTY_TEST_STARSHIP_CONFIG = promptConfig;

    PRELUDE_COMPLETION_INIT = completionInit;
    PRELUDE_MENU_CONFIG = menuConfig;
    shellHook = promptInit;
  };
}
