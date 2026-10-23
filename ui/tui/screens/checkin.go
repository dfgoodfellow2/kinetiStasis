package screens

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/models"
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

	preview       models.CheckinPreview
	previewLoaded bool
	previewErr    string
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

type checkinPreviewMsg struct {
	preview models.CheckinPreview
	err     error
}

type checkinResultMsg struct {
	msg string
	err error
}

func loadCheckinPreview(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		p, err := c.GetCheckin()
		return checkinPreviewMsg{preview: p, err: err}
	}
}

func (m *CheckInModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, spinner.Tick, loadCheckinPreview(m.client))
}

// submitCheckin saves the biometric entry and, when the preview allows it,
// records the weekly check-in and applies the recommended calorie target.
func submitCheckin(c *client.Client, b models.BiometricLog, preview models.CheckinPreview, loaded bool) tea.Cmd {
	return func() tea.Msg {
		if err := c.PostBiometric(b); err != nil {
			return checkinResultMsg{err: err}
		}

		if !loaded || !preview.CanCheckIn {
			if loaded {
				wait := preview.DaysSinceLastCheckIn
				if wait > 0 && wait < math.MaxInt32 {
					return checkinResultMsg{msg: fmt.Sprintf("Biometrics saved — check-in not available yet (%d day(s) since last)", wait)}
				}
			}
			return checkinResultMsg{msg: "Biometrics saved — no check-in recorded"}
		}

		weightBefore := preview.WeightEnd
		weightAfter := b.WeightKg
		if weightAfter <= 0 {
			weightAfter = weightBefore
		}
		checkin := models.CheckInLog{
			CheckInDate:    b.Date,
			WeightBefore:   weightBefore,
			WeightAfter:    weightAfter,
			CaloriesBefore: preview.CurrentCalories,
			CaloriesAfter:  float64(preview.RecommendedCalories),
			Reason:         "TUI weekly check-in",
		}
		res, err := c.CreateCheckin(checkin)
		if err != nil {
			return checkinResultMsg{msg: "Biometrics saved, but check-in failed", err: err}
		}
		if res.CaloriesAfter > 0 {
			return checkinResultMsg{msg: fmt.Sprintf("Check-in recorded — targets updated to %.0f kcal", res.CaloriesAfter)}
		}
		return checkinResultMsg{msg: "Check-in recorded — targets unchanged"}
	}
}

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
			date := m.inputs[0].Value()
			weight, _ := strconv.ParseFloat(m.inputs[1].Value(), 64)
			waist, _ := strconv.ParseFloat(m.inputs[2].Value(), 64)
			grip, _ := strconv.ParseFloat(m.inputs[3].Value(), 64)
			bolt, _ := strconv.ParseFloat(m.inputs[4].Value(), 64)
			sleepH, _ := strconv.ParseFloat(m.inputs[5].Value(), 64)
			sq, _ := strconv.ParseFloat(m.inputs[6].Value(), 64)
			subj, _ := strconv.Atoi(m.inputs[7].Value())
			notes := m.inputs[8].Value()
			b := models.BiometricLog{
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
			return m, submitCheckin(m.client, b, m.preview, m.previewLoaded)
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case checkinPreviewMsg:
		m.previewLoaded = true
		if msg.err != nil {
			m.previewErr = msg.err.Error()
			return m, nil
		}
		m.preview = msg.preview
		m.previewErr = ""
		return m, nil
	case checkinResultMsg:
		m.loading = false
		m.err = ""
		m.msg = msg.msg
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		// reset inputs except date
		for i := 1; i < len(m.inputs); i++ {
			m.inputs[i].SetValue("")
		}
		// Refresh the preview so the next view reflects the new target.
		return m, loadCheckinPreview(m.client)
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

func (m *CheckInModel) previewView() string {
	if !m.previewLoaded {
		return m.spin.View() + " Loading check-in preview...\n\n"
	}
	if m.previewErr != "" {
		return styles.Warning.Render("Preview unavailable: "+m.previewErr) + "\n\n"
	}

	p := m.preview
	var s string
	if p.CanCheckIn {
		s += styles.Success.Render("Check-in available") + "\n"
	} else {
		status := "Check-in not available yet"
		if p.DaysSinceLastCheckIn > 0 && p.DaysSinceLastCheckIn < math.MaxInt32 {
			status += fmt.Sprintf(" (%d day(s) since last)", p.DaysSinceLastCheckIn)
		}
		s += styles.Warning.Render(status) + "\n"
	}

	if p.ObservedTDEE > 0 || p.EstimatedTDEE > 0 {
		tdee := fmt.Sprintf("TDEE: observed %.0f kcal", p.ObservedTDEE)
		if p.ObservedTDEE <= 0 {
			tdee = fmt.Sprintf("TDEE: estimated %.0f kcal", p.EstimatedTDEE)
		}
		if p.Confidence != "" {
			tdee += fmt.Sprintf(" (%s confidence, %d days)", p.Confidence, p.DaysOfData)
		}
		s += styles.Label.Render(tdee) + "\n"
	}

	if p.WeightStart > 0 || p.WeightEnd > 0 {
		s += fmt.Sprintf("Weight trend: %.1f → %.1f kg (%+.1f kg)\n", p.WeightStart, p.WeightEnd, p.WeightChange)
	}

	if p.CurrentCalories > 0 || p.RecommendedCalories > 0 {
		s += fmt.Sprintf("Calories: %.0f → %d kcal (%+.0f)\n", p.CurrentCalories, p.RecommendedCalories, p.CalorieAdjustment)
	}

	if p.LowConfidenceWarning {
		s += styles.Warning.Render("Low confidence: recommendation is extrapolated from limited data") + "\n"
	}

	if p.CanCheckIn {
		s += styles.Help.Render(fmt.Sprintf("Enter submits biometrics and applies the recommended %d kcal target", p.RecommendedCalories)) + "\n"
	} else {
		s += styles.Help.Render("Enter saves biometrics only; no target change") + "\n"
	}

	return s + "\n"
}

func (m *CheckInModel) View() string {
	s := styles.Title.Render("Daily Check-In") + "\n\n"
	s += m.previewView()
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
