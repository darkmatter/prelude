# Prompt package builder: theme palette → starship.toml.
#
# The package is a starship config file, not a program. Starship re-resolves
# $STARSHIP_CONFIG on every prompt render, and direnv propagates plain env
# vars (only PS1 itself is stripped) — so a devshell that exports
#
#   export STARSHIP_CONFIG=${config.packages.prompt}
#
# re-themes the user's existing starship prompt while inside the project and
# reverts automatically when direnv unloads. `packages.prelude` sources one
# canonical, idempotent `prelude-init` file in the interactive shell that
# `nix develop` already started; it does not launch or reconstruct another
# shell. Non-interactive direnv evaluation stays inert so the user's existing
# login-shell prompt remains in control.
{
  lib,
  formats,
  ...
}:

# Component config: shared theme fields plus settings/configFile options.
# `shortcuts` is synthesized and injected by the flake-parts module.
config:

let
  d = import ./defaults.nix;
  plib = import ./lib.nix { inherit lib; };

  m = d.prompt // config;
  project = config.project or d.project;
  backdrop =
    config.backdropPalette
      or (plib.resolveBackdropPalette (config.theme or d.theme) (config.palette or d.palette
      ) m.windowBackgroundContext);
  pal = backdrop.palette;

  mkKey = char: label: "\\[[${char}](bold fg:accent)\\][─](fg:surface)${label}";
  keymap = lib.concatStringsSep "[──](fg:surface)" (
    map (s: mkKey s.alias s.command) (config.shortcuts or [ ])
  );

  # Styles reference palette tokens by name (bg:surface, fg:accent2, …);
  # `palettes.prelude` maps them to the resolved theme hex values, mirroring
  # how a hand-written starship config names its palette.
  #
  # Layout (cross-shell via Starship, with fixed status chrome only in Bash):
  #
  #   Context: ░▒▓ project  path  branch  status  duration
  #   Prompt:  ~/project ❯
  #
  # Shortcut chips are rendered by the Bash status callback and never by
  # Starship's normal prompt or right-format output.
  #
  # Each Powerline separator inherits the background of the segment on its left
  # as its foreground and the next segment's background as its own background.
  # `\[`/`\]` are literal brackets in Starship format strings.
  # The Powerline context is a separate, left-aligned row. Shortcut chips
  # belong to Bash's fixed status row and must never leak into Starship output.
  leftSegments = lib.concatStrings [
    "[╭░▒▓](fg:accent)"
    "[ π ](bold bg:accent fg:bg)"
    # "[ ${project} ](bg:secondary bold fg:accent2)"
    "[](fg:accent bg:bg)"
    "( $directory)"
    "[](fg:bg bg:fg)"
    "$git_branch"
    "[](fg:fg bg:surface)"
    "$git_status"
    "$git_metrics"
    "[$fill](fg:surface)[${keymap}](fg:muted bg:window)[─╮](fg:surface bg:window)"
  ];

  defaultSettings = {
    # Preserve two breathing rows, then keep context above the editable input.
    # Starship paints this whole projection; the shell owns only the input buffer.
    format = "[${leftSegments}\n[╰─](fg:accent bg:window) ]${
      if backdrop.windowBackgroundSet then "(bg:window)" else "()"
    }";
    add_newline = true;

    right_format = lib.concatStrings [
      "[──╯](fg:surface bg:window)"
      "\n\n"
    ];

    fill.symbol = "─";
    fill.style = "fg:surface bg:window";

    character.format = "[$symbol](bg:window) ";

    palette = "prelude";
    palettes.prelude = pal // {
      inherit (backdrop) window shadow;
    };

    directory = {
      style = "bg:bg fg:fg";
      format = "[ $path ]($style)";
      truncation_length = 8;
      truncation_symbol = "";
      truncate_to_repo = true;
      substitutions = [
        { from = "^[^~/][^/]*/"; to = "/"; regex = true; }
      ];
    };
    git_branch = {
      symbol = "";
      style = "bg:fg fg:bg";
      format = "[ $symbol $branch ]($style)";
    };
    git_status = {
      style = "bg:surface fg:fg";
      format = "[( $all_status$ahead_behind )]($style)";
    };
    git_metrics = {
      disabled = false;
      format = "([+$added ]($added_style))([-$deleted ]($deleted_style))";
      added_style = "fg:success bg:surface";
      deleted_style = "fg:error bg:surface";
    };
    cmd_duration = {
      style = "fg:dim";
      format = "[ $duration ]($style)";
    };
    # Always-on inside the devshell — pure noise there.
    nix_shell.disabled = true;
    continuation_prompt = "[·](fg:${pal.dim}) ";

  };

  settings = lib.recursiveUpdate defaultSettings m.settings;
in
if m.configFile != null then m.configFile else (formats.toml { }).generate "starship.toml" settings
