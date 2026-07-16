// prelude-motd renders the static devshell greeting from normalized JSON.
// Nix owns configuration; this command owns terminal probing and presentation.
package main

import (
	"prelude/internal/motd"
)

// defaultConfigPath is injected by Nix at link time. Keeping configuration in
// a data file preserves one Go renderer without reintroducing a shell wrapper.
var defaultConfigPath string

func main() {
	motd.Run(defaultConfigPath)
}
