# Docs Print Mode

**Date:** 2026-08-15\
**Status:** Draft design

## Goal

Give `docs` a non-alt-screen print surface so a page window can sit above the live prompt. The existing `docs` TUI is unchanged. Print mode writes one terminal-fitting slice of a docs leaf and keeps just enough pager state for `docs next` / `docs prev`.

## Decisions

1. **Same binary, args select the surface.** Bare `docs` stays the Bubble Tea alt-screen viewer. Any positional argument takes the print path and never constructs a `tea.Program`.
2. **Page is a 1-based leaf index.** Leaves are walked depth-first. Groups are not pages. `next` and `prev` are reserved tokens, not titles.
3. **Offset is a wrapped-line index.** Default `0`. It indexes the already-wrapped render, not raw Markdown.
4. **Book-style page crossing.** `next` past a leaf’s last window opens the next leaf at offset 0. `prev` before 0 opens the previous leaf’s last window. First/last leaf clamp.
5. **Missing state starts at page 1, offset 0.** Both `docs next` and `docs prev` with no usable state print that first window.
6. **State records the printed range, not a cursor plus height.** Persist `{page, start, end, width, config}` so `next` starts at the exclusive `end` and a resize can rebase line indices.
7. **No chrome.** Print emits content lines only. No sidebar, status bar, or pager trailer.

## CLI

```text
docs                         # existing TUI
docs --config PATH           # existing TUI with explicit bundle
docs <page> [offset]         # print one window
docs next                    # print the next window from saved state
docs prev                    # print the previous window from saved state
```

`--config` / `PRELUDE_DOCS_CONFIG` / the Nix-injected default still choose the bundle for every surface.

Rules:

- `<page>` is a positive integer. `<offset>` is a non-negative integer; omitted means `0`.
- `docs 2` ≡ `docs 2 0`.
- Extra tokens (`docs 1 0 junk`) are a usage error.
- A non-integer page that is not `next`/`prev` is a usage error. `docs foo` no longer opens the TUI.
- `x docs …` already forwards extra argv through the catalogue dispatcher; no new catalogue args.

Errors go to stderr and exit 1:

| Condition | Message shape |
| --- | --- |
| unknown token / extra args | `docs: usage: docs [<page> [<offset>] | next | prev]` |
| page `< 1` or `> leaf count` | `docs: page N out of range (1-M)` |
| negative / non-integer offset | `docs: usage: …` |
| config load failure | existing `docs: …` loader errors |

## Viewport

Terminal size comes from `term.GetSize(stdout)`. Fallback when that fails: 80×24 (same as `manual.New`).

```text
wrapWidth    = max(min(cols, 96), 16)
windowHeight = max(rows - 2, 1)
```

`96` is the existing TUI body soft cap (`manual.computeLayout` `textW`). Print has no sidebar, so it does not subtract sidebar/scrollbar. The 2-row height margin leaves room for the following prompt so the first printed line is not immediately pushed into scrollback.

Print never uses Bubble Tea window-size messages. Tests inject `cols`/`rows`.

## Renderer

Print reuses the TUI Markdown wrap (glamour / root-README hero+HTML), but it is not a screenshot of the TUI body.

Do **not** call `Viewer.View()` (that forces `AltScreen`). Do **not** reuse `renderLeaf` verbatim:

- The TUI seeds a painted blank row (`styles.blankLine`) so the viewport never holds a raw empty string. Print drops that seed; offset `0` is the first content line.
- The TUI `fillLine`s every row to `textW` with `bodySpace` background. Print is foreground-only so the live prompt and terminal default stay visible under and after the slice.

`pkg/manual` exports a leaf renderer used by the print surface:

```go
// RenderLeafLines paints one 1-based leaf as wrapped content lines.
// No seed blank, no body-fill pad, no sidebar/status chrome.
func RenderLeafLines(doc Document, palette shared.Palette, page int, width int) ([]string, error)
```

`internal/docs` owns CLI dispatch, leaf counting, window slicing, pager transitions, and state I/O.

Output:

- Slice `lines[start:min(start+windowHeight, len(lines))]` to stdout.
- Route through `shared.ColorWriter` so a TTY keeps color and a pipe strips ANSI (same as `x --list`).
- Trailing newline after the last printed line. No filler blank rows to pad a short last window.

An explicit `docs <page> <offset>` past the last line clamps `start` to that leaf’s last window (`max(0, len(lines)-windowHeight)`). It does not cross pages.

## Pager transitions

Let `H = windowHeight` at the current terminal. Let `lines` be the current leaf rendered at the current `wrapWidth`.

### `docs <page> [offset]`

1. Resolve the leaf. Out of range is an error (no clamp, no wrap).
2. `start = min(offset, lastWindowStart)` where `lastWindowStart = max(0, len(lines)-H)`.
3. Print `[start, start+H)`.
4. Write state.

### `docs next`

1. Load state. Missing / unreadable / config-path mismatch → virtual state `{page:1, start:0, end:0, width: current}`.
2. Rebase `end` to the current width (see below). That value is the candidate `start`.
3. If `start >= len(lines)`:
   - next leaf exists → `page++`, `start = 0`
   - else → `start = lastWindowStart` (clamp)
4. Print and write state.

### `docs prev`

