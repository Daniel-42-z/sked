package config

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the top-level configuration structure.
type Config struct {
	CycleDays  int    `toml:"cycle_days"`
	AnchorDate string `toml:"anchor_date"`
	CSVPath    string `toml:"csv_path"`
	DateFormat string `toml:"date_format"`
	Days       []Day  `toml:"day"`
}

// Day represents a single day's schedule in the cycle.
type Day struct {
	ID    int    `toml:"id"`
	Tasks []Task `toml:"tasks"`
}

// Task represents a specific activity.
type Task struct {
	Name  string `toml:"name"`
	Start string `toml:"start"`
	End   string `toml:"end"`
}

// Load reads the configuration from the specified path.
// It detects the format based on the file extension (.toml or .csv).
func Load(path string) (*Config, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".toml":
		return LoadTOML(path)
	case ".csv":
		return LoadCSV(path, "")
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// LoadTOML reads a TOML configuration file.
func LoadTOML(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	// Set defaults
	cfg.CycleDays = 7

	dec := toml.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		return nil, err
	}

	// Check for CSV redirection
	if cfg.CSVPath != "" {
		csvPath, err := expandTilde(cfg.CSVPath)
		if err != nil {
			return nil, err
		}

		// If path is relative, resolve it relative to the TOML file
		if !filepath.IsAbs(csvPath) {
			csvPath = filepath.Join(filepath.Dir(path), csvPath)
		}
		return LoadCSV(csvPath, cfg.DateFormat)
	}

	return &cfg, nil
}

// LoadCSV reads a CSV configuration file.
// CSV format assumes a standard 7-day cycle.
// Header: Start,End,Mon,Tue,Wed,Thu,Fri,Sat,Sun (flexible day column order)
func LoadCSV(path string, dateFormat string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("csv file is empty or missing header")
	}

	header := records[0]
	if len(header) < 3 {
		return nil, fmt.Errorf("header must have at least Start, End and one Day column")
	}

	// Map column index to day ID
	colToDay := make(map[int]int)
	startCol := -1
	endCol := -1

	for i, col := range header {
		col = strings.ToLower(strings.TrimSpace(col))
		if col == "start" || col == "time-start" {
			startCol = i
		} else if col == "end" || col == "time-end" {
			endCol = i
		} else {
			// Try to parse as day
			dayID, err := parseDayName(col)
			if err == nil {
				colToDay[i] = dayID
			}
		}
	}

	if startCol == -1 || endCol == -1 {
		return nil, fmt.Errorf("header must contain 'Start' and 'End' columns")
	}

	cfg := &Config{
		CycleDays: 7,
		Days:      make([]Day, 0),
		DateFormat: dateFormat,
	}
	dayMap := make(map[int][]Task)

	for _, record := range records[1:] {
		if len(record) <= startCol || len(record) <= endCol {
			continue // Skip invalid rows
		}

		start := strings.TrimSpace(record[startCol])
		end := strings.TrimSpace(record[endCol])

		if start == "" {
			continue // Skip rows without start time
		}

		for colIdx, dayID := range colToDay {
			if colIdx >= len(record) {
				continue
			}
			name := strings.TrimSpace(record[colIdx])
			if name != "" {
				task := Task{
					Name:  name,
					Start: start,
					End:   end,
				}
				dayMap[dayID] = append(dayMap[dayID], task)
			}
		}
	}

	// Convert map to slice
	for id, tasks := range dayMap {
		cfg.Days = append(cfg.Days, Day{
			ID:    id,
			Tasks: tasks,
		})
	}

	return cfg, nil
}

// expandTilde expands the '~' prefix in a path to the user's home directory.
func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not get user home directory: %w", err)
	}

	return filepath.Join(home, path[1:]), nil
}

