package motd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type recordingRuntime struct {
	checks []string
	result map[string]struct {
		ok  bool
		out string
	}
}

func (r *recordingRuntime) Probe(string) (string, error) { return "", nil }
func (r *recordingRuntime) Check(command string) (bool, string) {
	r.checks = append(r.checks, command)
	result := r.result[command]
	return result.ok, result.out
}

func TestStatusResolverSkipsAsyncChecks(t *testing.T) {
	runtime := &recordingRuntime{result: map[string]struct {
		ok  bool
		out string
	}{"sync": {ok: true, out: "ready"}}}
	cfg := Config{Header: Header{Status: []HeaderStatus{
		{Label: "sync", Check: "sync"},
		{Label: "async", Check: "slow", Async: true},
	}}}

	(StatusResolver{runtime: runtime}).Resolve(&cfg)

	if !reflect.DeepEqual(runtime.checks, []string{"sync"}) {
		t.Fatalf("foreground checks = %v, want only sync", runtime.checks)
	}
	if got := cfg.Header.Status[0].Status; got != "ready" {
		t.Fatalf("sync status = %q, want ready", got)
	}
	if got := cfg.Header.Status[1].Status; got != "" {
		t.Fatalf("async status was modified in foreground: %q", got)
	}
}

func TestApplyAsyncCacheRendersCachedStatusAndAge(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := testStatusStore(t, now)
	if err := store.write(statusCache{
		CheckedAt: now.Add(-17*time.Minute - 30*time.Second),
		Statuses:  []cachedStatus{{Index: 0, Check: "slow", Status: "healthy", Level: "success"}},
	}); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Header: Header{Status: []HeaderStatus{{Label: "service", Check: "slow", Async: true}}}}

	applyAsyncCache(&cfg, store)

	if got := cfg.Header.Status[0].Status; got != "healthy" {
		t.Fatalf("cached status = %q, want healthy", got)
	}
	want := "17 m ago • [r] to reload"
	if cfg.StatusHint != want {
		t.Fatalf("hint = %q, want %q", cfg.StatusHint, want)
	}
}

func TestApplyAsyncCacheUsesPendingWithoutCache(t *testing.T) {
	store := testStatusStore(t, time.Now())
	cfg := Config{Header: Header{Status: []HeaderStatus{{Label: "service", Check: "slow", Async: true}}}}

	applyAsyncCache(&cfg, store)

	if got := cfg.Header.Status[0].Status; got != "pending" {
		t.Fatalf("status = %q, want pending", got)
	}
	want := "[r] to reload"
	if cfg.StatusHint != want {
		t.Fatalf("hint = %q, want %q", cfg.StatusHint, want)
	}
}

func TestRefreshAsyncStatusesWritesOnlyAsyncResults(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := testStatusStore(t, now)
	runtime := &recordingRuntime{result: map[string]struct {
		ok  bool
		out string
	}{"slow": {ok: false, out: "offline"}}}
	cfg := Config{Header: Header{Status: []HeaderStatus{
		{Check: "foreground"},
		{Check: "slow", Async: true, FailLevel: "warning"},
	}}}

	if err := refreshAsyncStatuses(cfg, runtime, store); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runtime.checks, []string{"slow"}) {
		t.Fatalf("refresh checks = %v, want only slow", runtime.checks)
	}
	cache, err := store.load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cache.Statuses) != 1 || cache.Statuses[0].Status != "offline" || cache.Statuses[0].Level != "warning" {
		t.Fatalf("unexpected cache: %+v", cache)
	}
}

func TestDetachedRefreshCommandHasNoInheritedStdio(t *testing.T) {
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()

	cmd := detachedRefreshCommand("/bin/echo", "/tmp/config.json", null)
	if cmd.Stdin != null || cmd.Stdout != null || cmd.Stderr != null {
		t.Fatal("background child does not use detached stdio")
	}
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setsid {
		t.Fatal("background child does not start in a new session")
	}
	want := []string{"/bin/echo", "--refresh-status", "--config", "/tmp/config.json"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("args = %v, want %v", cmd.Args, want)
	}
}

func TestRefreshLockPreventsDuplicateChecks(t *testing.T) {
	store := testStatusStore(t, time.Now())
	unlock, acquired, err := store.tryLock()
	if err != nil || !acquired {
		t.Fatalf("first lock: acquired=%v err=%v", acquired, err)
	}
	defer unlock()
	runtime := &recordingRuntime{result: map[string]struct {
		ok  bool
		out string
	}{}}
	cfg := Config{Header: Header{Status: []HeaderStatus{{Check: "slow", Async: true}}}}

	if err := refreshAsyncStatuses(cfg, runtime, store); err != nil {
		t.Fatal(err)
	}
	if len(runtime.checks) != 0 {
		t.Fatalf("duplicate refresh ran checks: %v", runtime.checks)
	}
}

func testStatusStore(t *testing.T, now time.Time) statusCacheStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nested", "status.json")
	return statusCacheStore{path: path, now: func() time.Time { return now }}
}

func TestStatusCacheWritePreservesExistingFileOnMarshalIndependentFailure(t *testing.T) {
	// Atomic replacement should leave a complete JSON file, never a partial one.
	store := testStatusStore(t, time.Now())
	if err := store.write(statusCache{CheckedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.path)
	if err != nil || len(data) == 0 {
		t.Fatalf("cache file is incomplete: bytes=%d err=%v", len(data), err)
	}
}
