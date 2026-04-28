package screens

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/msgs"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/styles"
)

type menuItem struct{ label, screen string }

type MenuModel struct {
	cursor int
	client *client.Client
	items  []menuItem
}

func NewMenu(c *client.Client) *MenuModel {
	items := []menuItem{
		{"📊 Dashboard", "dashboard"},
		{"🍽️  Log Meal", "logmeal"},
		{"📋 Daily Check-In", "checkin"},
		{"🏋️  Log Workout", "workoutlog"},
		{"📈 Nutrition History", "history"},
		{"💪 Workout History", "workouthistory"},
		{"⚖️  Body Fat Calculator", "bodyfat"},
		{"📏 Body Measurements", "measurements"},
		{"👤 Profile Settings", "profile"},
		{"🎯 Nutrition Targets", "targets"},
		{"📤 Export Data", "export"},
		{"🚪 Logout", "logout"},
	}
	return &MenuModel{client: c, items: items}
}

func (m *MenuModel) Init() tea.Cmd { return nil }

func (m *MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor <= 0 {
				m.cursor = len(m.items) - 1
			} else {
				m.cursor--
			}
		case "down", "j":
			if m.cursor >= len(m.items)-1 {
				m.cursor = 0
			} else {
				m.cursor++
			}
		case "enter":
			sel := m.items[m.cursor]
			if sel.screen == "logout" {
				_ = m.client.Logout()
				return m, func() tea.Msg { return msgs.NavigateMsg{Screen: "login"} }
			}
			return m, func() tea.Msg { return msgs.NavigateMsg{Screen: sel.screen} }
		}
	}
	return m, nil
}

func (m *MenuModel) View() string {
	s := styles.Title.Render("Main Menu") + "\n\n"
	for i, it := range m.items {
		indicator := "  "
		if i == m.cursor {
			indicator = "▶ "
		}
		s += fmt.Sprintf("%s%2d. %s\n", indicator, i+1, it.label)
	}
	s += "\n" + styles.Help.Render("↑/k and ↓/j: navigate • Enter: select • q: quit")
	return s
}
