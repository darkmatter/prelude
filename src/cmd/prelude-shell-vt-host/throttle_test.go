package main

import (
	"sync"
	"testing"
	"time"
)

// frameSink counts repaint requests without racing the throttle goroutine.
type frameSink struct {
	mu     sync.Mutex
	count  int
	notify chan struct{}
}

func newFrameSink() *frameSink {
	return &frameSink{notify: make(chan struct{}, 64)}
}

func (s *frameSink) emit() {
	s.mu.Lock()
	s.count++
	s.mu.Unlock()
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *frameSink) total() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

func (s *frameSink) awaitFrame(t *testing.T) {
	t.Helper()
	select {
	case <-s.notify:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a frame")
	}
}

func TestThrottleEmitsTheFirstRequestImmediately(t *testing.T) {
	// An idle host must not add latency to the first byte the child prints.
	sink := newFrameSink()
	throttle := newThrottle(50*time.Millisecond, sink.emit)
	defer throttle.stop()

	start := time.Now()
	throttle.request()
	sink.awaitFrame(t)

	if elapsed := time.Since(start); elapsed > 25*time.Millisecond {
		t.Fatalf("first frame took %v, want it to fire without waiting out the interval", elapsed)
	}
}

func TestThrottleCoalescesABurstIntoOneTrailingFrame(t *testing.T) {
	sink := newFrameSink()
	throttle := newThrottle(60*time.Millisecond, sink.emit)
	defer throttle.stop()

	for range 200 {
		throttle.request()
	}
	sink.awaitFrame(t) // leading frame

	// One trailing frame covers the whole burst, no matter how many requests
	// arrived while the interval was open.
	time.Sleep(150 * time.Millisecond)

	if total := sink.total(); total != 2 {
		t.Fatalf("burst produced %d frames, want exactly 2 (leading + trailing)", total)
	}
}

func TestThrottleStaysQuietWhenNothingHappens(t *testing.T) {
	sink := newFrameSink()
	throttle := newThrottle(20*time.Millisecond, sink.emit)
	defer throttle.stop()

	throttle.request()
	sink.awaitFrame(t)
	time.Sleep(120 * time.Millisecond)

	if total := sink.total(); total != 1 {
		t.Fatalf("idle throttle emitted %d frames, want 1", total)
	}
}

func TestThrottleStopIsIdempotentAndSilencesLaterRequests(t *testing.T) {
	sink := newFrameSink()
	throttle := newThrottle(10*time.Millisecond, sink.emit)

	throttle.stop()
	throttle.stop() // must not panic on a double close

	throttle.request()
	time.Sleep(50 * time.Millisecond)

	if total := sink.total(); total != 0 {
		t.Fatalf("a stopped throttle emitted %d frames", total)
	}
}
