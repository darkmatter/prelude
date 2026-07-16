package motd

import (
	"os"
	"os/exec"
	"syscall"
)

func refreshAsyncStatuses(cfg Config, runtime Runtime, store statusCacheStore) error {
	unlock, acquired, err := store.tryLock()
	if err != nil || !acquired {
		return err
	}
	defer unlock()

	cache := statusCache{CheckedAt: store.now()}
	for i := range cfg.Header.Status {
		item := cfg.Header.Status[i]
		if !item.Async || item.Check == "" {
			continue
		}
		ok, out := runtime.Check(item.Check)
		diagnostics := resolveStatusItem(&item, ok, out)
		cache.Statuses = append(cache.Statuses, cachedStatus{
			Index: i, Check: item.Check, Status: item.Status, Level: item.Level, Diagnostic: diagnostics,
		})
	}
	if len(cache.Statuses) == 0 {
		return nil
	}
	return store.write(cache)
}

func startAsyncRefresh(configPath string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer null.Close()

	cmd := detachedRefreshCommand(executable, configPath, null)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func detachedRefreshCommand(executable, configPath string, null *os.File) *exec.Cmd {
	cmd := exec.Command(executable, "--refresh-status", "--config", configPath)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = null, null, null
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}

func hasAsyncStatuses(cfg Config) bool {
	for _, item := range cfg.Header.Status {
		if item.Async && item.Check != "" {
			return true
		}
	}
	return false
}
