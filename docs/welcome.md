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

## Silencing direnv noise under the MOTD

If you use [direnv](https://direnv.net/) alongside Prelude, its per-shell
log lines can print between the MOTD and your prompt. To suppress that noise,
run once per user account:

```sh
mkdir -p ~/.config/direnv && touch ~/.config/direnv/direnv.toml
grep -q 'log_format = "-"' ~/.config/direnv/direnv.toml \
  || printf '%s\n' '[global]' 'log_format = "-"' 'log_filter = "^$"' \
  >> ~/.config/direnv/direnv.toml
```

If you ship the MOTD to other users, point them at this snippet too — direnv
logging is a per-user config, so it must be applied on each account.
