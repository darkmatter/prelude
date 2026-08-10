#!/usr/bin/env bash
# Preflight - Bootstrap script to handle users without Nix or direnv.
set -euo pipefail

if [[ "$#" -eq 0 ]]; then
  echo "usage: preflight.sh command [args...]" >&2
  exit 64
fi

if [[ -n "${IN_NIX_SHELL:-}" ]]; then
  exec "$@"
fi

if command -v nix >/dev/null 2>&1; then
  echo "info: Not in a devshell; Run \`nix develop\` to avoid every command being prefixed"
  exec nix develop -c "$@"
fi

if [[ "${NIX_PREFLIGHT_ALLOW_HOST:-0}" == "1" ]]; then
  echo "warning: Nix unavailable; running on the host: $*" >&2
  exec "$@"
fi

echo "error: requires Nix or an active devshell (or set NIX_PREFLIGHT_ALLOW_HOST=1)" >&2
exit 127
