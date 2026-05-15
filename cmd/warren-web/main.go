package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lfu/warren/internal/core"
	"github.com/lfu/warren/internal/tmux"
	"github.com/lfu/warren/internal/web"
)

func main() {
	// Parse command-line flags
	addr := flag.String("addr", ":8080", "HTTP server address")
	dbPath := flag.String("db", "", "Database path (default: ~/.warren/warren.db)")
	pollInterval := flag.Duration("poll", 500*time.Millisecond, "Polling interval")
	minConfidence := flag.Float64("confidence", 0.7, "Minimum confidence for state transitions")
	flag.Parse()

	// Set default DB path if not provided
	if *dbPath == "" {
		*dbPath = os.ExpandEnv("$HOME/.warren/warren.db")
	}

	// Ensure .warren directory exists
	warrenDir := os.ExpandEnv("$HOME/.warren")
	if err := os.MkdirAll(warrenDir, 0755); err != nil {
		log.Fatalf("Failed to create Warren directory: %v", err)
	}

	// Create Warren orchestrator with defaults, then override from flags
	warrenConfig := core.DefaultConfig()
	warrenConfig.PollInterval = *pollInterval
	warrenConfig.MinConfidence = *minConfidence
	warrenConfig.DBPath = *dbPath
	warrenConfig.ConfigDir = warrenDir

	warren, err := core.NewWarren(warrenConfig)
	if err != nil {
		log.Fatalf("Failed to create Warren orchestrator: %v", err)
	}

	// Discover and register agent sessions
	log.Println("Discovering agent sessions...")
	if err := discoverAndRegisterSessions(warren); err != nil {
		log.Printf("Warning: Failed to discover sessions: %v", err)
	}

	// Start Warren monitoring
	if len(warren.GetAllSessions()) > 0 {
		log.Printf("Starting monitoring for %d agent sessions...", len(warren.GetAllSessions()))
		if err := warren.Start(); err != nil {
			log.Fatalf("Failed to start Warren: %v", err)
		}
	} else {
		log.Println("No agent sessions found. Web interface will be available but no agents will be monitored.")
	}

	// Create and start web server
	webConfig := &web.Config{
		Addr:   *addr,
		Warren: warren,
	}

	server := web.NewServer(webConfig)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}

	log.Printf("Warren web interface available at http://localhost%s", *addr)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		log.Printf("Error stopping web server: %v", err)
	}

	if err := warren.Stop(); err != nil {
		log.Printf("Error stopping Warren: %v", err)
	}

	log.Println("Shutdown complete")
}

// discoverAndRegisterSessions discovers tmux sessions and registers them with Warren
func discoverAndRegisterSessions(warren *core.Warren) error {
	// Get all servers from the registry
	servers := warren.GetServerRegistry().List()
	if len(servers) == 0 {
		log.Println("No servers configured. Add servers to ~/.warren/servers.yaml")
		return nil
	}

	totalSessions := 0

	// Discover sessions on each server
	for _, server := range servers {
		log.Printf("Discovering sessions on server: %s (%s)", server.Name, server.Host)

		// Create appropriate tmux client for this server
		var tmuxClient *tmux.Client
		if server.IsLocal() {
			// Use local executor for local server
			tmuxClient = tmux.NewClient(tmux.NewLocalExecutor())
		} else {
			// Use remote executor for remote servers
			if server.Port == 0 {
				server.Port = 22 // Default SSH port
			}
			tmuxClient = tmux.NewClient(tmux.NewRemoteExecutor(server.User, server.Host, server.Port))
		}

		// Use the discovery service to find agent sessions
		discoveryService := core.NewAgentDiscovery(tmuxClient)

		// Get topology for this server
		topology, err := tmuxClient.DiscoverTopology(server.Name)
		if err != nil {
			log.Printf("Warning: Failed to discover topology on %s: %v", server.Name, err)
			continue
		}

		// Discover all agent sessions on this server
		results, err := discoveryService.DiscoverAll(topology, 0.7)
		if err != nil {
			log.Printf("Warning: Failed to discover sessions on %s: %v", server.Name, err)
			continue
		}

		if len(results) == 0 {
			log.Printf("No agent sessions found on %s", server.Name)
			continue
		}

		// Register each discovered session
		for _, result := range results {
			session := result.ToAgentSession()

			// Register in both old and new systems for compatibility
			if err := warren.AddSession(session.ID, session.TmuxPaneID); err != nil {
				log.Printf("Warning: Failed to register session %s: %v", session.ID, err)
				continue
			}

			// Also register in the new AgentSessionRegistry
			if err := warren.RegisterAgentSession(session); err != nil {
				log.Printf("Warning: Failed to register session in registry %s: %v", session.ID, err)
			}

			log.Printf("Registered agent session: %s (pane: %s, type: %s)", session.ID, session.TmuxPaneID, session.AgentType)
			totalSessions++
		}
	}

	log.Printf("Total sessions discovered: %d", totalSessions)
	return nil
}
