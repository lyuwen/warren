package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lfu/warren/internal/core"
	"github.com/lfu/warren/internal/tui"
)

func main() {
	// Create Warren instance with default config
	config := core.DefaultConfig()
	config.DBPath = os.ExpandEnv("$HOME/.warren/warren.db")

	warren, err := core.NewWarren(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Warren: %v\n", err)
		os.Exit(1)
	}
	defer warren.Stop()

	// TODO: Add some example sessions for testing
	// In production, this would discover sessions automatically
	// For now, users need to manually register sessions via CLI

	// Create TUI model
	model := tui.NewModel(warren)

	// Start Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
