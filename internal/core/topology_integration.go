package core

import (
	"fmt"

	"github.com/lfu/warren/internal/tmux"
)

// GetSession retrieves an agent session by ID
func (w *Warren) GetSession(agentID string) (*AgentSession, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Try registry first
	if w.sessionRegistry != nil {
		session, err := w.sessionRegistry.Get(agentID)
		if err == nil {
			return session, nil
		}
		// If not found in registry, fall through to fallback
	}

	// Fallback: check old MonitoredSession map
	monitored, exists := w.sessions[agentID]
	if !exists {
		return nil, fmt.Errorf("agent session %q not found", agentID)
	}

	// Convert MonitoredSession to AgentSession (minimal conversion)
	return &AgentSession{
		ID:              monitored.AgentID,
		TmuxPaneID:      monitored.PaneID,
		CurrentState:    monitored.CurrentState,
		ServerName:      "localhost", // Default to localhost for now
		TmuxSessionName: "",          // Not available in MonitoredSession
		TmuxWindowIndex: 0,           // Not available in MonitoredSession
		TmuxPaneIndex:   0,           // Not available in MonitoredSession
	}, nil
}

// GetServer retrieves a server by name
func (w *Warren) GetServer(serverName string) (*Server, error) {
	// Try registry first
	if w.serverRegistry != nil {
		server, err := w.serverRegistry.Get(serverName)
		if err == nil {
			return server, nil
		}
		// If not found in registry, fall through to default
	}

	// Fallback: return localhost server
	if serverName == "localhost" || serverName == "" {
		hostname := "localhost"
		return &Server{
			Name: hostname,
			Host: "localhost",
			Kind: ServerKindLocal,
		}, nil
	}

	return nil, fmt.Errorf("server %q not found", serverName)
}

// GetPane retrieves a tmux pane object from an agent session
func (w *Warren) GetPane(session *AgentSession, server *Server) (*tmux.Pane, error) {
	// Create appropriate tmux client for this server
	var tmuxClient *tmux.Client
	if server.IsLocal() {
		tmuxClient = w.tmuxClient
	} else {
		// Create remote executor for remote servers
		if server.Port == 0 {
			server.Port = 22
		}
		tmuxClient = tmux.NewClient(tmux.NewRemoteExecutor(server.User, server.Host, server.Port))
	}

	// Discover topology to find the pane
	topology, err := tmuxClient.DiscoverTopology(server.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to discover topology: %w", err)
	}

	// Find the pane by ID
	pane, _, _, err := topology.FindPane(session.TmuxPaneID)
	if err != nil {
		return nil, fmt.Errorf("failed to find pane %s: %w", session.TmuxPaneID, err)
	}

	return pane, nil
}

// GetPaneByID retrieves a tmux pane by its pane ID
// This is a convenience method that doesn't require session/server lookup
func (w *Warren) GetPaneByID(paneID string) (*tmux.Pane, error) {
	// Discover local topology
	topology, err := w.tmuxClient.DiscoverTopology("localhost")
	if err != nil {
		return nil, fmt.Errorf("failed to discover topology: %w", err)
	}

	// Find the pane
	pane, _, _, err := topology.FindPane(paneID)
	if err != nil {
		return nil, fmt.Errorf("pane %s not found: %w", paneID, err)
	}

	return pane, nil
}
