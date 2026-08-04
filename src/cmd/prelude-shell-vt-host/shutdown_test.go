package main

import (
	"testing"
	"time"
)

// awaitReturn fails the test unless fn returns within grace. Shutdown paths
// touch an unbuffered reply pipe, so "hangs forever" is the failure mode worth
// guarding, not "returns the wrong value".
func awaitReturn(t *testing.T, what string, grace time.Duration, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(grace):
		t.Fatalf("%s did not return within %v", what, grace)
	}
}

func TestStopReturnsWhileTheChildIsStillRunning(t *testing.T) {
	child := startTestShell(t, 40, 4)
	child.run("sleep 30")

	awaitReturn(t, "stop with a live child", 5*time.Second, child.stop)
}

func TestStopReturnsAfterTheChildHasAlreadyExited(t *testing.T) {
	child := startTestShell(t, 40, 4)
	child.run("exit 0")

	select {
	case <-child.exited:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the child to exit")
	}

	awaitReturn(t, "stop after the child exited", 5*time.Second, child.stop)
}

func TestStopReturnsWhenTheReplyPumpDiedFirst(t *testing.T) {
	// The deadlock this guards: the reply pump exits on a failed write before
	// shutdown runs, leaving nothing to read the emulator's reply pipe. A poke
	// issued inline would block forever, because that pipe is unbuffered.
	child := startTestShell(t, 40, 4)

	// Kill the PTY out from under the pump, then hand it a reply to choke on.
	if err := child.ptmx.Close(); err != nil {
		t.Fatalf("close pty: %v", err)
	}
	child.sendText("x")

	select {
	case <-child.repliesDone:
	case <-time.After(5 * time.Second):
		t.Skip("reply pump did not exit on a failed write; nothing to regress against")
	}

	awaitReturn(t, "stop with a dead reply pump", 5*time.Second, child.stop)
}

func TestStopIsIdempotent(t *testing.T) {
	child := startTestShell(t, 40, 4)
	child.run("sleep 30")

	awaitReturn(t, "first stop", 5*time.Second, child.stop)
	awaitReturn(t, "second stop", time.Second, child.stop)
	awaitReturn(t, "third stop", time.Second, child.stop)
}

func TestRetireReplyPumpReportsAJoin(t *testing.T) {
	done := make(chan struct{})
	poked := make(chan struct{})

	go func() {
		<-poked
		close(done)
	}()

	if !retireReplyPump(done, func() { close(poked) }) {
		t.Fatal("a pump that returned must be reported as joined")
	}
}

func TestRetireReplyPumpSurvivesAPokeThatNeverReturns(t *testing.T) {
	// This is the whole reason the poke is asynchronous: with no reader on the
	// emulator's reply pipe the poke blocks forever, and shutdown must still
	// come back.
	done := make(chan struct{})
	close(done)

	blocked := make(chan struct{})
	defer close(blocked)

	awaitReturn(t, "retirement behind a wedged poke", time.Second, func() {
		if !retireReplyPump(done, func() { <-blocked }) {
			t.Error("an already-returned pump must be reported as joined")
		}
	})
}

func TestRetireReplyPumpGivesUpOnAPumpThatNeverReturns(t *testing.T) {
	// Reporting false is what keeps the caller from closing the emulator out
	// from under a live reader.
	start := time.Now()
	if retireReplyPump(make(chan struct{}), func() {}) {
		t.Fatal("a pump that never returned must not be reported as joined")
	}
	if elapsed := time.Since(start); elapsed < replyPumpGrace {
		t.Fatalf("gave up after %v, want at least the %v grace", elapsed, replyPumpGrace)
	}
}
