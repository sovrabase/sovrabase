package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDotEnvSetsVariablesIfNotSet(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	content := `
# comment line
TEST_VAR_ONE=value1
TEST_VAR_TWO=value2

TEST_VAR_THREE=value3
`
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write test .env failed: %v", err)
	}

	if err := loadDotEnvIfExists(envPath); err != nil {
		t.Fatalf("loadDotEnvIfExists() error = %v", err)
	}

	if got := os.Getenv("TEST_VAR_ONE"); got != "value1" {
		t.Errorf("TEST_VAR_ONE = %q, want %q", got, "value1")
	}
	if got := os.Getenv("TEST_VAR_TWO"); got != "value2" {
		t.Errorf("TEST_VAR_TWO = %q, want %q", got, "value2")
	}
	if got := os.Getenv("TEST_VAR_THREE"); got != "value3" {
		t.Errorf("TEST_VAR_THREE = %q, want %q", got, "value3")
	}
}

func TestLoadDotEnvDoesNotOverrideExisting(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	content := `EXISTING_VAR=from_dotenv`
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write test .env failed: %v", err)
	}

	t.Setenv("EXISTING_VAR", "from_shell")

	if err := loadDotEnvIfExists(envPath); err != nil {
		t.Fatalf("loadDotEnvIfExists() error = %v", err)
	}

	if got := os.Getenv("EXISTING_VAR"); got != "from_shell" {
		t.Errorf("EXISTING_VAR = %q, want %q (should not override)", got, "from_shell")
	}
}

func TestLoadDotEnvMissingFileIsNotAnError(t *testing.T) {
	if err := loadDotEnvIfExists(filepath.Join(t.TempDir(), "nonexistent.env")); err != nil {
		t.Fatalf("loadDotEnvIfExists() error = %v, want nil for missing file", err)
	}
}

func TestLoadDotEnvIgnoresInvalidLines(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	content := strings.Join([]string{
		"VALID_VAR=value",
		"INVALID_LINE_NO_EQUALS",
		"",
		"# comment",
		"ANOTHER_VALID=value2",
	}, "\n")
	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write test .env failed: %v", err)
	}

	if err := loadDotEnvIfExists(envPath); err != nil {
		t.Fatalf("loadDotEnvIfExists() error = %v", err)
	}

	if got := os.Getenv("VALID_VAR"); got != "value" {
		t.Errorf("VALID_VAR = %q, want %q", got, "value")
	}
	if got := os.Getenv("ANOTHER_VALID"); got != "value2" {
		t.Errorf("ANOTHER_VALID = %q, want %q", got, "value2")
	}
}
