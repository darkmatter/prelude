package shared

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadJSON reads a JSON config file and unmarshals it into T. Both menu-tui
// and motd have the same pattern: read the file, unmarshal, parse-error
// annotate with the path. This helper centralizes the boilerplate.
func LoadJSON[T any](path string) (*T, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg T
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &cfg, nil
}
