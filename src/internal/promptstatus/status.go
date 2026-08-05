package promptstatus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"prelude/internal/motd"
)

const (
	promptStatusCheckTimeout = 5 * time.Second
	maxDuration              = time.Duration(1<<63 - 1)
)

// Descriptor is the generated, explicit local-server health contract.
// Probe execution is intentionally absent from shell code and only happens in
// Refresh when the persisted entry is due.
type Descriptor struct {
	Project string `json:"project"`
	Command string `json:"command"`
	Check   string `json:"check"`
	TTL     string `json:"ttl"`
	Start   string `json:"start"`
}

// Record is the one-line shell boundary. Every field is sanitized before it is
// emitted so shell parsing never needs eval or command substitution.
type Record struct {
	State      string
	LastStatus string
	Age        string
	Message    string
	Start      string
	Revision   string
}

func LoadDescriptor(path string) (Descriptor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Descriptor{}, err
	}
	var descriptor Descriptor
	if err := json.Unmarshal(data, &descriptor); err != nil {
		return Descriptor{}, err
	}
	if strings.TrimSpace(descriptor.Project) == "" {
		return Descriptor{}, errors.New("prompt-status: descriptor project is empty")
	}
	if strings.TrimSpace(descriptor.Command) == "" {
		return Descriptor{}, errors.New("prompt-status: descriptor command is empty")
	}
	if strings.TrimSpace(descriptor.Check) == "" {
		return Descriptor{}, errors.New("prompt-status: descriptor check is empty")
	}
	if _, err := parseTTL(descriptor.TTL); err != nil {
		return Descriptor{}, err
	}
	if strings.TrimSpace(descriptor.Start) == "" {
		descriptor.Start = "x " + strings.TrimSpace(descriptor.Command)
	}
	return descriptor, nil
}

func revision(descriptor Descriptor) string {
	identity := strings.Join([]string{descriptor.Project, descriptor.Command, descriptor.Check, descriptor.TTL, descriptor.Start}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(digest[:8])
}

func parseTTL(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("prompt-status: ttl must be positive")
	}
	units := []struct {
		suffix string
		value  time.Duration
	}{
		{"ms", time.Millisecond},
		{"s", time.Second},
		{"m", time.Minute},
		{"h", time.Hour},
		{"d", 24 * time.Hour},
		{"w", 7 * 24 * time.Hour},
	}
	for _, unit := range units {
		if !strings.HasSuffix(value, unit.suffix) || len(value) <= len(unit.suffix) {
			continue
		}
		count, err := strconv.ParseInt(strings.TrimSuffix(value, unit.suffix), 10, 64)
		if err != nil || count <= 0 {
			break
		}
		if count > int64(maxDuration/unit.value) {
			break
		}
		return time.Duration(count) * unit.value, nil
	}
	return 0, fmt.Errorf("prompt-status: invalid ttl %q", value)
}

func cacheKey(descriptor Descriptor) string {
	return "prompt-status:" + revision(descriptor)
}

func checkEntry(descriptor Descriptor, cache motd.Cache) (motd.CacheEntry, bool) {
	entry, ok := cache.Entry(cacheKey(descriptor))
	return entry, ok && !entry.CheckedAt.IsZero()
}

func projectCache(path string, descriptor Descriptor) (motd.CacheStore, error) {
	// NewCacheStore includes both the absolute descriptor path and this
	// descriptor revision in its digest, so projects/configurations never share
	// local-health facts accidentally.
	return motd.NewCacheStore(path, descriptor.Project+"\x00"+revision(descriptor))
}

func record(descriptor Descriptor, cache motd.Cache, now time.Time) Record {
	revisionID := revision(descriptor)
	entry, ok := checkEntry(descriptor, cache)
	if !ok {
		return Record{State: "checking", Message: "checking local server", Start: descriptor.Start, Revision: revisionID}
	}
	age := formatAge(now.Sub(entry.CheckedAt))
	lastStatus := "stopped"
	if entry.Status == "healthy" {
		lastStatus = "healthy"
	}
	state := "stale"
	if entry.Fresh(now) {
		state = lastStatus
	}
	message := strings.TrimSpace(entry.Value)
	if message == "" {
		if lastStatus == "healthy" {
			message = "healthy"
		} else {
			message = "unavailable"
		}
	}
	return Record{State: state, LastStatus: lastStatus, Age: age, Message: message, Start: descriptor.Start, Revision: revisionID}
}

func formatAge(age time.Duration) string {
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age/time.Minute))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(age/time.Hour))
	default:
		return fmt.Sprintf("%dd ago", int(age/(24*time.Hour)))
	}
}

func sanitize(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, strings.TrimSpace(value))
}

func (r Record) Line() string {
	return strings.Join([]string{sanitize(r.State), sanitize(r.LastStatus), sanitize(r.Age), sanitize(r.Message), sanitize(r.Start), sanitize(r.Revision)}, "\t")
}

// Read returns the cached projection and never executes Descriptor.Check.
func Read(descriptorPath string, now time.Time) (Record, error) {
	descriptor, err := LoadDescriptor(descriptorPath)
	if err != nil {
		return Record{}, err
	}
	store, err := projectCache(descriptorPath, descriptor)
	if err != nil {
		return Record{}, err
	}
	return record(descriptor, store.LoadOrEmpty(), now), nil
}

// RefreshDue refreshes only a missing or expired entry, then returns the same
// pure projection. The cache lifetime begins after the probe completes, so a
// slow check cannot make a newly written result immediately stale.
func RefreshDue(descriptorPath string, clock func() time.Time) (Record, error) {
	descriptor, err := LoadDescriptor(descriptorPath)
	if err != nil {
		return Record{}, err
	}
	store, err := projectCache(descriptorPath, descriptor)
	if err != nil {
		return Record{}, err
	}
	now := clock()
	cache := store.LoadOrEmpty()
	entry, exists := checkEntry(descriptor, cache)
	if exists && entry.Fresh(now) {
		return record(descriptor, cache, now), nil
	}

	release, acquired, err := store.TryLock()
	if err != nil {
		return Record{}, err
	}
	if !acquired {
		return record(descriptor, cache, now), nil
	}
	defer release()

	// Another shell might have finished the due refresh while this invocation
	// was acquiring the descriptor-scoped lock.
	cache = store.LoadOrEmpty()
	entry, exists = checkEntry(descriptor, cache)
	now = clock()
	if !exists || !entry.Fresh(now) {
		ok, output := motd.CheckCommandWithTimeout(descriptor.Check, promptStatusCheckTimeout)
		status := "stopped"
		level := "error"
		if ok {
			status = "healthy"
			level = "success"
		}
		checkedAt := clock()
		cache.Set(cacheKey(descriptor), motd.CacheEntry{
			CheckedAt: checkedAt,
			TTL:       mustTTL(descriptor.TTL),
			Status:    status,
			Level:     level,
			Value:     strings.TrimSpace(output),
		})
		if err := store.Write(cache); err != nil {
			return record(descriptor, cache, now), err
		}
		now = checkedAt
	}
	return record(descriptor, cache, now), nil
}

func mustTTL(value string) time.Duration {
	ttl, _ := parseTTL(value)
	return ttl
}
