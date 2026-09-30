package menu

// Library entry points for hosts that embed the menu in their own process
// (libprelude, which the TypeScript API loads through bun:ffi). Unlike Run
// they never parse flags, exit, or exec: the host owns the process and runs
// whatever the picker selects, whether a shell command or its own function.

import (
	"io"

	"prelude/pkg/shared"
)

// Selection is what the picker resolved. Name is the chosen task's catalogue
// name and Line the argument text entered for it ("" when none). Command is
// the shell form the CLI would exec: the task's run text plus Line.
type Selection struct {
	Name    string `json:"name"`
	Line    string `json:"line"`
	Command string `json:"command"`
}

// ParseConfig decodes one strict menu Config JSON value and applies the same
// defaults and runtime Justfile import as the CLI's --config path.
func ParseConfig(raw []byte) (*Config, error) {
	cfg, err := shared.DecodeJSON[Config](raw)
	if err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	importJust(cfg)
	return cfg, nil
}

// Select resolves args exactly like `x` and opens the picker only where input
// is still needed: the root list for no args, argument entry for a task that
// declares arguments but received none, or a module's subcommand picker. It
// returns nil when the user leaves the picker without choosing.
func Select(cfg *Config, args []string) (*Selection, error) {
	st := newStyles(cfg)
	if len(args) == 0 {
		return runPicker(cfg, newPicker(cfg, st, nil, nil))
	}
	decision, err := resolveXInvocation(cfg, args)
	if err != nil {
		return nil, err
	}
	switch decision.kind {
	case collectArgumentsInvocation:
		return runPicker(cfg, newPicker(cfg, st, &decision.task, nil))
	case collectSubcommandInvocation:
		return runPicker(cfg, newPicker(cfg, st, nil, &decision.task))
	default:
		return decision.selection(), nil
	}
}

// List writes the non-interactive command table (`x --list`) at width into w;
// width <= 0 falls back to the CLI's 80 columns. Colors are downgraded for
// terminal, the host's stdout, which is where the caller prints the result.
func List(w, terminal io.Writer, environ []string, cfg *Config, width int) {
	if width <= 0 {
		width = 80
	}
	writeList(shared.ColorWriterFor(w, terminal, environ, cfg.ColorProfile), cfg, newStyles(cfg), width)
}
