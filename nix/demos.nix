# Feature-demo builders shared by packages.nix, apps.nix, and checks.nix.
# Evaluated once per system in flake.nix and passed down as `demos`.
{ pkgs, lib, ... }:
let
  motdDemos = import ./motd-demo-builder.nix { inherit pkgs lib; };
  menuDemo = import ./menu-demo-builder.nix { inherit pkgs lib; };
  ex = import ../src/prelude/examples.nix;
  typescriptPrelude = (import ../examples/typescript/prelude.nix { inherit lib; }).prelude;
  typescriptMenu = menuDemo.mkMenu (
    {
      inherit (typescriptPrelude) project groups;
    }
    // lib.removeAttrs typescriptPrelude.menu [ "enable" ]
  );

  # Feature demos — `example-<name>` packages/apps.
  examplePackages = motdDemos.examplePackages // {
    example-menu = menuDemo.package;
    example-typescript-menu = typescriptMenu;
  };

  # `nix run .#examples` — on a tty: a pager, one demo per screen,
  # ←/→ to navigate. Piped (CI): every demo rendered in sequence.
  examplesRunner =
    let
      entries =
        map (name: {
          label = "example-${name}";
          hint = "nix run .#example-${name}";
          cmd = lib.getExe examplePackages."example-${name}";
        }) (lib.attrNames ex.motdDemos)
        ++ [
          {
            label = "example-motd";
            hint = "nix run .#example-motd";
            cmd = lib.getExe examplePackages.example-motd;
          }
          {
            label = "example-themes";
            hint = "nix run .#example-themes";
            cmd = lib.getExe examplePackages.example-themes;
          }
          {
            label = "example-menu list";
            hint = "nix run .#example-menu -- list";
            cmd = "${lib.getExe examplePackages.example-menu} list";
          }
          {
            label = "example-typescript-menu list";
            hint = "nix run .#example-typescript-menu -- list";
            cmd = "${lib.getExe examplePackages.example-typescript-menu} list";
          }
        ];
      bashArray =
        name: f: "${name}=(${lib.concatMapStringsSep " " (e: lib.escapeShellArg (f e)) entries})";
    in
    pkgs.writeShellApplication {
      name = "prelude-examples";
      text = ''
        ${bashArray "labels" (e: e.label)}
        ${bashArray "hints" (e: e.hint)}
        ${bashArray "cmds" (e: e.cmd)}
        n=''${#labels[@]}

        if [ -t 0 ] && [ -t 1 ]; then
          # Interactive pager: one demo per screen.
          i=0
          while :; do
            clear || true
            printf '\033[1m── %s\033[0m  \033[2m(%d/%d · %s)\033[0m\n\n' \
              "''${labels[i]}" "$((i + 1))" "$n" "''${hints[i]}"
            bash -c "''${cmds[i]}" || true
            printf '\n\033[2m← → navigate · q quit\033[0m\n'
            IFS= read -rsn1 key || break
            case "$key" in
              q | Q) break ;;
              $'\x1b')
                rest=""
                IFS= read -rsn2 -t 1 rest || true
                case "$rest" in
                  '[C') i=$(((i + 1) % n)) ;;
                  '[D') i=$(((i - 1 + n) % n)) ;;
                  "") break ;; # bare esc quits
                  *) : ;;
                esac
                ;;
              l | n | ' ') i=$(((i + 1) % n)) ;;
              h | p) i=$(((i - 1 + n) % n)) ;;
              *) : ;;
            esac
          done
        else
          # Non-interactive: render everything in sequence (CI checks).
          i=0
          while [ "$i" -lt "$n" ]; do
            printf '\n\033[1m── %s\033[0m  (%s)\n' "''${labels[i]}" "''${hints[i]}"
            bash -c "''${cmds[i]}"
            i=$((i + 1))
          done
        fi
      '';
    };
in
{
  inherit examplePackages examplesRunner;
}
