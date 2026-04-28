package screens

import (
	"fmt"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/msgs"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/styles"
)

type measurementsLoadedMsg struct {
	ms  []client.BodyMeasurement
	err error
}

type MeasurementsModel struct {
	mode    string // log or history
	client  *client.Client
	inputs  []textinput.Model
	spin    spinner.Model
	loading bool
	err     string
	msg     string
	ms      []client.BodyMeasurement
}

func NewMeasurements(c *client.Client) *MeasurementsModel {
	m := &MeasurementsModel{mode: "log", client: c}
	labels := []string{"Date", "Neck (cm)", "Chest (cm)", "Waist (cm)", "Hips (cm)", "Thigh (cm)", "Bicep (cm)", "Notes"}
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

func loadMeasurements(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		to := time.Now().Format("2006-01-02")
		from := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
		ms, err := c.ListMeasurements(from, to)
		return measurementsLoadedMsg{ms: ms, err: err}
	}
}

func (m *MeasurementsModel) Init() tea.Cmd { return nil }

func (m *MeasurementsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return msgs.NavigateMsg{Screen: "menu"} }
		case "h":
			m.mode = "history"
			m.loading = true
			return m, loadMeasurements(m.client)
		case "l":
			m.mode = "log"
			return m, nil
		case "enter":
			if m.mode == "log" {
				var mm client.BodyMeasurement
				mm.Date = m.inputs[0].Value()
				mm.NeckCm = parseFloat(m.inputs[1].Value())
				mm.ChestCm = parseFloat(m.inputs[2].Value())
				mm.WaistCm = parseFloat(m.inputs[3].Value())
				mm.HipsCm = parseFloat(m.inputs[4].Value())
				mm.ThighCm = parseFloat(m.inputs[5].Value())
				mm.BicepCm = parseFloat(m.inputs[6].Value())
				mm.Notes = m.inputs[7].Value()
				if err := m.client.PostMeasurement(mm); err != nil {
					m.err = err.Error()
					return m, nil
				}
				m.msg = "Saved"
			}
			return m, nil
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case measurementsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.ms = msg.ms
		return m, nil
	}
	for i := range m.inputs {
		m.inputs[i], _ = m.inputs[i].Update(msg)
	}
	return m, nil
}

func (m *MeasurementsModel) View() string {
	s := styles.Title.Render("Body Measurements") + "\n\n"
	if m.mode == "log" {
		for _, in := range m.inputs {
			s += in.View() + "\n"
		}
		if m.msg != "" {
			s += styles.Success.Render(m.msg) + "\n"
		}
		if m.err != "" {
			s += styles.Error.Render(m.err) + "\n"
		}
		s += styles.Help.Render("Enter: submit • h: history • Esc: menu")
		return s
	}
	if m.loading {
		return s + m.spin.View() + " Loading..."
	}
	if m.err != "" {
		return s + styles.Error.Render(m.err)
	}
	s += "Date       Neck  Chest Waist Hips Thigh Bicep Notes\n"
	s += "────────────────────────────────────────────────\n"
	for _, mm := range m.ms {
		s += fmt.Sprintf("%s  %.1f  %.1f  %.1f  %.1f  %.1f  %.1f  %s\n", mm.Date, mm.NeckCm, mm.ChestCm, mm.WaistCm, mm.HipsCm, mm.ThighCm, mm.BicepCm, mm.Notes)
	}
	s += "\n" + styles.Help.Render("l: log mode • Esc: menu")
	return s
}

func parseFloat(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }
