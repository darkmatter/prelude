# App launcher: one row per app, an environment selector, a live health
# traffic light, and a clickable URL. Two front ends over one core —
# `portal` (terminal) and `portal-web` (local web page).
#
# The Go binary is config-independent, exactly like menu.nix: each
# configuration becomes a JSON file baked into a thin wrapper.
{
  lib,
  writeShellApplication,
  writeText,
  buildGoModule,
  symlinkJoin,
  ...
}:
# Flat config: { theme?, palette?, colorProfile?, project?, listen?,
#                timeoutMs?, maxWidth?, apps? }
config: let
  d = import ./defaults.nix;
  plib = import ./lib.nix {inherit lib;};

  pal = plib.resolvePalette (config.theme or d.theme) (config.palette or d.palette);

  cfg = {
    project = config.project or d.project;
    colorProfile = config.colorProfile or d.colorProfile;
    listen = config.listen or "127.0.0.1:7777";
    timeoutMs = config.timeoutMs or 3000;
    maxWidth = config.maxWidth or 76;
    apps = config.apps or {};
  };

  # Attribute sets are unordered in Nix, so ordering is explicit: `order`
  # first, then name. Without this the row order would change between
  # evaluations for no visible reason.
  sortedApps =
    lib.sort
    (a: b:
      if a.value.order != b.value.order
      then a.value.order < b.value.order
      else a.name < b.name)
    (lib.attrsToList cfg.apps);

  # Environments keep declaration-independent, stable order too, but here the
  # attribute name order is what a reader authored, so name order is the least
  # surprising choice.
  environmentsOf = app:
    map (entry: {
      name = entry.name;
      url = entry.value.url;
      health = entry.value.health;
      gated = entry.value.gated;
      headers = entry.value.headers;
      headersFromEnv = entry.value.headersFromEnv;
    }) (lib.attrsToList app.environments);

  configFile = writeText "prelude-portal.json" (
    builtins.toJSON {
      project = cfg.project;
      colorProfile = cfg.colorProfile;
      palette = pal;
      maxWidth = cfg.maxWidth;
      listen = cfg.listen;
      timeoutMs = cfg.timeoutMs;
      apps =
        map (entry: {
          name = entry.name;
          description = entry.value.description;
          environments = environmentsOf entry.value;
        })
        sortedApps;
    }
  );

  portalBin = buildGoModule {
    pname = "prelude-portal";
    version = "0.1.0";
    src = ../.;
    subPackages = ["cmd/portal"];
    doCheck = false;
    vendorHash = "sha256-BHrU5pKVDuGDq0ZHbHKcUBa5olzHzfgoJXzv2IGXY4U=";
    ldflags = [
      "-s"
      "-w"
      "-X main.defaultConfigPath=${configFile}"
    ];
    meta = {
      description = "App launcher with live health lights (terminal + web)";
      mainProgram = "portal";
    };
  };

  # Two wrappers over one binary: the front ends share a prober and a
  # catalogue, so shipping them as separate derivations would only duplicate
  # the config path.
  portalWrapper = writeShellApplication {
    name = "portal";
    text = ''
      exec ${lib.getExe portalBin} --config ${configFile} "$@"
    '';
  };

  portalWebWrapper = writeShellApplication {
    name = "portal-web";
    text = ''
      exec ${lib.getExe portalBin} --config ${configFile} --serve "$@"
    '';
  };
in
  symlinkJoin {
    name = "portal";
    paths = [portalWrapper portalWebWrapper];
    passthru = {
      inherit configFile portalBin;
      wrappers = [portalWrapper portalWebWrapper];
    };
  }
