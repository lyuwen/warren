package core

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lfu/warren/internal/events"
)

// testConfig creates a valid test configuration with a unique temp directory
func testConfig(dbPath string) *Config {
	return &Config{
		PollInterval:           100 * time.Millisecond,
		MinConfidence:          0.7,
		DBPath:                 dbPath,
		ConfigDir:              ".warren-test",
		EventRetentionPeriod:   30 * 24 * time.Hour,
		EventPruningInterval:   24 * time.Hour,
		CacheTTL:               5 * time.Second,
		RegistryPruneThreshold: 24 * time.Hour,
	}
}

func TestWarrenBasicLifecycle(t *testing.T) {
	// Create temporary database
	dbPath := "test_warren_basic.db"
	defer os.Remove(dbPath)

	config := testConfig(dbPath)

	warren, err := NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}
	defer warren.Stop()

	// Test adding sessions
	err = warren.AddSession("agent-1", "%0")
	if err != nil {
		t.Errorf("Failed to add session: %v", err)
	}

	// Test duplicate session
	err = warren.AddSession("agent-1", "%0")
	if err == nil {
		t.Error("Expected error when adding duplicate session")
	}

	// Test getting session state
	state, err := warren.GetSessionState("agent-1")
	if err != nil {
		t.Errorf("Failed to get session state: %v", err)
	}
	if state != StateUnknown {
		t.Errorf("Expected StateUnknown, got %v", state)
	}

	// Test removing session
	err = warren.RemoveSession("agent-1")
	if err != nil {
		t.Errorf("Failed to remove session: %v", err)
	}

	// Test removing non-existent session
	err = warren.RemoveSession("agent-1")
	if err == nil {
		t.Error("Expected error when removing non-existent session")
	}
}

func TestWarrenMultipleSessions(t *testing.T) {
	dbPath := "test_warren_multi.db"
	defer os.Remove(dbPath)

	config := testConfig(dbPath)

	warren, err := NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}
	defer warren.Stop()

	// Add multiple sessions
	sessions := []struct {
		agentID string
		paneID  string
	}{
		{"agent-1", "%0"},
		{"agent-2", "%1"},
		{"agent-3", "%2"},
	}

	for _, s := range sessions {
		err := warren.AddSession(s.agentID, s.paneID)
		if err != nil {
			t.Errorf("Failed to add session %s: %v", s.agentID, err)
		}
	}

	// Verify all sessions are tracked
	allSessions := warren.GetAllSessions()
	if len(allSessions) != len(sessions) {
		t.Errorf("Expected %d sessions, got %d", len(sessions), len(allSessions))
	}

	// Verify each session
	for _, s := range sessions {
		state, err := warren.GetSessionState(s.agentID)
		if err != nil {
			t.Errorf("Failed to get state for %s: %v", s.agentID, err)
		}
		if state != StateUnknown {
			t.Errorf("Expected StateUnknown for %s, got %v", s.agentID, state)
		}
	}
}

func TestWarrenEventStoreIntegration(t *testing.T) {
	dbPath := "test_warren_events.db"
	defer os.Remove(dbPath)

	config := testConfig(dbPath)

	warren, err := NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}
	defer warren.Stop()

	// Get event store
	store := warren.GetEventStore()
	if store == nil {
		t.Fatal("Event store is nil")
	}

	// Append test activity
	activity := &events.AgentActivityEvent{
		AgentID:      "test-agent",
		ActivityType: "chat",
		Content:      "user: test message",
		Metadata: map[string]string{
			"role": "user",
		},
		Timestamp: time.Now(),
	}

	err = store.AppendActivity(activity)
	if err != nil {
		t.Errorf("Failed to append activity: %v", err)
	}

	// Query activities
	activities, err := store.GetRecentActivities("test-agent", 10)
	if err != nil {
		t.Errorf("Failed to query activities: %v", err)
	}

	if len(activities) != 1 {
		t.Errorf("Expected 1 activity, got %d", len(activities))
	}

	if activities[0].Content != activity.Content {
		t.Errorf("Activity content mismatch: expected %s, got %s", activity.Content, activities[0].Content)
	}
}

func TestWarrenNotificationIntegration(t *testing.T) {
	dbPath := "test_warren_notif.db"
	defer os.Remove(dbPath)

	config := testConfig(dbPath)

	warren, err := NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}
	defer warren.Stop()

	// Get notification engine
	notifEngine := warren.GetNotificationEngine()
	if notifEngine == nil {
		t.Fatal("Notification engine is nil")
	}

	// Process state change that should trigger notification
	err = notifEngine.ProcessStateChange("test-agent", "idle", "waiting_permission")
	if err != nil {
		t.Errorf("Failed to process state change: %v", err)
	}

	// Check for unconsumed notifications
	notifications, err := warren.GetUnconsumedNotifications()
	if err != nil {
		t.Errorf("Failed to get notifications: %v", err)
	}

	if len(notifications) != 1 {
		t.Errorf("Expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].NotifType != "permission_required" {
		t.Errorf("Expected permission_required notification, got %s", notifications[0].NotifType)
	}
}

