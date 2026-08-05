# prelude.prompt.* options — themed starship config for the devshell.
#
# `packages.prompt` is a starship.toml themed from the palette. Starship
# re-resolves $STARSHIP_CONFIG on every prompt render and direnv propagates
# env vars, so the devshell only needs `packages.prelude` plus:
#
#   shellHook = ''export STARSHIP_CONFIG=${config.packages.prompt}'';
#
# The aggregate package supplies Starship, ble.sh, and bash-completion when
# this is enabled. Its setup hook sources one idempotent `prelude-init` file in
# the interactive shell that `nix develop` already started. That file renders
# the MOTD, installs catalogue completion, initializes Starship, and enables
# the generated left Powerline prompt plus fixed Bash status row.
# Non-interactive direnv evaluation remains inert.
{ lib, config, ... }:
let
  maxTTLCounts = {
    ms = 9223372036854;
    s = 9223372036;
    m = 153722867;
    h = 2562047;
    d = 106751;
    w = 15250;
  };
  positiveDuration =
    value:
    if !builtins.isString value then
      false
    else
      let
        match = builtins.match "([1-9][0-9]*)(ms|s|m|h|d|w)" value;
      in
      if match == null then
        false
      else
        let
          count = builtins.tryEval (builtins.fromJSON (builtins.elemAt match 0));
          unit = builtins.elemAt match 1;
        in
        count.success && count.value <= maxTTLCounts.${unit};

  localServerType = lib.types.submodule {
    options = {
      command = lib.mkOption {
        type = lib.types.str;
        description = "Canonical `prelude.commands` key used to start the local server.";
      };
      check = lib.mkOption {
        type = lib.types.str;
        description = "Explicit shell command used to check local-server health.";
      };
      ttl = lib.mkOption {
        type = lib.types.addCheck lib.types.str positiveDuration;
        description = "Positive cache lifetime (for example `5m` or `30s`); values that overflow the runtime duration are rejected.";
      };
    };
  };
  defaults = import ../defaults.nix;
in
{
  options.prelude.prompt = {
    enable = lib.mkEnableOption "themed starship prompt config (`packages.prompt` = starship.toml)";

    settings = lib.mkOption {
      type = (lib.types.attrsOf lib.types.anything) // {
        description = "TOML value";
      };
      default = defaults.prompt.settings;
      description = ''
        Starship settings merged (recursively) over the themed defaults.
        See <https://starship.rs/config/>.
      '';
      example = {
        add_newline = true;
        format = "$directory$git_branch$character";
      };
    };

    configFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = defaults.prompt.configFile;
      description = ''
        Use this starship.toml verbatim instead of the generated themed config.
        Prelude leaves this config and its prompt/status behavior fully user-owned.
      '';
    };

    localServer = lib.mkOption {
      type = lib.types.nullOr localServerType;
      default = null;
      description = ''
        Explicitly opt in to cached asynchronous local-server health. `command`
        must be a canonical key from `prelude.commands`; `check` is executed
        only by the generated refresh runtime.
      '';
      apply =
        value:
        if value == null then
          null
        else
          # Per-system package commands are merged later, so resolve the key
          # against that final catalogue in module.nix rather than rejecting a
          # valid package-backed command here.
          assert lib.assertMsg (
            lib.strings.trim value.check != ""
          ) "prelude.prompt.localServer.check must not be empty";
          value;
    };
  };
}
