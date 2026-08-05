# Shared helpers for the prelude generators (motd.nix, menu.nix).
{ lib }:
let

  themes = import ./themes.nix;
  themeNames = lib.attrNames themes;

  # Resolve a theme name + per-token overrides into a concrete palette.
  # Overrides with null values fall through to the theme.
  resolvePalette =
    theme: overrides:
    assert lib.assertMsg (
      themes ? ${theme}
    ) "prelude: unknown theme \"${theme}\" (expected one of: ${lib.concatStringsSep ", " themeNames})";
    themes.${theme} // lib.filterAttrs (_n: v: v != null) overrides;

  # Palette colors may be hex or the numeric forms Lip Gloss accepts. Convert
  # them before derivation so Starship and ble.sh get one stable truecolor value.
  colorToRGB =
    value:
    let
      raw =
        if builtins.isInt value || builtins.isString value then
          toString value
        else
          throw "prelude: color must be a hex string or unsigned integer";
      decimal =
        if builtins.isInt value then
          value
        else if builtins.match "^[0-9]+$" raw != null then
          lib.toInt raw
        else
          null;
      ansiBase = [
        {
          r = 0;
          g = 0;
          b = 0;
        }
        {
          r = 128;
          g = 0;
          b = 0;
        }
        {
          r = 0;
          g = 128;
          b = 0;
        }
        {
          r = 128;
          g = 128;
          b = 0;
        }
        {
          r = 0;
          g = 0;
          b = 128;
        }
        {
          r = 128;
          g = 0;
          b = 128;
        }
        {
          r = 0;
          g = 128;
          b = 128;
        }
        {
          r = 192;
          g = 192;
          b = 192;
        }
        {
          r = 128;
          g = 128;
          b = 128;
        }
        {
          r = 255;
          g = 0;
          b = 0;
        }
        {
          r = 0;
          g = 255;
          b = 0;
        }
        {
          r = 255;
          g = 255;
          b = 0;
        }
        {
          r = 0;
          g = 0;
          b = 255;
        }
        {
          r = 255;
          g = 0;
          b = 255;
        }
        {
          r = 0;
          g = 255;
          b = 255;
        }
        {
          r = 255;
          g = 255;
          b = 255;
        }
      ];
      xtermLevels = [
        0
        95
        135
        175
        215
        255
      ];
      modulo = number: divisor: number - divisor * builtins.div number divisor;
      ansiToRGB =
        index:
        if index < 16 then
          builtins.elemAt ansiBase index
        else if index < 232 then
          let
            offset = index - 16;
            r = builtins.div offset 36;
            g = builtins.div (modulo offset 36) 6;
            b = modulo offset 6;
          in
          {
            r = builtins.elemAt xtermLevels r;
            g = builtins.elemAt xtermLevels g;
            b = builtins.elemAt xtermLevels b;
          }
        else if index < 256 then
          let
            gray = 8 + 10 * (index - 232);
          in
          {
            r = gray;
            g = gray;
            b = gray;
          }
        else
          {
            r = modulo (builtins.div index 65536) 256;
            g = modulo (builtins.div index 256) 256;
            b = modulo index 256;
          };
      hexToRGB =
        hex:
        let
          expanded =
            if builtins.stringLength hex == 4 then
              "#${builtins.substring 1 1 hex}${builtins.substring 1 1 hex}${builtins.substring 2 1 hex}${builtins.substring 2 1 hex}${builtins.substring 3 1 hex}${builtins.substring 3 1 hex}"
            else
              hex;
        in
        {
          r = lib.fromHexString (builtins.substring 1 2 expanded);
          g = lib.fromHexString (builtins.substring 3 2 expanded);
          b = lib.fromHexString (builtins.substring 5 2 expanded);
        };
    in
    if decimal != null then
      assert lib.assertMsg (decimal >= 0) "prelude: color index must be unsigned";
      ansiToRGB decimal
    else if
      builtins.match "^#[0-9A-Fa-f]{3}$" raw != null || builtins.match "^#[0-9A-Fa-f]{6}$" raw != null
    then
      hexToRGB raw
    else
      throw "prelude: unsupported color \"${raw}\"";

  rgbToHex =
    rgb:
    let
      channel = value: lib.fixedWidthString 2 "0" (lib.toLower (lib.toHexString value));
    in
    "#${channel rgb.r}${channel rgb.g}${channel rgb.b}";

  # Match Lip Gloss Darken exactly: multiply each byte by 0.95 and truncate.
  darkenColor =
    value:
    let
      rgb = colorToRGB value;
      darken = channel: builtins.floor (channel * 0.95);
    in
    rgbToHex {
      r = darken rgb.r;
      g = darken rgb.g;
      b = darken rgb.b;
    };

  # Only an enabled MOTD can paint a Prelude-owned terminal backdrop. Relative
  # and blend modes resolve after a terminal query, so they intentionally carry
  # no static base and fall back to the resolved palette background.
  resolveWindowBackgroundContext =
    motdEnable: windowBackground:
    let
      set = motdEnable && windowBackground != null && windowBackground != false;
      literal = set && windowBackground != true && !builtins.isAttrs windowBackground;
    in
    {
      inherit set;
      base = if literal then windowBackground else null;
    };

  # Keep derived prompt/shell colors beside the Go-safe palette rather than
  # merging them into it: Go decoders reject unknown palette JSON fields.
  resolveBackdropPalette =
    theme: overrides: windowBackgroundContext:
    let
      palette = resolvePalette theme overrides;
      context = if windowBackgroundContext == null then { } else windowBackgroundContext;
      windowBackgroundSet = context.set or false;
      base = context.base or null;
    in
    {
      inherit palette windowBackgroundSet;
      shadow = darkenColor (if windowBackgroundSet && base != null then base else palette.bg);
    };

  # Resolve a spacing spec into explicit sides. `x`/`y` are axis shorthands
  # (left+right / top+bottom); explicit sides supersede them. Every field is
  # optional — missing sides fall through to the axis (or 0).
  resolveSpacing =
    spec:
    let
      s = if spec == null then { } else spec;
    in
    {
      top = if (s.top or null) != null then s.top else (s.y or 0);
      bottom = if (s.bottom or null) != null then s.bottom else (s.y or 0);
      left = if (s.left or null) != null then s.left else (s.x or 0);
      right = if (s.right or null) != null then s.right else (s.x or 0);
      minHeight = s.minHeight or 0;
    };

  textDefaults = {
    text = "";
    foreground = null;
    background = null;
    bold = false;
    italic = false;
    faint = false;
  };

  # Explicit colors override semantic palette roles at the Nix boundary so
  # renderers receive one normalized color rather than duplicating precedence.
  withRole =
    pal: role: text:
    text // { foreground = if text.foreground != null then text.foreground else pal.${role}; };

  sortOrderedAttrs =
    attrs:
    lib.sort (
      a: b:
      let
        aOrder = a.value.order or 1000;
        bOrder = b.value.order or 1000;
      in
      if aOrder != bOrder then aOrder < bOrder else a.name < b.name
    ) (lib.mapAttrsToList lib.nameValuePair attrs);

  # --- command catalogue (domain + projections) ---------------------------------
  # Identity, normalization, grouping, selection, and surface projections live
  # in command-catalogue.nix. lib.nix re-exports them so existing call sites
  # (`plib.normalizeCommand*`, etc.) keep working.
  catalogue = import ./command-catalogue.nix { inherit lib; };
  inherit (catalogue)
    normalizeArg
    commandIdentity
    normalizeCommand
    normalizeCommandEntries
    normalizeCommandGroups
    flatCommands
    selectCommands
    projectMenuGroups
    projectMotdCatalog
    projectMotdRows
    ;

  # Header status badges: order then key. Keep static text and/or live checks.
  normalizeHeaderStatus =
    status:
    builtins.filter (item: item.label != "" || item.status != "" || item.check != "") (
      map (
        { value, ... }:
        {
          label = value.label or "";
          status = value.status or "";
          check = value.check or "";
          async = value.async or true;
          ok = value.ok or "ok";
          fail = value.fail or "fail";
          failLevel = value.failLevel or "error";
          output = value.output or "";
        }
      ) (sortOrderedAttrs status)
    );

  # Normalize a free-form recipe line into a step. Empty lines are dropped;
  # "#…" becomes a comment; everything else is a command.
  lineToStep =
    line:
    let
      trimmed = lib.removePrefix " " (lib.removeSuffix " " line);
    in
    if trimmed == "" then
      null
    else if lib.hasPrefix "#" trimmed then
      {
        command = "";
        comment =
          if lib.hasPrefix "# " trimmed then lib.removePrefix "# " trimmed else lib.removePrefix "#" trimmed;
      }
    else
      {
        command = trimmed;
        comment = "";
      };

  normalizeRecipeStep = step: {
    command = step.command or "";
    comment = step.comment or "";
  };

  normalizeRecipes =
    recipes:
    map (
      { name, value }:
      let
        title = value.title or null;
        explicitSteps = value.steps or [ ];
        legacyLines = value.lines or [ ];
        steps =
          if explicitSteps != [ ] then
            map normalizeRecipeStep explicitSteps
          else
            builtins.filter (s: s != null) (map lineToStep legacyLines);
      in
      {
        title = if title == null then name else title;
        inherit steps;
      }
    ) (sortOrderedAttrs recipes);

  # Navigation shortcuts are part of the component contract, not user data.
  # Deriving them from enablement keeps every rendered chip executable and
  # prevents configurations from hiding Prelude's built-in navigation.
  componentShortcuts =
    enabled:
    lib.optionals enabled.motd [
      {
        command = "motd";
        alias = "?";
      }
    ]
    ++ lib.optionals enabled.menu [
      {
        command = "menu";
        alias = "m";
      }
    ]
    ++ lib.optionals enabled.docs [
      {
        command = "docs";
        alias = "d";
      }
    ];

  # Split markdown into a docs page node at H2 boundaries (`## `).
  # Fence-aware: `##` inside ``` / ~~~ blocks is ignored.
  #
  # Always returns one pages entry with a stable shape:
  #   {
  #     title = "README";
  #     text = <path|toFile>;           # provenance for rootReadme match
  #     children = [
  #       { title = <H1|Overview>; text = <preamble toFile>; }  # first section
  #       { title = <H2>; text = <section toFile>; } …
  #     ];
  #   }
  #
  # docs.nix renames the first child to config.project and marks it rootReadme
  # when `text` matches prelude.docs.rootReadme (FIGlet hero). Groups do not
  # render a body — only the first child carries the README intro.
  mdSplit =
    src:
    let
      isAbsPathString = s: builtins.isString s && builtins.substring 0 1 s == "/";

      srcPath =
        if builtins.isPath src then
          src
        else if isAbsPathString src && builtins.pathExists src then
          src
        else
          null;

      markdown =
        if srcPath != null then
          builtins.readFile srcPath
        else if builtins.isString src then
          src
        else
          throw "prelude.lib.mdSplit: expected a path, path string, or markdown string";

      normalized =
        if markdown == "" then
          ""
        else if lib.hasSuffix "\n" markdown then
          markdown
        else
          markdown + "\n";

      lines = lib.splitString "\n" normalized;
      lineList =
        let
          n = builtins.length lines;
        in
        if n > 0 && builtins.elemAt lines (n - 1) == "" then lib.take (n - 1) lines else lines;

      isFence =
        line:
        let
          t = lib.trim line;
        in
        lib.hasPrefix "```" t || lib.hasPrefix "~~~" t;

      isH2 = line: builtins.match "## [^#].*" line != null || builtins.match "##" line != null;

      h2Title =
        line:
        let
          stripped =
            if lib.hasPrefix "## " line then
              builtins.substring 3 (builtins.stringLength line - 3) line
            else if lib.hasPrefix "##" line then
              builtins.substring 2 (builtins.stringLength line - 2) line
            else
              line;
        in
        lib.trim stripped;

      isH1 = line: builtins.match "# [^#].*" line != null || line == "#";

      h1Title =
        line:
        if lib.hasPrefix "# " line then
          lib.trim (builtins.substring 2 (builtins.stringLength line - 2) line)
        else
          "";

      # Fold: preamble (title=null) + each H2 section. Boundary headings omitted.
      step =
        acc: line:
        if isFence line then
          acc
          // {
            inFence = !(acc.inFence or false);
            lines = acc.lines ++ [ line ];
          }
        else if !(acc.inFence or false) && isH2 line then
          {
            inFence = false;
            title = h2Title line;
            lines = [ ];
            sections = acc.sections ++ [
              {
                title = acc.title;
                lines = acc.lines;
              }
            ];
          }
        else
          acc // { lines = acc.lines ++ [ line ]; };

      folded = lib.foldl' step {
        inFence = false;
        title = null;
        lines = [ ];
        sections = [ ];
      } lineList;

      rawSections = folded.sections ++ [
        {
          title = folded.title;
          lines = folded.lines;
        }
      ];

      dropLeadingBlanks =
        xs:
        if xs == [ ] then
          [ ]
        else if lib.trim (builtins.head xs) == "" then
          dropLeadingBlanks (builtins.tail xs)
        else
          xs;
      trimBlankEdges = ls: lib.reverseList (dropLeadingBlanks (lib.reverseList (dropLeadingBlanks ls)));

      stripFirstH1 =
        ls:
        let
          go =
            inFence: acc: rest:
            if rest == [ ] then
              acc
            else
              let
                line = builtins.head rest;
                tail = builtins.tail rest;
                nowIn = if isFence line then !inFence else inFence;
              in
              if (!inFence) && isH1 line then acc ++ tail else go nowIn (acc ++ [ line ]) tail;
        in
        go false [ ] ls;

      firstNonFencedH1 =
        ls:
        let
          go =
            inFence: rest:
            if rest == [ ] then
              null
            else
              let
                line = builtins.head rest;
                tail = builtins.tail rest;
                nowIn = if isFence line then !inFence else inFence;
              in
              if (!inFence) && isH1 line then line else go nowIn tail;
        in
        go false ls;

      prepared = map (
        s:
        let
          isPreamble = s.title == null;
          rawLines = if isPreamble then stripFirstH1 s.lines else s.lines;
          bodyLines = trimBlankEdges rawLines;
          title =
            if s.title != null then
              s.title
            else
              let
                h1line = firstNonFencedH1 s.lines;
              in
              if h1line == null then
                "Overview"
              else
                let
                  t = h1Title h1line;
                in
                if t != "" then t else "Overview";
        in
        {
          inherit isPreamble title;
          lines = bodyLines;
        }
      ) rawSections;

      # Always materialize index 0 as the preamble leaf (even if empty body),
      # so docs.nix can rename it to project + rootReadme without stealing an H2.
      preamble =
        let
          p = lib.findFirst (s: s.isPreamble) null prepared;
        in
        if p != null then
          p
        else
          {
            isPreamble = true;
            title = "Overview";
            lines = [ ];
          };

      # Drop empty H2 sections only — never drop the preamble slot.
      h2Children = builtins.filter (
        s: (!s.isPreamble) && lib.any (line: lib.trim line != "") s.lines
      ) prepared;

      slug =
        index: title:
        let
          base = if title == null || title == "" then "section" else title;
          safe = lib.strings.sanitizeDerivationName base;
        in
        "mdsplit-${toString index}-${safe}.md";

      toChild =
        index: s:
        let
          body = lib.concatStringsSep "\n" s.lines;
          fileBody =
            if body == "" then
              "\n"
            else if lib.hasSuffix "\n" body then
              body
            else
              body + "\n";
        in
        {
          title = s.title;
          text = builtins.toFile (slug index s.title) fileBody;
        };

      children = lib.imap0 toChild ([ preamble ] ++ h2Children);

      # Always set text so the return shape is stable. Path src keeps the real
      # file for rootReadme matching; string src is materialized via toFile.
      text = if srcPath != null then srcPath else builtins.toFile "mdsplit-source.md" normalized;
    in
    {
      title = "README";
      inherit text children;
    };

in
{
  inherit
    themes
    themeNames
    resolvePalette
    darkenColor
    resolveWindowBackgroundContext
    resolveBackdropPalette
    resolveSpacing
    textDefaults
    withRole
    sortOrderedAttrs
    normalizeArg
    commandIdentity
    normalizeCommand
    normalizeCommandEntries
    normalizeCommandGroups
    flatCommands
    selectCommands
    projectMenuGroups
    projectMotdCatalog
    projectMotdRows
    normalizeHeaderStatus
    normalizeRecipes
    componentShortcuts
    mdSplit
    ;
}
