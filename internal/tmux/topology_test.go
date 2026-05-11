package tmux

import (
	"testing"
)

func TestTopology_FindPane(t *testing.T) {
	// Create test topology
	pane1 := &Pane{ID: "%1", Index: 0, Title: "pane1"}
	pane2 := &Pane{ID: "%2", Index: 1, Title: "pane2"}

	window1 := &Window{
		Index: 0,
		Name:  "window1",
		ID:    "@1",
		Panes: []*Pane{pane1, pane2},
	}

	session1 := &TmuxSession{
		Name:    "session1",
		ID:      "$1",
		Windows: []*Window{window1},
	}

	topology := &Topology{
		ServerName: "test-server",
		Sessions:   []*TmuxSession{session1},
	}

	// Test finding existing pane
	foundPane, foundWindow, foundSession, err := topology.FindPane("%1")
	if err != nil {
		t.Fatalf("Failed to find pane: %v", err)
	}

	if foundPane.ID != "%1" {
		t.Errorf("Expected pane ID %%1, got %s", foundPane.ID)
	}

	if foundWindow.ID != "@1" {
		t.Errorf("Expected window ID @1, got %s", foundWindow.ID)
	}

	if foundSession.ID != "$1" {
		t.Errorf("Expected session ID $1, got %s", foundSession.ID)
	}

	// Test finding non-existent pane
	_, _, _, err = topology.FindPane("%999")
	if err == nil {
		t.Error("Expected error when finding non-existent pane")
	}
}

func TestLocalExecutor(t *testing.T) {
	executor := NewLocalExecutor()

	// Test simple command
	output, err := executor.Execute("echo", "test")
	if err != nil {
		t.Fatalf("Failed to execute command: %v", err)
	}

	if output != "test\n" {
		t.Errorf("Expected 'test\\n', got %q", output)
	}

	// Test command with error
	_, err = executor.Execute("false")
	if err == nil {
		t.Error("Expected error from false command")
	}
}

func TestRemoteExecutor(t *testing.T) {
	// This is a placeholder - real testing would require SSH setup
	executor := NewRemoteExecutor("user", "localhost", 22)

	if executor == nil {
		t.Fatal("Failed to create remote executor")
	}

	if executor.host != "localhost" {
		t.Errorf("Expected host localhost, got %s", executor.host)
	}

	if executor.user != "user" {
		t.Errorf("Expected user 'user', got %s", executor.user)
	}

	if executor.port != 22 {
		t.Errorf("Expected port 22, got %d", executor.port)
	}
}

// Integration test - requires tmux to be running
func TestClient_ListSessions_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	executor := NewLocalExecutor()
	client := NewClient(executor)

	sessions, err := client.ListSessions()
	if err != nil {
		t.Skip("Tmux not running, skipping integration test")
	}

	// If tmux is running, we should get at least an empty list
	if sessions == nil {
		t.Error("Expected non-nil sessions list")
	}
}

// Integration test - requires tmux to be running
func TestClient_DiscoverTopology_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	executor := NewLocalExecutor()
	client := NewClient(executor)

	topology, err := client.DiscoverTopology("test-server")
	if err != nil {
		t.Skip("Tmux not running, skipping integration test")
	}

	if topology == nil {
		t.Fatal("Expected non-nil topology")
	}

	if topology.ServerName != "test-server" {
		t.Errorf("Expected server name 'test-server', got %s", topology.ServerName)
	}

	// If we have sessions, verify structure
	for _, session := range topology.Sessions {
		if session.ServerRef != "test-server" {
			t.Errorf("Expected session ServerRef 'test-server', got %s", session.ServerRef)
		}

		for _, window := range session.Windows {
			if window.SessionID != session.ID {
				t.Errorf("Expected window SessionID %s, got %s", session.ID, window.SessionID)
			}

			for _, pane := range window.Panes {
				if pane.WindowID != window.ID {
					t.Errorf("Expected pane WindowID %s, got %s", window.ID, pane.WindowID)
				}
			}
		}
	}
}
