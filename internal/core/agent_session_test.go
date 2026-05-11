package core

import (
	"testing"
	"time"
)

func TestAgentState_IsActionable(t *testing.T) {
	tests := []struct {
		state    AgentState
		expected bool
	}{
		{StateWaitingPermission, true},
		{StateAskingQuestion, true},
		{StateError, true},
		{StateFinished, true},
		{StateIdle, false},
		{StateThinking, false},
		{StateExecuting, false},
		{StateUnknown, false},
		{StateStopped, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsActionable(); got != tt.expected {
				t.Errorf("IsActionable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAgentSessionRegistry_Register(t *testing.T) {
	registry := NewAgentSessionRegistry()

	session := &AgentSession{
		ID:              "test-session-1",
		Name:            "Test Session",
		ServerName:      "localhost",
		TmuxSessionName: "test",
		TmuxWindowIndex: 0,
		TmuxPaneID:      "%1",
		AgentType:       "claude-code",
		CurrentState:    StateIdle,
	}

	// Register new session
	if err := registry.Register(session); err != nil {
		t.Fatalf("Failed to register session: %v", err)
	}

	// Verify CreatedAt and LastSeenAt are set
	if session.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	if session.LastSeenAt.IsZero() {
		t.Error("LastSeenAt should be set")
	}

	// Register again - should update LastSeenAt but not CreatedAt
	time.Sleep(10 * time.Millisecond)
	originalCreatedAt := session.CreatedAt
	if err := registry.Register(session); err != nil {
		t.Fatalf("Failed to re-register session: %v", err)
	}

	if session.CreatedAt != originalCreatedAt {
		t.Error("CreatedAt should not change on re-registration")
	}

	if session.LastSeenAt.Before(originalCreatedAt) {
		t.Error("LastSeenAt should be updated on re-registration")
	}
}

func TestAgentSessionRegistry_RegisterValidation(t *testing.T) {
	registry := NewAgentSessionRegistry()

	tests := []struct {
		name        string
		session     *AgentSession
		expectError bool
	}{
		{
			name: "valid session",
			session: &AgentSession{
				ID:         "test-1",
				ServerName: "localhost",
				TmuxPaneID: "%1",
			},
			expectError: false,
		},
		{
			name: "missing ID",
			session: &AgentSession{
				ServerName: "localhost",
				TmuxPaneID: "%1",
			},
			expectError: true,
		},
		{
			name: "missing server name",
			session: &AgentSession{
				ID:         "test-1",
				TmuxPaneID: "%1",
			},
			expectError: true,
		},
		{
			name: "missing pane ID",
			session: &AgentSession{
				ID:         "test-1",
				ServerName: "localhost",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.Register(tt.session)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestAgentSessionRegistry_Get(t *testing.T) {
	registry := NewAgentSessionRegistry()

	session := &AgentSession{
		ID:         "test-1",
		Name:       "Test",
		ServerName: "localhost",
		TmuxPaneID: "%1",
	}

	registry.Register(session)

	// Get existing session
	retrieved, err := registry.Get("test-1")
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}

	if retrieved.ID != "test-1" {
		t.Errorf("Expected ID test-1, got %s", retrieved.ID)
	}

	// Get non-existent session
	_, err = registry.Get("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent session")
	}
}

func TestAgentSessionRegistry_GetByPane(t *testing.T) {
	registry := NewAgentSessionRegistry()

	session := &AgentSession{
		ID:         "test-1",
		ServerName: "localhost",
		TmuxPaneID: "%1",
	}

	registry.Register(session)

	// Get by pane
	retrieved, err := registry.GetByPane("localhost", "%1")
	if err != nil {
		t.Fatalf("Failed to get session by pane: %v", err)
	}

	if retrieved.ID != "test-1" {
		t.Errorf("Expected ID test-1, got %s", retrieved.ID)
	}

	// Get non-existent pane
	_, err = registry.GetByPane("localhost", "%999")
	if err == nil {
		t.Error("Expected error for non-existent pane")
	}
}

func TestAgentSessionRegistry_List(t *testing.T) {
	registry := NewAgentSessionRegistry()

	sessions := []*AgentSession{
		{ID: "test-1", ServerName: "server1", TmuxPaneID: "%1"},
		{ID: "test-2", ServerName: "server1", TmuxPaneID: "%2"},
		{ID: "test-3", ServerName: "server2", TmuxPaneID: "%1"},
	}

	for _, s := range sessions {
		registry.Register(s)
	}

	// List all
	all := registry.List()
	if len(all) != 3 {
		t.Errorf("Expected 3 sessions, got %d", len(all))
	}

	// List by server
	server1Sessions := registry.ListByServer("server1")
	if len(server1Sessions) != 2 {
		t.Errorf("Expected 2 sessions on server1, got %d", len(server1Sessions))
	}
}

func TestAgentSessionRegistry_ListByState(t *testing.T) {
	registry := NewAgentSessionRegistry()

	sessions := []*AgentSession{
		{ID: "test-1", ServerName: "localhost", TmuxPaneID: "%1", CurrentState: StateIdle},
		{ID: "test-2", ServerName: "localhost", TmuxPaneID: "%2", CurrentState: StateWaitingPermission},
		{ID: "test-3", ServerName: "localhost", TmuxPaneID: "%3", CurrentState: StateWaitingPermission},
	}

	for _, s := range sessions {
		registry.Register(s)
	}

	// List by state
	waiting := registry.ListByState(StateWaitingPermission)
	if len(waiting) != 2 {
		t.Errorf("Expected 2 sessions waiting for permission, got %d", len(waiting))
	}

	idle := registry.ListByState(StateIdle)
	if len(idle) != 1 {
		t.Errorf("Expected 1 idle session, got %d", len(idle))
	}
}

func TestAgentSessionRegistry_ListActionable(t *testing.T) {
	registry := NewAgentSessionRegistry()

	sessions := []*AgentSession{
		{ID: "test-1", ServerName: "localhost", TmuxPaneID: "%1", CurrentState: StateIdle},
		{ID: "test-2", ServerName: "localhost", TmuxPaneID: "%2", CurrentState: StateWaitingPermission},
		{ID: "test-3", ServerName: "localhost", TmuxPaneID: "%3", CurrentState: StateAskingQuestion},
		{ID: "test-4", ServerName: "localhost", TmuxPaneID: "%4", CurrentState: StateError},
	}

	for _, s := range sessions {
		registry.Register(s)
	}

	actionable := registry.ListActionable()
	if len(actionable) != 3 {
		t.Errorf("Expected 3 actionable sessions, got %d", len(actionable))
	}
}

func TestAgentSessionRegistry_UpdateState(t *testing.T) {
	registry := NewAgentSessionRegistry()

	session := &AgentSession{
		ID:           "test-1",
		ServerName:   "localhost",
		TmuxPaneID:   "%1",
		CurrentState: StateIdle,
	}

	registry.Register(session)

	// Update state
	if err := registry.UpdateState("test-1", StateThinking); err != nil {
		t.Fatalf("Failed to update state: %v", err)
	}

	retrieved, _ := registry.Get("test-1")
	if retrieved.CurrentState != StateThinking {
		t.Errorf("Expected state StateThinking, got %s", retrieved.CurrentState)
	}

	// Update non-existent session
	if err := registry.UpdateState("non-existent", StateIdle); err == nil {
		t.Error("Expected error when updating non-existent session")
	}
}

func TestAgentSessionRegistry_Remove(t *testing.T) {
	registry := NewAgentSessionRegistry()

	session := &AgentSession{
		ID:         "test-1",
		ServerName: "localhost",
		TmuxPaneID: "%1",
	}

	registry.Register(session)

	// Remove session
	if err := registry.Remove("test-1"); err != nil {
		t.Fatalf("Failed to remove session: %v", err)
	}

	// Verify removed
	if _, err := registry.Get("test-1"); err == nil {
		t.Error("Session should be removed")
	}

	// Remove non-existent session
	if err := registry.Remove("non-existent"); err == nil {
		t.Error("Expected error when removing non-existent session")
	}
}

func TestAgentSessionRegistry_Count(t *testing.T) {
	registry := NewAgentSessionRegistry()

	if registry.Count() != 0 {
		t.Errorf("Expected count 0, got %d", registry.Count())
	}

	registry.Register(&AgentSession{ID: "test-1", ServerName: "localhost", TmuxPaneID: "%1"})
	registry.Register(&AgentSession{ID: "test-2", ServerName: "localhost", TmuxPaneID: "%2"})

	if registry.Count() != 2 {
		t.Errorf("Expected count 2, got %d", registry.Count())
	}
}
