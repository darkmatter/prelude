package title

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Recipe is an optional text/font preset accepted through --recipe.
type Recipe struct {
	Text string `json:"text"`
	Font string `json:"font"`
}

// loadRecipe reads a title recipe from path. JSON recipes are parsed
// directly; Nix recipes fall back to nix-instantiate. The JSON path matters
// inside the Nix build sandbox, where nix-instantiate cannot write to
// /nix/var/nix/profiles and so Nix-form recipes are unusable.
func loadRecipe(path string) (Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Recipe{}, fmt.Errorf("read %s: %w", path, err)
	}
	var recipe Recipe
	if err := json.Unmarshal(data, &recipe); err == nil {
		return validateRecipe(path, recipe)
	}
	// Not JSON — evaluate as Nix. This only works outside the build sandbox.
	cmd := exec.Command("nix-instantiate", "--eval", "--strict", "--json", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return Recipe{}, fmt.Errorf("evaluate %s: %s", path, message)
	}
	if err := json.Unmarshal(output, &recipe); err != nil {
		return Recipe{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return validateRecipe(path, recipe)
}

func validateRecipe(path string, recipe Recipe) (Recipe, error) {
	if strings.TrimSpace(recipe.Text) == "" {
		return Recipe{}, fmt.Errorf("%s must define a non-empty text string", path)
	}
	if recipe.Font == "" {
		return Recipe{}, fmt.Errorf("%s must define a font string", path)
	}
	return recipe, nil
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".prelude-title-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
