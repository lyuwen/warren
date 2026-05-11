package events

import (
	"os"
	"testing"
	"time"
)

func TestStore_Initialize(t *testing.T) {
	// Create temporary database
	tmpFile := t.TempDir() + "/test.db"

	store, err := NewStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Verify database file was created
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestStore_AppendActivity(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	store, err := NewStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	event := &AgentActivityEvent{
		AgentID:      "test-agent-1",
		ActivityType: "chat",
		Content:      "Hello, world!",
		Metadata: map[string]string{
			"role": "user",
		},
		Timestamp: time.Now(),
	}

	if err := store.AppendActivity(event); err != nil {
		t.Fatalf("Failed to append activity: %v", err)
	}

	// Verify event was stored
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 event, got %d", count)
	}
}

func TestStore_AppendNotification(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	store, err := NewStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	event := &NotificationEvent{
		AgentID:   "test-agent-1",
		NotifType: "permission_required",
		Message:   "Agent needs permission to edit file",
		Consumed:  false,
		Timestamp: time.Now(),
	}

	if err := store.AppendNotification(event); err != nil {
		t.Fatalf("Failed to append notification: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 event, got %d", count)
	}
}

func TestStore_AppendStateChange(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	store, err := NewStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	event := &StateChangeEvent{
		AgentID:   "test-agent-1",
		FromState: "idle",
		ToState:   "thinking",
		Reason:    "User sent message",
		Timestamp: time.Now(),
	}

	if err := store.AppendStateChange(event); err != nil {
		t.Fatalf("Failed to append state change: %v", err)
	}

	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 event, got %d", count)
	}
}

func TestStore_Query(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	store, err := NewStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Add multiple events
	now := time.Now()
	events := []*AgentActivityEvent{
		{AgentID: "agent-1", ActivityType: "chat", Content: "msg1", Timestamp: now.Add(-3 * time.Hour)},
		{AgentID: "agent-1", ActivityType: "file", Content: "edit1", Timestamp: now.Add(-2 * time.Hour)},
		{AgentID: "agent-2", ActivityType: "chat", Content: "msg2", Timestamp: now.Add(-1 * time.Hour)},
	}

	for _, event := range events {
		if err := store.AppendActivity(event); err != nil {
			t.Fatalf("Failed to append event: %v", err)
		}
	}

	// Query all events
	allEvents, err := store.Query(QueryOptions{})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(allEvents) != 3 {
		t.Errorf("Expected 3 events, got %d", len(allEvents))
	}

	// Query by agent ID
	agent1Events, err := store.Query(QueryOptions{AgentID: "agent-1"})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(agent1Events) != 2 {
		t.Errorf("Expected 2 events for agent-1, got %d", len(agent1Events))
	}

	// Query by type
	activityEvents, err := store.Query(QueryOptions{EventType: EventTypeActivity})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(activityEvents) != 3 {
		t.Errorf("Expected 3 activity events, got %d", len(activityEvents))
	}

	// Query with time range
	since := now.Add(-2*time.Hour - 30*time.Minute)
	timeRangeEvents, err := store.Query(QueryOptions{Since: &since})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(timeRangeEvents) != 2 {
		t.Errorf("Expected 2 events in time range, got %d", len(timeRangeEvents))
	}

	// Query with limit
	limitedEvents, err := store.Query(QueryOptions{Limit: 2})
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}

	if len(limitedEvents) != 2 {
		t.Errorf("Expected 2 events with limit, got %d", len(limitedEvents))
	}
}

func TestStore_GetRecentActivities(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	store, err := NewStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Add activities
	for i := 0; i < 5; i++ {
		event := &AgentActivityEvent{
			AgentID:      "test-agent",
			ActivityType: "chat",
			Content:      "message",
			Timestamp:    time.Now(),
		}
		store.AppendActivity(event)
	}

	activities, err := store.GetRecentActivities("test-agent", 3)
	if err != nil {
		t.Fatalf("Failed to get recent activities: %v", err)
	}

	if len(activities) != 3 {
		t.Errorf("Expected 3 activities, got %d", len(activities))
	}
}

func TestStore_GetUnconsumedNotifications(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	store, err := NewStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Add notifications
	consumed := time.Now()
	notifications := []*NotificationEvent{
		{AgentID: "agent-1", NotifType: "permission_required", Message: "msg1", Consumed: false, Timestamp: time.Now()},
		{AgentID: "agent-1", NotifType: "question_asked", Message: "msg2", Consumed: true, ConsumedAt: &consumed, Timestamp: time.Now()},
		{AgentID: "agent-2", NotifType: "error", Message: "msg3", Consumed: false, Timestamp: time.Now()},
	}

	for _, notif := range notifications {
		store.AppendNotification(notif)
	}

	unconsumed, err := store.GetUnconsumedNotifications()
	if err != nil {
		t.Fatalf("Failed to get unconsumed notifications: %v", err)
	}

	if len(unconsumed) != 2 {
		t.Errorf("Expected 2 unconsumed notifications, got %d", len(unconsumed))
	}
}

func TestStore_CleanupOldEvents(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	store, err := NewStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Add old and new events
	oldEvent := &AgentActivityEvent{
		AgentID:   "test-agent",
		Timestamp: time.Now().AddDate(0, 0, -40), // 40 days ago
	}

	newEvent := &AgentActivityEvent{
		AgentID:   "test-agent",
		Timestamp: time.Now(),
	}

	store.AppendActivity(oldEvent)
	store.AppendActivity(newEvent)

	// Cleanup events older than 30 days
	deleted, err := store.CleanupOldEvents(30)
	if err != nil {
		t.Fatalf("Failed to cleanup old events: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 deleted event, got %d", deleted)
	}

	// Verify only new event remains
	count, _ := store.Count()
	if count != 1 {
		t.Errorf("Expected 1 remaining event, got %d", count)
	}
}

func TestStore_Count(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	store, err := NewStore(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Initially empty
	count, err := store.Count()
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 events, got %d", count)
	}

	// Add events
	for i := 0; i < 3; i++ {
		event := &AgentActivityEvent{
			AgentID:   "test-agent",
			Timestamp: time.Now(),
		}
		store.AppendActivity(event)
	}

	count, err = store.Count()
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 events, got %d", count)
	}
}
