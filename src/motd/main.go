// prelude-motd renders the static devshell greeting from normalized JSON.
// Nix owns configuration; this command owns terminal probing and presentation.
package main

import (
	"flag"
	"fmt"
	"os"

	"prelude/shared"
)

// defaultConfigPath is injected by Nix at link time. Keeping configuration in
// a data file preserves one Go renderer without reintroducing a shell wrapper.
var defaultConfigPath string

func main() {
	configPathDefault := os.Getenv("PRELUDE_MOTD_CONFIG")
	if configPathDefault == "" {
		configPathDefault = defaultConfigPath
	}
	configPath := flag.String("config", configPathDefault, "path to the MOTD config JSON")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "motd:", err)
		os.Exit(1)
	}

	runtime := systemRuntime{}
	terminal := systemRenderTerminal{output: os.Stdout}
	output := shared.ColorWriter(os.Stdout, os.Environ(), cfg.ColorProfile)
	if _, err := fmt.Fprint(output, renderSession(cfg, runtime, terminal)); err != nil {
		fmt.Fprintln(os.Stderr, "motd:", err)
		os.Exit(1)
	}
}
