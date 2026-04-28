package screens

import (
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/msgs"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/styles"
)

type exportDoneMsg struct {
	content string
	err     error
}

type ExportModel struct {
	kind    string
	format  string
	from    textinput.Model
	to      textinput.Model
	client  *client.Client
	loading bool
	spin    spinner.Model
	vp      viewport.Model
	content string
	err     string
}

func NewExport(c *client.Client) *ExportModel {
	m := &ExportModel{kind: "nutrition", format: "md", client: c}
	m.from = textinput.New()
	m.from.Placeholder = "from (YYYY-MM-DD)"
	m.from.SetValue(time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	m.to = textinput.New()
	m.to.Placeholder = "to (YYYY-MM-DD)"
	m.to.SetValue(time.Now().Format("2006-01-02"))
	m.spin = spinner.New()
	m.spin.Spinner = spinner.Dot
	m.vp = viewport.New(80, 20)
	return m
}

func (m *ExportModel) Init() tea.Cmd { return textinput.Blink }

func (m *ExportModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.loading && m.content != "" {
				m.loading = false
				m.content = ""
				return m, nil
			}
			return m, func() tea.Msg { return msgs.NavigateMsg{Screen: "menu"} }
		case "e":
			if m.kind == "nutrition" {
				m.kind = "workouts"
			} else if m.kind == "workouts" {
				m.kind = "combined"
			} else {
				m.kind = "nutrition"
			}
			return m, nil
		case "f":
			if m.format == "md" {
				m.format = "csv"
			} else {
				m.format = "md"
			}
			return m, nil
		case "enter":
			m.loading = true
			m.err = ""
			return m, func() tea.Msg {
				content, err := m.client.ExportContent(m.kind, m.from.Value(), m.to.Value(), m.format)
				return exportDoneMsg{content: content, err: err}
			}
		case "c":
			if m.content == "" {
				m.err = "No content to copy"
				return m, nil
			}
			// try clipboard
			var cmd *exec.Cmd
			if runtime.GOOS == "darwin" {
				cmd = exec.Command("pbcopy")
			} else {
				cmd = exec.Command("xclip", "-selection", "clipboard")
			}
			cmd.Stdin = strings.NewReader(m.content)
			if err := cmd.Run(); err != nil {
				m.err = "copy failed"
			} else {
				m.err = "Copied!"
			}
			return m, nil
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case exportDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.content = msg.content
		m.vp.SetContent(m.content)
		return m, nil
	}
	m.from, _ = m.from.Update(msg)
	m.to, _ = m.to.Update(msg)
	return m, nil
}

func (m *ExportModel) View() string {
	s := styles.Title.Render("Export Data") + "\n\n"
	s += "Kind: " + m.kind + " (e to cycle)\n"
	s += m.from.View() + "\n" + m.to.View() + "\n"
	if m.kind != "combined" {
		s += "Format: " + m.format + " (f to toggle)\n"
	}
	s += "\n"
	if m.loading {
		s += m.spin.View() + " Exporting...\n"
		return s
	}
	if m.err != "" {
		s += styles.Error.Render(m.err) + "\n"
	}
	if m.content != "" {
		s += m.vp.View() + "\n" + styles.Help.Render("c: copy • Esc: back")
	}
	s += styles.Help.Render("Enter: export • e: kind • f: format • Esc: menu")
	return s
}
