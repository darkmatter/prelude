# Build the generated local-server health descriptor and executable.
#
# The descriptor is intentionally separate from both Starship and MOTD. The
# executable has a pure cache-read mode for prompt rendering and a due-only
# refresh mode that the shell can launch detached.
{ lib
, writeText
, buildGoModule
, ...
}:

config:
let
  descriptor = writeText "prelude-prompt-status.json" (builtins.toJSON {
    project = config.project;
    command = config.command;
    check = config.check;
    ttl = config.ttl;
    start = "x ${config.command}";
  });
in
buildGoModule {
  pname = "prompt-status";
  version = "0.1.0";
  src = ../.;
  subPackages = [ "cmd/prompt-status" ];
  doCheck = false;
  vendorHash = "sha256-xtubcnDtPcFPOr7Qj3hm2eSGxbACoabLyl/CTLlqp/U=";
  ldflags = [
    "-s"
    "-w"
    "-X main.defaultDescriptorPath=${descriptor}"
  ];
  passthru.configFile = descriptor;
  meta = {
    description = "Cached asynchronous Prelude local-server health status";
    mainProgram = "prompt-status";
  };
}
