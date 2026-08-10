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
# Component config: public MOTD options plus shared theme fields and command
# data. `shortcuts` is synthesized and injected by the flake-parts module.
config: let
  d = import ./defaults.nix;
  plib = import ./lib.nix {inherit lib;};

  pal = plib.resolvePalette (config.theme or d.theme) (config.palette or d.palette);
  colorProfile = config.colorProfile or d.colorProfile;
  project = config.project or d.project;
  m = d.motd // config;
  titleIn = d.motd.title // (m.title or {});
  title =
    if titleIn.text == null
    then ""
    else builtins.readFile titleIn.text;
  titleAlign = titleIn.align;
  headerIn = d.motd.header // (m.header or {});
  taglineIn = d.motd.header.tagline // (headerIn.tagline or {});
  statusHintIn = d.motd.header.statusHint // (headerIn.statusHint or {});
  # Header bg: true → raised auto bar; null/false → transparent; color/relative → solid.
  splitHeaderBg = value:
    if value == null || value == false
    then {
      color = null;
      relative = 0;
      raised = false;
    }
    else if value == true
    then {
      color = null;
      relative = 0;
      raised = true;
    }
    else if builtins.isAttrs value && value ? relative
    then {
      color = null;
      relative = value.relative;
      raised = false;
    }
    else {
      color = value;
      relative = 0;
      raised = false;
    };
  headerBg = splitHeaderBg (headerIn.background or true);
  header = {
    titleStyle = titleIn.style;
    tagline = taglineIn.text;
    subtitle = taglineIn.subtitle or null;
    taglineLayout = taglineIn.layout;
    taglineAlign = taglineIn.align;
    statusHintLayout = statusHintIn.layout;
    statusHintLinks = statusHintIn.links or [];
    status = plib.normalizeHeaderStatus (headerIn.status or {});
    background = headerBg.color;
    backgroundRelative = headerBg.relative;
    backgroundRaised = headerBg.raised;
  };
  gettingStarted = d.motd.gettingStarted // (m.gettingStarted or {});
  # Catalogue domain → flat entries → reduced MOTD rows. The Go renderer only
  # sees { command, description }; richer metadata stays on the Nix side.
  commands = plib.projectMotdRows (config.commandGroupOrder or []) (
    config.commandCatalog or d.commands
  );
  recipes = plib.normalizeRecipes (m.recipes or {});

  # Split a card/section bg option into concrete color (or null), relative
  # shade, or blend. Runtime values resolve against the terminal or card.
  splitBg = value:
    if value == null || value == false
    then {
      color = null;
      relative = 0;
      blend = 0;
      blendSet = false;
    }
    else if value == true
    then {
      color = pal.bg;
      relative = 0;
      blend = 0;
      blendSet = false;
    }
    else if builtins.isAttrs value && value ? relative
    then {
      color = null;
      relative = value.relative;
      blend = 0;
      blendSet = false;
    }
    else if builtins.isAttrs value && value ? blend
    then {
      color = null;
      relative = 0;
      blend = value.blend;
      blendSet = true;
    }
    else {
      color = value;
      relative = 0;
      blend = 0;
      blendSet = false;
    };

  cardBg = splitBg m.background;
  background = cardBg.color;
  backgroundRelative = cardBg.relative;
  backgroundBlend = cardBg.blend;
  backgroundBlendSet = cardBg.blendSet;

  margin = plib.resolveSpacing (d.motd.margin // (m.margin or {}));
  padding = plib.resolveSpacing (d.motd.padding // (m.padding or {}));

  descriptionIn = plib.textDefaults // d.motd.description // (m.description or {});
  descriptionBg = splitBg (descriptionIn.background or null);
  descriptionDefaults = plib.withRole pal "fg" descriptionIn;
  description =
    descriptionDefaults
    // {
      # Concrete description bg, else inherit concrete card, else leave empty for
      # runtime relative resolution against the card/terminal.
      background =
        if descriptionBg.color != null
        then descriptionBg.color
        else if descriptionBg.relative != 0
        then null
        else background;
      backgroundRelative = descriptionBg.relative;
    };

  # A probe remains a shell snippet by contract, but only the Go runtime
  # executes it. Empty strings encode the inactive side of the value/probe sum
  # so the JSON boundary contains no nullable scalar fields.
  env =
    map (
      item: let
        value = item.value or null;
        probe = item.probe or null;
      in
        assert lib.assertMsg (
          (value == null) != (probe == null)
        ) "motd: env item \"${item.label or "?"}\" must set exactly one of `value` or `probe`"; {
          label = item.label;
          value =
            if value == null
            then ""
            else value;
          probe =
            if probe == null
            then ""
            else probe;
        }
    )
    m.env;

  shortcuts = map (s: {
    command = s.command;
    alias = s.alias or "";
  }) (m.shortcuts or []);

  jsonColor = value:
    if value == null
    then ""
    else toString value;

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
      border = m.border;
      clearScreen = m.clearScreen;
      align = m.align;
      verticalAlign = m.verticalAlign;
      inherit padding;
      header = {
        inherit
          (header)
          titleStyle
          tagline
          subtitle
          taglineLayout
          taglineAlign
          statusHintLayout
          statusHintLinks
          status
          backgroundRelative
          backgroundRaised
          ;
        background = jsonColor header.background;
      };
      description = {
        inherit
          (description)
          text
          bold
          italic
          faint
          backgroundRelative
          ;
        tips = description.tips or [];
        foreground = jsonColor description.foreground;
        background = jsonColor description.background;
      };
      links = m.links;
      inherit gettingStarted;
      width =
        if m.fullscreen or false
        then 0
        else if m.width == "full"
        then 0
        else m.width;
      maxWidth =
        if m.fullscreen or false
        then 0
        else if m.maxWidth == null
        then 0
        else m.maxWidth;
    }
  );
in
  assert lib.assertOneOf "motd align" m.align [
    "left"
    "center"
    "right"
  ];
  assert lib.assertOneOf "motd verticalAlign" m.verticalAlign [
    "top"
    "center"
    "bottom"
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
      subPackages = ["cmd/motd"];
      # Banner layout is still in flux — don't block package builds on render tests.
      doCheck = false;
      vendorHash = "sha256-BHrU5pKVDuGDq0ZHbHKcUBa5olzHzfgoJXzv2IGXY4U=";
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
