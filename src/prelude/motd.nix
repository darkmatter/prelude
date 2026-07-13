# MOTD package builder. Nix resolves and validates configuration, then embeds a
# normalized JSON file into the Go renderer at link time. Runtime terminal
# layout, probes, Git state, and styling live in src/motd — never in generated
# shell source.
{
  lib,
  writeText,
  buildGoModule,
  ...
}:

# Component config: { theme?, palette?, project?, header?, description?, env?,
#                     commands?, recipes?, git?, gettingStarted?, shortcuts?,
#                     background?, clearScreen?, margin?, padding?, align?,
#                     width?, maxWidth? }
config:

let
  d = import ./defaults.nix;
  plib = import ./lib.nix { inherit lib; };

  pal = plib.resolvePalette (config.theme or d.theme) (config.palette or d.palette);
  colorProfile = config.colorProfile or d.colorProfile;
  project = config.project or d.project;
  m = d.motd // config;
  header = d.motd.header // (m.header or { });
  gettingStarted = d.motd.gettingStarted // (m.gettingStarted or { });
  commands = plib.normalizeCommands (m.commands or { });
  recipes = plib.normalizeRecipes (m.recipes or { });

  resolveBg =
    value:
    if value == null || value == false then
      null
    else if value == true then
      pal.bg
    else
      value;

  windowBackground = resolveBg m.windowBackground;
  explicitBackground = resolveBg m.background;
  background = if explicitBackground != null then explicitBackground else windowBackground;
  margin = plib.resolveSpacing (d.motd.margin // (m.margin or { }));
  padding = plib.resolveSpacing (d.motd.padding // (m.padding or { }));

  descriptionDefaults = plib.withRole pal "fg" (
    plib.textDefaults // d.motd.description // (m.description or { })
  );
  description = descriptionDefaults // {
    background =
      if descriptionDefaults.background != null then descriptionDefaults.background else background;
  };

  # A probe remains a shell snippet by contract, but only the Go runtime
  # executes it. Empty strings encode the inactive side of the value/probe sum
  # so the JSON boundary contains no nullable scalar fields.
  env = map (
    item:
    let
      value = item.value or null;
      probe = item.probe or null;
    in
    assert lib.assertMsg (
      (value == null) != (probe == null)
    ) "motd: env item \"${item.label or "?"}\" must set exactly one of `value` or `probe`";
    {
      label = item.label;
      value = if value == null then "" else value;
      probe = if probe == null then "" else probe;
    }
  ) m.env;

  shortcuts = map (s: {
    command = s.command;
    alias = s.alias or "";
  }) (m.shortcuts or [ ]);

  jsonColor = value: if value == null then "" else toString value;

  configFile = writeText "prelude-motd.json" (
    builtins.toJSON {
      inherit
        project
        colorProfile
        margin
        env
        commands
        recipes
        shortcuts
        ;

      palette = pal;
      background = jsonColor background;
      windowBackground = jsonColor windowBackground;
      clearScreen = m.clearScreen;
      align = m.align;
      inherit padding;
      header = {
        inherit (header)
          titleStyle
          tagline
          statusLabel
          statusLabelCompact
          statusText
          ;
      };
      description = {
        inherit (description)
          text
          bold
          italic
          faint
          ;
        tips = description.tips or [ ];
        foreground = jsonColor description.foreground;
        background = jsonColor description.background;
      };
      git = m.git;
      gettingStarted = {
        inherit (gettingStarted) heading commandsLabel examplesLabel;
      };
      width = if m.fullscreen or false then 0 else if m.width == "full" then 0 else m.width;
      maxWidth = if m.fullscreen or false then 0 else if m.maxWidth == null then 0 else m.maxWidth;
    }
  );
in
assert lib.assertOneOf "motd align" m.align [
  "left"
  "center"
  "right"
];
assert lib.assertOneOf "motd colorProfile" colorProfile [
  "auto"
  "truecolor"
  "ansi256"
];
assert lib.assertOneOf "motd header.titleStyle" header.titleStyle [
  "plain"
  "spine"
  "bracketed"
  "label"
];
assert lib.assertMsg (
  m.width == "full" || builtins.isInt m.width
) "motd: width must be an integer or \"full\"";
assert lib.assertMsg (
  m.maxWidth == null || builtins.isInt m.maxWidth
) "motd: maxWidth must be an integer or null";
buildGoModule {
  pname = "motd";
  version = "0.1.0";
  src = ../.;
  subPackages = [ "motd" ];
  vendorHash = "sha256-5Vq39NH18R7zee+LHANoHAbjw3iuE9+SoYxF9OqiamQ=";
  ldflags = [
    "-s"
    "-w"
    "-X main.defaultConfigPath=${configFile}"
  ];
  meta = {
    description = "Devshell MOTD banner rendered in Go";
    mainProgram = "motd";
  };
}
