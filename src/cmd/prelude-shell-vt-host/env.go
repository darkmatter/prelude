package main

import "strings"

// Environment helpers. Child environments are built by value so the host's own
// os.Environ() is never mutated and two concurrent captures cannot race over a
// shared slice.

func envValue(env []string, key, fallback string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			if value := entry[len(prefix):]; value != "" {
				return value
			}
			return fallback
		}
	}
	return fallback
}

// withEnv returns env with key set to value exactly once, preserving the
// position of an existing entry.
func withEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			out = append(out, entry)
			continue
		}
		if !replaced {
			out = append(out, prefix+value)
			replaced = true
		}
	}
	if !replaced {
		out = append(out, prefix+value)
	}
	return out
}

// withoutEnv returns env with every entry for key removed. Some Prelude
// switches are mode selectors rather than booleans, where "unset" is the only
// way to ask for the default behaviour.
func withoutEnv(env []string, key string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env))
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		out = append(out, entry)
	}
	return out
}
