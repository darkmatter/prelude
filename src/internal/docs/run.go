package docs

import (
	"flag"
	"fmt"
	"os"
)

// Run is the binary entry point. The Nix wrapper passes the config with
// --config; PRELUDE_DOCS_CONFIG supplies it when the binary runs unwrapped.
func Run() {
	configPath := flag.String("config", os.Getenv("PRELUDE_DOCS_CONFIG"), "path to the docs config JSON")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "docs:", err)
		os.Exit(1)
	}

	if args := flag.Args(); len(args) > 0 {
		env := printEnv{
			statePath:  defaultPagerStatePath(),
			configPath: *configPath,
			stdout:     os.Stdout,
			stderr:     os.Stderr,
			environ:    os.Environ(),
		}
		if err := runPrint(cfg, args, env); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := View(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "docs:", err)
		os.Exit(1)
	}
}
