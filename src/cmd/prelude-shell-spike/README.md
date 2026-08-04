# prelude-shell-spike

**Throwaway** prototype of devenv-style shell host with:

1. Bottom status row
2. Configurable **shell strip** height
3. **Pinned panel** above the shell, cycled with **Ctrl+G**

## Layout

```text
┌──────────────── pin (motd / menu list / docs placeholder) ────────────────┐
│  (total − shell − 1 rows when pin is on; 0 when pin off)                  │
├──────────────── shell PTY ( -shell-rows when pin on; else fill ) ─────────┤
│  $  …                                                                      │
├──────────────── status ───────────────────────────────────────────────────┤
│  pin:motd · shell:10row · ^G cycle …                                      │
└───────────────────────────────────────────────────────────────────────────┘
```

## Run

```bash
cd /Users/cm/git/darkmatter/prelude
direnv reload

cd src

# Spike 1 — shell keeps 10 rows; pin uses the rest
go run ./cmd/prelude-shell-spike
go run ./cmd/prelude-shell-spike -shell-rows 10

# Spike 2 — shell keeps 1 row; almost full terminal for pin
go run ./cmd/prelude-shell-spike -shell-rows 1
```

| Key           | Action                                        |
| ------------- | --------------------------------------------- |
| **Ctrl+G**    | Cycle pin: **off → motd → menu → docs → off** |
| exit / Ctrl-D | Leave inner shell (ends spike)                |

## What each pin shows (spike fidelity)

| Pin      | Content                                             |
| -------- | --------------------------------------------------- |
| **off**  | Shell expands to full height minus status           |
| **motd** | Snapshot via `motd` on a sized PTY (best-effort)    |
| **menu** | Snapshot via `x --list` / `menu -x --list`          |
| **docs** | Placeholder (interactive docs needs VT embed later) |

## Tests

```bash
cd src && go test ./cmd/prelude-shell-spike/ -count=1
```

## Why the pin was blank

Scroll regions do **not** remap absolute CUP. The shell still emits `ESC[1;1H`
for its home corner; on the host that is **physical row 1** (the pin), so
starship redraws wipe the panel. Cursor jumps, upper area stays empty.

**This iteration:** remap child CUP/VPA/clear into the shell strip (`remap.go`);
paint pin as plain absolute lines (ANSI stripped).

## Validated so far

| Issue                         | Mitigation                      |
| ----------------------------- | ------------------------------- |
| Prompt on status line         | DECSTBM + PTY height            |
| CSI garbage (`135m`)          | `escCoalescer`                  |
| +2 row gap                    | compact starship `format`       |
| Pin blank / cursor jump on ^G | **CSI remap** + plain pin paint |
| Pin layout                    | `-shell-rows` + ^G cycle        |

## Still open

- Full interactive menu/docs **inside** the pin (needs embed / nested VT)
- FIGlet missing-glyph (font) — later
- Edge-case CSI not covered by remap (true VT still wins long-term)
- Direnv log ordering (orthogonal)
