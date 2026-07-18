# Welcome

**Prelude** is a devshell UI suite for Nix flakes: a welcome banner (MOTD), an
interactive command menu, this Markdown docs viewer, and a themed Starship
prompt — declared as flake-parts options, validated by Nix at build time, and
rendered by small Go binaries.

This shell is Prelude developing itself: everything you see is produced from
root `prelude.nix` (plus the small `nix/prelude-*.nix` imports), the same shape
a downstream project would use.

Viewer keys: digits jump between pages, `Tab`/`Shift-Tab` step through them,
`j`/`k` scroll, `q` quits.
