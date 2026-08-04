package motd

import (
	"testing"
	"time"
)

func TestCheckCommandWithTimeoutKillsProcessGroup(t *testing.T) {
	started := time.Now()
	ok, _ := CheckCommandWithTimeout("sleep 1 & wait", 20*time.Millisecond)
	if ok {
		t.Fatal("timed-out check succeeded")
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("timed-out check ran for %s", elapsed)
	}
}
