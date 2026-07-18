#!/usr/bin/env bash
set -euo pipefail

# PROTOTYPE: determine whether scripted asciinema capture plus agg rendering
# gives Prelude a crisper, CI-runnable alternative to VHS rendering.

root="$(git rev-parse --show-toplevel)"
prototype="$root/dev/vhs/asciinema-agg-prototype"
output="$prototype/output"
cast="$output/motd.cast"
gif="$output/motd.gif"
png="$output/motd.png"
program="$prototype/.default-motd/bin/motd"

mkdir -p "$output"
cd "$root"

nix build .#motd --out-link "$prototype/.default-motd"

asciinema rec \
  --output-format asciicast-v3 \
  --headless \
  --overwrite \
  --return \
  --window-size 128x47 \
  --idle-time-limit 2 \
  --command "tput civis; clear; $program; sleep 2" \
  "$cast"

agg \
  --text-font-family "MonaspiceNe Nerd Font Mono" \
  --font-size 13 \
  --theme "0c0c13,b1b1bf,0c0c13,ee848e,b7ce99,f2c17d,89b4fa,cc99ff,89b4fa,b1b1bf,4a5585,ee848e,b7ce99,f2c17d,89b4fa,cc99ff,89b4fa,b1b1bf" \
  --idle-time-limit 2 \
  "$cast" "$gif"

ffmpeg -y -v error -i "$gif" -vf 'select=eq(n\,1)' -frames:v 1 "$png"

test -s "$cast"
test -s "$gif"
test -s "$png"

printf '\nPrototype output:\n  %s\n  %s\n  %s\n' "$cast" "$gif" "$png"
