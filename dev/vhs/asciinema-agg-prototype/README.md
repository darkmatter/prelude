# PROTOTYPE — asciinema + agg docs recording

Question: can a source-controlled script re-record a live Prelude demo with
`asciinema` and produce a crisper GIF through `agg` than the current VHS
browser renderer?

Run from anywhere inside the repository:

```sh
./dev/vhs/asciinema-agg-prototype/run.sh
```

The command builds the root `motd` package backed by `prelude.nix`
(`theme = "minted"`, project `prelude`) and writes disposable comparison
artifacts to `output/`:

- `output/motd.cast`
- `output/motd.gif`
- `output/motd.png`

The prototype intentionally covers the non-interactive MOTD first. If its
rendering is worth keeping, the next experiment is automating the menu TUI via
a small PTY/Expect driver.

## Verdict

Fill this in after comparing the generated image with `docs/media/motd.gif`,
then delete the prototype or fold the chosen approach into `docs-record`.
