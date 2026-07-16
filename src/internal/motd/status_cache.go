package motd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const statusLockMaxAge = 10 * time.Minute

type cachedStatus struct {
	Index      int      `json:"index"`
	Check      string   `json:"check"`
	Status     string   `json:"status"`
	Level      string   `json:"level"`
	Diagnostic []string `json:"diagnostic,omitempty"`
}

type statusCache struct {
	CheckedAt time.Time      `json:"checkedAt"`
	Statuses  []cachedStatus `json:"statuses"`
}

type statusCacheStore struct {
	path string
	now  func() time.Time
}

func newStatusCacheStore(configPath, project string) (statusCacheStore, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return statusCacheStore{}, err
	}
	absolute, err := filepath.Abs(configPath)
	if err != nil {
		return statusCacheStore{}, err
	}
	digest := sha256.Sum256([]byte(project + "\x00" + absolute))
	name := hex.EncodeToString(digest[:16]) + ".json"
	return statusCacheStore{path: filepath.Join(root, "prelude", "motd", "status", name), now: time.Now}, nil
}

func (s statusCacheStore) load() (statusCache, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return statusCache{}, err
	}
	var cache statusCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return statusCache{}, err
	}
	return cache, nil
}

func (s statusCacheStore) write(cache statusCache) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".status-*.tmp")
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

func (s statusCacheStore) tryLock() (func(), bool, error) {
	lockPath := s.path + ".lock"
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return nil, false, err
	}
	if err := os.Mkdir(lockPath, 0o700); err == nil {
		return func() { _ = os.Remove(lockPath) }, true, nil
	} else if !errors.Is(err, os.ErrExist) {
		return nil, false, err
	}
	info, err := os.Stat(lockPath)
	if err != nil {
		return nil, false, err
	}
	if s.now().Sub(info.ModTime()) <= statusLockMaxAge {
		return nil, false, nil
	}
	if err := os.Remove(lockPath); err != nil {
		return nil, false, err
	}
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return func() { _ = os.Remove(lockPath) }, true, nil
}

func applyAsyncCache(cfg *Config, store statusCacheStore) []string {
	hasAsync := false
	for _, item := range cfg.Header.Status {
		hasAsync = hasAsync || (item.Async && item.Check != "")
	}
	if !hasAsync {
		return nil
	}
	cache, err := store.load()
	if err != nil {
		cfg.StatusHint = "[r] to reload"
		for i := range cfg.Header.Status {
			if cfg.Header.Status[i].Async && cfg.Header.Status[i].Check != "" {
				cfg.Header.Status[i].Status = "pending"
				cfg.Header.Status[i].Level = "static"
			}
		}
		return nil
	}

	var diagnostics []string
	found := false
	for _, cached := range cache.Statuses {
		if cached.Index < 0 || cached.Index >= len(cfg.Header.Status) {
			continue
		}
		item := &cfg.Header.Status[cached.Index]
		if !item.Async || item.Check == "" || item.Check != cached.Check {
			continue
		}
		item.Status, item.Level = cached.Status, cached.Level
		diagnostics = append(diagnostics, cached.Diagnostic...)
		found = true
	}
	if !found {
		cfg.StatusHint = "[r] to reload"
		return diagnostics
	}
	age := naturalAge(store.now().Sub(cache.CheckedAt))
	if age == "just now" {
		cfg.StatusHint = "just now • [r] to reload"
	} else {
		cfg.StatusHint = fmt.Sprintf("%s ago • [r] to reload", age)
	}
	return diagnostics
}

func naturalAge(age time.Duration) string {
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%d m", int(age/time.Minute))
	case age < 24*time.Hour:
		return fmt.Sprintf("%d h", int(age/time.Hour))
	default:
		return fmt.Sprintf("%d d", int(age/(24*time.Hour)))
	}
}
