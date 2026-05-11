package core

import (
	"fmt"
	"sort"
	"time"

	"github.com/lfu/warren/internal/types"
)

// AgentSession represents a logical agent session mapped to a tmux pane
type AgentSession struct {
	// ID is a unique identifier for this agent session
	ID string `json:"id"`

	// Name is a human-readable name for this session
	Name string `json:"name"`

	// ServerName references the server this agent runs on
	ServerName string `json:"server_name"`

	// TmuxSessionName is the tmux session name
	TmuxSessionName string `json:"tmux_session_name"`

	// TmuxWindowIndex is the window index within the session
	TmuxWindowIndex int `json:"tmux_window_index"`

	// TmuxPaneIndex is the pane index within the window
	TmuxPaneIndex int `json:"tmux_pane_index"`

	// TmuxPaneID is the pane ID (e.g., "%1")
	TmuxPaneID string `json:"tmux_pane_id"`

	// AgentType identifies the type of agent (e.g., "claude-code", "copilot", "custom")
	AgentType string `json:"agent_type"`

	// CreatedAt is when this session was first registered
	CreatedAt time.Time `json:"created_at"`

	// LastSeenAt is when this session was last observed active
	LastSeenAt time.Time `json:"last_seen_at"`

	// CurrentState is the detected state of the agent
	CurrentState types.AgentState `json:"current_state"`

	// Metadata stores additional session-specific data
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Re-export AgentState and constants for backward compatibility
type AgentState = types.AgentState

const (
	StateUnknown           = types.StateUnknown
	StateIdle              = types.StateIdle
	StateThinking          = types.StateThinking
	StateExecuting         = types.StateExecuting
	StateWaitingPermission = types.StateWaitingPermission
	StateAskingQuestion    = types.StateAskingQuestion
	StateFinished          = types.StateFinished
	StateError             = types.StateError
	StateStopped           = types.StateStopped
)

// AgentSessionRegistry manages the collection of known agent sessions
type AgentSessionRegistry struct {
	sessions map[string]*AgentSession
}

// NewAgentSessionRegistry creates a new agent session registry
func NewAgentSessionRegistry() *AgentSessionRegistry {
	return &AgentSessionRegistry{
		sessions: make(map[string]*AgentSession),
	}
}

// Register adds or updates an agent session
func (r *AgentSessionRegistry) Register(session *AgentSession) error {
	if session.ID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	if session.ServerName == "" {
		return fmt.Errorf("server name cannot be empty")
	}

	if session.TmuxPaneID == "" {
		return fmt.Errorf("tmux pane ID cannot be empty")
	}

	// Update LastSeenAt
	session.LastSeenAt = time.Now()

	// If this is a new session, set CreatedAt
	if existing, exists := r.sessions[session.ID]; exists {
		session.CreatedAt = existing.CreatedAt
	} else {
		session.CreatedAt = time.Now()
	}

	r.sessions[session.ID] = session
	return nil
}

// Get retrieves an agent session by ID
func (r *AgentSessionRegistry) Get(id string) (*AgentSession, error) {
	session, exists := r.sessions[id]
	if !exists {
		return nil, fmt.Errorf("agent session %q not found", id)
	}
	return session, nil
}

// GetByPane retrieves an agent session by its tmux pane ID
func (r *AgentSessionRegistry) GetByPane(serverName, paneID string) (*AgentSession, error) {
	for _, session := range r.sessions {
		if session.ServerName == serverName && session.TmuxPaneID == paneID {
			return session, nil
		}
	}
	return nil, fmt.Errorf("no agent session found for pane %s on server %s", paneID, serverName)
}

// List returns all registered agent sessions in sorted order
// Sort order: server name → session name → window index → pane ID
func (r *AgentSessionRegistry) List() []*AgentSession {
	sessions := make([]*AgentSession, 0, len(r.sessions))
	for _, session := range r.sessions {
		sessions = append(sessions, session)
	}

	// Sort for stable, predictable ordering
	sort.Slice(sessions, func(i, j int) bool {
		// Primary: Server name
		if sessions[i].ServerName != sessions[j].ServerName {
			return sessions[i].ServerName < sessions[j].ServerName
		}
		// Secondary: Tmux session name
		if sessions[i].TmuxSessionName != sessions[j].TmuxSessionName {
			return sessions[i].TmuxSessionName < sessions[j].TmuxSessionName
		}
		// Tertiary: Window index
		if sessions[i].TmuxWindowIndex != sessions[j].TmuxWindowIndex {
			return sessions[i].TmuxWindowIndex < sessions[j].TmuxWindowIndex
		}
		// Quaternary: Pane index (not pane ID)
		return sessions[i].TmuxPaneIndex < sessions[j].TmuxPaneIndex
	})

	return sessions
}

// ListByServer returns all agent sessions on a specific server in sorted order
func (r *AgentSessionRegistry) ListByServer(serverName string) []*AgentSession {
	sessions := make([]*AgentSession, 0)
	for _, session := range r.sessions {
		if session.ServerName == serverName {
			sessions = append(sessions, session)
		}
	}

	// Sort by session → window → pane
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].TmuxSessionName != sessions[j].TmuxSessionName {
			return sessions[i].TmuxSessionName < sessions[j].TmuxSessionName
		}
		if sessions[i].TmuxWindowIndex != sessions[j].TmuxWindowIndex {
			return sessions[i].TmuxWindowIndex < sessions[j].TmuxWindowIndex
		}
		return sessions[i].TmuxPaneIndex < sessions[j].TmuxPaneIndex
	})

	return sessions
}

