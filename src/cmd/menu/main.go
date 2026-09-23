// prelude-menu: an interactive devshell command menu, ported from the
// cli-menu-design demo's command palette.
//
//	prelude-menu --config cfg.json               interactive picker
//	prelude-menu --config cfg.json --x <key> …   dispatch through the public x catalogue
//	prelude-menu --config cfg.json --x --list    print the command table
//
// Bare `menu` opens the picker only. Execution and listing go through `--x`
// (the public `x` wrapper). Tasks with declared args open argument-entry mode
// (option chips, boolean flags, required validation, live preview) unless
// extra CLI args are given. The selected command is exec'd via bash -c (or
// printed when execute=false).
package main

import (
	"prelude/internal/menu"
)

// Configuration arrives at run time (--config from the Nix wrappers), never at
// link time, so one build serves every command catalogue.
func main() {
	menu.Run()
}
