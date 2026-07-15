package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type strictTestConfig struct {
	Name string `json:"name"`
}

func writeTestConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadJSONDecodesKnownFields(t *testing.T) {
	path := writeTestConfig(t, `{"name":"prelude"}`)

	cfg, err := LoadJSON[strictTestConfig](path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "prelude" {
		t.Fatalf("name = %q, want prelude", cfg.Name)
	}
}

func TestLoadJSONRejectsUnknownFields(t *testing.T) {
	path := writeTestConfig(t, `{"name":"prelude","extra":true}`)

	_, err := LoadJSON[strictTestConfig](path)
	if err == nil {
		t.Fatal("unknown field must fail")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error %q does not identify config path %q", err, path)
	}
	if !strings.Contains(err.Error(), `unknown field "extra"`) {
		t.Fatalf("error = %q, want unknown-field detail", err)
	}
}

func TestLoadJSONRejectsAdditionalJSONValue(t *testing.T) {
	path := writeTestConfig(t, `{"name":"prelude"} {"name":"other"}`)

	_, err := LoadJSON[strictTestConfig](path)
	if err == nil {
		t.Fatal("additional JSON value must fail")
	}
	if !strings.Contains(err.Error(), "unexpected data after first JSON value") {
		t.Fatalf("error = %q, want trailing-value detail", err)
	}
}

func TestLoadJSONAllowsTrailingWhitespace(t *testing.T) {
	path := writeTestConfig(t, "{\"name\":\"prelude\"}\n\t ")

	if _, err := LoadJSON[strictTestConfig](path); err != nil {
		t.Fatalf("trailing whitespace must be accepted: %v", err)
	}
}
