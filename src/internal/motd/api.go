package motd

// Library entry points for hosts that render the MOTD in their own process
// (libprelude, which the TypeScript API loads through bun:ffi). A host has no
// wrapper, cache file, or detached refresh: RenderHosted runs the configured
// shell checks and probes inline, as blocking Preflight would, and accepts
// outcomes for checks the host evaluated itself (TypeScript functions).

import (
	"time"

	"prelude/pkg/shared"
)

// ttlHosted marks host-supplied outcomes fresh so the inline Preflight pass
// leaves them alone; the value only has to outlive one render.
const ttlHosted = time.Hour

// CheckResult is the outcome of a status check the host ran, keyed by the
// check string configured on the header status item.
type CheckResult struct {
	Check  string `json:"check"`
	OK     bool   `json:"ok"`
	Output string `json:"output"`
}

// ProbeResult is the value of an env probe the host ran; empty hides it the
// same way a failed shell probe does.
type ProbeResult struct {
	Probe string `json:"probe"`
	Value string `json:"value"`
}

// Results are the live facts a host gathered itself.
type Results struct {
	Status []CheckResult `json:"status"`
	Env    []ProbeResult `json:"env"`
}

// ParseConfig decodes one strict MOTD Config JSON value.
func ParseConfig(raw []byte) (Config, error) {
	cfg, err := shared.DecodeJSON[Config](raw)
	if err != nil {
		return Config{}, err
	}
	return *cfg, nil
}

// RenderHosted is Preflight plus Render for a library host: host outcomes are
// recorded through the same status rules as shell checks, every remaining
// check and probe runs inline (there is no detached refresh to defer async
// ones to), and the banner renders for the host's terminal size.
func RenderHosted(cfg Config, results Results, width, height int) string {
	now := time.Now
	cache := results.cache(cfg, now())
	runtime := systemRuntime{}
	preflightSyncStatuses(cfg, &cache, runtime, now, false)
	preflightAsyncStatuses(cfg, &cache, runtime, now)
	preflightEnv(cfg, &cache, runtime, now, false)
	return Render(RenderInput{Config: cfg, Cache: cache, TerminalWidth: width, TerminalHeight: height})
}

func (r Results) cache(cfg Config, now time.Time) Cache {
	cache := Cache{Entries: map[string]CacheEntry{}}
	outcomes := make(map[string]CheckResult, len(r.Status))
	for _, result := range r.Status {
		outcomes[statusKey(result.Check)] = result
	}
	for _, item := range cfg.Header.Status {
		key := statusKey(item.Check)
		result, ok := outcomes[key]
		if item.Check == "" || !ok {
			continue
		}
		status, level := resolveStatusItem(item, result.OK, result.Output)
		cache.set(key, CacheEntry{CheckedAt: now, TTL: ttlHosted, Status: status, Level: level})
	}
	for _, result := range r.Env {
		cache.set(envKey(result.Probe), CacheEntry{CheckedAt: now, TTL: ttlHosted, Value: result.Value})
	}
	return cache
}
