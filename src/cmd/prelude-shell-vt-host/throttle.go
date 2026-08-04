package main

import (
	"sync"
	"time"
)

// throttle collapses a burst of repaint requests into at most one call to fire
// per interval, while still firing immediately when the last call is already
// older than that.
//
// This is the difference between the host tracking a chatty child and drowning
// in it. A command like `find /` hands the PTY thousands of small writes per
// second; composing and rendering a frame for each one is pure waste, because
// only the last state of a burst is ever visible.
type throttle struct {
	interval time.Duration
	fire     func()

	mu      sync.Mutex
	timer   *time.Timer
	lastRun time.Time
	stopped bool
}

func newThrottle(interval time.Duration, fire func()) *throttle {
	return &throttle{interval: max(interval, 0), fire: fire}
}

// request asks for a call to fire. It never blocks on fire and is safe to call
// from any goroutine.
func (t *throttle) request() {
	t.mu.Lock()
	if t.stopped || t.timer != nil {
		// Already stopped, or a call is scheduled and will cover this request.
		t.mu.Unlock()
		return
	}
	wait := t.interval - time.Since(t.lastRun)
	if wait > 0 {
		t.timer = time.AfterFunc(wait, t.flush)
		t.mu.Unlock()
		return
	}
	t.lastRun = time.Now()
	t.mu.Unlock()
	t.fire()
}

func (t *throttle) flush() {
	t.mu.Lock()
	t.timer = nil
	if t.stopped {
		t.mu.Unlock()
		return
	}
	t.lastRun = time.Now()
	t.mu.Unlock()
	t.fire()
}

// stop drops any scheduled call. Requests after stop are ignored, so a late
// PTY read cannot push a frame at a program that has already exited.
func (t *throttle) stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
}