1. Same empty-state rule as `next` (print page 1, offset 0).
2. Rebase stored `start` to the current width. That value is the exclusive end of the window just shown.
3. If that cursor is `<= 0`:
   - previous leaf exists → `page--`, `start = lastWindowStart` of the new leaf
   - else → `start = 0`
4. Else `start = max(0, cursor-H)`.
5. Print and write state.

`next` then `prev` on an unchanged terminal returns the previous window. Crossing a page boundary is the one exception: `next` onto leaf N+1 at 0, then `prev`, returns leaf N’s last window, not the exact pre-crossing window, if that last window is shorter than `H`.

## State

JSON file, one pager per tty:

```json
{
  "page": 2,
  "start": 20,
  "end": 42,
  "width": 80,
  "config": "/nix/store/…/config.json"
}
```

- `page`: 1-based leaf that was printed.
- `start`: inclusive wrapped-line index of the printed window.
- `end`: exclusive wrapped-line index (`start + printed line count`).
- `width`: `wrapWidth` used to produce those lines.
- `config`: absolute path of the bundle. A different project/bundle is treated as empty state.

Path:

```text
$XDG_RUNTIME_DIR/prelude/docs-print-<tty>
```

Fallback when `XDG_RUNTIME_DIR` is unset:

```text
$TMPDIR/prelude-docs-print-<uid>-<tty>
```

`<tty>` is `filepath.Base` of the stdout tty name, or `notty` when stdout is not a tty. Create the `prelude` directory with `0700`. Write atomically (temp + rename). Tests inject the path.

Do not persist `H`. Height is always taken from the current terminal. Continuity across height changes is `end` (for `next`) and `start` (for `prev`), not `start+oldH`.

## Resize rebasing

Wrapped-line offsets are only comparable at the same `width`.

When the current `wrapWidth` equals the stored `width`, use stored `start`/`end` as-is.

When it differs:

1. Render the stored page at the stored width → `oldN` lines.
2. Render it at the current width → `newN` lines.
3. Map an old index `i` with `floor(i * newN / oldN)`, or `0` when `oldN == 0`.
4. Clamp to `[0, newN]`.

`docs next` rebases `end`. `docs prev` rebases `start`. Explicit `docs <page> [offset]` ignores stored indices and renders at the current width.

A height-only change (width unchanged) needs no rebase: `next` already starts at stored `end`.

## Architecture

```text
docs.Run
  ├─ no positionals → tea.Program (unchanged)
  └─ positionals    → printMode
        ├─ parse args
        ├─ flatten leaves
        ├─ manual.RenderLeafLines
        ├─ slice window
        ├─ ColorWriter → stdout
        └─ atomic state write
```

Locality:

- `src/internal/docs/run.go` — dispatch after `flag.Parse`.
- `src/internal/docs/print.go` — parse, viewport, transitions, I/O.
- `src/internal/docs/print_test.go` — contract tests with injected size/state/writers.
- `src/pkg/manual` — exported `RenderLeafLines` (+ leaf-count helper if flattening stays presentation-side).

Nix (`src/prelude/docs.nix`, command catalogue) does not change. `x docs next` already works because extra argv is forwarded.

## Testing

Interface-driven tests in `src/internal/docs`. Inject writers, size, and state path. Use `ansi.Strip` when asserting content. Do not start Bubble Tea.

1. **Arg parse.** `[]`, `["2"]`, `["2","20"]`, `["next"]`, `["prev"]`, `["foo"]`, `["2","x"]`, `["1","0","junk"]`, `["0"]`, `["-1"]`.
2. **Leaf index.** Nested groups: page 1 is the first depth-first leaf; groups are skipped.
3. **Window slice.** Given known wrapped lines, `offset=20` and `rows=24` prints 22 lines starting at 20. Width wrap is visible (a long line becomes multiple lines; offset steps those).
4. **Clamp.** Offset past end on an explicit page prints the last window and does not change page.
5. **Empty state.** No file / bad JSON / other `config` → `next` and `prev` both print page 1, offset 0.
6. **Next/prev.** Mid-page step by `H`. End of leaf N → leaf N+1 at 0. Start of leaf 1 → stay at 0. Start of leaf N>1 → leaf N-1 last window. Last leaf `next` clamps.
7. **Resize.** State `{start:20,end:42,width:80}` then `cols=40`: `next` rebases 42, does not print `20+newH`. Width-stable height change uses stored `end` unchanged.
8. **TUI untouched.** Zero positionals still reach `tea.NewProgram` (assert by keeping that branch free of print types; no need to run the program).

`pkg/manual` tests for `RenderLeafLines`: wrapped content matches the TUI body for a fixture leaf after dropping the seed blank and without body-fill pad, including a root-README fixture.

## Non-goals

- No page-title lookup, slugs, or path selectors.
- No print chrome (page number, progress, “more”).
- No catalogue argument chips for `page`/`offset`.
- No change to alt-screen TUI keys, layout, or mouse behavior.
- No Nix option, generated `docs/reference/options.md`, or media refresh.
- No shared pager across ttys or sessions after reboot (`XDG_RUNTIME_DIR` / tmp is enough).

## Verification

- `go test ./internal/docs ./pkg/manual`
- Smoke: `docs 1 0` prints a window and returns to the prompt; `docs next` advances; `docs prev` returns; bare `docs` still opens the TUI.
- Do not run formatters, `nix fmt`, or `nix flake check` unless a later implementation plan says so.
