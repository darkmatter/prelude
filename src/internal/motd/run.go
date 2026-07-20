package motd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"prelude/pkg/shared"
)

// Run is the binary entry point. defaultConfigPath is injected by Nix at link
// time via ldflags; it acts as the fallback when PRELUDE_MOTD_CONFIG is unset.
//
// Flow (E1): Preflight-if-needed → pure Render. Flags:
//
//	--preflight-only  write Cache only (blocking+async due, or --async subset)
//	--async           with --preflight-only, only async status entries
//	--pure            skip Preflight; Render from Config+Cache files only
func Run(defaultConfigPath string) {
	configPathDefault := os.Getenv("PRELUDE_MOTD_CONFIG")
	if configPathDefault == "" {
		configPathDefault = defaultConfigPath
	}
	configPath := flag.String("config", configPathDefault, "path to the MOTD config JSON")
	preflightOnly := flag.Bool("preflight-only", false, "run Preflight and write Cache without painting")
	asyncOnly := flag.Bool("async", false, "with --preflight-only, refresh only async status entries")
	pure := flag.Bool("pure", false, "skip Preflight; render from Config and Cache only")
	// Legacy alias kept for one release so existing wrappers keep working.
	refreshStatus := flag.Bool("refresh-status", false, "deprecated: use --preflight-only --async")
	flag.Parse()

	if *refreshStatus {
		*preflightOnly = true
		*asyncOnly = true
	}
	if os.Getenv("PRELUDE_MOTD_PURE") == "1" {
		*pure = true
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "motd:", err)
		os.Exit(1)
	}

	runtime := systemRuntime{}
	store, cacheErr := newCacheStore(*configPath, cfg.Project)
	terminal := systemRenderTerminal{output: os.Stdout}

	if *preflightOnly {
		if cacheErr != nil {
			fmt.Fprintln(os.Stderr, "motd:", cacheErr)
			os.Exit(1)
		}
		mode := PreflightAll
		if *asyncOnly {
			mode = PreflightAsync
		}
		if err := Preflight(cfg, store, runtime, terminal, mode, !*asyncOnly); err != nil {
			fmt.Fprintln(os.Stderr, "motd:", err)
			os.Exit(1)
		}
		return
	}

	if !*pure && cacheErr == nil {
		if err := Preflight(cfg, store, runtime, terminal, PreflightBlocking, true); err != nil {
			// Non-fatal: still paint with whatever cache we have (P1).
			fmt.Fprintln(os.Stderr, "motd: preflight:", err)
		}
	}

	cache := Cache{Entries: map[string]CacheEntry{}}
	if cacheErr == nil {
		cache = store.loadOrEmpty()
	}

	input := RenderInput{Config: cfg, Cache: cache}
	if *pure {
		// Pure: size only from cache or defaults inside Render.
	} else {
		// Prefer fresh terminal geometry when available.
		input.TerminalWidth = terminal.Width()
		input.TerminalHeight = terminal.Height()
	}

	output := shared.ColorWriter(os.Stdout, os.Environ(), cfg.ColorProfile)
	if _, err := fmt.Fprint(output, Render(input)); err != nil {
		fmt.Fprintln(os.Stderr, "motd:", err)
		os.Exit(1)
	}

	// Detached async preflight after stdout is fully written (B1).
	if !*pure && cacheErr == nil && hasDueAsync(cfg, cache, store.now) {
		_ = startAsyncPreflight(*configPath)
	}
}

func startAsyncPreflight(configPath string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer null.Close()

	cmd := detachedPreflightCommand(executable, configPath, null)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func detachedPreflightCommand(executable, configPath string, null *os.File) *exec.Cmd {
	cmd := exec.Command(executable, "--preflight-only", "--async", "--config", configPath)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = null, null, null
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}
