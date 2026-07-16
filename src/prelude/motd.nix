# MOTD package builder. Nix resolves and validates configuration, then embeds a
# normalized JSON file into the Go renderer at link time. Runtime terminal
# layout, probes, Git state, and styling live in internal/motd — never in generated
# shell source.
{
  lib,
  writeText,
  buildGoModule,
  ...
}:

# Component config: { theme?, palette?, project?, commandCatalog?, title?,
#                     header?, description?, env?, commands?, recipes?, gettingStarted?,
#                     shortcuts?, background?, clearScreen?, margin?, padding?,
#                     align?, width?, maxWidth? }
config:

let
  d = import ./defaults.nix;
  plib = import ./lib.nix { inherit lib; };

  pal = plib.resolvePalette (config.theme or d.theme) (config.palette or d.palette);
  colorProfile = config.colorProfile or d.colorProfile;
  project = config.project or d.project;
  m = d.motd // config;
  titleIn = d.motd.title // (m.title or { });
  title = if titleIn.text == null then "" else builtins.readFile titleIn.text;
  titleAlign = titleIn.align;
  headerIn = d.motd.header // (m.header or { });
  taglineIn = d.motd.header.tagline // (headerIn.tagline or { });
  statusHintIn = d.motd.header.statusHint // (headerIn.statusHint or { });
  # Header bg: true → raised auto bar; null/false → transparent; color/relative → solid.
  splitHeaderBg =
    value:
    if value == null || value == false then
      {
        color = null;
        relative = 0;
        raised = false;
      }
    else if value == true then
      {
        color = null;
        relative = 0;
        raised = true;
      }
    else if builtins.isAttrs value && value ? relative then
      {
        color = null;
        relative = value.relative;
        raised = false;
      }
    else
      {
        color = value;
        relative = 0;
        raised = false;
      };
  headerBg = splitHeaderBg (headerIn.background or true);
  header = {
    titleStyle = titleIn.style;
    tagline = taglineIn.text;
    subtitle = taglineIn.subtitle;
    taglineLayout = taglineIn.layout;
    taglineAlign = taglineIn.align;
    statusHintLayout = statusHintIn.layout;
    status = plib.normalizeHeaderStatus (headerIn.status or { });
    background = headerBg.color;
    backgroundRelative = headerBg.relative;
    backgroundRaised = headerBg.raised;
  };
  gettingStarted = d.motd.gettingStarted // (m.gettingStarted or { });
  commandCatalog = plib.flatCommands (
    plib.normalizeCommandGroups (config.commandCatalog or d.commands)
  );
  commands = plib.selectCommands (m.commands or [ ]) commandCatalog;
  recipes = plib.normalizeRecipes (m.recipes or { });

  # Split a bg option into concrete color (or null), relative shade, or blend.
  # Runtime values resolve against the terminal (card/window) or the resolved
  # card (description).
  splitBg =
    value:
    if value == null || value == false then
      {
        color = null;
        relative = 0;
        blend = 0;
        blendSet = false;
      }
    else if value == true then
      {
        color = pal.bg;
        relative = 0;
        blend = 0;
        blendSet = false;
      }
    else if builtins.isAttrs value && value ? relative then
      {
        color = null;
        relative = value.relative;
        blend = 0;
        blendSet = false;
      }
    else if builtins.isAttrs value && value ? blend then
      {
        color = null;
        relative = 0;
        blend = value.blend;
        blendSet = true;
      }
    else
      {
        color = value;
        relative = 0;
        blend = 0;
        blendSet = false;
      };

  windowBg = splitBg m.windowBackground;
  cardBg = splitBg m.background;
  # An unset card inherits every window background mode, including runtime
  # relative/blend resolution, so the card and full-width container stay solid.
  cardInheritsWindow = m.background == null || m.background == false;
  background = if cardInheritsWindow then windowBg.color else cardBg.color;
  backgroundRelative = if cardInheritsWindow then windowBg.relative else cardBg.relative;
  backgroundBlend = if cardInheritsWindow then windowBg.blend else cardBg.blend;
  backgroundBlendSet = if cardInheritsWindow then windowBg.blendSet else cardBg.blendSet;
  windowBackground = windowBg.color;
  windowBackgroundRelative = windowBg.relative;
  windowBackgroundBlend = windowBg.blend;
  windowBackgroundBlendSet = windowBg.blendSet;

  margin = plib.resolveSpacing (d.motd.margin // (m.margin or { }));
  padding = plib.resolveSpacing (d.motd.padding // (m.padding or { }));

  descriptionIn = plib.textDefaults // d.motd.description // (m.description or { });
  descriptionBg = splitBg (descriptionIn.background or null);
  descriptionDefaults = plib.withRole pal "fg" descriptionIn;
  description = descriptionDefaults // {
    # Concrete description bg, else inherit concrete card, else leave empty for
    # runtime relative resolution against the card/terminal.
    background =
      if descriptionBg.color != null then
        descriptionBg.color
      else if descriptionBg.relative != 0 then
        null
      else
        background;
    backgroundRelative = descriptionBg.relative;
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
        title
        titleAlign
        colorProfile
        margin
        env
        commands
        recipes
        shortcuts
        ;

      palette = pal;
      background = jsonColor background;
      inherit backgroundRelative backgroundBlend backgroundBlendSet;
      windowBackground = jsonColor windowBackground;
      inherit windowBackgroundRelative windowBackgroundBlend windowBackgroundBlendSet;
      clearScreen = m.clearScreen;
      align = m.align;
      inherit padding;
      header = {
        inherit (header)
          titleStyle
          tagline
          subtitle
          taglineLayout
          taglineAlign
          statusHintLayout
          status
          backgroundRelative
          backgroundRaised
          ;
        background = jsonColor header.background;
      };
      description = {
        inherit (description)
          text
          bold
          italic
          faint
          backgroundRelative
          ;
        tips = description.tips or [ ];
        foreground = jsonColor description.foreground;
        background = jsonColor description.background;
      };
      gettingStarted = {
        inherit (gettingStarted) heading commandsLabel examplesLabel;
      };
      width =
        if m.fullscreen or false then
          0
        else if m.width == "full" then
          0
        else
          m.width;
      maxWidth =
        if m.fullscreen or false then
          0
        else if m.maxWidth == null then
          0
        else
          m.maxWidth;
    }
  );
in
assert lib.assertOneOf "motd align" m.align [
  "left"
  "center"
  "right"
];
assert lib.assertOneOf "motd title.align" titleAlign [
  "left"
  "center"
  "right"
];
assert lib.assertOneOf "motd colorProfile" colorProfile [
  "auto"
  "truecolor"
  "ansi256"
];
assert lib.assertOneOf "motd title.style" header.titleStyle [
  "plain"
  "spine"
  "bracketed"
  "label"
  "inline"
  "inverted"
];
assert lib.assertOneOf "motd header.tagline.layout" header.taglineLayout [
  "stack"
  "inline"
];
assert lib.assertOneOf "motd header.tagline.align" header.taglineAlign [
  "left"
  "center"
];
assert lib.assertOneOf "motd header.statusHint.layout" header.statusHintLayout [
  "below"
  "inline"
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
  subPackages = [ "cmd/motd" ];
  # Banner layout is still in flux — don't block package builds on render tests.
  doCheck = false;
  vendorHash = "sha256-hKvYlJqQUQ3NrBRgWPZyvYhsCvceW1HbDRlzltKyCxQ=";
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
