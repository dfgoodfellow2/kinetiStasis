package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/msgs"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/screens"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/styles"
)

type Model struct {
	current string
	client  *client.Client
	active  tea.Model
}

func New(c *client.Client) Model {
	return Model{current: ScreenLogin, client: c, active: screens.NewLogin(c)}
}

func (m Model) Init() tea.Cmd {
	return m.active.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case msgs.NavigateMsg:
		return m.navigate(msg.Screen)
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	updated, cmd := m.active.Update(msg)
	m.active = updated
	return m, cmd
}

func (m Model) View() string {
	header := styles.Title.Render("🥗 Diet Tracker") + "\n\n"
	return header + m.active.View()
}

func (m Model) navigate(screen string) (Model, tea.Cmd) {
	var s tea.Model
	switch screen {
	case ScreenLogin:
		s = screens.NewLogin(m.client)
	case ScreenMenu:
		s = screens.NewMenu(m.client)
	case ScreenDashboard:
		s = screens.NewDashboard(m.client)
	case ScreenLogMeal:
		s = screens.NewLogMeal(m.client)
	case ScreenCheckIn:
		s = screens.NewCheckIn(m.client)
	case ScreenHistory:
		s = screens.NewHistory(m.client)
	case ScreenProfile:
		s = screens.NewProfile(m.client)
	case ScreenWorkoutLog:
		s = screens.NewWorkoutLog(m.client)
	case ScreenWorkoutHistory:
		s = screens.NewWorkoutHistory(m.client)
	case ScreenBodyFat:
		s = screens.NewBodyFat(m.client)
	case ScreenMeasurements:
		s = screens.NewMeasurements(m.client)
	case ScreenExport:
		s = screens.NewExport(m.client)
	case ScreenTargets:
		s = screens.NewTargets(m.client)
	default:
		s = screens.NewMenu(m.client)
	}
	m.current = screen
	m.active = s
	return m, s.Init()
}
