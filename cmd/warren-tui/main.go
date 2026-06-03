package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lfu/warren/internal/core"
	"github.com/lfu/warren/internal/tui"
)

func main() {
	// Parse command-line flags. -db defaults to core.DefaultDBPath() so the
	// --help text shows the resolved canonical path under $HOME/.warren.
	dbPath := flag.String("db", core.DefaultDBPath(), "Database path")
	flag.Parse()

	// Detect whether the user explicitly passed -db via flag.Visit (NOT by
	// value comparison — passing the default path explicitly is legitimate
	// and must not trigger the legacy-DB warning).
	userOverrodeDB := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "db" {
			userOverrodeDB = true
		}
	})

	// Refuse to start if a legacy cwd-relative warren.db would be silently
	// orphaned by the new $HOME/.warren default.
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve current working directory: %v\n", err)
		os.Exit(1)
	}
	if err := core.CheckLegacyDB(userOverrodeDB, cwd, core.DefaultDBPath()); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// Create Warren with default config; DBPath/ConfigDir resolve via the
	// canonical helpers. core.NewWarren ensures ConfigDir exists.
	config := core.DefaultConfig()
	config.DBPath = *dbPath

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

// discoverAndRegisterSessions discovers tmux sessions on all servers and registers them with Warren
func discoverAndRegisterSessions(warren *core.Warren) error {
	registry := warren.GetServerRegistry()
	if registry == nil {
		return fmt.Errorf("no server registry available")
	}

	totalRegistered := 0
	for _, server := range registry.List() {
		client := core.TmuxClientForServer(server)
		discoveryService := core.NewAgentDiscovery(client)

		topology, err := client.DiscoverTopology(server.Name)
		if err != nil {
			log.Printf("Warning: Failed to discover topology on %s: %v", server.Name, err)
			continue
		}

		results, err := discoveryService.DiscoverAll(topology, 0.7)
		if err != nil {
			log.Printf("Warning: Failed to discover sessions on %s: %v", server.Name, err)
			continue
		}

		for _, result := range results {
			session := result.ToAgentSession()
			session.ServerName = server.Name

			if err := warren.AddSessionWithClient(session.ID, session.TmuxPaneID, server.Name, session.Metadata["working_dir"], client); err != nil {
				log.Printf("Warning: Failed to register session %s: %v", session.ID, err)
				continue
			}

			if err := warren.RegisterAgentSession(session); err != nil {
				log.Printf("Warning: Failed to register session in registry %s: %v", session.ID, err)
			}

			log.Printf("Registered agent session: %s (pane: %s, type: %s, server: %s)", session.ID, session.TmuxPaneID, session.AgentType, server.Name)
			totalRegistered++
		}
	}

	if totalRegistered == 0 {
		log.Println("No agent sessions discovered on any server")
	}

	return nil
}
