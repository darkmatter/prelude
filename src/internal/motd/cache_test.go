package motd

import (
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
)

type recordingRuntime struct {
	checks []string
	probes []string
	result map[string]struct {
		ok  bool
		out string
	}
	probeResult map[string]string
}

func (r *recordingRuntime) Probe(command string) (string, error) {
	r.probes = append(r.probes, command)
	if r.probeResult != nil {
		return r.probeResult[command], nil
	}
	return "", nil
}

func (r *recordingRuntime) Check(command string) (bool, string) {
	r.checks = append(r.checks, command)
	result := r.result[command]
	return result.ok, result.out
}

type fixedTerminal struct {
	w, h int
	bg   string
}

func (t fixedTerminal) Width() int  { return t.w }
func (t fixedTerminal) Height() int { return t.h }
func (t fixedTerminal) Background() (color.Color, error) {
	if t.bg == "" {
		return nil, os.ErrInvalid
	}
	return lipgloss.Color(t.bg), nil
}

func testCacheStore(t *testing.T, now time.Time) cacheStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nested", "cache.json")
	return cacheStore{path: path, now: func() time.Time { return now }}
}

func TestPreflightBlockingSkipsAsyncChecks(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := testCacheStore(t, now)
	runtime := &recordingRuntime{result: map[string]struct {
		ok  bool
		out string
	}{"sync": {ok: true, out: "ready"}, "slow": {ok: true, out: "async-val"}}}
	cfg := Config{Header: Header{Status: []HeaderStatus{
		{Label: "sync", Check: "sync"},
		{Label: "async", Check: "slow", Async: true},
	}}}

	if err := Preflight(cfg, store, runtime, fixedTerminal{w: 80, h: 24}, PreflightBlocking, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runtime.checks, []string{"sync"}) {
		t.Fatalf("blocking checks = %v, want only sync", runtime.checks)
	}
	cache := store.loadOrEmpty()
	e, ok := cache.entry(statusKey("sync"))
	if !ok || e.Status != "ready" {
		t.Fatalf("sync cache = %+v ok=%v", e, ok)
	}
	if _, ok := cache.entry(statusKey("slow")); ok {
		t.Fatal("async status should not be written in blocking preflight")
	}
}

func TestApplyCacheRendersCachedAsyncAndAge(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	cache := Cache{Entries: map[string]CacheEntry{
		statusKey("slow"): {
			CheckedAt: now.Add(-17*time.Minute - 30*time.Second),
			TTL:       ttlAsyncStatus,
			Status:    "healthy",
			Level:     "success",
		},
	}}
	cfg := Config{Header: Header{Status: []HeaderStatus{{Label: "service", Check: "slow", Async: true}}}}

	got := applyCache(cfg, cache, now)
	if got.Header.Status[0].Status != "healthy" {
		t.Fatalf("cached status = %q, want healthy", got.Header.Status[0].Status)
	}
	if want := "17m ago"; got.StatusAge != want {
		t.Fatalf("age = %q, want %q", got.StatusAge, want)
	}
	if want := "[r] to reload"; got.StatusHint != want {
		t.Fatalf("hint = %q, want %q", got.StatusHint, want)
	}
}

func TestApplyCachePendingWithoutCache(t *testing.T) {
	cfg := Config{Header: Header{Status: []HeaderStatus{{Label: "service", Check: "slow", Async: true}}}}
	got := applyCache(cfg, Cache{}, time.Now())
	if got.Header.Status[0].Status != "pending" {
		t.Fatalf("status = %q, want pending", got.Header.Status[0].Status)
	}
	if got.StatusHint != "[r] to reload" {
		t.Fatalf("hint = %q", got.StatusHint)
	}
	if got.StatusAge != "" {
		t.Fatalf("age = %q, want empty", got.StatusAge)
	}
}

func TestPreflightAsyncWritesOnlyAsync(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := testCacheStore(t, now)
	runtime := &recordingRuntime{result: map[string]struct {
		ok  bool
		out string
	}{"slow": {ok: false, out: "offline"}}}
	cfg := Config{Header: Header{Status: []HeaderStatus{
		{Check: "foreground"},
		{Check: "slow", Async: true, FailLevel: "warning"},
	}}}

	if err := Preflight(cfg, store, runtime, fixedTerminal{w: 40, h: 20}, PreflightAsync, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runtime.checks, []string{"slow"}) {
		t.Fatalf("async checks = %v, want only slow", runtime.checks)
	}
	cache := store.loadOrEmpty()
	e, ok := cache.entry(statusKey("slow"))
	if !ok || e.Status != "offline" || e.Level != "warning" {
		t.Fatalf("unexpected cache entry: %+v ok=%v", e, ok)
	}
}

func TestDetachedPreflightCommand(t *testing.T) {
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()

	cmd := detachedPreflightCommand("/bin/echo", "/tmp/config.json", null)
	if cmd.Stdin != null || cmd.Stdout != null || cmd.Stderr != null {
		t.Fatal("background child does not use detached stdio")
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatal("background child does not start in a new session")
	}
	want := []string{"/bin/echo", "--preflight-only", "--async", "--config", "/tmp/config.json"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("args = %v, want %v", cmd.Args, want)
	}
}

func TestCacheWriteAtomic(t *testing.T) {
	store := testCacheStore(t, time.Now())
	if err := store.write(Cache{Entries: map[string]CacheEntry{
		keyTerminalSize: {Width: 80, Height: 24, CheckedAt: time.Now()},
	}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.path)
	if err != nil || len(data) == 0 {
		t.Fatalf("cache file incomplete: bytes=%d err=%v", len(data), err)
	}
}

func TestResolveTerminalSizeDefaults(t *testing.T) {
	w, h := resolveTerminalSize(RenderInput{})
	if w != 80 || h != 24 {
		t.Fatalf("defaults = %dx%d, want 80x24", w, h)
	}
	w, h = resolveTerminalSize(RenderInput{
		Cache: Cache{Entries: map[string]CacheEntry{
			keyTerminalSize: {Width: 100, Height: 40},
		}},
	})
	if w != 100 || h != 40 {
		t.Fatalf("from cache = %dx%d", w, h)
	}
	w, h = resolveTerminalSize(RenderInput{TerminalWidth: 50, TerminalHeight: 10})
	if w != 50 || h != 10 {
		t.Fatalf("override = %dx%d", w, h)
	}
}

func TestApplyCacheFillsEnvFromCache(t *testing.T) {
	cfg := Config{Env: []EnvItem{{Label: "node", Probe: "node -v"}}}
	cache := Cache{Entries: map[string]CacheEntry{
		envKey("node -v"): {Value: "v22", CheckedAt: time.Now(), TTL: ttlEnv},
	}}
	got := applyCache(cfg, cache, time.Now())
	if got.Env[0].Value != "v22" {
		t.Fatalf("env value = %q", got.Env[0].Value)
	}
}

func TestStatusKeyNormalizesWhitespace(t *testing.T) {
	if statusKey("  git status  ") != "status:git status" {
		t.Fatalf("key = %q", statusKey("  git status  "))
	}
}
