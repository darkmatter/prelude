// prelude-motd renders the static devshell greeting from normalized JSON.
// Nix owns configuration; this command owns terminal probing and presentation.
package main

import (
	"prelude/internal/motd"
)

// Configuration arrives at run time (--config from the Nix wrapper), never at
// link time, so one build serves every MOTD configuration.
func main() {
	motd.Run()
}
