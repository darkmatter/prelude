# Prompt package builder: theme palette → starship.toml.
#
# The package is a starship config file, not a program. Starship re-resolves
# $STARSHIP_CONFIG on every prompt render, and direnv propagates plain env
# vars (only PS1 itself is stripped) — so a devshell that exports
#
#   export STARSHIP_CONFIG=${config.packages.prompt}
#
# re-themes the user's existing starship prompt while inside the project and
# reverts automatically when direnv unloads. No rc hooks, no PATH-at-rc-time
# problems; the only requirement is that the user's shell already runs
# starship (`eval "$(starship init zsh)"` via home-manager or similar).
{ lib
, formats
, ...
}:

# Component config: shared theme fields plus settings/configFile options.
# `shortcuts` is synthesized and injected by the flake-parts module.
config:

let
  d = import ./defaults.nix;
  plib = import ./lib.nix { inherit lib; };

  pal = plib.resolvePalette (config.theme or d.theme) (config.palette or d.palette);
  project = config.project or d.project;
  m = d.prompt // config;

  # Styles reference palette tokens by name (bg:surface, fg:accent2, …);
  # `palettes.prelude` maps them to the resolved theme hex values, mirroring
  # how a hand-written starship config names its palette.
  #
  # Layout (two blank lines, then a two-line prompt with shortcuts right-aligned
  # on the Powerline):
  #
  #
  #
  #   ░▒▓ project  path  branch  status   ···  [?] motd  [m] menu  [d] docs
  #   ❯
  #
  # Each separator inherits the background of the segment on its left as its
  # foreground and the next segment's background as its own background. That
  # overlap is what makes adjacent modules one continuous Powerline rather
  # than independently styled labels. `\[`/`\]` are literal brackets in
  # Starship format strings.
  shortcutChip =
    { command, alias, ... }:
    "[\\[](fg:dim)[${alias}](bold fg:accent2)[\\]](fg:dim)[ ${command}](fg:muted)";
  chips = lib.concatMapStringsSep "  " shortcutChip m.shortcuts;

  # Keep the separator colors paired with the neighboring module backgrounds.
  # A separator is intentionally unconditional, matching Starship's Powerline
  # presets: stable geometry is preferable to caps shifting as Git state changes.
  leftSegments = lib.concatStrings [
    "[░▒▓](fg:secondary)"
    "[ ${project} ](bg:secondary bold fg:accent2)"
    "[](fg:secondary bg:bg)"
    "$directory"
    "[](fg:bg bg:fg)"
    "$git_branch"
    "[](fg:fg bg:surface)"
    "$git_status"
    "[ ](fg:surface)"
    "$cmd_duration"
  ];

  defaultSettings = {
    format =
      if m.shortcuts == [ ] then
        "\n\n${leftSegments}\n$character"
      else
        "\n\n${leftSegments}$fill${chips}\n$character";
    add_newline = false;
    palette = "prelude";
    palettes.prelude = pal;

    fill.symbol = " ";

    directory = {
      style = "bg:bg fg:fg";
      format = "[ $path ]($style)";
      truncation_length = 3;
      truncation_symbol = "…/";
    };
    git_branch = {
      symbol = "";
      style = "bg:fg fg:bg";
      format = "[ $symbol $branch ]($style)";
    };
    git_status = {
      style = "bg:surface fg:accent";
      format = "[( $all_status$ahead_behind )]($style)";
    };
    cmd_duration = {
      style = "fg:dim";
      format = "[ $duration ]($style)";
    };
    # Always-on inside the devshell — pure noise there.
    nix_shell.disabled = true;
    character = {
      success_symbol = "[❯](bold fg:success)";
      error_symbol = "[❯](bold fg:error)";
      vimcmd_symbol = "[❮](bold fg:accent)";
    };
    continuation_prompt = "[·](fg:${pal.dim}) ";
  };

  settings = lib.recursiveUpdate defaultSettings m.settings;
in
if m.configFile != null then m.configFile else (formats.toml { }).generate "starship.toml" settings
