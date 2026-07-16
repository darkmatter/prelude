package motd

import (
	"os"

	"golang.org/x/term"
)

// StatusResolver resolves live header-status checks for one render session.
// It owns the runtime dependency and deliberately mutates the provided
// configuration with resolved status values; it is not an interactive model.
type StatusResolver struct {
	runtime Runtime
}

// Resolve runs live checks for header badges, optionally showing a MiniDot
// spinner on stderr while each check runs. Static badges (no check) are left
// as-is with Level "static".
func (s StatusResolver) Resolve(cfg *Config) []string {
	items := cfg.Header.Status
	if len(items) == 0 {
		return nil
	}
	var diagnostics []string

	interactive := term.IsTerminal(int(os.Stderr.Fd()))
	// Resolve sequentially so the spinner line stays readable; checks are
	// usually cheap shell probes.
	for i := range items {
		item := &items[i]
		if item.Check == "" {
			if item.Status != "" || item.Label != "" {
				item.Level = "static"
			}
			continue
		}
		if item.Async {
			continue
		}

		label := item.Label
		if label == "" {
			label = "check"
		}

		var stop func()
		if interactive {
			stop = Spinner{}.Render(os.Stderr, label)
		}
		ok, out := s.runtime.Check(item.Check)
		if stop != nil {
			stop()
		}
		diagnostics = append(diagnostics, resolveStatusItem(item, ok, out)...)
	}
	cfg.Header.Status = items
	return diagnostics
}

func resolveStatusItem(item *HeaderStatus, ok bool, out string) []string {
	var diagnostics []string
	if ok {
		item.Level = "success"
		switch item.Output {
		case "light":
			item.Status = ""
		default:
			switch {
			case item.Ok != "":
				item.Status = item.Ok
				if out != "" {
					diagnostics = append(diagnostics, out)
				}
			case out != "":
				item.Status = firstLine([]byte(out))
			default:
				item.Status = "ok"
			}
		}
		return diagnostics
	}

	item.Level = "error"
	if item.FailLevel == "warning" {
		item.Level = "warning"
	}
	switch item.Output {
	case "light":
		item.Status = ""
	default:
		switch {
		case item.Fail != "":
			item.Status = item.Fail
			if out != "" {
				diagnostics = append(diagnostics, out)
			}
		case out != "":
			item.Status = firstLine([]byte(out))
		default:
			item.Status = "fail"
		}
	}
	return diagnostics
}
