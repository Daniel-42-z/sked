package main

import (
	"fmt"
	"time"

	"tock/internal/config"
	"tock/internal/scheduler"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Run the TUI interface",
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	// 1. Load Config (Reusing logic from run)
	if cfgFile == "" {
		var err error
		cfgFile, err = config.FindOrCreateDefault()
		if err != nil {
			return err
		}
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 2. Initialize Scheduler
	sched := scheduler.New(cfg)

	// 3. Start Bubble Tea program
	p := tea.NewProgram(initialModel(sched), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}
	return nil
}

// --- Model ---

type model struct {
	sched       *scheduler.Scheduler
	table       table.Model
	currentDate time.Time
	err         error
	width       int
	height      int
}

type tickMsg time.Time

func initialModel(sched *scheduler.Scheduler) model {
	columns := []table.Column{
		{Title: "Time", Width: 15},
		{Title: "Task", Width: 40},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m := model{
		sched:       sched,
		table:       t,
		currentDate: time.Now(),
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
		case "up", "k", "down", "j":
			return m, nil
		case "t": // Quick jump to today
			m.currentDate = time.Now()
			m.refreshTable()
		}
	case tickMsg:
		m.refreshTable()
		return m, tickCmd()
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width - 4)
		// Leave space for header and footer
		m.table.SetHeight(msg.Height - 5)
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *model) refreshTable() {
	tasks, err := m.sched.GetTasksForDate(m.currentDate)
	if err != nil {
		m.err = err
		return
	}
	m.err = nil

	rows := []table.Row{}
	now := time.Now()

	// Only calculate "active" if we are looking at today
	isToday := isSameDay(now, m.currentDate)

	activeRowIndex := -1

	for i, task := range tasks {
		timeStr := fmt.Sprintf("%s - %s", task.StartTime.Format("15:04"), task.EndTime.Format("15:04"))
		rows = append(rows, table.Row{timeStr, task.Name})

		if isToday && now.After(task.StartTime) && now.Before(task.EndTime) {
			activeRowIndex = i
		}
	}

	m.table.SetRows(rows)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)

	if activeRowIndex != -1 {
		s.Selected = s.Selected.
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("57")).
			Bold(false)
		m.table.SetCursor(activeRowIndex)
	} else {
		s.Selected = s.Cell
		m.table.SetCursor(0)
	}
	m.table.SetStyles(s)
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

	dateStr := m.currentDate.Format("Monday, January 2, 2006")
	if isSameDay(m.currentDate, time.Now()) {
		dateStr += " (Today)"
	}

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		PaddingBottom(1).
		Render(dateStr)

	baseStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	return baseStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			header,
			m.table.View(),
			"\n  ←/h: prev day • →/l: next day • q: quit",
		),
	) + "\n"
}
