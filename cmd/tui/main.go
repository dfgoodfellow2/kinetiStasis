package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/app"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/client"
	"github.com/dfgoodfellow2/diet-tracker/v2/ui/tui/config"
)

func main() {
	cfg := config.Load()
	c := client.New(cfg.BaseURL, cfg.SessionDir)
	p := tea.NewProgram(app.New(c), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
