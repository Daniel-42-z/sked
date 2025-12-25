# Tock Project Outline

## Overview
Tock is a CLI-based timetable management tool designed to help users track their daily schedules. It supports both standard weekly schedules and custom cycle-based schedules (e.g., 6-day school cycles). It features a "watch" mode for status bar integration, desktop notifications, and an interactive Terminal User Interface (TUI).

## Project Structure

### Root Directory
- `go.mod` / `go.sum`: Go module definitions and dependencies.
- `README.md`: User documentation.
- `sample_config.toml` / `sample.csv`: Example configuration files.

### `cmd/`
Entry points for the application.
- `cmd/tock/main.go`: The main CLI entry point. Handles flag parsing, configuration loading, and command routing (e.g., `run`, `runWatch`).
- `cmd/tock/tui.go`: Implementation of the interactive TUI command (`tock show`) using the Bubble Tea framework. Supports `tock show tmp` to view a temporary schedule defined in config.

### `internal/`
Core application logic, separated by domain.

#### `internal/config/`
Handles configuration loading and validation.
- Supports **TOML** for complex configurations (custom cycles, anchor dates).
- Supports **CSV** for simple weekly schedules.
- Supports **Temporary CSV** override via `tmp_csv_path` in TOML.
- `FindOrCreateDefault()`: Automatically creates a default configuration file if none exists.
- `Load()`: Dispatches to `LoadTOML` or `LoadCSV` based on file extension.

#### `internal/scheduler/`
The domain logic for schedule calculations.
- `Scheduler`: Main struct holding the loaded configuration.
- `GetCurrentTask(now)`: Returns the task active at a specific time.
- `GetNextTask(now)`: Finds the next upcoming task.
- `GetPreviousTask(now)`: Finds the most recently finished task.
- `getCycleDayID(date)`: Calculates the effective day ID in the cycle (handling 7-day weeks or custom cycles relative to an anchor date).

#### `internal/notifier/`
Cross-platform desktop notifications.
- Uses `notify-send` on **Linux**.
- Uses `osascript` (AppleScript) on **macOS**.
- Uses PowerShell script on **Windows**.
- `Notifier` struct provides a unified `Send(title, message)` method.

#### `internal/output/`
Handles formatting of CLI output.
- `Print()`: Main entry point for outputting data.
- Supports **Natural Language** (human-readable text).
- Supports **JSON** (`--json`) for machine consumption (e.g., for Polybar/Waybar scripts).

## Key Concepts

- **Cycle Days**: The length of the schedule cycle. Defaults to 7 (weekly). Can be customized in TOML.
- **Anchor Date**: Used for non-7-day cycles to establish a reference point ("Day 1").
- **Watch Mode**: A continuous loop that sleeps intelligently until the next event (task start/end or notification trigger) to update status bars or send notifications.

## Maintenance & Format
When updating the project structure or adding new features, update this file (`OUTLINE.md`) to reflect the changes.
- **Logic**: This outline serves as a high-level architectural map. It groups files by their semantic domain (e.g., `cmd` for entry points, `internal` for logic) rather than just listing them alphabetically.
- **Structure**:
  1.  **Overview**: One-sentence summary of the project.
  2.  **Project Structure**: Hierarchical list of directories with brief descriptions of *what* they contain and *why*.
      - List key files and their primary responsibilities.
      - Mention important functions or types if they are central to the module's purpose.
  3.  **Key Concepts**: Definitions of domain-specific terms (like "Cycle Days" or "Anchor Date") that are necessary to understand the code logic.
- **Goal**: Enable both AI agents and human developers to rapidly understand the codebase without reading every source file.