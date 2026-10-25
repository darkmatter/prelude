# Bash workspace for interactive init and explicit entry. Evaluated artifacts
# stay in the launcher; the native derivation is shared across configurations.
# Nixpkgs' lock pins the C API.
{
  pkgs,
  lib,
  menuPkg,
  docsPkg,
  motdPkg,
  completionInit,
  normalPromptConfig,
  promptConfig,
}: let
  # An inherited Prelude default is not a user-owned Starship override.
  # Keep this selection identical in the wrapper and the dedicated devshell.
  promptInit = ''
    if [ -z "''${STARSHIP_CONFIG-}" ] || [ "$STARSHIP_CONFIG" = ${lib.escapeShellArg (toString normalPromptConfig)} ]; then
      export STARSHIP_CONFIG=${lib.escapeShellArg (toString promptConfig)}
    fi

  '';
  menuConfig = menuPkg.menuConfig;
  # Inside the workspace the MOTD card prints at the top without clearing or
  # filling the screen, so the first prompt follows it. The launcher, the
  # dedicated devshell, and the check all point the private Bash's `motd` at
  # this variant; the plain-shell greeting is unchanged.
  motdWorkspaceConfig = motdPkg.workspaceConfigFile;
  runtime = [
    pkgs.bashInteractive
    pkgs.starship
  ];
  surfaces = [menuPkg docsPkg motdPkg];
  native = pkgs.buildGo126Module {
    pname = "prelude-workspace";
    version = "0.1.0";
    src = lib.fileset.toSource {
      root = ../cmd/prelude-workspace;
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
        ../cmd/prelude-workspace;
    };
    vendorHash = "sha256-dRR8YIxXmK6wT9S1R9uL2aW6Q3yb3t7SMLe/x/LrM/U=";
    nativeBuildInputs = [pkgs.pkg-config];
    buildInputs = [pkgs.libghostty-vt];
    env.CGO_ENABLED = "1";
    doCheck = false;
    meta.mainProgram = "prelude-workspace";
  };
  # The full native/PTY suite requires the repository's catalogue. The root
  # flake selects this check with dogfood inputs; consumer launchers use only
  # `native`, so building them never runs catalogue-dependent tests.
  check = native.overrideAttrs (old: {
    pname = "prelude-workspace-check";
    nativeCheckInputs = runtime ++ surfaces;
    env =
      (old.env or {})
      // {
        STARSHIP_CONFIG = promptConfig;
        PRELUDE_GHOSTTY_TEST_STARSHIP_CONFIG = promptConfig;
        PRELUDE_COMPLETION_INIT = completionInit;
        PRELUDE_MENU_CONFIG = menuConfig;
        PRELUDE_MOTD_CONFIG = motdWorkspaceConfig;
      };
    doCheck = true;
    checkFlags = ["-count=1"];
  });
  package = pkgs.symlinkJoin {
    name = "prelude-workspace";
    paths = [native];
    nativeBuildInputs = [pkgs.makeWrapper];
    postBuild = ''
      wrapProgram "$out/bin/prelude-workspace" \
        --prefix PATH : ${lib.makeBinPath (runtime ++ surfaces)} \
        --run ${lib.escapeShellArg promptInit} \
        --set PRELUDE_WORKSPACE_ACTIVE 1 \
        --set PRELUDE_COMPLETION_INIT ${completionInit} \
        --set PRELUDE_MENU_CONFIG ${menuConfig} \
        --set PRELUDE_MOTD_CONFIG ${motdWorkspaceConfig}
    '';
    passthru = {inherit native promptConfig promptInit;};
    meta = {
      description = "Bash workspace with Prelude keyboard chords and movable menu and docs panes";
      mainProgram = "prelude-workspace";
    };
  };
in {
  inherit package native check promptConfig promptInit;
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
    PRELUDE_MOTD_CONFIG = motdWorkspaceConfig;
    shellHook = promptInit;
  };
}
