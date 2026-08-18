{lib, ...}: let
  environmentType = lib.types.submodule {
    options = {
      url = lib.mkOption {
        type = lib.types.str;
        description = ''
          The URL to open. Also the probe target unless `health` is set.
        '';
        example = "http://127.0.0.1:1339";
      };

      health = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = ''
          Probe this instead of `url`. A UI route often answers 200 to an
          unauthenticated visitor while the thing worth knowing about is a
          health endpoint behind it.
        '';
        example = "http://127.0.0.1:1338/health";
      };

      headers = lib.mkOption {
        type = lib.types.attrsOf lib.types.str;
        default = {};
        description = ''
          Extra headers sent with the probe. For non-secret values only — an
          API version, a tenant id, a header a staging proxy expects. This
          catalogue is generated into the store and lives in a git repo, so a
          credential here is a credential in the repo; use `headersFromEnv`.
        '';
        example = lib.literalExpression ''
          { "X-Api-Version" = "2026-01-01"; }
        '';
      };

      headersFromEnv = lib.mkOption {
        type = lib.types.attrsOf lib.types.str;
        default = {};
        description = ''
          Headers whose values are read from the environment at probe time,
          mapping header name to variable name. Lets a gated environment be
          probed for real health rather than reported as `gated`, without the
          portal knowing anything about your auth provider or going looking for
          credentials on its own. A variable that is unset is skipped, so an
          unconfigured shell simply probes anonymously.
        '';
        example = lib.literalExpression ''
          {
            "CF-Access-Client-Id" = "CLOUDFLARE_ACCESS_CLIENT_ID";
            "CF-Access-Client-Secret" = "CLOUDFLARE_ACCESS_CLIENT_SECRET";
          }
        '';
      };

      gated = lib.mkOption {
        type = lib.types.bool;
        default = false;
        description = ''
          Marks an environment fronted by SSO. The probe detects a redirect to
          a known identity provider on its own and reports `gated` rather than
          up or down; this flag documents the intent for readers.
        '';
      };
    };
  };

  appType = lib.types.submodule {
    options = {
      description = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "One line under the app name.";
      };

      order = lib.mkOption {
        type = lib.types.int;
        default = 50;
        description = "Sort weight; ties fall back to the app name.";
      };

      environments = lib.mkOption {
        type = lib.types.attrsOf environmentType;
        default = {};
        description = ''
          Deployments of this app, keyed by environment name. Declare the one
          you use most first — the terminal front end selects the first
          environment on open.
        '';
        example = lib.literalExpression ''
          {
            local.url = "http://127.0.0.1:1339";
            prod = {
              url = "https://chat.example.dev";
              gated = true;
            };
          }
        '';
      };
    };
  };
in {
  options.prelude.portal = {
    enable = lib.mkEnableOption "the app launcher portal";

    listen = lib.mkOption {
      type = lib.types.str;
      default = "127.0.0.1:7777";
      description = ''
        Address for the web front end. Loopback by default: the portal lists
        internal hostnames and should not be reachable from the network.
      '';
    };

    timeoutMs = lib.mkOption {
      type = lib.types.int;
      default = 3000;
      description = ''
        Per-probe budget. Kept short so the portal stays responsive on a
        laptop that is off the VPN.
      '';
    };

    maxWidth = lib.mkOption {
      type = lib.types.int;
      default = 76;
      description = "Content width cap for the terminal front end.";
    };

    apps = lib.mkOption {
      type = lib.types.attrsOf appType;
      default = {};
      description = ''
        Launchable apps, keyed by name. Each declares one or more
        environments; the portal shows a health traffic light per environment
        and opens the URL on select.
      '';
      example = lib.literalExpression ''
        {
          chat = {
            description = "Waku chat UI";
            order = 10;
            environments = {
              local.url = "http://127.0.0.1:1339";
              prod = {
                url = "https://chat.example.dev";
                gated = true;
              };
            };
          };
        }
      '';
    };
  };
}
