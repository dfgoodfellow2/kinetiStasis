package screens

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/msgs"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/styles"
)

type workoutParsedMsg struct {
	parsed client.ParsedWorkout
	err    error
}

type workoutSavedMsg struct{ err error }

const (
	wlModeAI     = 0
	wlModeSimple = 1
)

type WorkoutLogModel struct {
	client  *client.Client
	mode    int
	loading bool
	spin    spinner.Model
	err     string
	msg     string

	aiInput textinput.Model
	parsed  *client.ParsedWorkout
	aiDate  string

	simpleInputs []textinput.Model
	simpleIdx    int
}

func NewWorkoutLog(c *client.Client) *WorkoutLogModel {
	m := &WorkoutLogModel{client: c, mode: wlModeAI}
	m.spin = spinner.New()
	m.spin.Spinner = spinner.Dot
	m.aiDate = time.Now().Format("2006-01-02")

	ai := textinput.New()
	ai.Placeholder = "Describe your workout (e.g. '3x5 squat 100kg, 3x8 bench 70kg')"
	ai.Focus()
	m.aiInput = ai

	labels := []string{"Date (YYYY-MM-DD)", "Slot (1/2/A/B)", "Title", "Duration (min)", "Notes"}
	for i, l := range labels {
		ti := textinput.New()
		ti.Placeholder = l
		if i == 0 {
			ti.SetValue(time.Now().Format("2006-01-02"))
		}
		m.simpleInputs = append(m.simpleInputs, ti)
	}
	return m
}

func (m *WorkoutLogModel) Init() tea.Cmd { return textinput.Blink }

func (m *WorkoutLogModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return msgs.NavigateMsg{Screen: "menu"} }
		case "ctrl+t":
			m.mode = (m.mode + 1) % 2
			m.err = ""
			m.msg = ""
			m.parsed = nil
			return m, nil
		case "enter":
			if m.loading {
				return m, nil
			}
			m.err = ""
			m.msg = ""
			if m.mode == wlModeAI {
				if m.parsed != nil {
					p := m.parsed
					m.loading = true
					parsed := *p
					date := m.aiDate
					return m, func() tea.Msg {
						w := client.WorkoutEntry{
							Date:           date,
							Slot:           parsed.Slot,
							Title:          parsed.Title,
							DurationMin:    parsed.DurationMin,
							CaloriesBurned: parsed.CaloriesBurned,
							Exercises:      parsed.Exercises,
							RawNotes:       parsed.Notes,
						}
						if w.Slot == "" {
							w.Slot = "1"
						}
						err := m.client.PostWorkout(w)
						return workoutSavedMsg{err: err}
					}
				}
				text := m.aiInput.Value()
				if strings.TrimSpace(text) == "" {
					m.err = "Enter a workout description first"
					return m, nil
				}
				m.loading = true
				return m, func() tea.Msg {
					parsed, err := m.client.ParseWorkout(text, "ai")
					return workoutParsedMsg{parsed: parsed, err: err}
				}
			}
			m.loading = true
			return m, func() tea.Msg {
				dur, _ := strconv.ParseFloat(m.simpleInputs[3].Value(), 64)
				slot := m.simpleInputs[1].Value()
				if slot == "" {
					slot = "1"
				}
				w := client.WorkoutEntry{
					Date:        m.simpleInputs[0].Value(),
					Slot:        slot,
					Title:       m.simpleInputs[2].Value(),
					DurationMin: dur,
					RawNotes:    m.simpleInputs[4].Value(),
				}
				err := m.client.PostWorkout(w)
				return workoutSavedMsg{err: err}
			}
		case "tab":
			if m.mode == wlModeAI {
				return m, nil
			}
			m.simpleIdx = (m.simpleIdx + 1) % len(m.simpleInputs)
			for i := range m.simpleInputs {
				m.simpleInputs[i].Blur()
			}
			m.simpleInputs[m.simpleIdx].Focus()
			return m, nil
		case "ctrl+d":
			if m.mode == wlModeAI && m.parsed != nil {
				m.parsed = nil
				m.aiInput.SetValue("")
				m.aiInput.Focus()
				m.msg = ""
			}
			return m, nil
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case workoutParsedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		p := msg.parsed
		m.parsed = &p
		m.msg = "Preview ready — press Enter to save, Ctrl+D to discard"
		return m, nil
	case workoutSavedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.msg = "Workout saved!"
			m.parsed = nil
			m.aiInput.SetValue("")
			for i := 1; i < len(m.simpleInputs); i++ {
				m.simpleInputs[i].SetValue("")
			}
		}
		return m, nil
	}
	if m.mode == wlModeAI {
		m.aiInput, _ = m.aiInput.Update(msg)
	} else {
		for i := range m.simpleInputs {
			m.simpleInputs[i], _ = m.simpleInputs[i].Update(msg)
		}
	}
	return m, nil
}

