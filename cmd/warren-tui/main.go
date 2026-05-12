package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lfu/warren/internal/core"
	"github.com/lfu/warren/internal/tui"
)

func main() {
	// Create Warren instance with default config
	config := core.DefaultConfig()
	config.DBPath = os.ExpandEnv("$HOME/.warren/warren.db")

	// Ensure .warren directory exists
	warrenDir := os.ExpandEnv("$HOME/.warren")
	if err := os.MkdirAll(warrenDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Warren directory: %v\n", err)
		os.Exit(1)
	}

	warren, err := core.NewWarren(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Warren: %v\n", err)
		os.Exit(1)
	}
	defer warren.Stop()

	// Discover and register agent sessions
	if err := discoverAndRegisterSessions(warren); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to discover sessions: %v\n", err)
	}

	// Start Warren monitoring if sessions found
	if len(warren.GetAllSessions()) > 0 {
		if err := warren.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start Warren: %v\n", err)
			os.Exit(1)
		}
	}

	// Create TUI model
	model := tui.NewModel(warren)

	// Start Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

// discoverAndRegisterSessions discovers tmux sessions and registers them with Warren
func discoverAndRegisterSessions(warren *core.Warren) error {
	// Create tmux client for discovery
	tmuxClient := warren.GetTmuxClient()

	// Use the discovery service to find agent sessions
	discoveryService := core.NewAgentDiscovery(tmuxClient)

	// Get topology (use "localhost" as server name)
	topology, err := tmuxClient.DiscoverTopology("localhost")
	if err != nil {
		return fmt.Errorf("failed to discover topology: %w", err)
	}

	// Discover all agent sessions
	results, err := discoveryService.DiscoverAll(topology, 0.7)
	if err != nil {
		return fmt.Errorf("failed to discover sessions: %w", err)
	}

	if len(results) == 0 {
		log.Println("No agent sessions discovered")
		return nil
	}

	// Register each discovered session
	for _, result := range results {
		session := result.ToAgentSession()

		// Register in both the old sessions map and new registry
		if err := warren.AddSession(session.ID, session.TmuxPaneID); err != nil {
			log.Printf("Warning: Failed to register session %s: %v", session.ID, err)
			continue
		}

		// Also register in the session registry for topology integration
		if err := warren.RegisterAgentSession(session); err != nil {
			log.Printf("Warning: Failed to register session in registry %s: %v", session.ID, err)
		}

		log.Printf("Registered agent session: %s (pane: %s, type: %s)", session.ID, session.TmuxPaneID, session.AgentType)
	}

	return nil
}
