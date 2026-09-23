// prelude-portal: a launcher for a project's apps.
//
//	portal                 terminal launcher — one row per app, environment
//	                       selector, live health light, clickable URL
//	portal --serve         the same catalogue as a local web page
//	portal --list          statuses on stdout, for scripts and CI
//
// Both front ends read one Nix-generated catalogue and share one prober, so
// they cannot disagree about whether something is up.
package main

import (
	"prelude/internal/portal"
)

// Configuration arrives at run time (--config from the Nix wrappers), never at
// link time, so one build serves every app catalogue.
func main() {
	portal.Run()
}
