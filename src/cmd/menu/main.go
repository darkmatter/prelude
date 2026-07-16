// prelude-menu: an interactive devshell command menu, ported from the
// cli-menu-design demo's command palette.
//
//	prelude-menu --config cfg.json               interactive picker
//	prelude-menu --config cfg.json list          print the task table
//	prelude-menu --config cfg.json help          sectioned manual viewer
//	prelude-menu --config cfg.json <name|key> …  run a task directly
//
// Tasks with declared args open argument-entry mode (option chips, boolean
// flags, required validation, live preview) unless extra CLI args are given.
// The selected command is exec'd via bash -c (or printed when execute=false).
package main

import (
	"prelude/internal/menu"
)

// defaultConfigPath is injected by Nix at link time. Keeping configuration in
// a data file preserves one Go renderer without reintroducing a shell wrapper.
var defaultConfigPath string

func main() {
	menu.Run(defaultConfigPath)
}
