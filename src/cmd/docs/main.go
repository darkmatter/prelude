// prelude-docs: Markdown project docs with a CONTENTS sidebar.
//
//	docs --config cfg.json
//
// Content comes from Markdown files declared in prelude.docs.pages; this binary
// renders and navigates them. Digits 1–9 jump to pages; j/k scroll; q quits.
package main

import (
	"prelude/internal/docs"
)

// defaultConfigPath is injected by Nix at link time. Keeping configuration in
// a data file preserves one Go renderer without reintroducing a shell wrapper.
var defaultConfigPath string

func main() {
	docs.Run(defaultConfigPath)
}
