// prelude-title interactively chooses a FIGlet title and renders it to stdout
// or an explicit output path for use with prelude.motd.title.text.
package main

import (
	"os"

	"prelude/internal/title"
)

// defaultConfigPath is injected by Nix at link time. The config contains the
// bundled FIGlet font names and their immutable store paths.
var defaultConfigPath string

func main() {
	os.Exit(title.Run(defaultConfigPath, os.Args[1:]))
}
