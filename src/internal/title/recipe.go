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

func loadRecipe(path string) (Recipe, error) {
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
	var recipe Recipe
	if err := json.Unmarshal(output, &recipe); err != nil {
		return Recipe{}, fmt.Errorf("decode %s: %w", path, err)
	}
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
