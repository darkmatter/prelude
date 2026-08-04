package main

import _ "embed"

// preludeBleScript is the version-controlled ble.sh integration sourced by the
// child shell after Starship/ble.sh setup. Keeping it as a real file makes the
// shell glue reviewable and shellcheckable; go:embed carries it into the binary
// so the host can write it next to the generated catalog fragment in the rc dir.
//
//go:embed ble/prelude-ble.sh
var preludeBleScript string
