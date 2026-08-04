# Stock answers used when previewing "what setup would emit".
#
# These match a completed setup wizard walkthrough: project identity, theme,
# a few Getting Started commands, MOTD + menu + prompt on, docs off. The
# emitted prelude.nix (see nix/internal/example.nix) lists every other option
# as a commented default from defaults.nix; only these fields are active.
#
# Keep in sync with:
#   - nix/internal/example.nix  (WRITE_EXAMPLE=1 go test …TestWriteExampleFixture)
#   - example-default MOTD build in nix/motd-demo-builder.nix
{
  theme = "prelude";
  colorProfile = "auto";
  project = "acme";
  # FIGlet font for the generated title.txt (wizard title step).
  font = "kompaktblk";

  motd = true;
  menu = true;
  prompt = true;
  docs = false;

  # Catalogue the user would enter on the commands step. Order becomes
  # motd = 1..n when MOTD is enabled (Getting Started).
  commands = [
    {
      name = "dev";
      exec = "pnpm dev";
      description = "start the dev server with hot reload";
    }
    {
      name = "test";
      exec = "pnpm test";
      description = "run the unit test suite";
    }
    {
      name = "build";
      exec = "pnpm build";
      description = "compile an optimized production bundle";
    }
  ];
}