func (m *WorkoutLogModel) View() string {
	modeLabel := "AI Parse"
	if m.mode == wlModeSimple {
		modeLabel = "Simple Form"
	}
	s := styles.Title.Render(fmt.Sprintf("Log Workout [%s]", modeLabel)) + "\n\n"

	if m.loading {
		s += m.spin.View() + " Working...\n"
		return s
	}

	if m.mode == wlModeAI {
		if m.parsed == nil {
			s += "Describe your workout in plain text:\n"
			s += m.aiInput.View() + "\n\n"
			s += styles.Help.Render("Enter: parse with AI • Ctrl+T: switch to simple form • Esc: menu")
		} else {
			p := m.parsed
			s += styles.Label.Render("Parsed Preview") + "\n"
			s += fmt.Sprintf("Title:    %s\n", p.Title)
			s += fmt.Sprintf("Slot:     %s\n", p.Slot)
			s += fmt.Sprintf("Duration: %.0f min\n", p.DurationMin)
			if p.CaloriesBurned > 0 {
				s += fmt.Sprintf("Calories: ~%.0f kcal\n", p.CaloriesBurned)
			}
			if p.Type != "" {
				s += fmt.Sprintf("Type:     %s", p.Type)
				if p.Style != "" {
					s += fmt.Sprintf(" / %s", p.Style)
				}
				s += "\n"
			}
			if p.RPE > 0 {
				s += fmt.Sprintf("RPE:      %.1f\n", p.RPE)
			}
			if len(p.Exercises) > 0 {
				s += fmt.Sprintf("\nExercises (%d):\n", len(p.Exercises))
				for _, ex := range p.Exercises {
					s += fmt.Sprintf("  • %s", ex.Name)
					if ex.Category != "" {
						s += fmt.Sprintf(" (%s)", ex.Category)
					}
					if len(ex.Sets) > 0 {
						var parts []string
						for _, set := range ex.Sets {
							if set.LoadKg > 0 {
								parts = append(parts, fmt.Sprintf("%dx@%.1fkg", set.Reps, set.LoadKg))
							} else if set.Reps > 0 {
								parts = append(parts, fmt.Sprintf("%d reps", set.Reps))
							}
						}
						if len(parts) > 0 {
							s += ": " + strings.Join(parts, ", ")
						}
					} else if ex.LoadRaw != "" {
						s += ": " + ex.LoadRaw
					}
					s += "\n"
				}
			}
			if p.Notes != "" {
				s += fmt.Sprintf("\nNotes: %s\n", p.Notes)
			}
			s += "\n" + styles.Help.Render("Enter: save • Ctrl+D: discard • Esc: menu")
		}
	} else {
		for i, in := range m.simpleInputs {
			if i == m.simpleIdx {
				s += in.View() + " <\n"
			} else {
				s += in.View() + "\n"
			}
		}
		s += "\n" + styles.Help.Render("Tab: next • Enter: save • Ctrl+T: switch to AI mode • Esc: menu")
	}

	if m.err != "" {
		s += "\n" + styles.Error.Render(m.err)
	}
	if m.msg != "" {
		s += "\n" + styles.Success.Render(m.msg)
	}
	return s
}
