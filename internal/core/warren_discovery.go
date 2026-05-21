package core

import (
	"fmt"
	"time"
)

// initializeDiscovery sets up the discovery service for Warren
func (w *Warren) initializeDiscovery(discoveryConfig *DiscoveryConfig) {
	// Create discovery service
	w.agentDiscovery = NewAgentDiscovery(w.tmuxClient)
	w.discoveryInterval = discoveryConfig.DiscoveryInterval
	w.discoveryEnabled = discoveryConfig.EnableAutoDiscovery
}

// runDiscovery discovers and registers agent sessions across all servers
func (w *Warren) runDiscovery() error {
	if w.agentDiscovery == nil {
		return fmt.Errorf("discovery not initialized")
	}

	servers := w.serverRegistry.List()
	if len(servers) == 0 {
		return fmt.Errorf("no servers configured")
	}

	discoveredCount := 0
	for _, server := range servers {
		// Discover topology for this server
		client := TmuxClientForServer(server)
		topology, err := client.DiscoverTopology(server.Name)
		if err != nil {
			fmt.Printf("Warning: failed to discover topology for server %s: %v\n", server.Name, err)
			continue
		}

		// Run agent discovery on this topology
		results, err := w.agentDiscovery.DiscoverAll(topology, w.minConfidence)
		if err != nil {
			fmt.Printf("Warning: agent discovery failed for server %s: %v\n", server.Name, err)
			continue
		}

		// Register discovered agents
		for _, result := range results {
			agentSession := result.ToAgentSession()

			// Check if already registered
			existing, _ := w.sessionRegistry.Get(agentSession.ID)
			if existing != nil {
				continue // Already registered
			}

			// Register in session registry
			w.sessionRegistry.Register(agentSession)

			// Add to active monitoring sessions using AddSession
			// Note: AddSession only takes agentID and paneID, so we'll need to
			// update the session metadata after adding it
			if err := w.AddSession(agentSession.ID, agentSession.TmuxPaneID); err != nil {
				fmt.Printf("Warning: failed to add session %s: %v\n", agentSession.ID, err)
				continue
			}

			// Update session metadata
			w.mu.Lock()
			if session, exists := w.sessions[agentSession.ID]; exists {
				session.ServerName = agentSession.ServerName
				session.WorkingDir = agentSession.Metadata["working_dir"]
				session.TmuxClient = client
			}
			w.mu.Unlock()

			discoveredCount++
		}
	}

	if discoveredCount > 0 {
		fmt.Printf("Discovered and registered %d agent sessions\n", discoveredCount)

		// Save registry to disk
		if err := w.sessionRegistry.Save(w.registryPath); err != nil {
			fmt.Printf("Warning: failed to save registry: %v\n", err)
		}
	}

	return nil
}

// startDiscoveryLoop runs periodic agent discovery
func (w *Warren) startDiscoveryLoop() {
	if w.discoveryInterval == 0 {
		return // Discovery disabled
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		ticker := time.NewTicker(w.discoveryInterval)
		defer ticker.Stop()

		for {
			select {
			case <-w.ctx.Done():
				return
			case <-ticker.C:
				if err := w.runDiscovery(); err != nil {
					fmt.Printf("Warning: periodic discovery failed: %v\n", err)
				}
			}
		}
	}()
}

// RunDiscovery manually triggers agent discovery (exposed for API)
func (w *Warren) RunDiscovery() error {
	return w.runDiscovery()
}
