# Docs package builder: hand-authored sections → JSON → Go man-style viewer.
{
  lib,
  writeText,
  buildGoModule,
  ...
}:

# Component config: { theme?, palette?, colorProfile?, project?, sections? }
config:

let
  d = import ./defaults.nix;
  plib = import ./lib.nix { inherit lib; };

  pal = plib.resolvePalette (config.theme or d.theme) (config.palette or d.palette);
  colorProfile = config.colorProfile or d.colorProfile;
  project = config.project or d.project;
  m = d.docs // config;

  normalizeBlock = b: {
    type = b.type;
    term = b.term or "";
    text = b.text or "";
    command = b.command or "";
    note = b.note or "";
  };

  sections = map (
    { name, value }:
    let
      title = value.title or null;
    in
    {
      title = if title == null then name else title;
      blocks = map normalizeBlock (value.blocks or [ ]);
    }
  ) (plib.sortOrderedAttrs (m.sections or { }));

  configFile = writeText "prelude-docs.json" (
    builtins.toJSON {
      inherit
        project
        colorProfile
        sections
        ;
      palette = pal;
    }
  );
in
assert lib.assertMsg (sections != [ ]) "docs: no sections configured — set prelude.docs.sections";
assert lib.assertOneOf "docs colorProfile" colorProfile [
  "auto"
  "truecolor"
  "ansi256"
];
buildGoModule {
  pname = "docs";
  version = "0.1.0";
  src = ../.;
  subPackages = [ "docs" ];
  doCheck = false;
  vendorHash = "sha256-a4FKIcqmKJ0TxRogtXe1T7iNf7mgX27GDtbnwf4FvxU=";
  ldflags = [
    "-s"
    "-w"
    "-X main.defaultConfigPath=${configFile}"
  ];
  meta = {
    description = "Hand-authored project manual (man-style TUI)";
    mainProgram = "docs";
  };
}
