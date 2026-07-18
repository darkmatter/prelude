# Docs package builder: Markdown page paths → embedded JSON → Go viewer.
{ lib
, writeText
, buildGoModule
, ...
}:

# Component config: { theme?, palette?, colorProfile?, project?, pages? }
config:

let
  d = import ./defaults.nix;
  plib = import ./lib.nix { inherit lib; };

  pal = plib.resolvePalette (config.theme or d.theme) (config.palette or d.palette);
  colorProfile = config.colorProfile or d.colorProfile;
  project = config.project or d.project;
  m = d.docs // config;

  pages = map (page: { text = builtins.readFile page.text; }) (m.pages or [ ]);

  configFile = writeText "prelude-docs.json" (
    builtins.toJSON {
      inherit
        project
        colorProfile
        pages
        ;
      palette = pal;
    }
  );
in
assert lib.assertMsg (pages != [ ]) "docs: no pages configured — set prelude.docs.pages";
assert lib.assertOneOf "docs colorProfile" colorProfile [
  "auto"
  "truecolor"
  "ansi256"
];
buildGoModule {
  pname = "docs";
  version = "0.1.0";
  src = ../.;
  subPackages = [ "cmd/docs" ];
  doCheck = false;
  vendorHash = "sha256-hKvYlJqQUQ3NrBRgWPZyvYhsCvceW1HbDRlzltKyCxQ=";
  ldflags = [
    "-s"
    "-w"
    "-X main.defaultConfigPath=${configFile}"
  ];
  meta = {
    description = "Markdown project docs viewer";
    mainProgram = "docs";
  };
}
