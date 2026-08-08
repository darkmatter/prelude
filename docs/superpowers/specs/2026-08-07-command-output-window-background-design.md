# Ordinary Command-Output Window Background

**Date:** 2026-08-07  
**Status:** Approved specification.

## Goal

When Prelude owns the terminal window background, ordinary Bash command output should begin with the same canonical static background as the generated Starship prompt and Blesh textarea. Running `echo "hi"` must paint the `h` and `i` cells with Prelude's resolved `window` color rather than the terminal default.

This extends, but does not replace, the contract in `2026-08-07-blesh-window-background-design.md`. The existing textarea, status, ownership, and fallback behavior remains unchanged.

## Scope and guarantees

1. **Ordinary output glyphs.** Plain line-oriented output inherits Prelude's background until the command deliberately emits another background or resets terminal graphics.
2. **Child ownership wins.** Explicit child-process SGR, including `SGR 0`, takes ownership. Prelude does not rewrite it.
3. **No full-width scrolling guarantee.** On non-BCE terminals, scroll-created blank cells can still use the terminal default. This design guarantees glyph backgrounds, not every blank tail in arbitrarily long output.
4. **Shared ownership, attachment-aware activation.** Output adaptation is active only when the MOTD successfully painted a background, Prelude owns the generated prompt, and Blesh is currently attached. Detach never mutates the persistent ownership facts.
5. **No terminal-default mutation.** Do not emit OSC 11 or change emulator profiles.
6. **No output interception.** Do not wrap commands, pipelines, builtins, file descriptors, PTYs, or byte streams.
7. **No command classification.** Full-screen and interactive programs receive the initial background handoff, then normally establish their own terminal state.

## Current-state facts

- `shell-init.nix` resolves one canonical static `window` color and generates a private background-only Blesh face.
- `textarea-background.bash` validates the pinned Blesh APIs, resolves that face to an SGR sequence, and activates only under the shared ownership-and-attachment guard.
- Blesh calls the public `PREEXEC` hook after moving from its editable layout to command execution and before running the Bash command.
- Blesh routes `PREEXEC` hooks to its live TUI output, but an `EXIT` hook can inherit the executing command's stdout redirection. A cleanup sequence written to ordinary stdout can therefore contaminate `exit >file` while failing to reset the terminal.
- Blesh exposes unique prepend registration through `blehook PREEXEC+-=handler`; this preserves other handlers while preventing duplicate Prelude registrations.
- A plain command such as `echo` emits no color sequence. Without a `PREEXEC` handoff, its glyphs use the terminal default even though MOTD and Blesh painted surrounding cells.

## Architecture

```text
Nix backdrop resolver
  └─ canonical window color
       └─ generated Blesh window face
            └─ guarded window-background runtime
                 ├─ textarea render/erase advice (existing)
                 └─ PREEXEC background handoff (new)
                       └─ ordinary command output glyphs

MOTD success + generated prompt ownership + Blesh attached
  └─ shared call-time active guard for both paths
```

The output handoff belongs beside the textarea adapter because both consume the same validated face, SGR value, ownership facts, attachment state, and Blesh compatibility boundary. Do not create a second color resolver or a second ownership state.

## Runtime lifecycle

### Installation

After all existing Blesh source validation and textarea advice installation succeeds:

1. Define one package-private handler, `prelude/window/background/preexec`.
2. Register it with unique-prepend semantics so it runs before existing user `PREEXEC` output without deleting user handlers.
3. Treat hook registration as part of the adapter's transactional install. Keep exact `hook-name=command` records in a separate transaction-local rollback list; a failure removes installed advice through the existing advice list and removes installed hooks with `blehook <name>-=<command>`, leaving stock Blesh behavior intact.
4. Removal must target only Prelude's exact hook command. Never clear or replace a hook array.

The compatibility guard must require the public `blehook` API and a writable live Blesh TUI stdout FD. Register `PREEXEC` with `+-=` and cleanup hooks with `-+=`; those operators remove an exact prior registration before prepending or appending it, so re-sourcing Prelude does not duplicate handlers even though sourcing re-runs installation.

### Command handoff

`prelude/window/background/preexec`:

1. Checks the shared ownership-and-attachment guard at call time.
2. Emits the already-resolved background-only window SGR with Bash's builtin `printf`, explicitly directed to Blesh's validated live TUI stdout FD, without a newline or visible cells.
3. Returns success without changing the command text, standard-stream bindings, `BLE_PIPESTATUS`, or user hook list. Command redirections never receive Prelude SGR bytes.

The handler does not reset foreground, bold, or other graphics attributes itself. Blesh has already left its internal rendering state; the emitted sequence adds only the background required by this contract.

### Return to Blesh

Do not add a `POSTEXEC` reset. User `POSTEXEC` and `ERREXEC` output remains part of ordinary command output and may retain the handoff. Blesh's next owned textarea render already emits its guarded reset/background composition.

