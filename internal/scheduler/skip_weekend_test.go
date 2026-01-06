package scheduler

import (
	"testing"
	"time"

	"github.com/Daniel-42-z/sked/internal/config"
)

func TestSkipWeekendsLogic(t *testing.T) {
	// Cycle: 2 days
	// Anchor: 2024-01-01 (Monday) -> Day 0
	// SkipWeekends: true
	cfg := &config.Config{
		CycleDays:    2,
		AnchorDate:   "2024-01-01",
		SkipWeekends: true,
		Days: []config.Day{
			{
				ID: 0,
				Tasks: []config.Task{
					{Name: "Work A", Start: "09:00", End: "17:00"},
				},
			},
			{
				ID: 1,
				Tasks: []config.Task{
					{Name: "Work B", Start: "09:00", End: "17:00"},
				},
			},
		},
		Saturday: []config.Task{
			{Name: "Weekend Chill", Start: "10:00", End: "12:00"},
		},
		Sunday: []config.Task{
			{Name: "Sunday Prep", Start: "18:00", End: "19:00"},
		},
	}
	sched := New(cfg)

	tests := []struct {
		dateStr      string
		expectedTask string
	}{
		// Week 1
		{"2024-01-01 10:00", "Work A"}, // Mon - Day 0
		{"2024-01-02 10:00", "Work B"}, // Tue - Day 1
		{"2024-01-03 10:00", "Work A"}, // Wed - Day 0
		{"2024-01-04 10:00", "Work B"}, // Thu - Day 1
		{"2024-01-05 10:00", "Work A"}, // Fri - Day 0

		// Weekend
		{"2024-01-06 11:00", "Weekend Chill"}, // Sat
		{"2024-01-07 18:30", "Sunday Prep"},   // Sun

		// Week 2
		{"2024-01-08 10:00", "Work B"}, // Mon - Day 1 (Skipped weekends, so sequence continues from Fri's Day 0)
		{"2024-01-09 10:00", "Work A"}, // Tue - Day 0
	}

	for _, tt := range tests {
		// Parse date
		// Using time.Parse with a custom format for simplicity in test table
		layout := "2006-01-02 15:04"
		checkTime, err := time.Parse(layout, tt.dateStr)
		if err != nil {
			t.Fatalf("failed to parse date %s: %v", tt.dateStr, err)
		}

		// Ensure UTC is used if scheduler assumes it, but scheduler uses time.Date with location from input.
		// However, parse "2006-01-02" in config uses default (UTC usually for Date function if not specified?).
		// Config uses time.Parse which returns UTC.
		// Scheduler: `date.Location()` is used.
		// So if we pass UTC, it works.

		task, err := sched.GetCurrentTask(checkTime)
		if err != nil {
			t.Errorf("date %s: unexpected error: %v", tt.dateStr, err)
			continue
		}

		if task == nil {
			t.Errorf("date %s: expected task %s, got nil", tt.dateStr, tt.expectedTask)
			continue
		}

		if task.Name != tt.expectedTask {
			t.Errorf("date %s: expected task %s, got %s", tt.dateStr, tt.expectedTask, task.Name)
		}
	}
}
