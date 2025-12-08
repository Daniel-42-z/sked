package scheduler

import (
	"fmt"
	"sort"
	"time"
	"tock/internal/config"
)

// Scheduler handles task lookups based on the configuration.
type Scheduler struct {
	cfg *config.Config
}

// New creates a new Scheduler.
func New(cfg *config.Config) *Scheduler {
	return &Scheduler{cfg: cfg}
}

// TaskEvent represents a scheduled task instance.
type TaskEvent struct {
	Name      string
	StartTime time.Time
	EndTime   time.Time
}

// GetCurrentTask returns the task currently in progress, if any.
func (s *Scheduler) GetCurrentTask(now time.Time) (*TaskEvent, error) {
	dayID, err := s.getCycleDayID(now)
	if err != nil {
		return nil, err
	}

	tasks := s.getTasksForDay(dayID)
	for _, t := range tasks {
		start, end, err := s.parseTaskTimes(now, t)
		if err != nil {
			return nil, err
		}

		if (now.Equal(start) || now.After(start)) && now.Before(end) {
			return &TaskEvent{
				Name:      t.Name,
				StartTime: start,
				EndTime:   end,
			}, nil
		}
	}

	return nil, nil
}

// GetNextTask returns the next upcoming task.
// It searches up to 2 full cycles ahead to find the next event.
func (s *Scheduler) GetNextTask(now time.Time) (*TaskEvent, error) {
	// Search for the next task starting from 'now'
	// We'll check the current day, then subsequent days.

	// Limit search to avoid infinite loops if schedule is empty
	maxDays := s.cfg.CycleDays * 2
	if maxDays < 7 {
		maxDays = 7
	}

	for i := 0; i < maxDays; i++ {
		checkDate := now.AddDate(0, 0, i)
		dayID, err := s.getCycleDayID(checkDate)
		if err != nil {
			return nil, err
		}

		tasks := s.getTasksForDay(dayID)

		// Sort tasks by start time to ensure we find the earliest one
		// This is inefficient to do every time but safe. Optimization can come later.
		// Actually, config order isn't guaranteed, so we MUST sort or iterate carefully.
		// Let's parse all valid tasks for the day first.
		var dayEvents []TaskEvent
		for _, t := range tasks {
			start, end, err := s.parseTaskTimes(checkDate, t)
			if err != nil {
				// Log error? Skip? For now, return error to be safe.
				return nil, fmt.Errorf("invalid time in config: %w", err)
			}
			dayEvents = append(dayEvents, TaskEvent{
				Name:      t.Name,
				StartTime: start,
				EndTime:   end,
			})
		}

		sort.Slice(dayEvents, func(j, k int) bool {
			return dayEvents[j].StartTime.Before(dayEvents[k].StartTime)
		})

		for _, event := range dayEvents {
			if event.StartTime.After(now) {
				return &event, nil
			}
		}
	}

	return nil, nil
}

// GetTasksForDate returns all tasks scheduled for the given date.
func (s *Scheduler) GetTasksForDate(date time.Time) ([]TaskEvent, error) {
	dayID, err := s.getCycleDayID(date)
	if err != nil {
		return nil, err
	}

	tasks := s.getTasksForDay(dayID)
	var events []TaskEvent
	for _, t := range tasks {
		start, end, err := s.parseTaskTimes(date, t)
		if err != nil {
			return nil, fmt.Errorf("invalid time in config: %w", err)
		}
		events = append(events, TaskEvent{
			Name:      t.Name,
			StartTime: start,
			EndTime:   end,
		})
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].StartTime.Before(events[j].StartTime)
	})

	return events, nil
}

// GetPreviousTask returns the most recently finished task.
func (s *Scheduler) GetPreviousTask(now time.Time) (*TaskEvent, error) {
	// Search backwards from 'now'
	maxDays := s.cfg.CycleDays * 2
	if maxDays < 7 {
		maxDays = 7
	}

	for i := 0; i < maxDays; i++ {
		checkDate := now.AddDate(0, 0, -i)
		dayID, err := s.getCycleDayID(checkDate)
		if err != nil {
			return nil, err
		}

		tasks := s.getTasksForDay(dayID)

		var dayEvents []TaskEvent
		for _, t := range tasks {
			start, end, err := s.parseTaskTimes(checkDate, t)
			if err != nil {
				return nil, fmt.Errorf("invalid time in config: %w", err)
			}
			dayEvents = append(dayEvents, TaskEvent{
				Name:      t.Name,
				StartTime: start,
				EndTime:   end,
			})
		}

		// Sort by EndTime descending to find the latest one
		sort.Slice(dayEvents, func(j, k int) bool {
			return dayEvents[j].EndTime.After(dayEvents[k].EndTime)
		})

		for _, event := range dayEvents {
			// We want the task with the latest EndTime that is <= now.
			if !event.EndTime.After(now) {
				return &event, nil
			}
		}
	}

	return nil, nil
}

// getCycleDayID calculates the 0-indexed day ID in the cycle for a given date.
func (s *Scheduler) getCycleDayID(date time.Time) (int, error) {
	// If standard 7-day cycle and no anchor, use weekday
	if s.cfg.CycleDays == 7 && s.cfg.AnchorDate == "" {
		// time.Weekday: Sunday=0, ... Saturday=6
		return int(date.Weekday()), nil
	}

	if s.cfg.AnchorDate == "" {
		return 0, fmt.Errorf("anchor_date is required for non-standard cycles")
	}

	anchor, err := time.Parse("2006-01-02", s.cfg.AnchorDate)
	if err != nil {
		return 0, err
	}

	// Normalize to midnight to calculate day difference
	d1 := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	// Ideally anchor is just a date. Let's assume user local time for simplicity of "days".
	// But time.Parse returns UTC if no timezone info.
	// Let's force anchor to be midnight in the requested timezone (date.Location)
	anchorInLoc := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, date.Location())

	diff := int(d1.Sub(anchorInLoc).Hours() / 24)

	// Handle negative difference (date before anchor)
	mod := diff % s.cfg.CycleDays
	if mod < 0 {
		mod += s.cfg.CycleDays
	}
	return mod, nil
}

func (s *Scheduler) getTasksForDay(dayID int) []config.Task {
	for _, d := range s.cfg.Days {
		if d.ID == dayID {
			return d.Tasks
		}
	}
	return nil
}

// parseTaskTimes converts "HH:MM" strings to time.Time objects on the given date.
func (s *Scheduler) parseTaskTimes(date time.Time, t config.Task) (time.Time, time.Time, error) {
	start, err := parseTimeOnDate(date, t.Start)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("task '%s' start: %w", t.Name, err)
	}
	end, err := parseTimeOnDate(date, t.End)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("task '%s' end: %w", t.Name, err)
	}
	return start, end, nil
}

func parseTimeOnDate(date time.Time, timeStr string) (time.Time, error) {
	t, err := time.Parse("15:04", timeStr)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(
		date.Year(), date.Month(), date.Day(),
		t.Hour(), t.Minute(), 0, 0,
		date.Location(),
	), nil
}
