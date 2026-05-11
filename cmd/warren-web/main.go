package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lfu/warren/internal/core"
	"github.com/lfu/warren/internal/web"
)

func main() {
	// Parse command-line flags
	addr := flag.String("addr", ":8080", "HTTP server address")
	dbPath := flag.String("db", "warren.db", "Database path")
	pollInterval := flag.Duration("poll", 500*time.Millisecond, "Polling interval")
	minConfidence := flag.Float64("confidence", 0.7, "Minimum confidence for state transitions")
	flag.Parse()

	// Create Warren orchestrator
	warrenConfig := &core.Config{
		PollInterval:  *pollInterval,
		MinConfidence: *minConfidence,
		DBPath:        *dbPath,
	}

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
		if err := warren.AddSession(session.ID, session.TmuxPaneID); err != nil {
			log.Printf("Warning: Failed to register session %s: %v", session.ID, err)
			continue
		}
		log.Printf("Registered agent session: %s (pane: %s, type: %s)", session.ID, session.TmuxPaneID, session.AgentType)
	}

	return nil
}
