package main

import (
	"fmt"
	"os"
	"time"

	"tock/internal/config"
	"tock/internal/output"
	"tock/internal/scheduler"

	"github.com/spf13/cobra"
)

var (
	cfgFile    string
	jsonFmt    bool
	showTime   bool
	nextTask   bool
	watchMode  bool
	noTaskText string
	lookahead  time.Duration
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
	rootCmd.Flags().BoolVarP(&showTime, "time", "t", false, "show time ranges in output")
	rootCmd.Flags().BoolVarP(&nextTask, "next", "n", false, "show next task instead of current")
	rootCmd.Flags().BoolVarP(&watchMode, "watch", "w", false, "continuous mode (watch for changes)")
	rootCmd.Flags().StringVar(&noTaskText, "no-task-text", "No task currently.", "text to display when no task is found")
	rootCmd.Flags().DurationVarP(&lookahead, "lookahead", "l", 0, "lookahead duration for watch mode")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
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
		return runWatch(sched)
	}

	// 5. Output
	now := time.Now()
	var currentTask, nextTaskEvent, previousTask *scheduler.TaskEvent

	// If JSON, we want both
	if jsonFmt {
		currentTask, err = sched.GetCurrentTask(now)
		if err != nil {
			return err
		}
		nextTaskEvent, err = sched.GetNextTask(now)
		if err != nil {
			return err
		}
		previousTask, err = sched.GetPreviousTask(now)
		if err != nil {
			return err
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

	return output.Print(previousTask, currentTask, nextTaskEvent, jsonFmt, showTime, noTaskText)
}

func runWatch(sched *scheduler.Scheduler) error {
	for {
		now := time.Now()
		effectiveNow := now.Add(lookahead)

		// Always fetch current and next for scheduling purposes
		realCurrent, err := sched.GetCurrentTask(effectiveNow)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current task: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		realNext, err := sched.GetNextTask(effectiveNow)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting next task: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}

		var realPrevious *scheduler.TaskEvent
		if jsonFmt {
			realPrevious, err = sched.GetPreviousTask(effectiveNow)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting previous task: %v\n", err)
				time.Sleep(5 * time.Second)
				continue
			}
		}

		// Prepare output
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

		output.Print(outPrevious, outCurrent, outNext, jsonFmt, showTime, noTaskText)

		// Calculate sleep duration
		targetTime := time.Time{}

		if realCurrent != nil {
			targetTime = realCurrent.EndTime
		}

		if realNext != nil {
			if targetTime.IsZero() || realNext.StartTime.Before(targetTime) {
				targetTime = realNext.StartTime
			}
		}

		var waitDuration time.Duration
		if targetTime.IsZero() {
			// No known future events. Check back in a minute.
			waitDuration = 1 * time.Minute
		} else {
			waitDuration = targetTime.Sub(effectiveNow)
		}

		// Add a small buffer to ensure we land in the next state
		if waitDuration < 0 {
			waitDuration = 0
		}

		// Sleep
		if waitDuration > 0 {
			time.Sleep(waitDuration + 50*time.Millisecond)

			// Ensure we actually reached the target time (handle spurious wakeups)
			// Only if we had a valid target time
			if !targetTime.IsZero() {
				for {
					now = time.Now()
					if !now.Add(lookahead).Before(targetTime) {
						break
					}
					remaining := targetTime.Sub(now.Add(lookahead))
					if remaining > 0 {
						time.Sleep(remaining + 50*time.Millisecond)
					}
				}
			}
		} else {
			// If we are already past target, just yield briefly to avoid tight loop in weird cases
			time.Sleep(50 * time.Millisecond)
		}
	}
}
