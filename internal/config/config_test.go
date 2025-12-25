package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTOML_TildeExpansion(t *testing.T) {
	// --- Setup ---
	// Create a dummy CSV file in a temporary directory
	tmpDir, err := os.MkdirTemp("", "sked_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dummyCSVPath := filepath.Join(tmpDir, "test.csv")
	csvContent := "Start,End,Mon\n09:00,10:00,Test Task"
	if err := os.WriteFile(dummyCSVPath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("Failed to write dummy CSV: %v", err)
	}

	// Create a temporary TOML file that points to the dummy CSV using a tilde path
	// To make this test hermetic, we can't rely on the actual user's home directory.
	// Instead, we'll temporarily set the HOME environment variable.
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	// Note: The path in the toml content now needs to be relative to the *new* HOME.
	tomlContent := `csv_path = "~/test.csv"`
	tmpFile, err := os.CreateTemp("", "test*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(tomlContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()


	// --- Test ---
	cfg, err := Load(tmpFile.Name())

	// --- Assert ---
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned a nil config")
	}
	if len(cfg.Days) != 1 {
		t.Errorf("Expected 1 day loaded from CSV, got %d", len(cfg.Days))
	}
	if len(cfg.Days[0].Tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(cfg.Days[0].Tasks))
	}
	if cfg.Days[0].Tasks[0].Name != "Test Task" {
		t.Errorf("Expected task name 'Test Task', got '%s'", cfg.Days[0].Tasks[0].Name)
	}
}

func TestLoadCSV_EmptyContent(t *testing.T) {
	content := "Start,End,Mon,Tue,Wed,Thu,Fri,Sat,Sun"
	tmpFile, err := os.CreateTemp("", "empty*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	cfg, err := LoadCSV(tmpFile.Name(), "")
	if err != nil {
		t.Fatalf("LoadCSV() returned unexpected error for header-only file: %v", err)
	}
	if len(cfg.Days) != 0 {
		t.Errorf("Expected 0 days for empty CSV, got %d", len(cfg.Days))
	}
}

func TestLoadTmpCSV_EmptyContent(t *testing.T) {
	content := "Start,End,Task"
	tmpFile, err := os.CreateTemp("", "empty_tmp*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	cfg, err := LoadTmpCSV(tmpFile.Name())
	if err != nil {
		t.Fatalf("LoadTmpCSV() returned unexpected error for header-only file: %v", err)
	}
	if len(cfg.Days) != 1 {
		t.Errorf("Expected 1 day for TmpCSV (current day), got %d", len(cfg.Days))
	}
	if len(cfg.Days[0].Tasks) != 0 {
		t.Errorf("Expected 0 tasks, got %d", len(cfg.Days[0].Tasks))
	}
}
