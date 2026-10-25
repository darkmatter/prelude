# Dogfood runtime environment. Package builders and the dedicated native
# workspace shell keep their normal stdenv; this shell loads tools explicitly.
{
  pkgs,
  lib,
  config,
  docsAutomation,
  previews,
  typescript,
  ...
}: let
  packages =
    [
      config.packages.prelude-shell
      config.packages.prelude-title-previews
      docsAutomation.record
      docsAutomation.sync
      previews
      typescript.sync
      # These are still Nix's wrapped native tools. Skipping their setup hooks
      # avoids exporting build metadata; their built-in libc paths still work.
      pkgs.stdenv.cc
      pkgs.stdenv.cc.bintools
    ]
    ++ (with pkgs; [
      bashInteractive
      git
      nix
      curl
      pkg-config
      shellcheck
      nixfmt
    ])
    ++ pkgs.stdenvNoCC.initialPath;
  # Select executable outputs from the authored packages. Reading normalized
  # mkShell.nativeBuildInputs instead can select a library output (ShellCheck).
  commandPath = lib.makeBinPath (lib.unique packages);
  bash = lib.getExe pkgs.bashInteractive;
  setup = pkgs.writeTextDir "setup" ''
    : "''${outputs:=out}"
    runHook() { eval "$shellHook"; }
  '';
  # A buildable output roots all tools and activation artifacts against GC.
  profile = pkgs.writeText "prelude-devshell-packages" (
    lib.concatMapStringsSep "\n" toString (packages ++ [pkgs.cacert typescript.libraryPath])
  );
in
  builtins.derivation {
    name = "prelude-devshell";
    system = pkgs.stdenv.hostPlatform.system;
    builder = bash;
    args = ["-c" "${pkgs.coreutils}/bin/ln -s ${profile} $out"];
    stdenv = setup;
    # Nix itself needs mktemp before shellHook, including in a pure shell.
    # nix develop / nix-direnv append the caller's PATH when loading this value.
    PATH = commandPath;
    shellHook = ''
      unset NIX_BUILD_TOP NIX_BUILD_CORES NIX_STORE
      unset TEMP TEMPDIR TMP TMPDIR builder out outputs stdenv system dontAddDisableDepTrack
      case "''${SHELL-}" in
        ""|/noshell|/sbin/nologin) export SHELL=${bash} ;;
      esac

      # Explicit runtime configuration replaces the compiler/build setup hooks.
      export PRELUDE_LIB=${typescript.libraryPath}
      export SSL_CERT_FILE="''${SSL_CERT_FILE:-${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt}"
      export NIX_SSL_CERT_FILE="''${NIX_SSL_CERT_FILE:-$SSL_CERT_FILE}"
      . ${config.packages.prelude-shell}/nix-support/setup-hook
      # The package setup hook appends activation to shellHook for mkShell.
      # We own activation here, so discard that exported build-time string.
      unset shellHook

      # Plain function: exporting it creates BASH_FUNC_r%% and breaks zsh.
      r() { exec nix develop "$@"; }
      if [ "''${_PRELUDE_INIT_LOADED-}" != "$PRELUDE_INIT" ]; then
        . "$PRELUDE_INIT"
      fi
    '';
  }
