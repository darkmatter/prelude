# This shell

## Layout

- `src/` — Go renderers: Lip Gloss for the static MOTD, Bubble Tea for the menu and this viewer
- `src/prelude/` — the flake-parts modules and option declarations
- `examples/default/` — the configuration this shell is built from
- `examples/reference/` — a complete, copyable consumer flake
- `docs/` — guides, generated references, and recorded showcases

## Workflow

Edit, then verify — from the repo root:

```sh
x go:test    # Go unit tests (dispatches to go test -C src ./...)
x go:vet     # Go vet over src/
x fmt        # format Nix sources
x check      # nix flake check
```

Changed anything user-visible? Regenerate the docs:

```sh
x sync-docs     # regenerate option + showcase markdown
x record-docs   # re-record stale VHS showcases, then sync
```

The complete command key is the public interface: `x go:test` appears as `test`
under the inferred `go` group and dispatches to canonical `go test`. Prelude
adds no punctuation-derived executable such as `go-test`.
