# MOTD generator: a static devshell welcome banner, ported from the
# cli-menu-design demo's devshell view. (Subsumes the former card
# component — status chips, styled description, configurable border.)
#
# Layout:
#   direnv-style load line (dim, themed ✓)
#   banner box    — accent border: "▓▒░ <project> · development shell"
#                   + tagline + right-aligned status chips
#   description   — styled text beneath the banner
#   env row       — label/value chips + git segment (branch ↑ahead ●dirty)
#   divider       — dashed
#   commands      — exact runnable next steps + descriptions
#   recipes       — titled multi-step workflows with comments and commands
#   footer bar    — inverted: "devshell  <project>    <hint>"
#
# Static by design: the interactive picker is the menu component.
{
  lib,
  writeShellApplication,
  gum,
  ncurses,
  ...
}:

# Component config: { theme?, palette?, project?, banner?, loadLine?,
#                     description?, env?, commands?, recipes?, git?, footer?,
#                     footerHint?, background?, clearScreen?, margin?, align?,
#                     width?, maxWidth? }
#
# env items: { label; value; } for static chips or { label; probe; } to run
# a command at render time (first output line; skipped on failure).
config:

let
  d = import ./defaults.nix;
  plib = import ./lib.nix { inherit lib; };
  inherit (plib)
    q
    textDefaults
    withRole
    statusColor
    styleFlags
    ;

  pal = plib.resolvePalette (config.theme or d.theme) (config.palette or d.palette);

  colorProfile = config.colorProfile or d.colorProfile;

  project = config.project or d.project;
  m = d.motd // config;
  banner = d.motd.banner // (m.banner or { });
  commands = plib.normalizeCommands (m.commands or { });
  recipes = plib.normalizeRecipes (m.recipes or { });

  resolveBg =
    v:
    if v == null || v == false then
      null
    else if v == true then
      pal.bg
    else
      v;

  # Window background: paints the full terminal width — margins, alignment
  # gutters, and line remainders. null/false = transparent, true = theme
  # `bg` token, or an explicit color.
  windowBg = resolveBg m.windowBackground;

  # Block background: null/false = transparent, true = theme `bg` token,
  # or an explicit color. Falls back to the window background so a single
  # windowBackground setting paints everything uniformly.
  bg =
    let
      v = resolveBg m.background;
    in
    if v != null then v else windowBg;

  # Every fragment must carry the background itself (lipgloss does not
  # re-apply a parent background after a child style's reset).
  bgFlag = lib.optionalString (bg != null) " --background ${q (toString bg)}";

  # Pad a captured line to full block width, painting the remainder.
  fillLine =
    el:
    if bg == null then
      el
    else
      lib.concatStringsSep "\n" [
        "line=$(${el})"
        "gum style --width \"$card_w\"${bgFlag} \"$line\""
      ];

  blankLine = if bg == null then "echo \"\"" else "gum style --width \"$card_w\"${bgFlag} ''";

  clearScreen = m.clearScreen;
  # Explicit sides (top/bottom/left/right) supersede the x/y axes.
  margin = plib.resolveSpacing (d.motd.margin // (m.margin or { }));
  align = m.align;
  loadLine = m.loadLine;
  badge = banner.badge;
  label = banner.label;
  tagline = banner.tagline;
  # One ordered list; each item is a static chip ({ label; value; }) or a
  # runtime probe ({ label; probe; }) — exactly one of the two.
  env = map (
    e:
    let
      v = e.value or null;
      p = e.probe or null;
    in
    assert lib.assertMsg (
      (v == null) != (p == null)
    ) "motd: env item \"${e.label or "?"}\" must set exactly one of `value` or `probe`";
    {
      label = e.label;
      value = v;
      probe = p;
    }
  ) m.env;
  gitSegment = m.git;
  footer = m.footer;
  footerHint = m.footerHint;
  width = m.width;
  maxWidth = m.maxWidth;

  border =
    let
      b = d.motd.banner.border // (banner.border or { });
    in
    b // { foreground = if b.foreground != null then b.foreground else pal.accentBorder; };

  statusItems = map (i: {
    text = i.text;
    status = i.status or "success";
  }) (banner.statusItems or [ ]);

  # Styled text item beneath the banner (theme fg role); the block
  # background fills in unless an explicit background is set.
  description =
    let
      t = withRole pal "fg" (textDefaults // d.motd.description // (m.description or { }));
    in
    t // { background = if t.background != null then t.background else bg; };

  widthSetup = plib.mkWidthSetup {
    isFull = width == "full";
    fixedWidth = if width == "full" then 0 else width;
    inherit maxWidth;
    sideCols = 0;
    padCols = 0;
    needsInner = false;
  };

  # --- load line -----------------------------------------------------------------

  loadLineEl = fillLine (
    "gum join --horizontal --align top"
    + " \"$(gum style --foreground '${pal.dim}'${bgFlag} ${q loadLine})\""
    + " \"$(gum style --foreground '${pal.accent}'${bgFlag} ' ✓')\""
  );

  # --- banner box ------------------------------------------------------------

  # Literal spaces between the head segments are folded into the styled
  # args so the background stays contiguous.
  bannerHead =
    "$(gum style --foreground '${pal.accent}'${bgFlag} ${q (badge + " ")})"
    + "$(gum style --bold --foreground '${pal.accent}'${bgFlag} ${q project})"
    + "$(gum style --foreground '${pal.muted}'${bgFlag} ${q (" · " + label)})";

  # Status chips: right-aligned inside the banner, first line (hugging the
  # box like the former card). Label spaces are folded into styled args.
  statusChip =
    i:
    "$(gum style --foreground '${pal.dim}'${bgFlag} ${q (i.text + " ")})$(gum style --foreground ${q (statusColor pal i.status)}${bgFlag} '●')";
  statusRow = lib.concatStringsSep (if bg != null then "$(gum style${bgFlag} '  ')" else "  ") (
    map statusChip statusItems
  );

  # gum has no true border width; 0 disables, >= 2 maps to "thick".
  borderStyle =
    if border.width == 0 then
      "none"
    else if border.width >= 2 then
      "thick"
    else if border.rounded then
      "rounded"
    else
      "normal";

  bannerContent =
    lib.optionalString (
      statusItems != [ ]
    ) "$(gum style --align right --width $(( card_w - 4 ))${bgFlag} \"${statusRow}\")\"$'\\n'\""
    + "$banner_head\"$'\\n'\"$banner_body";

  bannerLines = [
    "banner_head=\"${bannerHead}\""
    "banner_body=$(gum style --foreground '${pal.muted}'${bgFlag} --width $(( card_w - 4 )) ${q tagline})"
    # The demo draws the banner border in term-green at 40% alpha; the
    # palette's accentBorder token carries that composited color (border
    # foreground defaults to it).
    (
      "banner=$(gum style --border '${borderStyle}' --border-foreground ${q (toString border.foreground)}"
      + lib.optionalString (
        bg != null
      ) " --background ${q (toString bg)} --border-background ${q (toString bg)}"
      + " --width $(( card_w - 2 )) --padding '0 1' \"${bannerContent}\")"
    )
    "printf '%s\\n' \"$banner\""
  ];

  # --- description -----------------------------------------------------------

  # Word-wrapped in the same gum call that applies its styling (lipgloss
  # hard-wraps mid-word once background codes are embedded in a line).
  descriptionLines = lib.optionals (description.text != "") [
    "gum style${styleFlags description} --width \"$card_w\" ${q description.text}"
    blankLine
  ];

  # --- env row -----------------------------------------------------------------

  # Chips append to env_row in declaration order, so static values and
  # probes interleave as written. Chip-internal and inter-chip spaces are
  # folded into styled args so the background stays contiguous.
  labelEl = e: "$(gum style --foreground '${pal.dim}'${bgFlag} ${q (e.label + " ")})";

  envItemLines = lib.imap0 (
    i: e:
    if e.probe == null then
      "env_row=\"$env_row${labelEl e}$(gum style --foreground '${pal.fg}'${bgFlag} ${q (e.value + "   ")})\""
    else
      lib.concatStringsSep "\n" [
        "probe_${toString i}=$({ ${e.probe} ; } 2>/dev/null | head -n1 || true)"
        (
          "if [ -n \"$probe_${toString i}\" ]; then"
          + " env_row=\"$env_row${labelEl e}$(gum style --foreground '${pal.fg}'${bgFlag} \"$probe_${toString i}   \")\";"
          + " fi"
        )
      ]
  ) env;

  gitLines = lib.optionals gitSegment [
    ''
      if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
        git_branch=$(git symbolic-ref --short HEAD 2>/dev/null || git rev-parse --short HEAD 2>/dev/null || echo "?")
        git_dirty=$(git status --porcelain 2>/dev/null | wc -l | tr -d '[:space:]')
        git_ahead=$(git rev-list --count '@{upstream}..HEAD' 2>/dev/null || echo 0)
        env_row="$env_row$(gum style --foreground '${pal.dim}'${bgFlag} 'git ')$(gum style --foreground '${pal.accent2}'${bgFlag} "$git_branch")$(gum style --foreground '${pal.muted}'${bgFlag} " ↑$git_ahead ●$git_dirty")"
      fi''
  ];

  hasEnvRow = env != [ ] || gitSegment;

  envRowLines = lib.optionals hasEnvRow (
    [ "env_row=\"\"" ]
    ++ envItemLines
    ++ gitLines
    ++ [
      (
        "if [ -n \"${"$"}{env_row// /}\" ]; then "
        + (
          if bg == null then
            "printf '%s\\n\\n' \"$env_row\""
          else
            "gum style --width \"$card_w\"${bgFlag} \"$env_row\"; ${blankLine}"
        )
        + "; fi"
      )
    ]
  );

  # --- divider -------------------------------------------------------------------

  dividerLines = [
    "divider=$(printf '%*s' \"$card_w\" '' | tr ' ' '-')"
    # `--` keeps gum from parsing the all-dashes divider as a flag.
    "gum style --foreground '${pal.border}'${bgFlag} -- \"$divider\""
    blankLine
  ];

  # --- commands ----------------------------------------------------------------

  commandLabel = fillLine (
    "gum style --foreground '${pal.dim}'${bgFlag} --width \"$card_w\" -- " + q "next steps"
  );

  maxCommandW = lib.min 40 (2 + lib.foldl' lib.max 4 (map (c: lib.stringLength c.command) commands));

  commandRow =
    command:
    let
      wideRow = fillLine (
        "gum join --horizontal --align top --"
        + " \"$(gum join --horizontal --align top --"
        + " \"$(gum style --foreground '${pal.accent}'${bgFlag} '$ ')\""
        + " \"$(gum style --foreground '${pal.fg}'${bgFlag} --width \"$(( command_w - 2 ))\" -- ${q command.command})\")\""
        + " \"$(gum style${bgFlag} '  ')\""
        + " \"$(gum style --foreground '${pal.muted}'${bgFlag} --width \"$command_desc_w\" -- ${q command.description})\""
      );
      stackedCommand = fillLine (
        "gum join --horizontal --align top --"
        + " \"$(gum style --foreground '${pal.accent}'${bgFlag} '$ ')\""
        + " \"$(gum style --foreground '${pal.fg}'${bgFlag} --width \"$(( card_w - 2 ))\" -- ${q command.command})\""
      );
      stackedDescription = fillLine (
        "gum join --horizontal --align top --"
        + " \"$(gum style${bgFlag} '  ')\""
        + " \"$(gum style --foreground '${pal.muted}'${bgFlag} --width \"$(( card_w - 2 ))\" -- ${q command.description})\""
      );
    in
    lib.concatStringsSep "\n" (
      [
        "if [ \"$command_desc_w\" -ge 10 ]; then"
        wideRow
        "else"
        stackedCommand
      ]
      ++ lib.optional (command.description != "") stackedDescription
      ++ [ "fi" ]
    );

  commandLines = lib.optionals (commands != [ ]) (
    [
      commandLabel
      blankLine
      "command_w='${toString maxCommandW}'"
      "if [ \"$command_w\" -gt $(( card_w - 2 )) ]; then command_w=$(( card_w - 2 )); fi"
      "command_desc_w=$(( card_w - command_w - 2 ))"
    ]
    ++ map commandRow commands
  );

  # --- recipes -----------------------------------------------------------------

  recipeLabel = fillLine (
    "gum style --foreground '${pal.dim}'${bgFlag} --width \"$card_w\" -- "
    + q "recipes — common flows that take a few steps"
  );

  recipeTitle =
    recipe:
    fillLine (
      "gum join --horizontal --align top --"
      + " \"$(gum style --foreground '${pal.dim}'${bgFlag} '# ')\""
      + " \"$(gum style --foreground '${pal.muted}'${bgFlag} --width $(( card_w > 2 ? card_w - 2 : 1 )) -- ${q recipe.title})\""
    );

  recipeBody =
    recipe:
    (lib.foldl'
      (
        state: line:
        if line == "" then
          # Preserve even a trailing visual blank: command substitution strips
          # newlines, so keep one space on otherwise empty rows.
          state // { output = state.output ++ [ "printf ' \\n'" ]; }
        else if lib.hasPrefix "#" line then
          state
          // {
            output = state.output ++ [
              "gum style --foreground '${pal.dim}'${bgFlag} --width $(( card_w > 4 ? card_w - 4 : 1 )) -- ${q ("  " + line)}"
            ];
          }
        else
          let
            number = state.number + 1;
          in
          {
            inherit number;
            output = state.output ++ [
              (
                "gum join --horizontal --align top --"
                + " \"$(gum style --foreground '${pal.dim}'${bgFlag} --align right --width '3' '${toString number}')\""
                + " \"$(gum style --foreground '${pal.accent}'${bgFlag} '  $ ')\""
                + " \"$(gum style --foreground '${pal.fg}'${bgFlag} --width $(( card_w > 11 ? card_w - 11 : 1 )) -- ${q line})\""
              )
            ];
          }
      )
      {
        number = 0;
        output = [ ];
      }
      recipe.lines
    ).output;

  recipeBlock = recipe: [
    (recipeTitle recipe)
    ''
      recipe_body=$(
      ${lib.concatStringsSep "\n" (recipeBody recipe)}
      printf '\034'
      )''
    "recipe_body=${"$"}{recipe_body%$'\\034'}"
    (
      "gum style --border 'rounded' --border-foreground '${pal.border}'"
      + lib.optionalString (
        bg != null
      ) " --background ${q (toString bg)} --border-background ${q (toString bg)}"
      + " --width $(( card_w - 2 )) --padding '0 1' -- \"$recipe_body\""
    )
  ];

  recipeLines = lib.optionals (recipes != [ ]) (
    [
      recipeLabel
      blankLine
    ]
    ++ lib.concatLists (lib.intersperse [ blankLine ] (map recipeBlock recipes))
  );

  # --- footer ------------------------------------------------------------------

  footerLines = lib.optionals footer [
    ("footer_left=" + q ("devshell  " + project))
    ("footer_right=" + q footerHint)
    "footer_gap=$(( card_w - 2 - ${"$"}{#footer_left} - ${"$"}{#footer_right} ))"
    "if [ \"$footer_gap\" -lt 1 ]; then footer_gap=1; fi"
    "footer_pad=$(printf '%*s' \"$footer_gap\" '')"
    "gum style --background '${pal.fg}' --foreground '${pal.bg}' --width \"$card_w\" --padding '0 1' \"$footer_left$footer_pad$footer_right\""
  ];

  # The block renders into a variable so `align` can place it against the
  # terminal window as a whole (content inside stays left-aligned).
  renderLines =
    lib.optionals (loadLine != "") [
      loadLineEl
      blankLine
    ]
    ++ bannerLines
    ++ [ blankLine ]
    ++ descriptionLines
    ++ envRowLines
    # The divider separates the header area from authored guidance.
    ++ lib.optionals (commands != [ ] || recipes != [ ]) dividerLines
    ++ commandLines
    ++ lib.optionals (commands != [ ] && recipes != [ ]) ([ blankLine ] ++ dividerLines)
    ++ recipeLines
    ++ lib.optionals footer [ blankLine ]
    ++ footerLines;

  # Manual placement: shift every line by the same offset so the block
  # moves as one unit. A nested gum wrap also works — pad lines to a
  # uniform width with `gum style --width`, then wrap in `gum style
  # --align` (plain --align centers each line individually, so the pad
  # stage is required) — but it right-pads every line to the terminal
  # width with trailing spaces, and asymmetric margin.left/right under
  # center alignment needs offset arithmetic anyway, so the explicit
  # offset loop stays simpler. margin.left/right shift the offset;
  # margin.top/bottom print blank lines around the block.
  offsetExpr =
    if align == "right" then
      "term_w - card_w - ${toString margin.right}"
    else
      "(term_w - card_w) / 2 + ${toString margin.left} - ${toString margin.right}";

  windowBgFlag = lib.optionalString (windowBg != null) " --background ${q (toString windowBg)}";

  # With a window background each shifted line is re-wrapped at the full
  # terminal width: the leading offset spaces are painted by the wrap
  # (they precede any reset) and lipgloss paints the trailing padding it
  # adds itself.
  padLoop =
    if windowBg == null then
      [
        "pad=$(printf '%*s' \"$offset\" '')"
        "while IFS= read -r line; do printf '%s%s\\n' \"$pad\" \"$line\"; done <<< \"$body\""
      ]
    else
      [
        "pad=$(printf '%*s' \"$offset\" '')"
        "while IFS= read -r line; do gum style --width \"$term_w\"${windowBgFlag} \"$pad$line\"; done <<< \"$body\""
      ];

  # Margin blank lines: painted full-width bars under a window background.
  blankLines =
    n:
    lib.optional (n > 0) (
      if windowBg == null then
        "printf '%s' ${q (lib.strings.replicate n "\n")}"
      else
        "gum style --width \"$term_w\" --height '${toString n}'${windowBgFlag} ''"
    );

  needsTermW = windowBg != null || align != "left";

  placeLines =
    if align == "left" && margin.left == 0 && windowBg == null then
      [ "printf '%s\\n' \"$body\"" ]
    else if align == "left" then
      [ "offset=${toString margin.left}" ] ++ padLoop
    else
      [
        "offset=$(( ${offsetExpr} ))"
        "if [ \"$offset\" -lt 0 ]; then offset=0; fi"
      ]
      ++ padLoop;

  scriptText = lib.concatStringsSep "\n" (
    plib.colorProfileSetup colorProfile
    ++ lib.optional clearScreen "clear || true"
    ++ [ widthSetup ]
    # term_w is set before the body capture so the top-margin bars can use
    # it (widthSetup may already define it for width = "full"; the
    # reassignment is harmless).
    ++ lib.optional needsTermW "term_w=$(tput cols 2>/dev/null || echo 80)"
    ++ [ "body=$(" ]
    ++ renderLines
    ++ [ ")" ]
    ++ blankLines margin.top
    ++ placeLines
    ++ blankLines margin.bottom
  );
in
assert lib.assertOneOf "motd align" align [
  "left"
  "center"
  "right"
];
assert lib.assertMsg (
  width == "full" || builtins.isInt width
) "motd: width must be an integer or \"full\"";
assert lib.assertMsg (
  maxWidth == null || builtins.isInt maxWidth
) "motd: maxWidth must be an integer or null";
writeShellApplication {
  name = "motd";

  runtimeInputs = [
    gum
    ncurses
  ];

  text = scriptText;

  meta = {
    description = "Devshell MOTD banner (themed, generated by Nix)";
    mainProgram = "motd";
  };
}
