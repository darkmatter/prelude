{ lib
, writeShellApplication
, writeText
, buildGoModule
, figlet
, nix
, ...
}:
let
  fonts = import ./fonts.nix;
  defaults = import ./defaults.nix;
  themes = import ./themes.nix;
  configFile = writeText "prelude-title.json" (
    builtins.toJSON {
      defaultFont = "thin";
      fonts = lib.mapAttrsToList
        (name: path: {
          inherit name path;
        })
        fonts;
      # The wizard iteration offers the bundled palettes; the plain chooser
      # ignores these fields.
      defaultTheme = defaults.theme;
      themes = lib.mapAttrsToList
        (name: palette: {
          inherit name palette;
        })
        themes;
    }
  );

  titleChooser = buildGoModule {
    pname = "prelude-title";
    version = "0.1.0";
    src = ../.;
    subPackages = [ "cmd/title" ];
    doCheck = false;
    vendorHash = "sha256-qHpXE7MVG06KxY/2eLnqUva3/FHjAdQceH6A/5sn7mU=";
    ldflags = [
      "-s"
      "-w"
      "-X main.defaultConfigPath=${configFile}"
    ];
    meta = {
      description = "Interactive Prelude MOTD title renderer";
      mainProgram = "title";
    };
  };
in
writeShellApplication {
  name = "prelude-title";
  runtimeInputs = [
    figlet
    nix
  ];
  text = ''
    exec ${lib.getExe titleChooser} "$@"
  '';
  meta.description = "Choose a Prelude MOTD title and render it to stdout or an explicit path";
}
