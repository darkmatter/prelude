package motd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Cache entry kinds and default TTLs (Go-owned; see CONTEXT.md).
const (
	keyTerminalSize = "terminal:size"
	keyTerminalBG   = "terminal:bg"

	ttlEveryRun    = time.Duration(0)
	ttlAsyncStatus = 5 * time.Minute
	ttlEnv         = 5 * time.Minute
)

// Cache is the single JSON map of live MOTD facts written by Preflight.
type Cache struct {
	Entries map[string]CacheEntry `json:"entries"`
}

// CacheEntry is one live fact with identity (map key), value payload, and TTL.
type CacheEntry struct {
	CheckedAt time.Time     `json:"checkedAt"`
	TTL       time.Duration `json:"ttl"` // 0 = every non-pure run

	// terminal:size
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`

	// terminal:bg
	Color string `json:"color,omitempty"`

	// status:*
	Status string `json:"status,omitempty"`
	Level  string `json:"level,omitempty"`

	// env:*
	Value string `json:"value,omitempty"`
}

func (c Cache) entry(key string) (CacheEntry, bool) {
	if c.Entries == nil {
		return CacheEntry{}, false
	}
	e, ok := c.Entries[key]
	return e, ok
}

func (c *Cache) set(key string, entry CacheEntry) {
	if c.Entries == nil {
		c.Entries = make(map[string]CacheEntry)
	}
	c.Entries[key] = entry
}

func (e CacheEntry) fresh(now time.Time) bool {
	if e.CheckedAt.IsZero() {
		return false
	}
	if e.TTL <= 0 {
		return false // TTL 0 ⇒ always due on non-pure preflight
	}
	return now.Sub(e.CheckedAt) < e.TTL
}

func statusKey(check string) string {
	return "status:" + strings.TrimSpace(check)
}

func envKey(probe string) string {
	return "env:" + strings.TrimSpace(probe)
}

// cacheStore is the on-disk Cache location (last-write-wins; atomic rename).
type cacheStore struct {
	path string
	now  func() time.Time
}

func newCacheStore(configPath, project string) (cacheStore, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return cacheStore{}, err
	}
	absolute, err := filepath.Abs(configPath)
	if err != nil {
		return cacheStore{}, err
	}
	digest := sha256.Sum256([]byte(project + "\x00" + absolute))
	name := hex.EncodeToString(digest[:16]) + ".json"
	return cacheStore{
		path: filepath.Join(root, "prelude", "motd", name),
		now:  time.Now,
	}, nil
}

func (s cacheStore) load() (Cache, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return Cache{}, err
	}
	var cache Cache
	if err := json.Unmarshal(data, &cache); err != nil {
		return Cache{}, err
	}
	if cache.Entries == nil {
		cache.Entries = make(map[string]CacheEntry)
	}
	return cache, nil
}

func (s cacheStore) loadOrEmpty() Cache {
	cache, err := s.load()
	if err != nil {
		return Cache{Entries: make(map[string]CacheEntry)}
	}
	return cache
}

func (s cacheStore) write(cache Cache) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	if cache.Entries == nil {
		cache.Entries = make(map[string]CacheEntry)
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".cache-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}

func naturalAge(age time.Duration) string {
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%dm", int(age/time.Minute))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh", int(age/time.Hour))
	default:
		return fmt.Sprintf("%dd", int(age/(24*time.Hour)))
	}
}
