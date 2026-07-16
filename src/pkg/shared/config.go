package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// LoadJSON reads one strict JSON config value into T. Unknown fields and
// additional JSON values are rejected so generated Nix payloads cannot drift
// silently from their Go representations.
func LoadJSON[T any](path string) (*T, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var cfg T
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	var trailing json.RawMessage
	switch err := decoder.Decode(&trailing); err {
	case io.EOF:
		return &cfg, nil
	case nil:
		return nil, fmt.Errorf("parsing %s: unexpected data after first JSON value", path)
	default:
		return nil, fmt.Errorf("parsing %s: trailing data: %w", path, err)
	}
}