func TestWarrenArtifactProfileIntegration(t *testing.T) {
	dbPath := "test_warren_artifacts.db"
	defer os.Remove(dbPath)

	config := testConfig(dbPath)

	warren, err := NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}
	defer warren.Stop()

	// Simulate file activity
	activity := &events.AgentActivityEvent{
		AgentID:      "test-agent",
		ActivityType: "file",
		Content:      "Reading file: /tmp/test.go",
		Metadata: map[string]string{
			"operation": "read",
			"file_path": "/tmp/test.go",
		},
		Timestamp: time.Now(),
	}

	// Process through artifact manager
	err = warren.artifactManager.ProcessActivity(activity)
	if err != nil {
		t.Errorf("Failed to process activity: %v", err)
	}

	// Get artifact profile
	profile, err := warren.GetArtifactProfile("test-agent")
	if err != nil {
		t.Errorf("Failed to get artifact profile: %v", err)
	}

	if profile.TotalReads != 1 {
		t.Errorf("Expected 1 read, got %d", profile.TotalReads)
	}

	if len(profile.FilesVisited) != 1 {
		t.Errorf("Expected 1 file visited, got %d", len(profile.FilesVisited))
	}
}

func TestWarrenConcurrentSessions(t *testing.T) {
	dbPath := "test_warren_concurrent.db"
	defer os.Remove(dbPath)

	config := testConfig(dbPath)

	warren, err := NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}
	defer warren.Stop()

	// Add 10 concurrent sessions
	sessionCount := 10
	for i := 0; i < sessionCount; i++ {
		agentID := fmt.Sprintf("agent-%d", i)
		paneID := fmt.Sprintf("%%%d", i)
		err := warren.AddSession(agentID, paneID)
		if err != nil {
			t.Errorf("Failed to add session %s: %v", agentID, err)
		}
	}

	// Verify all sessions
	allSessions := warren.GetAllSessions()
	if len(allSessions) != sessionCount {
		t.Errorf("Expected %d sessions, got %d", sessionCount, len(allSessions))
	}

	// Concurrent reads
	done := make(chan bool)
	for i := 0; i < 20; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				warren.GetAllSessions()
				time.Sleep(5 * time.Millisecond)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestWarrenErrorHandling(t *testing.T) {
	dbPath := "test_warren_errors.db"
	defer os.Remove(dbPath)

	config := testConfig(dbPath)

	warren, err := NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}
	defer warren.Stop()

	// Add session with invalid pane ID
	err = warren.AddSession("error-agent", "invalid-pane")
	if err != nil {
		t.Errorf("Failed to add session: %v", err)
	}

	// Simulate poll error
	warren.handlePollError("error-agent", fmt.Errorf("test error"))

	// Check error count
	warren.mu.RLock()
	session := warren.sessions["error-agent"]
	warren.mu.RUnlock()

	if session.ErrorCount != 1 {
		t.Errorf("Expected error count 1, got %d", session.ErrorCount)
	}

	// Simulate multiple consecutive errors
	for i := 0; i < 5; i++ {
		warren.handlePollError("error-agent", fmt.Errorf("test error %d", i))
	}

	// Check state transitioned to error
	state, err := warren.GetSessionState("error-agent")
	if err != nil {
		t.Errorf("Failed to get session state: %v", err)
	}

	if state != StateError {
		t.Errorf("Expected StateError after consecutive errors, got %v", state)
	}
}

func TestWarrenGracefulShutdown(t *testing.T) {
	dbPath := "test_warren_shutdown.db"
	defer os.Remove(dbPath)

	config := testConfig(dbPath)

	warren, err := NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}

	// Add sessions
	warren.AddSession("agent-1", "%0")
	warren.AddSession("agent-2", "%1")

	// Test graceful shutdown
	err = warren.Stop()
	if err != nil {
		t.Errorf("Failed to stop Warren: %v", err)
	}

	// Verify context is cancelled
	select {
	case <-warren.ctx.Done():
		// Expected
	default:
		t.Error("Context not cancelled after Stop()")
	}
}

func TestWarrenStateTransition(t *testing.T) {
	dbPath := "test_warren_state.db"
	defer os.Remove(dbPath)

	config := testConfig(dbPath)

	warren, err := NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}
	defer warren.Stop()

	// Add session
	warren.AddSession("test-agent", "%0")

	// Manually update session state
	warren.mu.Lock()
	warren.sessions["test-agent"].CurrentState = StateIdle
	warren.mu.Unlock()

	// Process state change that triggers notification (idle -> waiting_permission)
	err = warren.notifEngine.ProcessStateChange("test-agent", "idle", "waiting_permission")
	if err != nil {
		t.Errorf("Failed to process state change: %v", err)
	}

	// Verify state change event was stored
	stateEvents, err := warren.eventStore.Query(events.QueryOptions{
		AgentID:   "test-agent",
		EventType: events.EventTypeStateChange,
		Limit:     10,
	})

	if err != nil {
		t.Errorf("Failed to query state events: %v", err)
	}

	if len(stateEvents) != 1 {
		t.Errorf("Expected 1 state change event, got %d", len(stateEvents))
	}
}
