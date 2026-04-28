package screens

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/msgs"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/styles"
)

type historyLoadedMsg struct {
	logs []client.NutritionLog
	err  error
}

type HistoryModel struct {
	client  *client.Client
	loading bool
	spin    spinner.Model
	err     string
	logs    []client.NutritionLog
	offset  int
}

func NewHistory(c *client.Client) *HistoryModel {
	m := &HistoryModel{client: c, loading: true}
	m.spin = spinner.New()
	m.spin.Spinner = spinner.Dot
	return m
}

func loadHistory(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		to := time.Now().Format("2006-01-02")
		from := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
		logs, err := c.ListNutritionLogs(from, to)
		return historyLoadedMsg{logs: logs, err: err}
	}
}

func (m *HistoryModel) Init() tea.Cmd { return tea.Batch(spinner.Tick, loadHistory(m.client)) }

func (m *HistoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return msgs.NavigateMsg{Screen: "menu"} }
		case "r":
			m.loading = true
			return m, loadHistory(m.client)
		case "up":
			if m.offset > 0 {
				m.offset--
			}
		case "down":
			if m.offset < len(m.logs)-1 {
				m.offset++
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case historyLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.logs = msg.logs
		return m, nil
	}
	return m, nil
}

func (m *HistoryModel) View() string {
	s := styles.Title.Render("Nutrition History") + "\n\n"
	if m.loading {
		return s + m.spin.View() + " Loading..."
	}
	if m.err != "" {
		return s + styles.Error.Render(m.err)
	}
	s += "Date        Calories  Protein  Carbs  Fat   Notes\n"
	s += "────────────────────────────────────────────────────────\n"
	start := m.offset
	end := start + 15
	if end > len(m.logs) {
		end = len(m.logs)
	}
	for i := start; i < end; i++ {
		ln := m.logs[i]
		s += fmt.Sprintf("%s  %4.0f      %4.0fg   %4.0fg  %4.0fg  %s\n", ln.Date, ln.Calories, ln.ProteinG, ln.CarbsG, ln.FatG, ln.MealNotes)
	}
	s += "\n" + styles.Help.Render("↑↓: scroll • r: refresh • Esc: menu")
	return s
}
