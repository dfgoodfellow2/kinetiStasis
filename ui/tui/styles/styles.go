package styles

import "github.com/charmbracelet/lipgloss"

var (
	Title     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).MarginBottom(1)
	Subtitle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	Highlight = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	Success   = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	Error     = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	Warning   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	Muted     = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	Label     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	Box       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	Help      = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
)
