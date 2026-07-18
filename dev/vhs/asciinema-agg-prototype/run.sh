#!/usr/bin/env bash
set -euo pipefail

root="$(git rev-parse --show-toplevel)"
prototype="$root/dev/vhs/asciinema-agg-prototype"

exec nix shell nixpkgs#asciinema nixpkgs#agg nixpkgs#ffmpeg --command \
  bash "$prototype/record.sh"
