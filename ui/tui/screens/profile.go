package screens

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/msgs"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/styles"
)

type profileLoadedMsg struct {
	p   client.Profile
	err error
}

// field indices
const (
	pfName = iota
	pfAge
	pfSex
	pfHeightCm
	pfActivity
	pfExerciseFreq
	pfRunningKm
	pfIsLifter
	pfGoal
	pfPrioritizeCarbs
	pfBfPct
	pfHRRest
	pfHRMax
	pfGripWeight
	pfTDEELookback
	pfSleepQualityMax
	pfUnits
	pfCount
)

var profileLabels = [pfCount]string{
	"Name",
	"Age",
	"Sex (male/female)",
	"Height (cm)",
	"Activity (sedentary/light/moderate/active/very_active)",
	"Exercise freq (days/week)",
	"Running (km/week)",
	"Is lifter (true/false)",
	"Goal (maintenance/cut/bulk/aggressive_cut/aggressive_bulk)",
	"Prioritize carbs (true/false)",
	"Body fat %",
	"HR rest (bpm)",
	"HR max (bpm)",
	"Grip weight (readiness weighting 0.0-1.0)",
	"TDEE lookback days",
	"Sleep quality max (scale top, e.g. 10)",
	"Units (metric/imperial)",
}

type ProfileModel struct {
	client  *client.Client
	loading bool
	spin    spinner.Model
	err     string
	msg     string
	inputs  []textinput.Model
	focus   int
	offset  int // for scrolling through many fields
}

func NewProfile(c *client.Client) *ProfileModel {
	m := &ProfileModel{client: c, loading: true}
	m.spin = spinner.New()
	m.spin.Spinner = spinner.Dot
	for i := 0; i < pfCount; i++ {
		ti := textinput.New()
		ti.Placeholder = profileLabels[i]
		m.inputs = append(m.inputs, ti)
	}
	m.inputs[0].Focus()
	return m
}

func loadProfile(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		p, err := c.GetProfile()
		return profileLoadedMsg{p: p, err: err}
	}
}

func (m *ProfileModel) Init() tea.Cmd { return tea.Batch(spinner.Tick, loadProfile(m.client)) }

