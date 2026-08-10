package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/claymor333/squal/internal/config"
	"github.com/claymor333/squal/internal/tui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}

	if len(cfg.Profiles) == 0 {
		fmt.Fprintln(os.Stderr, "No connection profiles configured.")
		fmt.Fprintln(os.Stderr, "Add profiles to "+configPathHint())
		fmt.Fprintln(os.Stderr, "Profile JSON: {\"name\",\"host\",\"port\",\"user\",\"password\",\"database\"}")
		os.Exit(1)
	}

	p := tea.NewProgram(tui.New(cfg, cfg.Profiles), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func configPathHint() string {
	if p, err := config.Path(); err == nil {
		return p
	}
	return "the SQUAL_CONFIG file"
}
