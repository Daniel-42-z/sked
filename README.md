# Tock

Tock is a CLI tool that helps you keep track of your schedule. It reads your timetable from a TOML or CSV configuration and tells you what you should be doing right now or what's coming up next.

## Features

- **Flexible Configuration**: Supports both TOML (for complex cycles) and CSV (for simple weekly schedules).
- **Cycle Support**: Handle non-standard schedules (e.g., 6-day school cycles) using TOML.
- **Output Formats**: Natural language or JSON output for integration with scripts.
- **Continuous Mode**: Watch mode for status bars (Waybar, Polybar, etc.).
- **Desktop Notifications**: Native notifications on Linux, macOS, and Windows.

## Installation

```bash
# Build from source
mkdir -p build
go build -o build/tock cmd/tock/main.go

# Install to system (example)
sudo cp build/tock /usr/local/bin/
```

## Usage

```bash
tock                  # Show current task
tock --next           # Show next task
tock --time           # Include time range
tock --json           # Output as JSON
tock --watch          # Run in continuous mode
tock --watch --notify-ahead 5m # Notify 5m before tasks (requires --watch), will still show task info in output based on the other flags
tock --config my.toml # Use specific config file
```

## Configuration

### TOML (Recommended for complex cycles)

```toml
cycle_days = 7
anchor_date = "2024-01-01" # Optional anchor for cycle calculation

[[day]]
id = 1 # Monday (if 7-day cycle)
tasks = [
  { name = "Math", start = "09:00", end = "10:00" }
]
```

### CSV (Simple weekly schedule)

```csv
Start,End,Mon,Tue,Wed,Thu,Fri,Sat,Sun
09:00,10:00,Math,History,Math,History,Math,,
```
