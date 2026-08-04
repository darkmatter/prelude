# pin + prelude-shell + Zellij workspace assets.
#
# Interactive TUIs (docs, motd, …) run in real Zellij panes — not a host-side
# VT capture. The shell pane is a full interactive Bash with the same Starship
# + ble.sh stack as `nix develop` (via the canonical prelude-init source).
#
#   pin docs          # pin docs above the shell (starts Zellij if needed)
#   pin motd          # replace the pin with motd
#   pin               # shell-only workspace
#
# `menu` is intentionally not special-cased: it is a full-screen interactive
# TUI — run it in the shell pane (or any focused terminal), not as a pin.
{
  lib,
  writeText,
  writeShellApplication,
  writeShellScriptBin,
  runCommand,
  symlinkJoin,
  zellij,
  bash,
  jq,
}:

{
  # Absolute path to generated starship.toml (packages.prompt), or null.
  promptConfig ? null,
  # Canonical generated shell initialization shared with nix develop.
  shellInit,
}:

let
  pinPaneName = "prelude-pin";

  # Interactive Bash rc used only to enter the canonical Prelude init from a
  # Zellij-created shell. Prompt/editor behavior itself is not duplicated here.
  bashRc = writeText "prelude-shell.bash" ''
    # Minimal interactive profile for prelude workspace panes.
    # Do not source the user's full bashrc — that re-enters direnv/zellij loops.

    # Inherit / keep project STARSHIP_CONFIG from the parent environment.
    ${lib.optionalString (promptConfig != null) ''
      if [ -z "''${STARSHIP_CONFIG:-}" ] && [ -f ${lib.escapeShellArg promptConfig} ]; then
        export STARSHIP_CONFIG=${lib.escapeShellArg promptConfig}
      fi
    ''}

    prelude-init() {
      # shellcheck source=/dev/null
      . ${lib.escapeShellArg shellInit}
    }
    PRELUDE_INIT_QUIET=1 prelude-init

    # Gentle cue once per shell (not when nested pin opens).
    if [ -z "''${PRELUDE_SHELL_QUIET:-}" ] && [ -n "''${ZELLIJ:-}" ]; then
      printf '%s\n' "prelude-shell · Alt+p swap · middle rail shows focus · pin <cmd>" 1>&2
    fi
  '';

  preludeShell = writeShellScriptBin "prelude-shell" ''
    #!${bash}/bin/bash
    # Zellij default_shell / layout command: interactive Bash with prelude prompt.
    export PRELUDE_SHELL=1
    exec ${bash}/bin/bash --rcfile ${bashRc} -i "$@"
  '';

  # 1-row focus rail: frameless workspaces have no selected frame, so this
  # pane draws a changing label (● shell / ● pin · docs). If the rail itself
  # gains focus, it immediately hands focus to the next pane.
  focusRail = writeShellApplication {
    name = "prelude-focus-rail";
    runtimeInputs = [
      zellij
      jq
    ];
    text = ''
      set -euo pipefail
      # Hide cursor; restore on exit.
      printf '\033[?25l' 2>/dev/null || true
      trap 'printf "\033[?25h\033[0m" 2>/dev/null || true' EXIT

      RAIL_MARK="prelude-focus-rail"
      # Zellij sets this for terminal panes.
      MY_ID="''${ZELLIJ_PANE_ID:-}"

      draw() {
        local label=$1
        local cols hint
        cols=$(tput cols 2>/dev/null || echo 80)
        hint=" Alt+p swap · Ctrl+g unlock "
        # Reverse-video label so focus is obvious without pane frames.
        local left=" ● ''${label} "
        local fill=$(( cols - ''${#left} - ''${#hint} ))
        if [ "$fill" -lt 0 ]; then
          fill=0
          hint=""
        fi
        printf '\r\033[2K\033[0;7m%s\033[0;90m%*s%s\033[0m' \
          "$left" "$fill" "" "$hint"
      }

      while true; do
        if [ -z "''${ZELLIJ:-}" ]; then
          draw "(not in zellij)"
          sleep 1
          continue
        fi

        json=$(zellij action list-panes -j -a 2>/dev/null || true)
        if [ -z "$json" ]; then
          draw "…"
          sleep 0.3
          continue
        fi

        # Bounce focus away if this rail pane is selected.
        if [ -n "$MY_ID" ]; then
          focused_id=$(printf '%s' "$json" | jq -r --arg me "$MY_ID" '
            .[]
            | select(.is_focused == true)
            | if (.pane_id | type) == "string" then .pane_id
              elif (.id | type) == "string" then .id
              else "terminal_\(.id|tostring)" end
          ' 2>/dev/null | head -1 || true)
          if [ "$focused_id" = "$MY_ID" ] || [ "$focused_id" = "terminal_''${MY_ID#terminal_}" ]; then
            zellij action focus-next-pane 2>/dev/null || true
            sleep 0.05
            continue
          fi
        fi

        label=$(printf '%s' "$json" | jq -r --arg mark "$RAIL_MARK" '
          [
            .[]
            | select((.is_plugin // false) | not)
            | select(.is_focused == true)
            | select((.title // "") != $mark)
            | select((.title // "") | startswith("prelude-focus") | not)
            | (.title // "pane")
          ]
          | if length == 0 then "—" else .[0] end
        ' 2>/dev/null || echo "—")

        draw "$label"
        sleep 0.12
      done
    '';
    meta.description = "1-row Zellij focus indicator for frameless prelude workspaces";
  };

  # Locked-by-default so shell autocomplete/ble keys reach the terminal.
  # Frameless: no box borders. Focus is the middle rail label (● name).
  # Alt+p swaps focus pin ⇄ shell without leaving locked mode.
  zellijConfig = writeText "prelude-zellij-config.kdl" ''
    // Prelude workspace — generated; prefer `pin` over hand-editing.
    pane_frames false
    mouse_hover_effects false
    simplified_ui true
    default_mode "locked"
    mouse_mode true
    copy_on_select true
    show_startup_tips false
    show_release_notes false
    default_shell "${preludeShell}/bin/prelude-shell"
    ui {
      pane_frames {
        rounded_corners false
        hide_session_name true
      }
    }
    keybinds {
      // Bound in locked so it works without Ctrl+g first.
      // Override default Alt+p (TogglePaneInGroup) everywhere.
      locked {
        bind "Ctrl g" { SwitchToMode "Normal"; }
        bind "Alt p" { FocusNextPane; }
        bind "Alt Tab" { FocusNextPane; }
      }
      normal {
        bind "Ctrl g" { SwitchToMode "Locked"; }
        bind "Alt p" { FocusNextPane; }
        bind "Alt Tab" { FocusNextPane; }
      }
      shared_except "locked" {
        unbind "Alt p"
        bind "Alt p" { FocusNextPane; }
        bind "Alt Tab" { FocusNextPane; }
      }
    }
    // Session name left unset so concurrent projects do not collide.
  '';

  focusRailBin = "${focusRail}/bin/prelude-focus-rail";

  # Shell-only: focus rail on top (acts as the label strip).
  shellLayout = writeText "prelude-zellij-shell.kdl" ''
    layout {
      cwd "."
      default_tab_template {
        children
      }
      pane split_direction="horizontal" {
        pane size=1 borderless=true {
          name "prelude-focus-rail"
          command "${focusRailBin}"
        }
        pane borderless=true {
          name "shell"
          command "${preludeShell}/bin/prelude-shell"
          focus true
        }
      }
    }
  '';

  # Real directory (not a lone store file) so --config-dir is not /nix/store.
  zellijDir = runCommand "prelude-zellij-dir" { } ''
    mkdir -p "$out/layouts"
    cp -f ${zellijConfig} "$out/config.kdl"
    cp -f ${shellLayout} "$out/layouts/shell.kdl"
  '';

  pinBin = writeShellApplication {
    name = "pin";
    runtimeInputs = [
      zellij
      jq
      bash
    ];
    text = ''
      set -euo pipefail

      PIN_NAME=${lib.escapeShellArg pinPaneName}
      ZELLIJ_CONFIG_DIR=${lib.escapeShellArg zellijDir}
      ZELLIJ_CONFIG="$ZELLIJ_CONFIG_DIR/config.kdl"
      SHELL_LAYOUT="$ZELLIJ_CONFIG_DIR/layouts/shell.kdl"
      PRELUDE_SHELL_BIN=${lib.escapeShellArg "${preludeShell}/bin/prelude-shell"}
      FOCUS_RAIL_BIN=${lib.escapeShellArg focusRailBin}

      # Must use exec *on zellij*, not `exec zellij_launch` — exec only resolves
      # PATH binaries, not shell functions.
      zellij_launch() {
        # Drop user overrides so ~/.config/zellij pane_frames cannot win.
        exec env -u ZELLIJ_CONFIG_FILE \
          zellij --config "$ZELLIJ_CONFIG" --config-dir "$ZELLIJ_CONFIG_DIR" "$@"
      }

      ensure_frameless() {
        zellij options --pane-frames false 2>/dev/null || true
      }

      usage() {
        cat <<'EOF'
      usage: pin [<command> [args...]]

      Open a command in a pinned Zellij pane above the shell (real TTY —
      full TUI fidelity). Starts a prelude workspace session when needed.

      examples:
        pin              # shell-only workspace (Starship + ble + completions)
        pin docs         # docs above shell
        pin motd         # welcome banner above shell
        pin htop         # any command on PATH

      notes:
        • menu is interactive — run `menu` in the shell pane, not as a pin
        • frameless panes; the 1-row rail shows ● focused name
        • Alt+p (or Alt+Tab) swaps focus between pin and shell
        • Ctrl+g unlocks Zellij modes; close pin (or q in the app) to drop it
      EOF
      }

      if [ "''${1:-}" = "-h" ] || [ "''${1:-}" = "--help" ]; then
        usage
        exit 0
      fi

      # Escape a string as a KDL double-quoted value.
      kdl_str() {
        local s=$1
        s=''${s//\\/\\\\}
        s=''${s//\"/\\\"}
        s=''${s//$'\n'/\\n}
        printf '"%s"' "$s"
      }

      close_existing_pin() {
        # Close any non-plugin pin pane (title "pin · …" or legacy prelude-pin).
        local json id
        json=$(zellij action list-panes -j -a 2>/dev/null || true)
        if [ -z "$json" ]; then
          return 0
        fi
        while IFS= read -r id; do
          [ -n "$id" ] || continue
          zellij action close-pane -p "$id" 2>/dev/null || true
        done < <(
          printf '%s' "$json" | jq -r --arg n "$PIN_NAME" '
            .[]
            | select((.is_plugin // false) | not)
            | select(
                ((.title // "") | startswith("pin"))
                or (.title // "") == $n
                or (.name // "") == $n
                or ((.name // "") | startswith("pin"))
              )
            | if (.pane_id | type) == "string" then .pane_id
              elif (.id | type) == "string" then .id
              else "terminal_\(.id|tostring)"
              end
          ' 2>/dev/null || true
        )
      }

      open_pin_in_session() {
        close_existing_pin
        ensure_frameless
        local title
        title="pin · $1"
        # Frameless top pane; focus rail (if present) keeps showing ● name.
        zellij run \
          --name "$title" \
          --direction up \
          --close-on-exit \
          --borderless true \
          --cwd "''${PWD:-.}" \
          -- "$@" >/dev/null 2>&1 || true
        # Prefer typing in the shell after the pin opens.
        zellij action move-focus down 2>/dev/null || true
        zellij action move-focus down 2>/dev/null || true
      }

      start_workspace() {
        # Outside Zellij: shell-only layout, then optionally open pin.
        local -a pin_cmd=("$@")
        if [ "''${#pin_cmd[@]}" -eq 0 ]; then
          zellij_launch --layout "$SHELL_LAYOUT"
        fi

        # Resolve command for a one-shot layout (reliable PATH at pin time).
        local resolved args_kdl tmp a
        if ! resolved=$(command -v "''${pin_cmd[0]}" 2>/dev/null); then
          echo "pin: command not found: ''${pin_cmd[0]}" >&2
          exit 127
        fi
        args_kdl=""
        if [ "''${#pin_cmd[@]}" -gt 1 ]; then
          for a in "''${pin_cmd[@]:1}"; do
            args_kdl+=" $(kdl_str "$a")"
          done
          args_kdl="args$args_kdl"
        fi

        tmp=$(mktemp "''${TMPDIR:-/tmp}/prelude-pin-XXXXXX.kdl")
        # shellcheck disable=SC2064
        trap 'rm -f "$tmp"' EXIT
        local pin_title
        pin_title="pin · ''${pin_cmd[0]}"
        # pin | focus-rail | shell — rail is the changing middle label.
        cat >"$tmp" <<EOF
      layout {
        cwd $(kdl_str "''${PWD:-.}")
        default_tab_template {
          children
        }
        pane split_direction="horizontal" {
          pane borderless=true {
            name $(kdl_str "$pin_title")
            command $(kdl_str "$resolved")
            $args_kdl
            size "70%"
            close_on_exit true
          }
          pane size=1 borderless=true {
            name "prelude-focus-rail"
            command $(kdl_str "$FOCUS_RAIL_BIN")
          }
          pane borderless=true {
            name "shell"
            command $(kdl_str "$PRELUDE_SHELL_BIN")
            focus true
          }
        }
      }
      EOF
        zellij_launch --layout "$tmp"
      }

      if [ -z "''${ZELLIJ:-}" ]; then
        start_workspace "$@"
      fi

      if [ "$#" -eq 0 ]; then
        echo "pin: already inside Zellij; pass a command (e.g. pin docs)" >&2
        exit 2
      fi

      if ! command -v "$1" >/dev/null 2>&1; then
        echo "pin: command not found: $1" >&2
        exit 127
      fi

      open_pin_in_session "$@"
    '';
    meta.description = "Open a command in a pinned Zellij pane above the prelude shell";
  };
in
symlinkJoin {
  name = "prelude-pin";
  paths = [
    pinBin
    preludeShell
    focusRail
  ];
  postBuild = ''
    mkdir -p "$out/share/prelude/zellij"
    cp -a ${zellijDir}/. "$out/share/prelude/zellij/"
    cp -f ${bashRc} "$out/share/prelude/shell.bash"
  '';
  passthru = {
    inherit
      bashRc
      zellijConfig
      shellLayout
      zellijDir
      preludeShell
      pinBin
      focusRail
      ;
  };
  meta = {
    description = "Prelude Zellij pin workspace (pin, prelude-shell)";
  };
}
