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

type LogMeal struct {
	client  *client.Client
	mode    string // manual or ai
	inputs  []textinput.Model
	ta      textinput.Model // used for AI textarea
	loading bool
	spin    spinner.Model
	err     string
}

func NewLogMeal(c *client.Client) tea.Model {
	l := &LogMeal{client: c, mode: "manual"}
	date := textinput.New()
	date.Placeholder = "date"
	date.SetValue(time.Now().Format("2006-01-02"))
	calories := textinput.New()
	calories.Placeholder = "calories"
	prot := textinput.New()
	prot.Placeholder = "protein_g"
	carbs := textinput.New()
	carbs.Placeholder = "carbs_g"
	fat := textinput.New()
	fat.Placeholder = "fat_g"
	fiber := textinput.New()
	fiber.Placeholder = "fiber_g"
	water := textinput.New()
	water.Placeholder = "water_ml"
	notes := textinput.New()
	notes.Placeholder = "meal notes"
	l.inputs = []textinput.Model{date, calories, prot, carbs, fat, fiber, water, notes}
	l.ta = textinput.New()
	l.ta.Placeholder = "Describe your meal..."
	l.spin = spinner.New()
	l.spin.Spinner = spinner.Dot
	return l
}

type parsedMealMsg struct {
	pm  client.ParsedMeal
	err error
}

func (l *LogMeal) Init() tea.Cmd { return textinput.Blink }

func (l *LogMeal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			if l.mode == "manual" {
				l.mode = "ai"
			} else {
				l.mode = "manual"
			}
			return l, nil
		case "esc":
			return l, func() tea.Msg { return msgs.NavigateMsg{Screen: "menu"} }
		case "q":
			return l, tea.Quit
		case "enter":
			if l.mode == "manual" {
				var nl client.NutritionLog
				nl.Date = l.inputs[0].Value()
				nl.Calories, _ = strconv.ParseFloat(l.inputs[1].Value(), 64)
				nl.ProteinG, _ = strconv.ParseFloat(l.inputs[2].Value(), 64)
				nl.CarbsG, _ = strconv.ParseFloat(l.inputs[3].Value(), 64)
				nl.FatG, _ = strconv.ParseFloat(l.inputs[4].Value(), 64)
				nl.FiberG, _ = strconv.ParseFloat(l.inputs[5].Value(), 64)
				nl.WaterMl, _ = strconv.ParseFloat(l.inputs[6].Value(), 64)
				nl.MealNotes = l.inputs[7].Value()
				l.loading = true
				return l, func() tea.Msg {
					err := l.client.PostNutritionLog(nl)
					return parsedMealMsg{err: err}
				}
			} else {
				l.loading = true
				return l, func() tea.Msg {
					pm, err := l.client.ParseMeal(l.ta.Value())
					return parsedMealMsg{pm: pm, err: err}
				}
			}
		}
	case parsedMealMsg:
		l.loading = false
		if msg.err != nil {
			if msg.err == client.ErrUnauthorized {
				return l, func() tea.Msg { return msgs.NavigateMsg{Screen: "login"} }
			}
			l.err = msg.err.Error()
			return l, nil
		}
		if l.mode == "ai" {
			pm := msg.pm
			nl := client.NutritionLog{
				Date:      time.Now().Format("2006-01-02"),
				Calories:  pm.Calories,
				ProteinG:  pm.ProteinG,
				CarbsG:    pm.CarbsG,
				FatG:      pm.FatG,
				FiberG:    pm.FiberG,
				WaterMl:   pm.WaterMl,
				MealNotes: pm.MealNotes,
			}
			if err := l.client.PostNutritionLog(nl); err != nil {
				l.err = err.Error()
				return l, nil
			}
			l.err = "Saved."
			return l, nil
		}
		if msg.err == nil {
			l.err = "Saved."
		}
		return l, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		l.spin, cmd = l.spin.Update(msg)
		return l, cmd
	}

	if l.mode == "manual" {
		for i := range l.inputs {
			l.inputs[i], _ = l.inputs[i].Update(msg)
		}
	} else {
		l.ta, _ = l.ta.Update(msg)
	}
	return l, nil
}

func (l *LogMeal) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Log Meal") + "\n")
	b.WriteString(styles.Subtitle.Render(fmt.Sprintf("Mode: %s (Tab to toggle)\n\n", l.mode)))
	if l.loading {
		b.WriteString(l.spin.View() + " Working...\n")
	}
	if l.mode == "manual" {
		for _, in := range l.inputs {
			b.WriteString(in.View() + "\n")
		}
	} else {
		b.WriteString(l.ta.View() + "\n")
	}
	if l.err != "" {
		b.WriteString(styles.Error.Render(l.err) + "\n")
	}
	b.WriteString(styles.Help.Render("Tab toggle mode • Enter submit • Esc menu • q quit"))
	return b.String()
}
