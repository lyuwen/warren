package core

import (
	"strings"
	"testing"

	"github.com/lfu/warren/internal/tmux"
)

// MockExecutor for testing
type mockExecutor struct {
	captureContent string
}

func (m *mockExecutor) Execute(command string, args ...string) (string, error) {
	// Mock tmux capture-pane
	if command == "tmux" && len(args) > 0 && args[0] == "capture-pane" {
		return m.captureContent, nil
	}
	return "", nil
}

func TestAgentDiscovery_DetectClaudeCode(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		shouldFind bool
		minConf    float64
	}{
		{
			name:       "clear claude code indicator",
			content:    "Running claude code in this terminal",
			shouldFind: true,
			minConf:    0.9,
		},
		{
			name:       "anthropic sdk",
			content:    "import @anthropic-ai/sdk",
			shouldFind: true,
			minConf:    0.1,
		},
		{
			name:       "claude version",
			content:    "Claude 4.6 is running",
			shouldFind: true,
			minConf:    0.1,
		},
		{
			name:       "thinking tags",
			content:    "<thinking>analyzing the code</thinking>",
			shouldFind: true,
			minConf:    0.1,
		},
		{
			name:       "no agent",
			content:    "just a regular terminal session",
			shouldFind: false,
			minConf:    0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{captureContent: tt.content}
			client := tmux.NewClient(executor)
			discovery := NewAgentDiscovery(client)

			result, err := discovery.DiscoverInPane("test-server", "test-session", 0, "%1")
			if err != nil {
				t.Fatalf("Discovery failed: %v", err)
			}

			if tt.shouldFind {
				if result == nil {
					t.Error("Expected to find agent but got nil")
				} else {
					if result.AgentType != "claude-code" {
						t.Errorf("Expected agent type claude-code, got %s", result.AgentType)
					}
					if result.Confidence < tt.minConf {
						t.Errorf("Expected confidence >= %.2f, got %.2f", tt.minConf, result.Confidence)
					}
				}
			} else {
				if result != nil {
					t.Errorf("Expected no agent but found %s with confidence %.2f", result.AgentType, result.Confidence)
				}
			}
		})
	}
}

func TestAgentDiscovery_DetectCopilot(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		shouldFind bool
	}{
		{
			name:       "github copilot",
			content:    "Running GitHub Copilot CLI",
			shouldFind: true,
		},
		{
			name:       "gh copilot command",
			content:    "gh copilot suggest",
			shouldFind: true,
		},
		{
			name:       "no agent",
			content:    "just a regular terminal",
			shouldFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{captureContent: tt.content}
			client := tmux.NewClient(executor)
			discovery := NewAgentDiscovery(client)

			result, err := discovery.DiscoverInPane("test-server", "test-session", 0, "%1")
			if err != nil {
				t.Fatalf("Discovery failed: %v", err)
			}

			if tt.shouldFind {
				if result == nil {
					t.Error("Expected to find agent but got nil")
				} else if result.AgentType != "copilot" {
					t.Errorf("Expected agent type copilot, got %s", result.AgentType)
				}
			} else {
				if result != nil && result.AgentType == "copilot" {
					t.Error("Expected no copilot agent but found one")
				}
			}
		})
	}
}

func TestGenerateSessionID(t *testing.T) {
	result := &DiscoveryResult{
		ServerName:      "server1",
		TmuxSessionName: "session1",
		TmuxWindowIndex: 0,
		TmuxPaneID:      "%1",
		TmuxPaneIndex:   1,
	}

	id := GenerateSessionID(result)
	// New format: server:session:window.pane
	expected := "server1:session1:0.1"

	if id != expected {
		t.Errorf("Expected ID %s, got %s", expected, id)
	}
}

func TestDiscoveryResult_ToAgentSession(t *testing.T) {
	result := &DiscoveryResult{
		ServerName:      "server1",
		TmuxSessionName: "session1",
		TmuxWindowIndex: 0,
		TmuxPaneID:      "%1",
		TmuxPaneIndex:   1,
		AgentType:       "claude-code",
		Confidence:      0.95,
		Evidence:        []string{"matched pattern: claude code"},
	}

	session := result.ToAgentSession()

	// New format: server:session:window.pane
	expectedID := "server1:session1:0.1"
	if session.ID != expectedID {
		t.Errorf("Expected ID %s, got %s", expectedID, session.ID)
	}

	// New format with hostname: hostname/session:window.pane
	expectedName := "server1/session1:0.1"
	if session.Name != expectedName {
		t.Errorf("Expected Name %s, got %s", expectedName, session.Name)
	}

	if session.ServerName != "server1" {
		t.Errorf("Expected server name server1, got %s", session.ServerName)
	}

	if session.AgentType != "claude-code" {
		t.Errorf("Expected agent type claude-code, got %s", session.AgentType)
	}

	if session.CurrentState != StateUnknown {
		t.Errorf("Expected state unknown, got %s", session.CurrentState)
	}

	// Check metadata
	if conf, ok := session.Metadata["discovery_confidence"]; !ok || conf != "0.95" {
		t.Errorf("Expected confidence metadata 0.95, got %s", conf)
	}

	if evidence, ok := session.Metadata["discovery_evidence"]; !ok || !strings.Contains(evidence, "claude code") {
		t.Errorf("Expected evidence in metadata, got %s", evidence)
	}
}

func TestAgentDiscovery_DiscoverAll(t *testing.T) {
	// Create a mock topology
	pane1 := &tmux.Pane{ID: "%1", Index: 0}
	pane2 := &tmux.Pane{ID: "%2", Index: 1}

	window := &tmux.Window{
		Index: 0,
		Name:  "window1",
		Panes: []*tmux.Pane{pane1, pane2},
	}

	session := &tmux.TmuxSession{
		Name:    "session1",
		Windows: []*tmux.Window{window},
	}

	topology := &tmux.Topology{
		ServerName: "test-server",
		Sessions:   []*tmux.TmuxSession{session},
	}

	// Mock executor that returns claude code content for pane1
	executor := &mockExecutor{captureContent: "claude code is running here"}
	client := tmux.NewClient(executor)
	discovery := NewAgentDiscovery(client)

	results, err := discovery.DiscoverAll(topology, 0.5)
	if err != nil {
		t.Fatalf("DiscoverAll failed: %v", err)
	}

	// Should find at least one agent (both panes will match with same content)
	if len(results) == 0 {
		t.Error("Expected to find at least one agent")
	}

	for _, result := range results {
		if result.Confidence < 0.5 {
			t.Errorf("Expected confidence >= 0.5, got %.2f", result.Confidence)
		}
	}
}
