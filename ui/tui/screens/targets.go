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

type targetsLoadedMsg struct {
	targets client.Targets
	tdee    client.TDEEResult
	err     error
}

type targetsSavedMsg struct{ err error }

type TargetsModel struct {
	client  *client.Client
	loading bool
	spin    spinner.Model
	err     string
	msg     string
	inputs  []textinput.Model
	focus   int
	tdee    client.TDEEResult
}

func NewTargets(c *client.Client) *TargetsModel {
	m := &TargetsModel{client: c, loading: true}
	m.spin = spinner.New()
	m.spin.Spinner = spinner.Dot

	labels := []string{
		"Calories (kcal)",
		"Protein (g)",
		"Carbs (g)",
		"Fat (g)",
		"Fiber (g)",
		"Water (ml)",
	}
	for _, l := range labels {
		ti := textinput.New()
		ti.Placeholder = l
		m.inputs = append(m.inputs, ti)
	}
	m.inputs[0].Focus()
	return m
}

func loadTargets(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		t, err := c.GetTargets()
		if err != nil {
			return targetsLoadedMsg{err: err}
		}
		tdee, _ := c.GetTDEE(0)
		return targetsLoadedMsg{targets: t, tdee: tdee}
	}
}

func (m *TargetsModel) Init() tea.Cmd {
	return tea.Batch(spinner.Tick, loadTargets(m.client))
}

func (m *TargetsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return msgs.NavigateMsg{Screen: "menu"} }
		case "tab":
			m.inputs[m.focus].Blur()
			m.focus = (m.focus + 1) % len(m.inputs)
			m.inputs[m.focus].Focus()
			return m, nil
		case "u":
			// Apply TDEE as calorie target
			if m.tdee.ObservedTDEE > 0 {
				m.inputs[0].SetValue(fmt.Sprintf("%.0f", m.tdee.ObservedTDEE))
				m.msg = fmt.Sprintf("Applied observed TDEE (%.0f kcal) as calorie target", m.tdee.ObservedTDEE)
			} else if m.tdee.EstimatedTDEE > 0 {
				m.inputs[0].SetValue(fmt.Sprintf("%.0f", m.tdee.EstimatedTDEE))
				m.msg = fmt.Sprintf("Applied estimated TDEE (%.0f kcal) as calorie target", m.tdee.EstimatedTDEE)
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
				t := client.Targets{}
				t.Calories, _ = strconv.ParseFloat(m.inputs[0].Value(), 64)
				t.ProteinG, _ = strconv.ParseFloat(m.inputs[1].Value(), 64)
				t.CarbsG, _ = strconv.ParseFloat(m.inputs[2].Value(), 64)
				t.FatG, _ = strconv.ParseFloat(m.inputs[3].Value(), 64)
				t.FiberG, _ = strconv.ParseFloat(m.inputs[4].Value(), 64)
				t.WaterMl, _ = strconv.ParseFloat(m.inputs[5].Value(), 64)
				err := m.client.PutTargets(t)
				return targetsSavedMsg{err: err}
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case targetsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		t := msg.targets
		m.tdee = msg.tdee
		m.inputs[0].SetValue(fmt.Sprintf("%.0f", t.Calories))
		m.inputs[1].SetValue(fmt.Sprintf("%.0f", t.ProteinG))
		m.inputs[2].SetValue(fmt.Sprintf("%.0f", t.CarbsG))
		m.inputs[3].SetValue(fmt.Sprintf("%.0f", t.FatG))
		m.inputs[4].SetValue(fmt.Sprintf("%.0f", t.FiberG))
		m.inputs[5].SetValue(fmt.Sprintf("%.0f", t.WaterMl))
		return m, nil
	case targetsSavedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.msg = "Targets saved"
		}
		return m, nil
	}
	for i := range m.inputs {
		m.inputs[i], _ = m.inputs[i].Update(msg)
	}
	return m, nil
}

func (m *TargetsModel) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Nutrition Targets") + "\n\n")

	if m.loading {
		b.WriteString(m.spin.View() + " Loading...\n")
		return b.String()
	}

	// TDEE suggestion
	if m.tdee.ObservedTDEE > 0 || m.tdee.EstimatedTDEE > 0 {
		tdeeInfo := "TDEE suggestion: "
		if m.tdee.ObservedTDEE > 0 {
			tdeeInfo += fmt.Sprintf("observed %.0f kcal", m.tdee.ObservedTDEE)
		} else {
			tdeeInfo += fmt.Sprintf("estimated %.0f kcal", m.tdee.EstimatedTDEE)
		}
		if m.tdee.Confidence != "" {
			tdeeInfo += fmt.Sprintf(" (%s confidence, %d days)", m.tdee.Confidence, m.tdee.DaysOfData)
		}
		b.WriteString(styles.Label.Render(tdeeInfo) + "\n")
		b.WriteString(styles.Help.Render("u: apply TDEE as calorie target") + "\n")
		b.WriteString("\n")
	}

	for _, in := range m.inputs {
		b.WriteString(in.View() + "\n")
	}

	b.WriteString("\n")
	if m.err != "" {
		b.WriteString(styles.Error.Render(m.err) + "\n")
	}
	if m.msg != "" {
		b.WriteString(styles.Success.Render(m.msg) + "\n")
	}

	b.WriteString(styles.Help.Render("Tab: next field • u: use TDEE • Enter: save • Esc: menu"))
	return b.String()
}