func (m *ProfileModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return msgs.NavigateMsg{Screen: "menu"} }
		case "tab":
			m.inputs[m.focus].Blur()
			m.focus = (m.focus + 1) % pfCount
			m.inputs[m.focus].Focus()
			// scroll window
			if m.focus >= m.offset+8 {
				m.offset = m.focus - 7
			} else if m.focus < m.offset {
				m.offset = m.focus
			}
			return m, nil
		case "shift+tab":
			m.inputs[m.focus].Blur()
			m.focus = (m.focus - 1 + pfCount) % pfCount
			m.inputs[m.focus].Focus()
			if m.focus < m.offset {
				m.offset = m.focus
			} else if m.focus >= m.offset+8 {
				m.offset = m.focus - 7
			}
			return m, nil
		case "enter":
			if m.loading {
				return m, nil
			}
			m.err = ""
			m.msg = ""
			m.loading = true
			return m, func() tea.Msg {
				p := client.Profile{}
				p.Name = m.inputs[pfName].Value()
				p.Age, _ = strconv.Atoi(m.inputs[pfAge].Value())
				p.Sex = m.inputs[pfSex].Value()
				p.HeightCm, _ = strconv.ParseFloat(m.inputs[pfHeightCm].Value(), 64)
				p.Activity = m.inputs[pfActivity].Value()
				p.ExerciseFreq, _ = strconv.Atoi(m.inputs[pfExerciseFreq].Value())
				p.RunningKm, _ = strconv.ParseFloat(m.inputs[pfRunningKm].Value(), 64)
				p.IsLifter = strings.ToLower(m.inputs[pfIsLifter].Value()) == "true"
				p.Goal = m.inputs[pfGoal].Value()
				p.PrioritizeCarbs = strings.ToLower(m.inputs[pfPrioritizeCarbs].Value()) == "true"
				p.BfPct, _ = strconv.ParseFloat(m.inputs[pfBfPct].Value(), 64)
				p.HRRest, _ = strconv.Atoi(m.inputs[pfHRRest].Value())
				p.HRMax, _ = strconv.Atoi(m.inputs[pfHRMax].Value())
				p.GripWeight, _ = strconv.ParseFloat(m.inputs[pfGripWeight].Value(), 64)
				p.TDEELookbackDays, _ = strconv.Atoi(m.inputs[pfTDEELookback].Value())
				p.SleepQualityMax, _ = strconv.ParseFloat(m.inputs[pfSleepQualityMax].Value(), 64)
				p.Units = m.inputs[pfUnits].Value()
				err := m.client.UpdateProfile(p)
				return profileLoadedMsg{err: err}
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case profileLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		p := msg.p
		// if this is a save result (p is zero value), just show success
		if p.Name == "" && p.Age == 0 && p.Units == "" {
			m.msg = "Profile saved"
			return m, nil
		}
		// populate inputs from loaded profile
		m.inputs[pfName].SetValue(p.Name)
		m.inputs[pfAge].SetValue(fmt.Sprintf("%d", p.Age))
		m.inputs[pfSex].SetValue(p.Sex)
		m.inputs[pfHeightCm].SetValue(fmt.Sprintf("%.1f", p.HeightCm))
		m.inputs[pfActivity].SetValue(p.Activity)
		m.inputs[pfExerciseFreq].SetValue(fmt.Sprintf("%d", p.ExerciseFreq))
		m.inputs[pfRunningKm].SetValue(fmt.Sprintf("%.1f", p.RunningKm))
		m.inputs[pfIsLifter].SetValue(fmt.Sprintf("%v", p.IsLifter))
		m.inputs[pfGoal].SetValue(p.Goal)
		m.inputs[pfPrioritizeCarbs].SetValue(fmt.Sprintf("%v", p.PrioritizeCarbs))
		m.inputs[pfBfPct].SetValue(fmt.Sprintf("%.1f", p.BfPct))
		m.inputs[pfHRRest].SetValue(fmt.Sprintf("%d", p.HRRest))
		m.inputs[pfHRMax].SetValue(fmt.Sprintf("%d", p.HRMax))
		m.inputs[pfGripWeight].SetValue(fmt.Sprintf("%.2f", p.GripWeight))
		m.inputs[pfTDEELookback].SetValue(fmt.Sprintf("%d", p.TDEELookbackDays))
		m.inputs[pfSleepQualityMax].SetValue(fmt.Sprintf("%.1f", p.SleepQualityMax))
		m.inputs[pfUnits].SetValue(p.Units)
		return m, nil
	}
	for i := range m.inputs {
		m.inputs[i], _ = m.inputs[i].Update(msg)
	}
	return m, nil
}

func (m *ProfileModel) View() string {
	s := styles.Title.Render("Profile Settings") + "\n\n"
	if m.loading {
		s += m.spin.View() + " Loading...\n"
		return s
	}
	if m.err != "" {
		s += styles.Error.Render(m.err) + "\n"
	}
	// show a window of 8 fields at a time (scrollable)
	end := m.offset + 8
	if end > pfCount {
		end = pfCount
	}
	for i := m.offset; i < end; i++ {
		label := styles.Label.Render(profileLabels[i] + ": ")
		if i == m.focus {
			s += label + m.inputs[i].View() + " <\n"
		} else {
			s += label + m.inputs[i].View() + "\n"
		}
	}
	s += fmt.Sprintf("\n(%d/%d fields — Tab/Shift+Tab to navigate)\n", m.focus+1, pfCount)
	if m.msg != "" {
		s += styles.Success.Render(m.msg) + "\n"
	}
	s += styles.Help.Render("Tab/Shift+Tab: navigate • Enter: save • Esc: menu")
	return s
}

// small helpers used by other screens too — keep them here
func strconvParseFloat(s string) (float64, error) { return strconv.ParseFloat(s, 64) }
func strconvAtoi(s string) (int, error)           { return strconv.Atoi(s) }