Register bounded exit/detach cleanup so a command such as `exit` cannot leave Prelude's ambient SGR active in the parent terminal. Extend the shared call-time guard to require Blesh's existing `_ble_attached` state; Blesh clears it before invoking `DETACH` and restores it before attach-time rendering. The cleanup handlers run after existing user handlers and emit Blesh's captured original SGR reset explicitly to the validated live TUI stdout FD, never inherited command stdout, and only if Prelude previously performed an active handoff. On `DETACH`, the false attachment state makes Blesh's final textarea render take the stock path without permanently clearing MOTD or prompt ownership, so a later `ble-attach` reactivates styling. Both handlers preserve every existing `EXIT` and `DETACH` handler and never alter the terminal default color.

An application that calls `exec`, terminates the shell abnormally, or explicitly controls terminal graphics remains outside the guarantee; Prelude cannot safely interpose without command wrapping or terminal-default mutation.

## Error handling and compatibility

- Missing or incompatible Blesh hook APIs, attachment state, or live TUI output FD fail the same closed installation transaction as incompatible textarea internals.
- An inactive, detached, quiet, failed-MOTD, disabled-MOTD, or custom-Starship session emits no command-output SGR.
- A failed TUI `printf` during terminal teardown is best-effort, writes nothing to inherited command streams, and must not change the command's execution result.
- User `PREEXEC`, `POSTEXEC`, `ERREXEC`, `EXIT`, and `DETACH` handlers remain registered in their original relative order. Prelude uniquely prepends its background handoff and uniquely appends bounded cleanup.
- Relative/blend `windowBackground` modes retain the approved canonical `palette.bg` fallback. There is no runtime MOTD-to-shell color protocol.

## Verification

Extend the existing focused Nix check and terminal harness:

1. **Hook contract.** Assert unique-prepend handoff registration, unique-append cleanup registration, idempotent re-sourcing, exact removal, and preservation/order of pre-existing user hooks.
2. **Activation matrix.** Active literal/`true` attached cases emit the canonical window SGR. Detached, disabled, quiet, failed-MOTD, custom-config, and unsupported-Blesh cases emit nothing.
3. **Exit-status contract.** The handoff does not change successful or failing command statuses, `PIPESTATUS`, or user hook execution.
4. **Redirection isolation.** Execute `exit >file` in the real PTY. Assert the file contains no Prelude SGR bytes and the terminal's final graphics state has returned to the captured default after the handoff.
5. **Real PTY cell state.** In the packaged Bash+Blesh shell, execute `echo "hi"`, feed output into the existing non-BCE pyte screen, and assert the `h` and `i` cells carry the expected truecolor background.
6. **Child reset boundary.** Execute output that emits `SGR 0` followed by further glyphs; on the same non-BCE pyte screen used above, assert those subsequent glyph cells report the terminal-default background rather than the window background, proving Prelude does not intercept or rewrite child output.
7. **Exit/detach cleanup.** Assert that on `EXIT` an active handoff is followed on the live TUI FD by the captured original SGR reset. On `DETACH`/EOF, assert `_ble_attached` is false, Blesh's final textarea render emits no adapter-added window background, and the cleanup reset is not followed by a Prelude window SGR.
8. **Detach/reattach lifecycle.** Detach and reattach in one shell; assert persistent MOTD/prompt ownership is unchanged, the detached render is stock, and the first reattached textarea render resumes the canonical window background.
9. **Source drift.** Extend the pinned `${pkgs.blesh}` guard for the hook API, live TUI FD, attachment-state ordering, exit redirection, and execution-order sentinel used by the handoff.
10. Re-run the focused check, static shell checks, Go gates, and repository Nix checks.

The PTY assertion must inspect terminal cells, not merely search output bytes for `48;2`.

## Definition of done

- `echo "hi"` renders `hi` with the same canonical static background as the owned prompt and textarea.
- Existing user hook handlers still run exactly once.
- Child resets and explicit backgrounds remain untouched.
- Quiet, failed, disabled, custom-config, and compatibility-failure paths emit no handoff.
- Normal exit/detach does not leak an active Prelude SGR into the parent terminal, and reattach restores adaptation without repainting the MOTD.
- Redirected commands, including `exit >file`, receive no Prelude SGR bytes; cleanup reaches the live terminal.
- Existing textarea, status-row, status-cap, and generated Starship behavior remains unchanged.
- Focused behavioral and real-PTY cell-state tests pass.

## Alternatives rejected

- **OSC 11:** complete across resets and scrolling, but mutates the terminal default for child applications and requires terminal-specific restoration.
- **Output rewriting/filtering:** cannot preserve Bash builtin state, pipelines, binary streams, and interactive application behavior safely.
- **Command heuristics:** classifying full-screen or color-aware programs is incomplete and creates surprising per-command behavior.
- **Starship-only styling:** Starship is not active while command output is written.
