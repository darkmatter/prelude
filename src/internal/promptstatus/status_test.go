package promptstatus

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	"unicode"
)

func writeDescriptor(t *testing.T, descriptor Descriptor) string {
	t.Helper()
	path := t.TempDir() + "/prompt-status.json"
	data, err := json.Marshal(descriptor)
	if err != nil {
		t.Fatalf("marshal descriptor: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write descriptor: %v", err)
	}
	return path
}

func isolateCache(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
}

func testDescriptor(check, ttl string) Descriptor {
	return Descriptor{
		Project: "demo",
		Command: "dev",
		Check:   check,
		TTL:     ttl,
		Start:   "x dev",
	}
}

func TestReadMissingCacheIsChecking(t *testing.T) {
	isolateCache(t)
	path := writeDescriptor(t, testDescriptor("true", "5m"))

	record, err := Read(path, time.Unix(100, 0))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if record.State != "checking" || record.LastStatus != "" || record.Message != "checking local server" || record.Start != "x dev" {
		t.Fatalf("missing cache record = %#v", record)
	}
}

func TestRefreshDueSuccessfulCheckIsFreshHealthy(t *testing.T) {
	isolateCache(t)
	path := writeDescriptor(t, testDescriptor("printf ready", "5m"))
	now := time.Unix(200, 0)
	record, err := RefreshDue(path, func() time.Time { return now })
	if err != nil {
		t.Fatalf("RefreshDue: %v", err)
	}
	if record.State != "healthy" || record.LastStatus != "healthy" || record.Message != "ready" || record.Age != "just now" {
		t.Fatalf("successful refresh record = %#v", record)
	}
}

func TestRefreshDueFailedCheckIsStoppedWithCanonicalStart(t *testing.T) {
	isolateCache(t)
	path := writeDescriptor(t, testDescriptor("printf down; exit 1", "5m"))
	record, err := RefreshDue(path, func() time.Time { return time.Unix(300, 0) })
	if err != nil {
		t.Fatalf("RefreshDue: %v", err)
	}
	if record.State != "stopped" || record.LastStatus != "stopped" || record.Message != "down" || record.Start != "x dev" {
		t.Fatalf("failed refresh record = %#v", record)
	}
}

func TestReadExpiredCacheIsStaleAndPreservesOutcomeMessageAndCompactAge(t *testing.T) {
	isolateCache(t)
	path := writeDescriptor(t, testDescriptor("printf down; exit 1", "5m"))
	checkedAt := time.Unix(400, 0)
	if _, err := RefreshDue(path, func() time.Time { return checkedAt }); err != nil {
		t.Fatalf("RefreshDue: %v", err)
	}

	record, err := Read(path, checkedAt.Add(17*time.Minute))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if record.State != "stale" || record.LastStatus != "stopped" || record.Message != "down" || record.Age != "17m ago" || record.Start != "x dev" {
		t.Fatalf("expired cache record = %#v", record)
	}
}

func TestLoadDescriptorFallbackUsesCanonicalStartInstruction(t *testing.T) {
	isolateCache(t)
	descriptor := testDescriptor("true", "5m")
	descriptor.Start = ""
	path := writeDescriptor(t, descriptor)

	loaded, err := LoadDescriptor(path)
	if err != nil {
		t.Fatalf("LoadDescriptor: %v", err)
	}
	if loaded.Start != "x dev" {
		t.Fatalf("fallback start = %q, want %q", loaded.Start, "x dev")
	}
}

func TestLoadDescriptorRejectsTTLOverflow(t *testing.T) {
	path := writeDescriptor(t, testDescriptor("true", "1000000000000000000ms"))

	if _, err := LoadDescriptor(path); err == nil {
		t.Fatal("LoadDescriptor accepted an overflowing ttl")
	}
}

func TestRecordLineSanitizesControlOutputWithoutChangingFieldShape(t *testing.T) {
	record := Record{
		State:      "stopped\x1b[31m",
		LastStatus: "stopped",
		Age:        "1m\r",
		Message:    "ordinary\ttext\n\x00\a\x1b[2K",
		Start:      "x dev",
		Revision:   "abcd",
	}

	line := record.Line()
	fields := strings.Split(line, "\t")
	if len(fields) != 6 {
		t.Fatalf("line has %d tab-separated fields: %q", len(fields), line)
	}
	for _, field := range fields {
		for _, r := range field {
			if unicode.IsControl(r) {
				t.Fatalf("field contains control rune %q: %q", r, line)
			}
		}
	}
	if fields[3] != "ordinary text    [2K" {
		t.Fatalf("sanitized message = %q", fields[3])
	}
}

func TestRefreshDueStampsCacheAtProbeCompletion(t *testing.T) {
	isolateCache(t)
	marker := t.TempDir() + "/probe-complete"
	path := writeDescriptor(t, testDescriptor(fmt.Sprintf("touch %q; printf ready", marker), "1m"))
	started := time.Unix(500, 0)
	completed := started.Add(2 * time.Minute)
	clock := func() time.Time {
		if _, err := os.Stat(marker); err == nil {
			return completed
		}
		return started
	}

	if _, err := RefreshDue(path, clock); err != nil {
		t.Fatalf("RefreshDue: %v", err)
	}
	record, err := Read(path, completed)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if record.State != "healthy" {
		t.Fatalf("completion-stamped cache state = %q, want healthy", record.State)
	}
}
