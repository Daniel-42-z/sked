package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"tock/internal/config"
	"tock/internal/notifier"
	"tock/internal/output"
	"tock/internal/scheduler"

	"github.com/spf13/cobra"
)

var (
	cfgFile     string
	jsonFmt     bool
	jsonAll     bool
	showTime    bool
	nextTask    bool
	watchMode   bool
	noTaskText  string
	lookahead   time.Duration
	notifyAhead time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "tock",
	Short: "A CLI timetable tool",
	Long:  `tock reads your timetable configuration and tells you what you should be doing.`,
	RunE:  run,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $XDG_CONFIG_HOME/tock/config.toml)")
	rootCmd.Flags().BoolVarP(&jsonFmt, "json", "j", false, "output in JSON format")
	rootCmd.Flags().BoolVar(&jsonAll, "all", false, "include all tasks for today in JSON output (only with --json)")
	rootCmd.Flags().BoolVarP(&showTime, "time", "t", false, "show time ranges in output")
	rootCmd.Flags().BoolVarP(&nextTask, "next", "n", false, "show next task instead of current")
	rootCmd.Flags().BoolVarP(&watchMode, "watch", "w", false, "continuous mode (watch for changes)")
	rootCmd.Flags().StringVar(&noTaskText, "no-task-text", "No task currently.", "text to display when no task is found")
	rootCmd.Flags().DurationVarP(&lookahead, "lookahead", "l", 0, "lookahead duration for watch mode (affects output time)")
	rootCmd.Flags().DurationVar(&notifyAhead, "notify-ahead", 0, "enable notifications with this lookahead duration (use 0s for immediate)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	notifyEnabled := cmd.Flags().Changed("notify-ahead")

	if notifyEnabled && !watchMode {
		return fmt.Errorf("--notify-ahead can only be used with --watch (-w)")
	}

	var err error
	// 1. Resolve config file path
	if cfgFile == "" {
		cfgFile, err = config.FindOrCreateDefault()
		if err != nil {
			return err
		}
	}

	// 2. Load Config
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 3. Initialize Scheduler
	sched := scheduler.New(cfg)

	// 4. Handle Watch Mode
	if watchMode {
		return runWatch(sched, notifyEnabled)
	}

	// 5. Output
	now := time.Now()
	var currentTask, nextTaskEvent, previousTask *scheduler.TaskEvent
	var dayTasks []scheduler.TaskEvent

	// If JSON, we want both
	if jsonFmt {
		var wg sync.WaitGroup
		var errCurrent, errNext, errPrevious, errDayTasks error

		wg.Add(3)

		go func() {
			defer wg.Done()
			currentTask, errCurrent = sched.GetCurrentTask(now)
		}()

		go func() {
			defer wg.Done()
			nextTaskEvent, errNext = sched.GetNextTask(now)
		}()

		go func() {
			defer wg.Done()
			previousTask, errPrevious = sched.GetPreviousTask(now)
		}()

		if jsonAll {
			wg.Add(1)
			go func() {
				defer wg.Done()
				dayTasks, errDayTasks = sched.GetTasksForDate(now)
			}()
		}

		wg.Wait()

		if errCurrent != nil {
			return errCurrent
		}
		if errNext != nil {
			return errNext
		}
		if errPrevious != nil {
			return errPrevious
		}
		if errDayTasks != nil {
			return errDayTasks
		}
	} else {
		// Natural language mode: depends on flag
		if nextTask {
			// If user asked for next, we treat it as the "primary" task to print
			currentTask, err = sched.GetNextTask(now)
		} else {
			currentTask, err = sched.GetCurrentTask(now)
		}
		if err != nil {
			return err
		}
	}

	return output.Print(previousTask, currentTask, nextTaskEvent, dayTasks, jsonFmt, showTime, noTaskText)
}

