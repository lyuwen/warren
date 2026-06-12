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
	// Resolve the default bind address. CLI --bind flag wins; otherwise
	// WARREN_WEB_BIND env (mirrors WARREN_SSH_INSECURE_HOSTKEY pattern from
	// Batch 1); otherwise the loopback-only default.
	defaultBind := web.DefaultBindAddr
	if env := os.Getenv(web.EnvBindAddr); env != "" {
		defaultBind = env
	}

	// Parse command-line flags. -db defaults to core.DefaultDBPath() so the
	// --help text shows the resolved canonical path under $HOME/.warren.
	bind := flag.String("bind", defaultBind, "HTTP bind address (host:port). Defaults to "+web.DefaultBindAddr+" (loopback only); set "+web.EnvBindAddr+" or --bind 0.0.0.0:8080 to expose on the network")
	addr := flag.String("addr", "", "DEPRECATED: use --bind. Retained for backwards compatibility")
	dbPath := flag.String("db", core.DefaultDBPath(), "Database path")
	pollInterval := flag.Duration("poll", 500*time.Millisecond, "Polling interval")
	minConfidence := flag.Float64("confidence", 0.7, "Minimum confidence for state transitions")
	flag.Parse()

	// --addr is the legacy flag. If the operator passed --addr but not --bind,
	// honor it; otherwise --bind wins.
	bindAddr := *bind
	addrSet, bindSet := false, false
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "addr":
			addrSet = true
		case "bind":
			bindSet = true
		}
	})
	if addrSet && !bindSet {
		log.Printf("WARN: --addr is deprecated; use --bind. Honoring --addr=%q for now.", *addr)
		bindAddr = *addr
	}

	// Detect whether the user explicitly passed -db via flag.Visit. We do
	// NOT compare *dbPath to core.DefaultDBPath() — a user who legitimately
	// passes the same string explicitly must not trigger the legacy-DB
	// warning, and a user who simply launched without -db must trigger it.
	userOverrodeDB := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "db" {
			userOverrodeDB = true
		}
	})

	// Refuse to start if a legacy cwd-relative warren.db would be silently
	// orphaned by the new $HOME/.warren default. Print the migration hint
	// to stderr and exit non-zero — better than silent data loss.
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to resolve current working directory: %v", err)
	}
	if err := core.CheckLegacyDB(userOverrodeDB, cwd, core.DefaultDBPath()); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// Create Warren orchestrator. ConfigDir resolves under $HOME/.warren via
	// core.DefaultConfigDir(); core.NewWarren ensures the directory exists.
	warrenConfig := core.DefaultConfig()
	warrenConfig.PollInterval = *pollInterval
	warrenConfig.MinConfidence = *minConfidence
	warrenConfig.DBPath = *dbPath

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
		Addr:   bindAddr,
		Warren: warren,
	}

	server := web.NewServer(webConfig)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}

	log.Printf("Warren web interface available at http://%s", bindAddr)

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
