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

// defaultConfigPath is injected by Nix at link time, matching cmd/menu: config
// lives in a data file so one Go renderer serves every project.
var defaultConfigPath string

func main() {
	portal.Run(defaultConfigPath)
}
