package screens

import (
	"fmt"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/msgs"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/styles"
)

type bfLoadedMsg struct {
	result client.BodyFatResult
	err    error
}

type BodyFatModel struct {
	client  *client.Client
	loading bool
	spin    spinner.Model
	err     string
	res     client.BodyFatResult
}

func NewBodyFat(c *client.Client) *BodyFatModel {
	m := &BodyFatModel{client: c, loading: true}
	m.spin = spinner.New()
	m.spin.Spinner = spinner.Dot
	return m
}

func loadBF(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		r, err := c.GetBodyFat("navy")
		return bfLoadedMsg{result: r, err: err}
	}
}

func (m *BodyFatModel) Init() tea.Cmd { return tea.Batch(spinner.Tick, loadBF(m.client)) }

func (m *BodyFatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			m.loading = true
			return m, loadBF(m.client)
		case "esc":
			return m, func() tea.Msg { return msgs.NavigateMsg{Screen: "menu"} }
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case bfLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.res = msg.result
		return m, nil
	}
	return m, nil
}

func (m *BodyFatModel) View() string {
	s := styles.Title.Render("Body Fat Calculator (Navy)") + "\n\n"
	if m.loading {
		return s + m.spin.View() + " Loading..."
	}
	if m.err != "" {
		return s + styles.Error.Render(m.err)
	}
	col := styles.Success
	if m.res.BfPct >= 25 {
		col = styles.Error
	} else if m.res.BfPct >= 15 {
		col = styles.Warning
	}
	body := fmt.Sprintf("Body Fat:   %.1f%%\nLean Mass:  %.1f lbs\nFat Mass:   %.1f lbs", m.res.BfPct, m.res.LeanMassKg*2.20462, m.res.FatMassKg*2.20462)
	s += styles.Box.Render(col.Render(body))
	s += "\n" + styles.Help.Render("r: refresh • Esc: menu")
	return s
}
