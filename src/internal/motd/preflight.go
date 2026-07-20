package motd

import (
	"image/color"
	"os"
	"time"

	"golang.org/x/term"
)

// PreflightMode selects which due cache entries Preflight may refresh.
type PreflightMode int

const (
	// PreflightBlocking refreshes terminal size/bg, sync statuses, and env when due.
	PreflightBlocking PreflightMode = iota
	// PreflightAsync refreshes only async status checks when due.
	PreflightAsync
	// PreflightAll refreshes every due entry (blocking + async).
	PreflightAll
)

// preflightTerminal supplies size and background for Preflight.
type preflightTerminal interface {
	Width() int
	Height() int
	Background() (color.Color, error)
}

// Preflight runs impure work for due Cache entries and writes the store.
// Last-write-wins: concurrent writers may clobber; each write is atomic.
// spinner enables MiniDot on stderr for interactive blocking checks/probes.
func Preflight(cfg Config, store cacheStore, runtime Runtime, terminal preflightTerminal, mode PreflightMode, spinner bool) error {
	now := store.now
	cache := store.loadOrEmpty()
	interactive := spinner && term.IsTerminal(int(os.Stderr.Fd()))

	wantBlocking := mode == PreflightBlocking || mode == PreflightAll
	wantAsync := mode == PreflightAsync || mode == PreflightAll

	if wantBlocking {
		preflightTerminalSize(&cache, terminal, now)
		if needsTerminalBackground(cfg) || needsRelativeBackgrounds(cfg) {
			preflightTerminalBG(&cache, terminal, now)
		}
		preflightSyncStatuses(cfg, &cache, runtime, now, interactive)
		preflightEnv(cfg, &cache, runtime, now, interactive)
	}
	if wantAsync {
		preflightAsyncStatuses(cfg, &cache, runtime, now)
	}

	return store.write(cache)
}

func preflightTerminalSize(cache *Cache, terminal preflightTerminal, now func() time.Time) {
	// TTL 0: always refresh on blocking preflight.
	cache.set(keyTerminalSize, CacheEntry{
		CheckedAt: now(),
		TTL:       ttlEveryRun,
		Width:     max(terminal.Width(), 1),
		Height:    max(terminal.Height(), 1),
	})
}

func preflightTerminalBG(cache *Cache, terminal preflightTerminal, now func() time.Time) {
	bg, err := terminal.Background()
	if err != nil || bg == nil {
		return
	}
	cache.set(keyTerminalBG, CacheEntry{
		CheckedAt: now(),
		TTL:       ttlEveryRun,
		Color:     colorHex(bg),
	})
}

func preflightSyncStatuses(cfg Config, cache *Cache, runtime Runtime, now func() time.Time, interactive bool) {
	for i := range cfg.Header.Status {
		item := cfg.Header.Status[i]
		if item.Check == "" || item.Async {
			continue
		}
		key := statusKey(item.Check)
		if e, ok := cache.entry(key); ok && e.fresh(now()) {
			continue
		}
		label := item.Label
		if label == "" {
			label = "check"
		}
		var stop func()
		if interactive {
			stop = Spinner{}.Render(os.Stderr, label)
		}
		ok, out := runtime.Check(item.Check)
		if stop != nil {
			stop()
		}
		resolved := item
		_ = resolveStatusItem(&resolved, ok, out)
		cache.set(key, CacheEntry{
			CheckedAt: now(),
			TTL:       ttlEveryRun,
			Status:    resolved.Status,
			Level:     resolved.Level,
		})
	}
}

func preflightAsyncStatuses(cfg Config, cache *Cache, runtime Runtime, now func() time.Time) {
	for i := range cfg.Header.Status {
		item := cfg.Header.Status[i]
		if item.Check == "" || !item.Async {
			continue
		}
		key := statusKey(item.Check)
		if e, ok := cache.entry(key); ok && e.fresh(now()) {
			continue
		}
		ok, out := runtime.Check(item.Check)
		resolved := item
		_ = resolveStatusItem(&resolved, ok, out)
		cache.set(key, CacheEntry{
			CheckedAt: now(),
			TTL:       ttlAsyncStatus,
			Status:    resolved.Status,
			Level:     resolved.Level,
		})
	}
}

func preflightEnv(cfg Config, cache *Cache, runtime Runtime, now func() time.Time, interactive bool) {
	for _, item := range cfg.Env {
		if item.Probe == "" {
			continue
		}
		key := envKey(item.Probe)
		if e, ok := cache.entry(key); ok && e.fresh(now()) {
			continue
		}
		label := item.Label
		if label == "" {
			label = "env"
		}
		var stop func()
		if interactive {
			stop = Spinner{}.Render(os.Stderr, label)
		}
		value, err := runtime.Probe(item.Probe)
		if stop != nil {
			stop()
		}
		if err != nil || value == "" {
			cache.set(key, CacheEntry{CheckedAt: now(), TTL: ttlEnv, Value: ""})
			continue
		}
		cache.set(key, CacheEntry{CheckedAt: now(), TTL: ttlEnv, Value: value})
	}
}

// hasDueAsync reports whether any async status is missing or past TTL.
func hasDueAsync(cfg Config, cache Cache, now func() time.Time) bool {
	for _, item := range cfg.Header.Status {
		if !item.Async || item.Check == "" {
			continue
		}
		e, ok := cache.entry(statusKey(item.Check))
		if !ok || !e.fresh(now()) {
			return true
		}
	}
	return false
}

func hasAsyncStatuses(cfg Config) bool {
	for _, item := range cfg.Header.Status {
		if item.Async && item.Check != "" {
			return true
		}
	}
	return false
}