// parseDayName converts a day name (e.g., "Monday") to a cycle ID (0-6).
// Assumes 0=Sunday, 1=Monday, ..., 6=Saturday to match time.Weekday().
func parseDayName(name string) (int, error) {
	name = strings.ToLower(name)
	switch name {
	case "sunday", "sun":
		return 0, nil
	case "monday", "mon":
		return 1, nil
	case "tuesday", "tue":
		return 2, nil
	case "wednesday", "wed":
		return 3, nil
	case "thursday", "thu":
		return 4, nil
	case "friday", "fri":
		return 5, nil
	case "saturday", "sat":
		return 6, nil
	default:
		return -1, fmt.Errorf("invalid day name: %s", name)
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.CycleDays <= 0 {
		return fmt.Errorf("cycle_days must be positive")
	}
	if c.CycleDays != 7 && c.AnchorDate == "" {
		return fmt.Errorf("anchor_date is required for non-7-day cycles")
	}
	if c.AnchorDate != "" {
		_, err := time.Parse("2006-01-02", c.AnchorDate)
		if err != nil {
			return fmt.Errorf("invalid anchor_date format (expected YYYY-MM-DD): %w", err)
		}
	}
	// TODO: Validate time formats (HH:MM)
	return nil
}

// FindOrCreateDefault finds the default config file, creating it if it doesn't exist.
// It returns the path to the config file.
func FindOrCreateDefault() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("could not find user config directory: %w", err)
	}

	tockCfgDir := filepath.Join(cfgDir, "tock")
	configPath := filepath.Join(tockCfgDir, "config.toml")

	// Check if the config file already exists
	if _, err := os.Stat(configPath); err == nil {
		return configPath, nil
	}

	// --- Config file does not exist, so create it ---
	fmt.Fprintf(os.Stderr, "No config file found. Creating a self-documenting default at %s\n", configPath)

	// Create the directory
	if err := os.MkdirAll(tockCfgDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create the default, self-documenting config.toml
	tomlContent := `# Welcome to Tock! This is your main configuration file.
#
# Tock can read your schedule in two ways:
#  1. From a simple CSV file (e.g., for a standard weekly schedule).
#  2. Directly from this TOML file (e.g., for complex, multi-day cycles).

# --- Option 1: Using a CSV file (default for new setups) ---
#
# Point to a CSV file. The path can be absolute (/path/to/your/file.csv)
# or relative to this config file's directory.
# A sample.csv file has been created for you in this directory.
#
# The CSV file should have a header like:
# Start,End,Mon,Tue,Wed,Thu,Fri,Sat,Sun
#
# Tasks named "/" will be ignored and treated as empty time slots.
csv_path = "sample.csv"


# The format for displaying dates in the TUI mode.
# Uses Go's time.Format reference time to define layouts.
# For example, "Mon Jan 2 2006" or "2006-01-02".
# Default is "Monday, January 2, 2006".
# date_format = "2006-01-02"


# --- Option 2: Using TOML for your full schedule ---
#
# To define your schedule here, first comment out the "csv_path" line above.
# Then, you can define your schedule cycle and days below.
#
# cycle_days: The number of days in your repeating schedule cycle (e.g., 7 for a week, or 6 for a 6-day school cycle).
# anchor_date: A specific date (YYYY-MM-DD) that corresponds to day 1 of your cycle.
#              This is required for cycles that are not 7 days.
#
# Example for a 2-day cycle:
# cycle_days = 2
# anchor_date = "2025-01-20" # A day that is "Day 1"

# "[[day]]" represents a single day in your cycle.
# "id" is the day number in the cycle (from 1 to cycle_days).
#
# [[day]]
#   id = 1
#   tasks = [
#     { name = "Morning Project", start = "09:00", end = "12:00" },
#     { name = "Team Sync",       start = "14:00", end = "14:30" },
#   ]
#
# [[day]]
#   id = 2
#   tasks = [
#     { name = "Client Meeting", start = "11:00", end = "12:30" },
#     { name = "Code Review",    start = "15:00", end = "16:00" },
#   ]
`
	if err := os.WriteFile(configPath, []byte(tomlContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write default config.toml: %w", err)
	}

	// Create the default sample.csv
	csvPath := filepath.Join(tockCfgDir, "sample.csv")
	csvContent := `Start,End,Mon,Tue,Wed,Thu,Fri,Sat,Sun
09:00,09:50,Math,History,Math,History,Math,,
10:04,11:00,History,Math,History,Math,History,,
12:00,13:00,Lunch,Lunch,Lunch,Lunch,Lunch,,
`
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
			return "", fmt.Errorf("failed to write default sample.csv: %w", err)
		}
	}

	return configPath, nil
}
