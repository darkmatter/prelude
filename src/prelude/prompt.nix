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
config: let
  d = import ./defaults.nix;
  plib = import ./lib.nix {inherit lib;};
  m = d.prompt // config;
  pal =
    config.resolvedPalette
    or (plib.resolvePalette (config.theme or d.theme) (config.palette or d.palette));

  mkKey = char: label: "\\[[${char}](bold fg:accent)\\][─](fg:surface)${label}";
  keymap = lib.concatStringsSep "[──](fg:surface)" (
    map (shortcut: mkKey shortcut.alias shortcut.command) (config.shortcuts or [])
  );

  # Styles reference palette tokens by name (bg:surface, fg:accent2, …);
  # `palettes.prelude` maps them to the resolved theme hex values, mirroring
  # how a hand-written starship config names its palette.
  #
  # Layout (cross-shell via Starship, with fixed status chrome only in Bash):
  #
  #   Context: ╭░▒▓ π  path  branch  status  duration ── [keys] ─╮
  #   Prompt:  ╰─
  #
  # Each Powerline separator inherits the background of the segment at its left
  # as its foreground and the next segment's background as its own background.
  # `\[`/`\]` are literal brackets in Starship format strings.
  # The full-width context and editable input occupy separate rows above Blesh's
  # bottom-docked status row. Unbounded cells remain transparent; only named
  # Powerline segments paint a background.
  leftSegments = lib.concatStrings [
    "[╭░▒▓](fg:accent)"
    "[ π ](bold bg:accent fg:bg)"
    "[](fg:accent bg:bg)"
    "( $directory)"
    "[](fg:bg bg:fg)"
    "$git_branch"
    "[](fg:fg bg:surface)"
    "$git_status"
    "$git_metrics"
    "[$fill](fg:surface)[${keymap}](fg:muted)[─╮](fg:surface)"
  ];

  defaultSettings = {
    # One breathing row, then separate context and editable-input rows. The
    # input row stays distinct from Bash's fixed status row.
    format = "[${leftSegments}\n[│](fg:accent)\n[╰─](fg:accent) ]()";
    add_newline = true;

    right_format = "[│](fg:surface)\n[──╯](fg:surface)\n\n\n";

    fill.symbol = "─";
    fill.style = "fg:surface";

    character.format = "[$symbol]() ";

    palette = "prelude";
    palettes.prelude = pal;

    directory = {
      style = "bg:bg fg:fg";
      format = "[ $path ]($style)";
      truncation_length = 8;
      truncation_symbol = "";
      truncate_to_repo = true;
      substitutions = [
        {
          from = "^[^~/][^/]*/";
          to = "/";
          regex = true;
        }
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
  if m.configFile != null
  then m.configFile
  else (formats.toml {}).generate "starship.toml" settings
