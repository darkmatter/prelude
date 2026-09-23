// prelude-docs: Markdown project docs with a CONTENTS sidebar.
//
//	docs --config cfg.json
//	docs 2 20
//	docs next
//
// Content comes from Markdown files declared in prelude.docs.pages; this binary
// renders and navigates them. Digits 1–9 jump to pages; j/k scroll; q quits.
package main

import (
	"prelude/internal/docs"
)

// Configuration arrives at run time (--config from the Nix wrapper), never at
// link time, so editing a page does not recompile the viewer.
func main() {
	docs.Run()
}
