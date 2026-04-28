package screens

import (
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/msgs"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/styles"
)

type CheckInModel struct {
	inputs     []textinput.Model
	focusIndex int
	client     *client.Client
	loading    bool
	spin       spinner.Model
	err        string
	msg        string
}

func NewCheckIn(c *client.Client) *CheckInModel {
	m := &CheckInModel{client: c}
	labels := []string{"Date (YYYY-MM-DD)", "Weight (kg)", "Waist (cm)", "Grip (kg)", "BOLT Score", "Sleep Hours", "Sleep Quality (1-10)", "Subjective Feel (1-10)", "Notes"}
	for i, l := range labels {
		ti := textinput.New()
		ti.Placeholder = l
		if i == 0 {
			ti.SetValue(time.Now().Format("2006-01-02"))
		}
		m.inputs = append(m.inputs, ti)
	}
	m.spin = spinner.New()
	m.spin.Spinner = spinner.Dot
	return m
}

type checkinResultMsg struct{ err error }

func (m *CheckInModel) Init() tea.Cmd { return textinput.Blink }

func (m *CheckInModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return msgs.NavigateMsg{Screen: "menu"} }
		case "tab":
			m.focusIndex = (m.focusIndex + 1) % len(m.inputs)
			for i := range m.inputs {
				m.inputs[i].Blur()
			}
			m.inputs[m.focusIndex].Focus()
			return m, nil
		case "enter":
			if m.loading {
				return m, nil
			}
			m.err = ""
			m.msg = ""
			m.loading = true
			return m, func() tea.Msg {
				// gather
				date := m.inputs[0].Value()
				weight, _ := strconv.ParseFloat(m.inputs[1].Value(), 64)
				waist, _ := strconv.ParseFloat(m.inputs[2].Value(), 64)
				grip, _ := strconv.ParseFloat(m.inputs[3].Value(), 64)
				bolt, _ := strconv.ParseFloat(m.inputs[4].Value(), 64)
				sleepH, _ := strconv.ParseFloat(m.inputs[5].Value(), 64)
				sq, _ := strconv.ParseFloat(m.inputs[6].Value(), 64)
				subj, _ := strconv.Atoi(m.inputs[7].Value())
				notes := m.inputs[8].Value()
				b := client.BiometricLog{
					Date:           date,
					WeightKg:       weight,
					WaistCm:        waist,
					GripKg:         grip,
					BoltScore:      bolt,
					SleepHours:     sleepH,
					SleepQuality:   sq,
					SubjectiveFeel: subj,
					Notes:          notes,
				}
				err := m.client.PostBiometric(b)
				return checkinResultMsg{err: err}
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case checkinResultMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.msg = "Check-in saved"
			// reset inputs except date
			for i := 1; i < len(m.inputs); i++ {
				m.inputs[i].SetValue("")
			}
		}
		return m, nil
	}
	// forward to inputs
	for i := range m.inputs {
		var cmd tea.Cmd
		m.inputs[i], cmd = m.inputs[i].Update(msg)
		if cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

func (m *CheckInModel) View() string {
	s := styles.Title.Render("Daily Check-In") + "\n\n"
	for i := range m.inputs {
		if i == m.focusIndex {
			s += m.inputs[i].View() + " <--\n"
		} else {
			s += m.inputs[i].View() + "\n"
		}
	}
	s += "\n"
	if m.loading {
		s += m.spin.View() + " Saving...\n"
	}
	if m.err != "" {
		s += styles.Error.Render(m.err) + "\n"
	}
	if m.msg != "" {
		s += styles.Success.Render(m.msg) + "\n"
	}
	s += styles.Help.Render("Tab: next • Enter: submit • Esc: menu")
	return s
}
