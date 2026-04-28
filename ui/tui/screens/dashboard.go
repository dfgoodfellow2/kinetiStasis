package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/msgs"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/styles"
)

type Dashboard struct {
	client  *client.Client
	data    client.DashboardData
	loading bool
	spin    spinner.Model
	err     string
}

func NewDashboard(c *client.Client) tea.Model {
	d := &Dashboard{client: c, loading: true}
	d.spin = spinner.New()
	d.spin.Spinner = spinner.Line
	return d
}

type dashboardLoadedMsg struct {
	data client.DashboardData
	err  error
}

func loadDashboard(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		data, err := c.GetDashboard()
		return dashboardLoadedMsg{data: data, err: err}
	}
}

func (d *Dashboard) Init() tea.Cmd { return tea.Batch(spinner.Tick, loadDashboard(d.client)) }

func (d *Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			d.loading = true
			return d, loadDashboard(d.client)
		case "m", "esc":
			return d, func() tea.Msg { return msgs.NavigateMsg{Screen: "menu"} }
		case "q", "ctrl+c":
			return d, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		d.spin, cmd = d.spin.Update(msg)
		return d, cmd
	case dashboardLoadedMsg:
		d.loading = false
		if msg.err != nil {
			if msg.err == client.ErrUnauthorized {
				return d, func() tea.Msg { return msgs.NavigateMsg{Screen: "login"} }
			}
			d.err = msg.err.Error()
			return d, nil
		}
		d.data = msg.data
		return d, nil
	}
	return d, nil
}

func (d *Dashboard) View() string {
	var b strings.Builder
	title := fmt.Sprintf("Dashboard — %s", time.Now().Format("2006-01-02"))
	b.WriteString(styles.Title.Render(title) + "\n")
	if d.loading {
		b.WriteString(d.spin.View() + " Loading...\n")
		return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
	}
	if d.err != "" {
		b.WriteString(styles.Error.Render(d.err) + "\n")
	}

	today := d.data.Today
	con := today.Consumed
	tgt := today.Targets

	// ── Today's intake box ───────────────────────────────────────────
	todayBox := fmt.Sprintf("Date: %s\n", today.Date)
	todayBox += fmt.Sprintf("Calories: %s %.0f/%.0f kcal\n", progressBar(con.Calories, tgt.Calories, 20), con.Calories, tgt.Calories)
	todayBox += fmt.Sprintf("Protein:  %s %.0fg/%.0fg\n", progressBar(con.ProteinG, tgt.ProteinG, 16), con.ProteinG, tgt.ProteinG)
	todayBox += fmt.Sprintf("Carbs:    %s %.0fg/%.0fg\n", progressBar(con.CarbsG, tgt.CarbsG, 16), con.CarbsG, tgt.CarbsG)
	todayBox += fmt.Sprintf("Fat:      %s %.0fg/%.0fg\n", progressBar(con.FatG, tgt.FatG, 16), con.FatG, tgt.FatG)
	todayBox += fmt.Sprintf("Left:     %.0f kcal  (%.0f%%)\n", today.CaloriesLeft, today.ProgressPct*100)

	// ── Today checklist ──────────────────────────────────────────────
	checkWeight := "☐ Weight"
	checkFood := "☐ Food"
	checkSleep := "☐ Sleep"
	checkWorkout := "☐ Workout"
	if d.data.TodayBio != nil {
		if d.data.TodayBio.WeightKg > 0 {
			checkWeight = fmt.Sprintf("✓ Weight (%.1f kg)", d.data.TodayBio.WeightKg)
		}
		if d.data.TodayBio.SleepHours > 0 {
			checkSleep = fmt.Sprintf("✓ Sleep (%.1f hrs)", d.data.TodayBio.SleepHours)
		}
	}
	if con.Calories > 0 {
		checkFood = "✓ Food"
	}
	if d.data.WorkoutToday {
		checkWorkout = "✓ Workout"
	}
	todayBox += fmt.Sprintf("\n%s  %s\n%s  %s\n", checkWeight, checkFood, checkSleep, checkWorkout)

	// ── TDEE box ─────────────────────────────────────────────────────
	tdee := d.data.TDEE
	tdeeBox := "TDEE\n"
	if tdee.ObservedTDEE > 0 {
		tdeeBox += fmt.Sprintf("Observed: %.0f kcal\n", tdee.ObservedTDEE)
	}
	if tdee.EstimatedTDEE > 0 {
		tdeeBox += fmt.Sprintf("Estimated: %.0f kcal\n", tdee.EstimatedTDEE)
	}
	tdeeBox += fmt.Sprintf("Confidence: %s\n", tdee.Confidence)
	tdeeBox += fmt.Sprintf("Data days: %d\n", tdee.DaysOfData)
	tdeeBox += fmt.Sprintf("Method: %s\n", tdee.Method)

	// ── Readiness box ────────────────────────────────────────────────
	r := d.data.Readiness
	readLine := fmt.Sprintf("Readiness: %s — %s (%.0f)", strings.ToUpper(r.Level), r.Message, r.Score)
	var readStyled string
	switch r.Level {
	case "green":
		readStyled = styles.Success.Render(readLine)
	case "yellow":
		readStyled = styles.Warning.Render(readLine)
	default:
		readStyled = styles.Error.Render(readLine)
	}
	velLine := fmt.Sprintf("Trend: %s (%+.2f)", r.VelocityTrend, r.VelocityDelta)
	readBox := readStyled + "\n" + velLine
	if len(r.Notes) > 0 {
		readBox += "\n" + strings.Join(r.Notes, " · ")
	}

	// ── Weekly stats box ─────────────────────────────────────────────
	w := d.data.WeeklyStats
	weeklyBox := fmt.Sprintf("Weekly (7d)\n")
	weeklyBox += fmt.Sprintf("Avg cal:     %.0f kcal\n", w.AvgCalories)
	weeklyBox += fmt.Sprintf("Avg protein: %.0fg\n", w.AvgProteinG)
	weeklyBox += fmt.Sprintf("Avg weight:  %.1f kg\n", w.AvgWeightKg)
	weeklyBox += fmt.Sprintf("Avg sleep:   %.1f hrs\n", w.AvgSleepHours)
	weeklyBox += fmt.Sprintf("Workouts:    %d\n", w.TotalWorkouts)

	// ── Recent weights ───────────────────────────────────────────────
	wt := "Weight trend\n"
	for i, e := range d.data.WeightTrend {
		if i >= 7 {
			break
		}
		wt += fmt.Sprintf("  %s: %.1f kg\n", e.Date, e.WeightKg)
	}

	// ── Layout ───────────────────────────────────────────────────────
	left := styles.Box.Render(todayBox)
	right := styles.Box.Render(tdeeBox)
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, right) + "\n\n")
	b.WriteString(styles.Box.Render(readBox) + "\n")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, styles.Box.Render(weeklyBox), styles.Box.Render(wt)) + "\n")

	b.WriteString(styles.Help.Render("r: refresh • Esc: menu • q: quit"))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func progressBar(current, max float64, width int) string {
	if max <= 0 {
		max = 1
	}
	pct := current / max
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	bar := "["
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := filled; i < width; i++ {
		bar += "░"
	}
	bar += "]"
	return bar
}
