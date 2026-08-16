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

If you use [direnv](https://direnv.net/) alongside Prelude, its log lines print
around the MOTD, and a project with a slow Nix evaluation adds a "taking a
while to execute" warning on top. Four settings cover every message direnv
emits — each one suppresses a different line:

```sh
mkdir -p ~/.config/direnv
grep -q hide_env_diff ~/.config/direnv/direnv.toml 2>/dev/null \
  || printf '%s\n' \
    '[global]' \
    'log_format = "-"' \
    'log_filter = "^$"' \
    'hide_env_diff = true' \
    'warn_timeout = "1h"' \
    >> ~/.config/direnv/direnv.toml
```

- `log_format` — `direnv: loading …`
- `log_filter` — output your `.envrc` itself writes
- `hide_env_diff` — `direnv: export +FOO +BAR …`
- `warn_timeout` — `… is taking a while to execute`

This has to live in the user's own direnv config. `direnv export` reads its
logging settings at startup, *before* it runs `.envrc`, so exporting
`DIRENV_LOG_FORMAT` from a devshell or from `.envrc` cannot silence the load
that is already underway — the variable only lands in the environment being
produced. Prelude therefore cannot apply this for you. If you ship the MOTD to
a team, pass the snippet along; it applies per user account.
