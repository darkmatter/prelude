package shared

import (
	"bytes"
	"encoding/json"
	"errors"
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
	cfg, err := DecodeJSON[T](raw)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return cfg, nil
}

// DecodeJSON is LoadJSON for a payload already in memory, such as the config a
// TypeScript host hands libprelude. The same strictness applies: its JSON is a
// second author of the Nix boundary and must not drift from it either.
func DecodeJSON[T any](raw []byte) (*T, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var cfg T
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}

	var trailing json.RawMessage
	switch err := decoder.Decode(&trailing); err {
	case io.EOF:
		return &cfg, nil
	case nil:
		return nil, errors.New("unexpected data after first JSON value")
	default:
		return nil, fmt.Errorf("trailing data: %w", err)
	}
}
