# Pinned MOTD shell experiment

> **PROTOTYPE — delete after evaluation.**

Question: is a tmux-hosted Prelude workspace—with pinned MOTD/docs, popup menu,
captured init logs, and a real child shell—pleasant enough to justify promoting?

## Run

Launch directly:

```sh
nix run path:.#motd-shell-experiment
```

Or evaluate shell-hook capture through the parallel devshell:

```sh
nix develop path:.#motd-shell-experiment
```

`path:.` includes this untracked throwaway directory; ordinary `.#...` works
once the files are tracked.

The shell receives half the terminal by default, but waits for the real tmux
client dimensions before starting and pads its first prompt to the bottom row.
Override its share if needed:

```sh
PRELUDE_SHELL_PERCENT=40 nix run path:.#motd-shell-experiment
```

The value must be between 30 and 70. The shell owns a real PTY; `C-g z` remains
available to zoom it for full-screen tools.

## Shortcuts

The isolated tmux prefix is `C-g`:

- `C-g h` — restore pinned MOTD and focus shell
- `C-g d` — replace upper pane with persistent interactive docs
- `C-g s` — focus the shell input
- `C-g z` — zoom/unzoom the shell for output and full-screen tools
- `C-g m` — command menu popup over the workspace
- `C-g l` — captured shell-init log popup
- `C-g q` — exit the workspace
- `C-g C-g` — forward a literal prefix to the active program

When shell-hook output exists, a compact pane shows it for four seconds and
collapses. The complete log remains available through `C-g l`.

The experiment uses a unique tmux socket and ignores the user's tmux config.
Exiting the child shell tears down the session and returns to the parent shell.

## Verdict

Fill this in after trying it, then either delete the experiment or promote the
validated behavior into a deliberate design.

## Deletion

Remove this directory and the `motdShellExperimentModule` import/export entries
from `flake.nix`.
