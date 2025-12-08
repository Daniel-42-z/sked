package output

import (
	"encoding/json"
	"fmt"
	"os"
	"tock/internal/scheduler"
)

// Print displays the task information.
func Print(previous *scheduler.TaskEvent, current *scheduler.TaskEvent, next *scheduler.TaskEvent, dayTasks []scheduler.TaskEvent, asJSON bool, showTime bool, noTaskText string) error {
	if asJSON {
		return printJSON(previous, current, next, dayTasks)
	}
	// JSON mode outputs all three tasks (previous, current, next).
	// Natural language mode outputs only the 'current' task (which main sets based on flags).

	return printNatural(current, showTime, noTaskText)
}

type ExtendedTaskEvent struct {
	scheduler.TaskEvent
	IsCurrent bool `json:"is_current"`
}

type jsonOutput struct {
	Previous *scheduler.TaskEvent `json:"previous"`
	Current  *scheduler.TaskEvent `json:"current"`
	Next     *scheduler.TaskEvent `json:"next"`
	Tasks    []ExtendedTaskEvent  `json:"tasks"`
}

func printJSON(previous *scheduler.TaskEvent, current *scheduler.TaskEvent, next *scheduler.TaskEvent, dayTasks []scheduler.TaskEvent) error {
	extendedTasks := make([]ExtendedTaskEvent, len(dayTasks))
	for i, t := range dayTasks {
		isCurrent := false
		if current != nil {
			// Compare exact times and name to identify the current task
			if t.Name == current.Name && t.StartTime.Equal(current.StartTime) && t.EndTime.Equal(current.EndTime) {
				isCurrent = true
			}
		}
		extendedTasks[i] = ExtendedTaskEvent{
			TaskEvent: t,
			IsCurrent: isCurrent,
		}
	}

	out := jsonOutput{
		Previous: previous,
		Current:  current,
		Next:     next,
		Tasks:    extendedTasks,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func printNatural(task *scheduler.TaskEvent, showTime bool, noTaskText string) error {
	if task == nil {
		if noTaskText != "" {
			fmt.Println(noTaskText)
		} else {
			fmt.Println("No task currently.")
		}
		return nil
	}

	if showTime {
		fmt.Printf("%s (%s - %s)\n", task.Name, task.StartTime.Format("15:04"), task.EndTime.Format("15:04"))
	} else {
		fmt.Println(task.Name)
	}
	return nil
}
