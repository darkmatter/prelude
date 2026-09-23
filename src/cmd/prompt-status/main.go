// prelude-prompt-status exposes cached local-server health to the shell.
// `--cached` is pure; `--refresh` performs a due-only probe and is suitable for
// detached lifecycle calls from shell PRECMD hooks.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"prelude/internal/promptstatus"
)

// The shell passes the descriptor with --config, so one build serves every
// local-server configuration.
func main() {
	configPath := flag.String("config", os.Getenv("PRELUDE_PROMPT_STATUS_CONFIG"), "path to the generated prompt-status descriptor")
	cached := flag.Bool("cached", false, "read only the persisted health result")
	refresh := flag.Bool("refresh", false, "refresh the result only when it is due")
	flag.Parse()
	if *configPath == "" {
		fatal("descriptor path is empty")
	}
	if *cached && *refresh {
		fatal("--cached and --refresh are mutually exclusive")
	}

	var (
		record promptstatus.Record
		err    error
	)
	if *refresh {
		record, err = promptstatus.RefreshDue(*configPath, time.Now)
	} else {
		// Cached read is the safe default as well as the explicit shell mode.
		record, err = promptstatus.Read(*configPath, time.Now())
	}
	if err != nil {
		fatal(err.Error())
	}
	fmt.Println(record.Line())
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, "prompt-status:", message)
	os.Exit(1)
}
