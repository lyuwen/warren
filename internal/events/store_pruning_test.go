package events

import (
	"os"
	"testing"
	"time"
)

func TestPruneOldEvents(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "warren-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create store with short retention for testing
	config := &StoreConfig{
		DBPath:          tmpFile.Name(),
		RetentionPeriod: 7 * 24 * time.Hour, // 7 days
		PruningInterval: 1 * time.Hour,
	}
	store, err := NewStoreWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Insert events with different ages
	now := time.Now()

	// Old events (should be pruned)
	oldEvent1 := &AgentActivityEvent{
		AgentID:      "agent1",
		ActivityType: "chat",
		Content:      "old message 1",
		Timestamp:    now.Add(-10 * 24 * time.Hour), // 10 days ago
	}
	oldEvent2 := &AgentActivityEvent{
		AgentID:      "agent2",
		ActivityType: "file",
		Content:      "old file edit",
		Timestamp:    now.Add(-30 * 24 * time.Hour), // 30 days ago
	}

	// Recent events (should be kept)
	recentEvent1 := &AgentActivityEvent{
		AgentID:      "agent1",
		ActivityType: "chat",
		Content:      "recent message",
		Timestamp:    now.Add(-2 * 24 * time.Hour), // 2 days ago
	}
	recentEvent2 := &AgentActivityEvent{
		AgentID:      "agent3",
		ActivityType: "tool",
		Content:      "recent tool use",
		Timestamp:    now.Add(-1 * time.Hour), // 1 hour ago
	}

	// Insert all events
	if err := store.AppendActivity(oldEvent1); err != nil {
		t.Fatalf("Failed to insert old event 1: %v", err)
	}
	if err := store.AppendActivity(oldEvent2); err != nil {
		t.Fatalf("Failed to insert old event 2: %v", err)
	}
	if err := store.AppendActivity(recentEvent1); err != nil {
		t.Fatalf("Failed to insert recent event 1: %v", err)
	}
	if err := store.AppendActivity(recentEvent2); err != nil {
		t.Fatalf("Failed to insert recent event 2: %v", err)
	}

	// Verify all events exist
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if count != 4 {
		t.Errorf("Expected 4 events, got %d", count)
	}

	// Prune events older than 7 days
	deleted, err := store.PruneOldEvents(7 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("Failed to prune events: %v", err)
	}

	// Should have deleted 2 old events
	if deleted != 2 {
		t.Errorf("Expected 2 deleted events, got %d", deleted)
	}

	// Verify only recent events remain
	count, err = store.Count()
	if err != nil {
		t.Fatalf("Failed to count events after pruning: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 events after pruning, got %d", count)
	}

	// Verify the correct events were kept
	events, err := store.Query(QueryOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events in query, got %d", len(events))
	}

	// Check that recent events are present
	foundRecent1 := false
	foundRecent2 := false
	for _, event := range events {
		var activity AgentActivityEvent
		if err := event.UnmarshalData(&activity); err != nil {
			t.Fatalf("Failed to unmarshal event: %v", err)
		}

		if activity.Content == "recent message" {
			foundRecent1 = true
		}
		if activity.Content == "recent tool use" {
			foundRecent2 = true
		}
	}

	if !foundRecent1 || !foundRecent2 {
		t.Error("Recent events were not found after pruning")
	}
}

func TestPruneOldEvents_NoOldEvents(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "warren-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := NewStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Insert only recent events
	now := time.Now()
	recentEvent := &AgentActivityEvent{
		AgentID:      "agent1",
		ActivityType: "chat",
		Content:      "recent message",
		Timestamp:    now.Add(-1 * time.Hour),
	}

	if err := store.AppendActivity(recentEvent); err != nil {
		t.Fatalf("Failed to insert event: %v", err)
	}

	// Prune events older than 30 days
	deleted, err := store.PruneOldEvents(30 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("Failed to prune events: %v", err)
	}

	// Should have deleted 0 events
	if deleted != 0 {
		t.Errorf("Expected 0 deleted events, got %d", deleted)
	}

	// Verify event still exists
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 event, got %d", count)
	}
}

func TestPruneOldEvents_EmptyDatabase(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "warren-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := NewStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Prune on empty database
	deleted, err := store.PruneOldEvents(30 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("Failed to prune events: %v", err)
	}

	// Should have deleted 0 events
	if deleted != 0 {
		t.Errorf("Expected 0 deleted events, got %d", deleted)
	}
}

