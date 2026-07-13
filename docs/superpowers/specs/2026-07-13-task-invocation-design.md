# Concentrated task invocation

## Goal

Put task selection, declared-argument decisions, command assembly, and invocation errors behind one package-local module used by both the direct CLI and Bubble Tea adapters.

Process replacement remains outside this module. The module prepares an invocation; `finish` continues to print the command or replace the process according to `Config.Execute`.

## Module boundary

Add `src/menu-tui/invocation.go` as a pure task-invocation module. It owns:

- exact-name-then-key task resolution;
- the decision to run immediately or collect declared arguments;
- option-chip token construction;
- opaque shell-text normalization and command assembly; and
- errors for unknown selections and blank submissions with required arguments.

Direct resolution and TUI selection return a closed, package-local decision with exactly one variant:

- `Command`, containing the complete normalized shell command; or
- `CollectArguments`, containing the selected task.

Argument completion returns `Command` or an error; it cannot return `CollectArguments`. Errors carry no decision. The Bubble Tea model retains both the command text and a command-present discriminator, so an intentionally empty normalized command is not confused with the absence of an invocation.

`Config.find` and `tokenFor` move from `config.go` into this module. The live preview calls the same command assembler as final submission. `finish` consumes the prepared command verbatim and only prints or executes it; process replacement remains outside the module.

## Adapter responsibilities

`fastPath` passes the selector and extra CLI arguments to the invocation module. `Command` goes to `finish`; `CollectArguments` opens the TUI for the returned task. An error is rendered to configured stderr with the adapter-owned `menu:` prefix, then exits with status 1.

List-mode Enter passes the selected task to the invocation module. `CollectArguments` enters argument mode. `Command` stores the command, marks it present, and quits.

Argument-mode submission passes the selected task and entered shell text to the invocation module. An error is stored in `argErr` without quitting or leaving argument mode. `Command` stores the command, marks it present, and quits.

The TUI may inspect `Task.Args` to render labels, descriptions, and option chips. Adapters and views do not use it to decide invocation behavior or manufacture shell text. The live preview renders the shared assembler's command; its ellipsis remains presentation-only when the entered text is wholly blank.

## Preserved behavior

- Exact task-name lookup precedes accelerator-key lookup, including when one task's name equals another task's key.
- An unknown direct selection is an error.
- A task without declared arguments runs immediately.
- A task with declared arguments and no direct extras requests argument entry.
- Direct extras are detected by slice length. An explicitly supplied empty argument therefore bypasses argument entry.
- Direct extras are joined with one ASCII space, argv boundaries are intentionally discarded, and no quoting or escaping is added.
- Explicit direct extras bypass interactive required-argument validation and produce a command immediately.
- Interactive argument text is opaque shell source. It is not parsed against tokens, arity, options, or required declarations.
- Only wholly blank interactive text is validated. If multiple arguments are required, the first required token in declaration order is reported.
- Blank interactive input is accepted when every declared argument is optional.
- Interactive text and the complete assembled command are trimmed. Direct argument elements are otherwise preserved through the one-space join.
- The assembled command remains shell text for the existing `bash -c` execution boundary.
- Printing versus process replacement remains in `finish`.

## Error behavior

The invocation module returns an unknown-task error containing the rejected selector and no `menu:` prefix. `fastPath` adds the styled prefix, writes the error to configured stderr, and exits with status 1.

Interactive missing-required-argument errors contain the task name and first required token in declaration order. The TUI stores the message in `argErr`, remains in argument mode, and does not emit `tea.Quit`.

## Validation

Use test-driven development for the module interface. Focused module tests cover exact-name precedence over another task's key, key selection, unknown selection, immediate invocation, argument collection, equivalence between direct-no-extra and TUI-begin decisions, explicitly empty direct arguments, verbatim one-space joining without quoting, required blank interactive input, first-required-token ordering, optional blank input, opaque interactive shell text, trimming, option-token construction, and empty normalized commands.

Focused adapter tests cover list Enter dispatching to argument mode versus command-and-quit, invalid submission remaining in argument mode, valid submission quitting with a command-present decision, and live-preview command text matching final submission. A focused CLI subprocess test covers unknown-selector stderr and exit status 1.

Run the focused Go package tests after every red-green-refactor cycle. Preserve unrelated worktree changes to the Bubble Tea window background behavior.
