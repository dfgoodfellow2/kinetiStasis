package screens

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/msgs"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/styles"
)

type LoginModel struct {
	mode       string
	inputs     []textinput.Model
	focusIndex int
	client     *client.Client
	err        string
	msg        string
	loading    bool
	spin       spinner.Model
}

func NewLogin(c *client.Client) *LoginModel {
	l := &LoginModel{mode: "login", client: c}
	u := textinput.New()
	u.Placeholder = "username"
	u.Focus()
	e := textinput.New()
	e.Placeholder = "email"
	p := textinput.New()
	p.Placeholder = "password"
	p.EchoMode = textinput.EchoPassword
	p.EchoCharacter = '*'
	l.inputs = []textinput.Model{u, e, p}
	l.spin = spinner.New()
	l.spin.Spinner = spinner.Dot
	return l
}

type loginResultMsg struct{ err error }

func (l *LoginModel) Init() tea.Cmd { return textinput.Blink }

func (l *LoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return l, tea.Quit
		case "t":
			if l.mode == "login" {
				l.mode = "register"
			} else {
				l.mode = "login"
			}
			return l, nil
		case "tab":
			// in login mode skip email (index 1)
			if l.mode == "login" {
				if l.focusIndex == 0 {
					l.focusIndex = 2
				} else {
					l.focusIndex = 0
				}
			} else {
				l.focusIndex = (l.focusIndex + 1) % 3
			}
			for i := range l.inputs {
				l.inputs[i].Blur()
			}
			l.inputs[l.focusIndex].Focus()
			return l, nil
		case "enter":
			// submit on last field
			if l.mode == "login" && l.focusIndex != 2 {
				// move focus to next
				l.focusIndex = 2
				for i := range l.inputs {
					l.inputs[i].Blur()
				}
				l.inputs[l.focusIndex].Focus()
				return l, nil
			}
			l.loading = true
			l.err = ""
			return l, func() tea.Msg {
				uname := strings.TrimSpace(l.inputs[0].Value())
				email := strings.TrimSpace(l.inputs[1].Value())
				pwd := l.inputs[2].Value()
				var err error
				if l.mode == "login" {
					err = l.client.Login(uname, pwd)
				} else {
					err = l.client.Register(uname, email, pwd)
				}
				return loginResultMsg{err: err}
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		l.spin, cmd = l.spin.Update(msg)
		return l, cmd
	case loginResultMsg:
		l.loading = false
		if msg.err != nil {
			l.err = msg.err.Error()
			return l, nil
		}
		return l, func() tea.Msg { return msgs.NavigateMsg{Screen: "menu"} }
	}
	// update inputs
	for i := range l.inputs {
		l.inputs[i], _ = l.inputs[i].Update(msg)
	}
	return l, nil
}

func (l *LoginModel) View() string {
	var b strings.Builder
	b.WriteString(styles.Title.Render("Login / Register") + "\n\n")
	b.WriteString(styles.Subtitle.Render("t: toggle mode • Tab: next field • Enter: submit • Esc/ctrl+c: quit") + "\n\n")
	if l.mode == "register" {
		for i := 0; i < 3; i++ {
			b.WriteString(l.inputs[i].View() + "\n")
		}
	} else {
		b.WriteString(l.inputs[0].View() + "\n")
		b.WriteString(l.inputs[2].View() + "\n")
	}
	if l.loading {
		b.WriteString(l.spin.View() + " Working...\n")
	}
	if l.err != "" {
		b.WriteString(styles.Error.Render(l.err) + "\n")
	}
	if l.msg != "" {
		b.WriteString(styles.Success.Render(l.msg) + "\n")
	}
	return b.String()
}