func TestPruneOldEvents_ConfigurableRetention(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "warren-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := NewStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now()

	// Insert events at different ages
	events := []*AgentActivityEvent{
		{
			AgentID:      "agent1",
			ActivityType: "chat",
			Content:      "1 day old",
			Timestamp:    now.Add(-1 * 24 * time.Hour),
		},
		{
			AgentID:      "agent2",
			ActivityType: "chat",
			Content:      "5 days old",
			Timestamp:    now.Add(-5 * 24 * time.Hour),
		},
		{
			AgentID:      "agent3",
			ActivityType: "chat",
			Content:      "15 days old",
			Timestamp:    now.Add(-15 * 24 * time.Hour),
		},
		{
			AgentID:      "agent4",
			ActivityType: "chat",
			Content:      "60 days old",
			Timestamp:    now.Add(-60 * 24 * time.Hour),
		},
	}

	for _, event := range events {
		if err := store.AppendActivity(event); err != nil {
			t.Fatalf("Failed to insert event: %v", err)
		}
	}

	// Test different retention periods
	testCases := []struct {
		retention       time.Duration
		expectedDeleted int
		expectedRemain  int
	}{
		{90 * 24 * time.Hour, 0, 4}, // Keep all
		{30 * 24 * time.Hour, 1, 3}, // Delete 60-day-old
		{10 * 24 * time.Hour, 2, 2}, // Delete 60 and 15-day-old
		{3 * 24 * time.Hour, 3, 1},  // Delete all except 1-day-old
	}

	for _, tc := range testCases {
		// Reset database
		store.db.Exec("DELETE FROM events")
		for _, event := range events {
			store.AppendActivity(event)
		}

		deleted, err := store.PruneOldEvents(tc.retention)
		if err != nil {
			t.Fatalf("Failed to prune with retention %v: %v", tc.retention, err)
		}

		if deleted != tc.expectedDeleted {
			t.Errorf("Retention %v: expected %d deleted, got %d",
				tc.retention, tc.expectedDeleted, deleted)
		}

		count, err := store.Count()
		if err != nil {
			t.Fatalf("Failed to count events: %v", err)
		}

		if int(count) != tc.expectedRemain {
			t.Errorf("Retention %v: expected %d remaining, got %d",
				tc.retention, tc.expectedRemain, count)
		}
	}
}

func TestPruneOldEvents_MultipleEventTypes(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "warren-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := NewStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	now := time.Now()

	// Insert old events of different types
	oldActivity := &AgentActivityEvent{
		AgentID:      "agent1",
		ActivityType: "chat",
		Content:      "old activity",
		Timestamp:    now.Add(-40 * 24 * time.Hour),
	}

	oldNotification := &NotificationEvent{
		AgentID:   "agent1",
		NotifType: "permission_required",
		Message:   "old notification",
		Timestamp: now.Add(-40 * 24 * time.Hour),
	}

	oldStateChange := &StateChangeEvent{
		AgentID:   "agent1",
		FromState: "idle",
		ToState:   "working",
		Timestamp: now.Add(-40 * 24 * time.Hour),
	}

	// Insert recent events
	recentActivity := &AgentActivityEvent{
		AgentID:      "agent1",
		ActivityType: "chat",
		Content:      "recent activity",
		Timestamp:    now.Add(-1 * time.Hour),
	}

	if err := store.AppendActivity(oldActivity); err != nil {
		t.Fatalf("Failed to insert old activity: %v", err)
	}
	if err := store.AppendNotification(oldNotification); err != nil {
		t.Fatalf("Failed to insert old notification: %v", err)
	}
	if err := store.AppendStateChange(oldStateChange); err != nil {
		t.Fatalf("Failed to insert old state change: %v", err)
	}
	if err := store.AppendActivity(recentActivity); err != nil {
		t.Fatalf("Failed to insert recent activity: %v", err)
	}

	// Prune events older than 30 days
	deleted, err := store.PruneOldEvents(30 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("Failed to prune events: %v", err)
	}

	// Should delete all 3 old events regardless of type
	if deleted != 3 {
		t.Errorf("Expected 3 deleted events, got %d", deleted)
	}

	// Verify only recent event remains
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 event after pruning, got %d", count)
	}
}

