package motd

import (
	"flag"
	"fmt"
	"os"

	"prelude/pkg/shared"
)

// Run is the binary entry point. defaultConfigPath is injected by Nix at link
// time via ldflags; it acts as the fallback when PRELUDE_MOTD_CONFIG is unset.
func Run(defaultConfigPath string) {
	configPathDefault := os.Getenv("PRELUDE_MOTD_CONFIG")
	if configPathDefault == "" {
		configPathDefault = defaultConfigPath
	}
	configPath := flag.String("config", configPathDefault, "path to the MOTD config JSON")
	refreshStatus := flag.Bool("refresh-status", false, "refresh cached async status checks")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "motd:", err)
		os.Exit(1)
	}

	runtime := systemRuntime{}
	store, cacheErr := newStatusCacheStore(*configPath, cfg.Project)
	if *refreshStatus {
		if cacheErr != nil {
			fmt.Fprintln(os.Stderr, "motd:", cacheErr)
			os.Exit(1)
		}
		if err := refreshAsyncStatuses(cfg, runtime, store); err != nil {
			fmt.Fprintln(os.Stderr, "motd:", err)
			os.Exit(1)
		}
		return
	}

	terminal := systemRenderTerminal{output: os.Stdout}
	output := shared.ColorWriter(os.Stdout, os.Environ(), cfg.ColorProfile)
	rendered := renderSession(cfg, runtime, terminal, store)
	if _, err := fmt.Fprint(output, rendered); err != nil {
		fmt.Fprintln(os.Stderr, "motd:", err)
		os.Exit(1)
	}
	// Start only after foreground output is fully written. The child owns no
	// inherited terminal descriptors, so shell startup never waits for it.
	if cacheErr == nil && hasAsyncStatuses(cfg) {
		_ = startAsyncRefresh(*configPath)
	}
}
