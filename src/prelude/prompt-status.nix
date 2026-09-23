# Build the generated local-server health descriptor and executable.
#
# The descriptor is intentionally separate from both Starship and MOTD. The
# executable has a pure cache-read mode for prompt rendering and a due-only
# refresh mode that the shell can launch detached. status.bash passes the
# descriptor with --config, so the executable stays config-independent.
{
  lib,
  writeText,
  buildGoModule,
  ...
}: config: let
  descriptor = writeText "prelude-prompt-status.json" (
    builtins.toJSON {
      project = config.project;
      command = config.command;
      check = config.check;
      ttl = config.ttl;
      start = config.start;
    }
  );
in
  buildGoModule {
    pname = "prompt-status";
    version = "0.1.0";
    src = import ./go-source.nix {inherit lib;};
    subPackages = ["cmd/prompt-status"];
    doCheck = false;
    vendorHash = "sha256-BHrU5pKVDuGDq0ZHbHKcUBa5olzHzfgoJXzv2IGXY4U=";
    ldflags = [
      "-s"
      "-w"
    ];
    passthru.configFile = descriptor;
    meta = {
      description = "Cached asynchronous Prelude local-server health status";
      mainProgram = "prompt-status";
    };
  }
