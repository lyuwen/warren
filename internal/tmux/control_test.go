package tmux

import (
	"testing"
)

// Integration test - requires tmux to be running
func TestClient_ValidatePane_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	executor := NewLocalExecutor()
	client := NewClient(executor)

	// Try to get a pane to test with
	sessions, err := client.ListSessions()
	if err != nil || len(sessions) == 0 {
		t.Skip("No tmux sessions found, skipping integration test")
	}

	session := sessions[0]
	windows, err := client.ListWindows(session.Name)
	if err != nil || len(windows) == 0 {
		t.Skip("No windows found, skipping integration test")
	}

	window := windows[0]
	panes, err := client.ListPanes(session.Name, window.Index)
	if err != nil || len(panes) == 0 {
		t.Skip("No panes found, skipping integration test")
	}

	pane := panes[0]

	// Test validation of existing pane
	if err := client.ValidatePane(pane.ID); err != nil {
		t.Errorf("Failed to validate existing pane: %v", err)
	}

	// Test validation of non-existent pane
	if err := client.ValidatePane("%999"); err == nil {
		t.Error("Expected error when validating non-existent pane")
	}
}

// Integration test - requires tmux to be running
func TestClient_GetPaneInfo_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	executor := NewLocalExecutor()
	client := NewClient(executor)

	// Try to get a pane to test with
	sessions, err := client.ListSessions()
	if err != nil || len(sessions) == 0 {
		t.Skip("No tmux sessions found, skipping integration test")
	}

	session := sessions[0]
	windows, err := client.ListWindows(session.Name)
	if err != nil || len(windows) == 0 {
		t.Skip("No windows found, skipping integration test")
	}

	window := windows[0]
	panes, err := client.ListPanes(session.Name, window.Index)
	if err != nil || len(panes) == 0 {
		t.Skip("No panes found, skipping integration test")
	}

	pane := panes[0]

	// Get pane info
	info, err := client.GetPaneInfo(pane.ID)
	if err != nil {
		t.Fatalf("Failed to get pane info: %v", err)
	}

	if info.ID != pane.ID {
		t.Errorf("Expected pane ID %s, got %s", pane.ID, info.ID)
	}

	if info.Width <= 0 {
		t.Errorf("Expected positive width, got %d", info.Width)
	}

	if info.Height <= 0 {
		t.Errorf("Expected positive height, got %d", info.Height)
	}
}

func TestSendTextOptions(t *testing.T) {
	opts := &SendTextOptions{
		Literal: true,
		Enter:   true,
	}

	if !opts.Literal {
		t.Error("Expected Literal to be true")
	}

	if !opts.Enter {
		t.Error("Expected Enter to be true")
	}
}

func TestSendKeysOptions(t *testing.T) {
	opts := &SendKeysOptions{
		Literal: true,
	}

	if !opts.Literal {
		t.Error("Expected Literal to be true")
	}
}