func TestStoreConfig_Defaults(t *testing.T) {
	config := DefaultStoreConfig()

	if config.RetentionPeriod != 30*24*time.Hour {
		t.Errorf("Expected default retention period of 30 days, got %v", config.RetentionPeriod)
	}

	if config.PruningInterval != 24*time.Hour {
		t.Errorf("Expected default pruning interval of 24 hours, got %v", config.PruningInterval)
	}

	if config.DBPath != "warren.db" {
		t.Errorf("Expected default DB path 'warren.db', got %s", config.DBPath)
	}
}

func TestNewStoreWithConfig(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "warren-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	config := &StoreConfig{
		DBPath:          tmpFile.Name(),
		RetentionPeriod: 7 * 24 * time.Hour,
		PruningInterval: 12 * time.Hour,
	}

	store, err := NewStoreWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create store with config: %v", err)
	}
	defer store.Close()

	if store.retentionPeriod != config.RetentionPeriod {
		t.Errorf("Expected retention period %v, got %v", config.RetentionPeriod, store.retentionPeriod)
	}

	if store.pruningInterval != config.PruningInterval {
		t.Errorf("Expected pruning interval %v, got %v", config.PruningInterval, store.pruningInterval)
	}
}

func TestNewStoreWithConfig_InvalidValues(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "warren-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Test with zero/negative values - should use defaults
	config := &StoreConfig{
		DBPath:          tmpFile.Name(),
		RetentionPeriod: 0,
		PruningInterval: -1 * time.Hour,
	}

	store, err := NewStoreWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create store with invalid config: %v", err)
	}
	defer store.Close()

	// Should fall back to defaults
	if store.retentionPeriod != 30*24*time.Hour {
		t.Errorf("Expected default retention period 30 days, got %v", store.retentionPeriod)
	}

	if store.pruningInterval != 24*time.Hour {
		t.Errorf("Expected default pruning interval 24 hours, got %v", store.pruningInterval)
	}
}

func TestStartPruningJob(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "warren-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create store with very short intervals for testing
	config := &StoreConfig{
		DBPath:          tmpFile.Name(),
		RetentionPeriod: 1 * time.Second,
		PruningInterval: 100 * time.Millisecond,
	}

	store, err := NewStoreWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Insert old and new events
	now := time.Now()
	oldEvent := &AgentActivityEvent{
		AgentID:      "agent1",
		ActivityType: "chat",
		Content:      "old event",
		Timestamp:    now.Add(-2 * time.Second),
	}
	newEvent := &AgentActivityEvent{
		AgentID:      "agent2",
		ActivityType: "chat",
		Content:      "new event",
		Timestamp:    now,
	}

	if err := store.AppendActivity(oldEvent); err != nil {
		t.Fatalf("Failed to insert old event: %v", err)
	}
	if err := store.AppendActivity(newEvent); err != nil {
		t.Fatalf("Failed to insert new event: %v", err)
	}

	// Verify both events exist
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 events, got %d", count)
	}

	// Start pruning job
	store.StartPruningJob()

	// Wait for pruning to run (initial run + one interval)
	time.Sleep(300 * time.Millisecond)

	// Old event should be pruned
	count, err = store.Count()
	if err != nil {
		t.Fatalf("Failed to count events after pruning: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 event after pruning, got %d", count)
	}

	// Verify the correct event remains
	events, err := store.Query(QueryOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	var activity AgentActivityEvent
	if err := events[0].UnmarshalData(&activity); err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	if activity.Content != "new event" {
		t.Errorf("Expected 'new event', got '%s'", activity.Content)
	}
}

func TestStartPruningJob_StopOnClose(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "warren-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	config := &StoreConfig{
		DBPath:          tmpFile.Name(),
		RetentionPeriod: 1 * time.Hour,
		PruningInterval: 50 * time.Millisecond,
	}

	store, err := NewStoreWithConfig(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Start pruning job
	store.StartPruningJob()

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)

	// Close should stop the pruning job
	if err := store.Close(); err != nil {
		t.Fatalf("Failed to close store: %v", err)
	}

	// Give goroutine time to exit
	time.Sleep(100 * time.Millisecond)

	// Test passes if no panic or deadlock occurs
}
