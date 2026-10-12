package motd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRunDirenvClosesCapturePipeBeforeAsyncPreflightCompletes(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "motd")
	buildCtx, cancelBuild := context.WithTimeout(context.Background(), time.Minute)
	defer cancelBuild()
	if output, err := exec.CommandContext(buildCtx, "go", "build", "-o", binary, "../../cmd/motd").CombinedOutput(); err != nil {
		t.Fatalf("build MOTD CLI: %v\n%s", err, output)
	}

	started := filepath.Join(dir, "started")
	release := filepath.Join(dir, "release")
	// Relative paths avoid shell quoting of TempDir. The bounded loop also
	// lets the detached check exit if the test process itself is interrupted.
	check := ": > started; for ((i=0; i<300; i++)); do if [ -f release ]; then exit 0; fi; sleep 0.1; done; exit 1"
	cfg := Config{Project: "direnv-fd3-regression", Header: Header{Status: []HeaderStatus{
		{Label: "gated", Check: check, Async: true},
	}}}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("HOME", dir)
	store, err := newCacheStore(configPath, cfg.Project)
	if err != nil {
		t.Fatal(err)
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close() })
	t.Cleanup(func() { writer.Close() })
	null, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--config", configPath)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "DIRENV_IN_ENVRC=1", "PRELUDE_MOTD_PURE=0")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = null, null, null
	cmd.ExtraFiles = []*os.File{writer} // direnv's JSON capture pipe is fd3.
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	cliDone := make(chan struct{})
	var cliErr error
	go func() {
		cliErr = cmd.Wait()
		close(cliDone)
	}()

	waitFor := func(ready func() bool) bool {
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		for {
			if ready() {
				return true
			}
			select {
			case <-deadline.C:
				return false
			case <-tick.C:
			}
		}
	}
	t.Cleanup(func() {
		if err := os.WriteFile(release, nil, 0o600); err != nil {
			t.Errorf("release async check: %v", err)
		}
		cancel()
		<-cliDone
		// Preflight publishes Cache only after the shell check has exited;
		// wait for its final write before TempDir cleanup removes its files.
		if !waitFor(func() bool {
			cache, err := store.load()
			_, present := cache.entry(statusKey(check))
			return err == nil && present
		}) {
			t.Error("async Preflight did not finish writing Cache after release")
		}
	})

	select {
	case <-cliDone:
		if cliErr != nil {
			t.Fatalf("MOTD CLI: %v", cliErr)
		}
	case <-ctx.Done():
		t.Fatal("MOTD CLI did not return while async check was gated")
	}
	if !waitFor(func() bool {
		_, err := os.Stat(started)
		return err == nil
	}) {
		t.Fatal("async check did not start")
	}

	pipeDone := make(chan error, 1)
	go func() {
		var b [1]byte
		n, err := reader.Read(b[:])
		if n != 0 || err != io.EOF {
			pipeDone <- fmt.Errorf("capture pipe read = (%d, %v), want (0, EOF)", n, err)
			return
		}
		pipeDone <- nil
	}()
	select {
	case err := <-pipeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("direnv capture pipe fd3 remained open after MOTD returned, while async check was still gated")
	}
	if _, err := os.Stat(release); !os.IsNotExist(err) {
		t.Fatalf("async check gate was released before EOF: %v", err)
	}
	if _, present := store.loadOrEmpty().entry(statusKey(check)); present {
		t.Fatal("async check completed before capture pipe EOF was observed")
	}
}