func runWatch(sched *scheduler.Scheduler, notifyEnabled bool) error {
	var notif *notifier.Notifier
	if notifyEnabled {
		notif = notifier.New()
	}

	// Keep track of the last task we notified about to avoid spamming
	// We use a signature "Name|StartTime"
	var lastNotifiedSig string

	for {
		now := time.Now()
		effectiveNow := now.Add(lookahead)

		var realCurrent, realNext, realPrevious *scheduler.TaskEvent
		var dayTasks []scheduler.TaskEvent
		var errCurrent, errNext, errPrevious, errDayTasks error

		// Parallelize task fetching
		var wg sync.WaitGroup

		// Always fetch current and next
		wg.Add(2)

		go func() {
			defer wg.Done()
			realCurrent, errCurrent = sched.GetCurrentTask(effectiveNow)
		}()

		go func() {
			defer wg.Done()
			realNext, errNext = sched.GetNextTask(effectiveNow)
		}()

		if jsonFmt {
			wg.Add(1)
			go func() {
				defer wg.Done()
				realPrevious, errPrevious = sched.GetPreviousTask(effectiveNow)
			}()
			if jsonAll {
				wg.Add(1)
				go func() {
					defer wg.Done()
					dayTasks, errDayTasks = sched.GetTasksForDate(effectiveNow)
				}()
			}
		}

		wg.Wait()

		if errCurrent != nil {
			fmt.Fprintf(os.Stderr, "Error getting current task: %v\n", errCurrent)
			time.Sleep(5 * time.Second)
			continue
		}
		if errNext != nil {
			fmt.Fprintf(os.Stderr, "Error getting next task: %v\n", errNext)
			time.Sleep(5 * time.Second)
			continue
		}
		if jsonFmt {
			if errPrevious != nil {
				fmt.Fprintf(os.Stderr, "Error getting previous task: %v\n", errPrevious)
				time.Sleep(5 * time.Second)
				continue
			}
			if errDayTasks != nil {
				fmt.Fprintf(os.Stderr, "Error getting day tasks: %v\n", errDayTasks)
				time.Sleep(5 * time.Second)
				continue
			}
		}

		// --- Notification Logic ---
		if notifyEnabled && notif != nil && realNext != nil {
			// Check if we should notify about the next task
			// We notify if:
			// 1. We haven't notified about this specific task instance yet
			// 2. We are within the notify-ahead window relative to the *actual* start time (not lookahead time)

			// So we use `now` to check against `realNext.StartTime`.
			// `realNext` is the next task relative to `effectiveNow`. If `lookahead` is 0, it's the next task relative to now.

			triggerTime := realNext.StartTime.Add(-notifyAhead)
			sig := fmt.Sprintf("%s|%s", realNext.Name, realNext.StartTime.Format(time.RFC3339))

			if sig != lastNotifiedSig {
				// If we are past the trigger time, send notification
				if !now.Before(triggerTime) {
					// Send notification asynchronously
					msg := fmt.Sprintf("Starts at %s", realNext.StartTime.Format("15:04"))
					if notifyAhead > 0 {
						msg += fmt.Sprintf(" (in %s)", notifyAhead)
					}

					go func(name, message string) {
						if err := notif.Send(name, message); err != nil {
							fmt.Fprintf(os.Stderr, "Failed to send notification: %v\n", err)
						}
					}(realNext.Name, msg)

					lastNotifiedSig = sig
				}
			}
		}

		// --- Output Logic ---
		var outCurrent, outNext, outPrevious *scheduler.TaskEvent

		if jsonFmt {
			outCurrent = realCurrent
			outNext = realNext
			outPrevious = realPrevious
		} else {
			if nextTask {
				outCurrent = realNext
			} else {
				outCurrent = realCurrent
			}
		}

		output.Print(outPrevious, outCurrent, outNext, dayTasks, jsonFmt, showTime, noTaskText)

		// --- Sleep Calculation ---
		// We need to wake up for:
		// 1. Current task ending (status update)
		// 2. Next task starting (status update)
		// 3. Notification trigger time (if enabled)

		targetTimes := []time.Time{}

		if realCurrent != nil {
			targetTimes = append(targetTimes, realCurrent.EndTime.Add(-lookahead))
		}

		if realNext != nil {
			// Wake up when next task starts (status update)
			targetTimes = append(targetTimes, realNext.StartTime.Add(-lookahead))

			// Wake up for notification
			if notifyEnabled && notif != nil {
				// We want to wake up exactly at triggerTime
				triggerTime := realNext.StartTime.Add(-notifyAhead)
				// Only if it's in the future
				if triggerTime.After(now) {
					targetTimes = append(targetTimes, triggerTime)
				}
			}
		}

		// Find the earliest target time that is in the future
		var earliestTarget time.Time
		for _, t := range targetTimes {
			if t.After(now) {
				if earliestTarget.IsZero() || t.Before(earliestTarget) {
					earliestTarget = t
				}
			}
		}

		var waitDuration time.Duration
		if earliestTarget.IsZero() {
			// No known future events. Check back in a minute.
			waitDuration = 1 * time.Minute
		} else {
			waitDuration = earliestTarget.Sub(now)
		}

		// Add a small buffer to ensure we land in the next state
		if waitDuration < 0 {
			waitDuration = 0
		}

		// Sleep
		if waitDuration > 0 {
			time.Sleep(waitDuration + 50*time.Millisecond)
		} else {
			// If we are already past target, just yield briefly to avoid tight loop in weird cases
			time.Sleep(50 * time.Millisecond)
		}
	}
}
