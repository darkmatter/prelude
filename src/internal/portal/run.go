package portal

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"prelude/pkg/shared"
)

// Run is the portal entrypoint.
//
//	portal                     terminal launcher
//	portal --serve             local web launcher on the configured address
//	portal --serve :9000       …on an explicit address
//	portal --list              print every app/environment and its status
//
// Two front ends over one core: the terminal view shows one environment per
// app with a selector, the web view shows the whole grid at once.
func Run(defaultConfigPath string) {
	args := os.Args[1:]
	configPath := defaultConfigPath
	serve := false
	listen := ""
	list := false

	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "-h", "--help":
			usage()
			return
		case "--config":
			if index+1 >= len(args) {
				fail("--config needs a path")
			}
			configPath = args[index+1]
			index++
		case "--serve":
			serve = true
			// An address is optional, but must not swallow the next flag.
			if index+1 < len(args) && !isFlag(args[index+1]) {
				listen = args[index+1]
				index++
			}
		case "--list":
			list = true
		default:
			fail(fmt.Sprintf("unknown argument %q", args[index]))
		}
	}

	if configPath == "" {
		fail("no config: pass --config <path>")
	}
	cfg, err := shared.LoadJSON[Config](configPath)
	if err != nil {
		fail(err.Error())
	}

	switch {
	case list:
		printList(*cfg)
	case serve:
		// Ctrl-C must shut the listener down rather than leave a port held by
		// a detached process — this is a devshell tool, run and killed often.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := Serve(ctx, *cfg, listen); err != nil {
			fail(err.Error())
		}
	default:
		if err := RunTUI(*cfg); err != nil {
			fail(err.Error())
		}
	}
}

func printList(cfg Config) {
	statuses := NewProber(cfg.Timeout()).ProbeAll(context.Background(), cfg.Apps)
	for _, app := range cfg.Apps {
		for _, env := range app.Environments {
			status := statuses[Key(app.Name, env.Name)]
			fmt.Printf("%-6s %-10s %-8s %-28s %s\n",
				status.State, app.Name, env.Name, env.URL, detailText(status))
		}
	}
}

func isFlag(value string) bool {
	return len(value) > 1 && value[0] == '-'
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: portal [--config path]            terminal launcher")
	fmt.Fprintln(os.Stderr, "       portal --serve [addr]             local web launcher")
	fmt.Fprintln(os.Stderr, "       portal --list                     print statuses and exit")
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "portal:", message)
	os.Exit(1)
}
