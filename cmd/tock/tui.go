package main

import (
	"fmt"
	"time"

	"tock/internal/config"
	"tock/internal/scheduler"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const (
	dateDisplayColor          = lipgloss.Color("40")
	taskHighlightBackground   = lipgloss.Color("22")
	borderHighlightBackground = lipgloss.Color("40")
	taskHighlightForeground   = lipgloss.Color("7")
	borderColor               = lipgloss.Color("240")
)

var tuiCmd = &cobra.Command{
	Use:   "show",
	Short: "Show interactive timetable",
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	var cfg *config.Config
	var err error

	if tmpFile != "" {
		cfg, err = config.LoadTmpCSV(tmpFile)
		if err != nil {
			return fmt.Errorf("failed to load temporary config: %w", err)
		}
	} else {
		// 1. Load Config (Reusing logic from run)
		if cfgFile == "" {
			cfgFile, err = config.FindOrCreateDefault()
			if err != nil {
				return err
			}
		}

		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Check for "tmp" mode argument
		if len(args) > 0 && args[0] == "tmp" {
			if cfg.TmpCSVPath == "" {
				return fmt.Errorf("no 'tmp_csv_path' configured in %s", cfgFile)
			}
			cfg, err = config.LoadTmpCSV(cfg.TmpCSVPath)
			if err != nil {
				return fmt.Errorf("failed to load configured temporary config from %s: %w", cfg.TmpCSVPath, err)
			}
		}
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 2. Initialize Scheduler
	sched := scheduler.New(cfg)

	// 3. Start Bubble Tea program
	p := tea.NewProgram(initialModel(sched, cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	return nil
}

// --- Model ---

type model struct {
	sched       *scheduler.Scheduler
	viewport    viewport.Model
	currentDate time.Time
	err         error
	width       int
	height      int
	dateFormat  string
}

type tickMsg time.Time

func initialModel(sched *scheduler.Scheduler, cfg *config.Config) model {
	vp := viewport.New(0, 0)

	dateFormat := cfg.DateFormat
	if dateFormat == "" {
		dateFormat = "2006-01-02 Mon"
	}

	m := model{
		sched:       sched,
		viewport:    vp,
		currentDate: time.Now(),
		dateFormat:  dateFormat,
	}

	m.refreshTable()
	return m
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "left", "h":
			m.currentDate = m.currentDate.AddDate(0, 0, -1)
			m.refreshTable()
		case "right", "l":
			m.currentDate = m.currentDate.AddDate(0, 0, 1)
			m.refreshTable()
		case "t": // Quick jump to today
			m.currentDate = time.Now()
			m.refreshTable()
		case "up", "k":
			m.viewport.ScrollUp(1)
			return m, nil
		case "down", "j":
			m.viewport.ScrollDown(1)
			return m, nil
		}
	case tickMsg:
		m.refreshTable()
		return m, tickCmd()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		// Leave space for header and footer (approx 6 lines)
		m.viewport.Height = msg.Height - 6
		m.refreshTable()
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *model) refreshTable() {
	tasks, err := m.sched.GetTasksForDate(m.currentDate)
	if err != nil {
		m.err = err
		return
	}
	m.err = nil

	now := time.Now()
	isToday := isSameDay(now, m.currentDate)

	totalWidth := m.viewport.Width
	if totalWidth == 0 {
		totalWidth = 80
	}

	// Calculate columns width
	timeColWidth := 15
	taskColWidth := totalWidth - timeColWidth - 4 // Adjust for borders
	if taskColWidth < 10 {
		taskColWidth = 10
	}

	// Determine if header bottom border should be highlighted (between header and first task)
	headerBottomBorderColor := borderColor
	if isToday && len(tasks) > 0 && now.Before(tasks[0].StartTime) {
		headerBottomBorderColor = borderHighlightBackground
	}

	// Base styles
	baseStyle := lipgloss.NewStyle().Padding(0, 1)
	headerStyle := baseStyle.Bold(true).Align(lipgloss.Center)

	// Custom borders for table continuity
	hTimeBorder := lipgloss.NormalBorder()
	hTimeBorder.TopRight = "┬"
	hTimeBorder.BottomLeft = "├"
	hTimeBorder.BottomRight = "┼"

	hTaskBorder := lipgloss.NormalBorder()
	hTaskBorder.TopLeft = "─"
	hTaskBorder.BottomLeft = "─"
	hTaskBorder.BottomRight = "┤"

	// If no tasks, close the table
	if len(tasks) == 0 {
		hTimeBorder.BottomLeft = "└"
		hTimeBorder.BottomRight = "┴"
		hTaskBorder.BottomRight = "┘"
	}

	// Build Header
	// Time: Top, Right, Bottom, Left borders
	// Task: Top, Right, Bottom borders (Left shared)
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		headerStyle.Width(timeColWidth).
			Border(hTimeBorder, true, true, true, true).
			BorderForeground(borderColor).
			BorderBottomForeground(headerBottomBorderColor).
			Render("Time"),
		headerStyle.Width(taskColWidth).
			Border(hTaskBorder, true, true, true, false).
			BorderForeground(borderColor).
			BorderBottomForeground(headerBottomBorderColor).
			Render("Task"),
	)

	content := header + "\n"

	// Build Rows
	for i, task := range tasks {
		isActive := isToday && now.After(task.StartTime) && now.Before(task.EndTime)

		timeStr := fmt.Sprintf("%s - %s", task.StartTime.Format("15:04"), task.EndTime.Format("15:04"))

		// Check if we need to highlight the bottom border (gap between this and next task, or after last task)
		bottomBorderColor := borderColor
		if isToday {
			if i < len(tasks)-1 {
				nextTask := tasks[i+1]
				// Gap detection
				if now.After(task.EndTime) && now.Before(nextTask.StartTime) {
					bottomBorderColor = borderHighlightBackground
				}
			} else {
				// After last task
				if now.After(task.EndTime) {
					bottomBorderColor = borderHighlightBackground
				}
			}
		}

		rowStyle := baseStyle
		if isActive {
			rowStyle = rowStyle.Foreground(taskHighlightForeground).Background(taskHighlightBackground)
		}

		// Determine border style
		timeBorder := lipgloss.NormalBorder()
		taskBorder := lipgloss.NormalBorder()

		if i == len(tasks)-1 {
			// Last row
			timeBorder.BottomRight = "┴"
			taskBorder.BottomLeft = "─"
			// BottomLeft/BottomRight already └/┘
		} else {
			// Middle row
			timeBorder.BottomLeft = "├"
			timeBorder.BottomRight = "┼"
			taskBorder.BottomLeft = "─"
			taskBorder.BottomRight = "┤"
		}

		// Time Cell: Bottom, Right, Left borders
		tStyle := rowStyle.Width(timeColWidth).
			Border(timeBorder, false, true, true, true).
			BorderForeground(borderColor).
			BorderBottomForeground(bottomBorderColor)

		// Task Cell: Bottom, Right borders
		tskStyle := rowStyle.Width(taskColWidth).
			Border(taskBorder, false, true, true, false).
			BorderForeground(borderColor).
			BorderBottomForeground(bottomBorderColor)

		row := lipgloss.JoinHorizontal(lipgloss.Top,
			tStyle.Render(timeStr),
			tskStyle.Render(task.Name),
		)

		content += row + "\n"
	}

	m.viewport.SetContent(content)
}

func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	dateStr := m.currentDate.Format(m.dateFormat)
	if isSameDay(m.currentDate, time.Now()) {
		dateStr += " (Today)"
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(dateDisplayColor).
		PaddingBottom(1).
		Render(dateStr)

	baseStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	return baseStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.viewport.View(),
			"\n  ←/h: prev day • →/l: next day • ↑/k/u: up • ↓/j/d: down • t: today • q: quit",
		),
	) + "\n"
}
