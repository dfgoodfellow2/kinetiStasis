package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/msgs"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/styles"
)

type workoutsLoadedMsg struct {
	workouts []client.WorkoutEntry
	err      error
}

type WorkoutHistoryModel struct {
	client   *client.Client
	loading  bool
	spin     spinner.Model
	err      string
	workouts []client.WorkoutEntry
	offset   int
}

func NewWorkoutHistory(c *client.Client) *WorkoutHistoryModel {
	m := &WorkoutHistoryModel{client: c, loading: true}
	m.spin = spinner.New()
	m.spin.Spinner = spinner.Dot
	return m
}

func loadWorkouts(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		to := time.Now().Format("2006-01-02")
		from := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
		w, err := c.ListWorkouts(from, to)
		return workoutsLoadedMsg{workouts: w, err: err}
	}
}

func (m *WorkoutHistoryModel) Init() tea.Cmd { return tea.Batch(spinner.Tick, loadWorkouts(m.client)) }

func (m *WorkoutHistoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return msgs.NavigateMsg{Screen: "menu"} }
		case "r":
			m.loading = true
			return m, loadWorkouts(m.client)
		case "up", "k":
			if m.offset > 0 {
				m.offset--
			}
		case "down", "j":
			if m.offset < len(m.workouts)-1 {
				m.offset++
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case workoutsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.workouts = msg.workouts
		return m, nil
	}
	return m, nil
}

func (m *WorkoutHistoryModel) View() string {
	s := styles.Title.Render("Workout History (30d)") + "\n\n"
	if m.loading {
		return s + m.spin.View() + " Loading..."
	}
	if m.err != "" {
		return s + styles.Error.Render(m.err)
	}
	if len(m.workouts) == 0 {
		return s + "No workouts logged yet.\n\n" + styles.Help.Render("r: refresh • Esc: menu")
	}

	var last string
	shown := 0
	for i, w := range m.workouts {
		if i < m.offset {
			continue
		}
		if shown >= 8 {
			break
		}
		if last != w.Date {
			s += "\n" + styles.Label.Render(w.Date) + "\n"
			last = w.Date
		}
		// Session header
		dur := fmt.Sprintf("%.0f", w.DurationMin)
		cal := ""
		if w.CaloriesBurned > 0 {
			cal = fmt.Sprintf(", ~%.0f kcal", w.CaloriesBurned)
		}
		mwv := ""
		if w.MWV > 0 {
			mwv = fmt.Sprintf(", MWV %.0f", w.MWV)
		}
		s += fmt.Sprintf("  [%s] %s — %s min%s%s\n", w.Slot, w.Title, dur, cal, mwv)

		// Exercise list
		for _, ex := range w.Exercises {
			label := ex.Name
			if ex.Category != "" {
				label += " (" + ex.Category + ")"
			}
			s += fmt.Sprintf("    • %s\n", label)
			if len(ex.Sets) > 0 {
				var setParts []string
				for _, set := range ex.Sets {
					if set.LoadKg > 0 {
						// show one decimal for kg unless it's integer
						setParts = append(setParts, fmt.Sprintf("%dx@%.1fkg", set.Reps, set.LoadKg))
					} else if set.Reps > 0 {
						setParts = append(setParts, fmt.Sprintf("%d reps", set.Reps))
					}
				}
				if len(setParts) > 0 {
					s += fmt.Sprintf("      %s\n", strings.Join(setParts, "  "))
				}
			} else if ex.LoadRaw != "" {
				s += fmt.Sprintf("      %s\n", ex.LoadRaw)
			}
			if ex.Notes != "" {
				s += fmt.Sprintf("      note: %s\n", ex.Notes)
			}
		}
		shown++
	}

	s += "\n" + styles.Help.Render("↑/k ↓/j: scroll • r: refresh • Esc: menu")
	return s
}
