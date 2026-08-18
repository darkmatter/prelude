# Build the canonical interactive-shell initialization used by packages.prelude-shell.
#
# Runtime behavior lives in checked-in Bash modules under ./shell. Nix only
# serializes the evaluated command catalogue and injects immutable dependency
# paths, so changing shell behavior does not mean editing a large Nix string.
{
  lib,
  writeText,
  runCommand,
  starship,
  blesh,
  bash-completion,
  stdenv,
}: {
  palette,
  shadow ? null,
  commandEntries ? [],
  projectName ? "your project",
  navigation ? [],
  motdCommand ? null,
  motdRevision ? null,
  statusEnabled ? false,
  promptFinalConfig ? null,
  promptStatusCommand ? null,
  promptStatusConfig ? null,
  # When the prompt component is off, the init must not name Starship, ble.sh,
  # or bash-completion at all. Nix derives runtime references by scanning output
  # text for store hashes, so merely mentioning those paths would pull all three
  # into every consumer's closure — which is exactly what the prompt gating in
  # module.nix exists to avoid. The MOTD is then the whole surface.
  promptEnabled ? true,
}: let
  plib = import ./lib.nix {inherit lib;};
  resolvedShadow =
    if shadow == null
    then plib.lightenColor palette.bg
    else shadow;
  # Prompt markup is zero-width after ble.sh parses it, while status-row layout
  # runs before parsing. Keep a literal twin so padding always uses cell widths.
  # The status row is standalone; each `\g` span resets SGR, so spans without a
  # background render transparent over the prompt_status_line face.
  navigationText = lib.concatStringsSep "  " (
    map (shortcut: "[${shortcut.alias}] ${shortcut.command}") navigation
  );
  navigationRendered = lib.concatStringsSep "  " (
    map (
      shortcut:
        lib.concatStrings [
          "\\g{fg=${palette.dim}}["
          "\\g{bold,fg=${palette.accent2}}${shortcut.alias}"
          "\\g{fg=${palette.dim}}]"
          "\\g{fg=${palette.muted}} ${shortcut.command}"
        ]
    )
    navigation
  );
  # Keep a visible-text twin: status.bash calculates padding before ble.sh
  # strips the trusted `\g` markup.
  hintPrefix = "Run commands: ";
  hintCommand = "x <cmd>";
  hintText = hintPrefix + hintCommand;
  # Sample the subtle bg → shadow transition densely. status.bash maps these
  # colors to terminal columns, so resizing moves the stops with the viewport
  # instead of tying them to hint fragments.
  gradientStopCount = 64;
  gradientColors =
    lib.genList (
      index:
        plib.mixColor palette.bg resolvedShadow (index * 1.0 / (gradientStopCount - 1))
    )
    gradientStopCount;
  # Fallback for an older runtime that does not understand the gradient palette.
  hintRendered = lib.concatStrings [
    "\\g{fg=${palette.dim}}${hintPrefix}"
    "\\g{bold,fg=${palette.dim}}${hintCommand}"
  ];
  catalogue = writeText "prelude-shell-catalogue.bash" (
    import ./shell/catalogue.nix {inherit lib;} {inherit commandEntries;}
  );
  schemePalette = {
    inherit
      (palette)
      bg
      surface
      secondary
      fg
      muted
      dim
      border
      accent
      accent2
      success
      warning
      info
      error
      ;
    accent_border = palette.accentBorder;
    selection_fg = palette.selectionFg;
    shadow = resolvedShadow;
  };
  # Replace longer names first (`accent_border` and `accent2` before `accent`)
  # because the semantic markers intentionally share readable prefixes.
  schemeTokens = lib.sort (left: right: builtins.stringLength left > builtins.stringLength right) (
    lib.attrNames schemePalette
  );
  scheme = writeText "prelude.bash" (
    lib.replaceStrings (map (token: "%prelude_${token}") schemeTokens) (map (
        token: schemePalette.${token}
      )
      schemeTokens) (builtins.readFile ./shell/scheme.bash)
  );
  # Same palette, rendered into a vim-airline theme. Installed under
  # `contrib/airline/` so `bleopt vim_airline_theme=prelude` resolves
  # `airline/prelude` through the runtime's contrib entry on ble.sh's
  # import_path.
  airlineTheme = writeText "prelude-airline.bash" (
    lib.replaceStrings (map (token: "%prelude_${token}") schemeTokens) (map (
        token: schemePalette.${token}
      )
      schemeTokens) (builtins.readFile ./shell/airline-theme.bash)
  );

  runtime = runCommand "prelude-shell-runtime" {} ''
    install -d "$out" "$out/contrib/scheme" "$out/contrib/airline"
    install -m 0444 ${./shell/init.bash} "$out/init.bash"
    install -m 0444 ${./shell/bash-init.bash} "$out/bash-init.bash"
    # Emitted verbatim by `prelude hook` and `prelude preflight`. These are the
    # only files meant to be copied into a user's rc file or eval'd by a loader,
    # so they stay free of build-time paths.
    install -m 0444 ${./shell/hook.bash} "$out/hook.bash"
    install -m 0444 ${./shell/hook.zsh} "$out/hook.zsh"
    install -m 0444 ${./shell/preflight.bash} "$out/preflight.bash"
    install -m 0444 ${./shell/status.bash} "$out/status.bash"
    install -m 0444 ${./shell/status-cap.bash} "$out/status-cap.bash"
    install -m 0444 ${./shell/completion.bash} "$out/completion.bash"
    install -m 0444 ${scheme} "$out/contrib/scheme/prelude.bash"
    install -m 0444 ${airlineTheme} "$out/contrib/airline/prelude.bash"
    install -m 0444 ${catalogue} "$out/catalogue.bash"
  '';

  init = writeText "prelude-init.bash" ''
    # Generated by Prelude. Source this file; do not execute it in a subshell.
    # MOTD revision: ${
      if motdRevision == null
      then "disabled"
      else motdRevision
    }
    _PRELUDE_SHELL_RUNTIME=${lib.escapeShellArg runtime}
    _PRELUDE_PROMPT_ENABLED=${
      if promptEnabled
      then "1"
      else "0"
    }
    _PRELUDE_MOTD=${lib.escapeShellArg (
      if motdCommand == null
      then ""
      else motdCommand
    )}
    _PRELUDE_DARWIN=${
      if stdenv.isDarwin
      then "1"
      else ""
    }
    ${lib.optionalString promptEnabled ''
      _PRELUDE_BASH_COMPLETION=${lib.escapeShellArg "${bash-completion}/etc/profile.d/bash_completion.sh"}
      _PRELUDE_BLESH=${lib.escapeShellArg "${blesh}/share/blesh/ble.sh"}
      _PRELUDE_STARSHIP=${lib.escapeShellArg (lib.getExe starship)}
      _PRELUDE_STARSHIP_FINAL_CONFIG=${lib.escapeShellArg (
        if promptFinalConfig == null
        then ""
        else promptFinalConfig
      )}
      _PRELUDE_STARSHIP_STATUS_ENABLED=${
        if statusEnabled
        then "1"
        else "0"
      }
      _PRELUDE_PROMPT_PROJECT=${lib.escapeShellArg projectName}
      _PRELUDE_PROMPT_NAVIGATION=${lib.escapeShellArg navigationText}
      _PRELUDE_PROMPT_NAVIGATION_RENDERED=${lib.escapeShellArg navigationRendered}
      _PRELUDE_PROMPT_STATUS_HINT=${lib.escapeShellArg hintText}
      _PRELUDE_PROMPT_STATUS_HINT_RENDERED=${lib.escapeShellArg hintRendered}
      _PRELUDE_PROMPT_STATUS_HINT_BOLD_START=${toString (builtins.stringLength hintPrefix)}
      _PRELUDE_PROMPT_STATUS_HINT_BOLD_WIDTH=${toString (builtins.stringLength hintCommand)}
      _PRELUDE_PROMPT_STATUS_GRADIENT=${lib.escapeShellArg (lib.concatStringsSep ":" gradientColors)}
      _PRELUDE_PROMPT_STATUS_GRADIENT_FG=${lib.escapeShellArg palette.dim}
      _PRELUDE_PROMPT_STATUS=${
        lib.escapeShellArg (
          if promptStatusCommand == null
          then ""
          else promptStatusCommand
        )
      }
      _PRELUDE_PROMPT_STATUS_CONFIG=${
        lib.escapeShellArg (
          if promptStatusConfig == null
          then ""
          else promptStatusConfig
        )
      }
    ''}

    # shellcheck source=/dev/null
    . ${lib.escapeShellArg "${runtime}/init.bash"}

    unset _PRELUDE_SHELL_RUNTIME _PRELUDE_PROMPT_ENABLED
    unset _PRELUDE_MOTD _PRELUDE_DARWIN
    ${lib.optionalString promptEnabled ''
      unset _PRELUDE_BASH_COMPLETION _PRELUDE_BLESH
      unset _PRELUDE_STARSHIP _PRELUDE_STARSHIP_FINAL_CONFIG _PRELUDE_STARSHIP_STATUS_ENABLED
      unset _PRELUDE_PROMPT_PROJECT
      unset _PRELUDE_PROMPT_NAVIGATION _PRELUDE_PROMPT_NAVIGATION_RENDERED
      unset _PRELUDE_PROMPT_STATUS_HINT _PRELUDE_PROMPT_STATUS_HINT_RENDERED
      unset _PRELUDE_PROMPT_STATUS_HINT_BOLD_START _PRELUDE_PROMPT_STATUS_HINT_BOLD_WIDTH
      unset _PRELUDE_PROMPT_STATUS_GRADIENT _PRELUDE_PROMPT_STATUS_GRADIENT_FG
      unset _PRELUDE_PROMPT_STATUS _PRELUDE_PROMPT_STATUS_CONFIG
    ''}
  '';
in {
  inherit init runtime;
}