// ListByState returns all agent sessions in a specific state in sorted order
func (r *AgentSessionRegistry) ListByState(state AgentState) []*AgentSession {
	sessions := make([]*AgentSession, 0)
	for _, session := range r.sessions {
		if session.CurrentState == state {
			sessions = append(sessions, session)
		}
	}

	// Sort by server → session → window → pane
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].ServerName != sessions[j].ServerName {
			return sessions[i].ServerName < sessions[j].ServerName
		}
		if sessions[i].TmuxSessionName != sessions[j].TmuxSessionName {
			return sessions[i].TmuxSessionName < sessions[j].TmuxSessionName
		}
		if sessions[i].TmuxWindowIndex != sessions[j].TmuxWindowIndex {
			return sessions[i].TmuxWindowIndex < sessions[j].TmuxWindowIndex
		}
		return sessions[i].TmuxPaneIndex < sessions[j].TmuxPaneIndex
	})

	return sessions
}

// ListActionable returns all agent sessions that need user attention in sorted order
func (r *AgentSessionRegistry) ListActionable() []*AgentSession {
	sessions := make([]*AgentSession, 0)
	for _, session := range r.sessions {
		if session.CurrentState.IsActionable() {
			sessions = append(sessions, session)
		}
	}

	// Sort by server → session → window → pane
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].ServerName != sessions[j].ServerName {
			return sessions[i].ServerName < sessions[j].ServerName
		}
		if sessions[i].TmuxSessionName != sessions[j].TmuxSessionName {
			return sessions[i].TmuxSessionName < sessions[j].TmuxSessionName
		}
		if sessions[i].TmuxWindowIndex != sessions[j].TmuxWindowIndex {
			return sessions[i].TmuxWindowIndex < sessions[j].TmuxWindowIndex
		}
		return sessions[i].TmuxPaneIndex < sessions[j].TmuxPaneIndex
	})

	return sessions
}

// Remove removes an agent session from the registry
func (r *AgentSessionRegistry) Remove(id string) error {
	if _, exists := r.sessions[id]; !exists {
		return fmt.Errorf("agent session %q not found", id)
	}
	delete(r.sessions, id)
	return nil
}

// UpdateState updates the state of an agent session
func (r *AgentSessionRegistry) UpdateState(id string, state AgentState) error {
	session, exists := r.sessions[id]
	if !exists {
		return fmt.Errorf("agent session %q not found", id)
	}
	session.CurrentState = state
	session.LastSeenAt = time.Now()
	return nil
}

// Count returns the total number of registered sessions
func (r *AgentSessionRegistry) Count() int {
	return len(r.sessions)
}
