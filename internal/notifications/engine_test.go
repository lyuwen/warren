package notifications

import (
	"os"
	"testing"
	"time"

	"github.com/lfu/warren/internal/types"
	"github.com/lfu/warren/internal/events"
)

func setupTestStore(t *testing.T) *events.Store {
	tmpFile, err := os.CreateTemp("", "warren-test-*.db")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	store, err := events.NewStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
		os.Remove(tmpFile.Name())
	})

	return store
}

func TestNewEngine(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	if engine == nil {
		t.Fatal("expected non-nil engine")
	}

	if engine.store != store {
		t.Error("expected store to be set")
	}

	if engine.lastKnownStates == nil {
		t.Error("expected lastKnownStates to be initialized")
	}
}

func TestEngine_ProcessStateChange_PermissionRequired(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	agentID := "agent-1"
	fromState := types.StateIdle
	toState := types.StateWaitingPermission

	err := engine.ProcessStateChange(agentID, string(fromState), string(toState))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check that notification was created
	notifications, err := engine.GetUnconsumedNotifications()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}

	notif := notifications[0]
	if notif.AgentID != agentID {
		t.Errorf("expected agent ID %s, got %s", agentID, notif.AgentID)
	}

	if notif.NotifType != string(TriggerPermissionRequired) {
		t.Errorf("expected notification type %s, got %s", TriggerPermissionRequired, notif.NotifType)
	}

	if notif.Consumed {
		t.Error("expected notification to be unconsumed")
	}
}

func TestEngine_ProcessStateChange_QuestionAsked(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	agentID := "agent-1"
	err := engine.ProcessStateChange(agentID, string(types.StateThinking), string(types.StateAskingQuestion))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	notifications, err := engine.GetUnconsumedNotifications()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].NotifType != string(TriggerQuestionAsked) {
		t.Errorf("expected notification type %s, got %s", TriggerQuestionAsked, notifications[0].NotifType)
	}
}

func TestEngine_ProcessStateChange_Finished(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	agentID := "agent-1"
	err := engine.ProcessStateChange(agentID, string(types.StateExecuting), string(types.StateFinished))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	notifications, err := engine.GetUnconsumedNotifications()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].NotifType != string(TriggerFinished) {
		t.Errorf("expected notification type %s, got %s", TriggerFinished, notifications[0].NotifType)
	}
}

func TestEngine_ProcessStateChange_Error(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	agentID := "agent-1"
	err := engine.ProcessStateChange(agentID, string(types.StateExecuting), string(types.StateError))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	notifications, err := engine.GetUnconsumedNotifications()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].NotifType != string(TriggerError) {
		t.Errorf("expected notification type %s, got %s", TriggerError, notifications[0].NotifType)
	}
}

func TestEngine_ProcessStateChange_Stopped(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	agentID := "agent-1"
	err := engine.ProcessStateChange(agentID, string(types.StateExecuting), string(types.StateStopped))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	notifications, err := engine.GetUnconsumedNotifications()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}

	if notifications[0].NotifType != string(TriggerStopped) {
		t.Errorf("expected notification type %s, got %s", TriggerStopped, notifications[0].NotifType)
	}
}

func TestEngine_ProcessStateChange_NoNotification(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	agentID := "agent-1"
	// Idle and Thinking states should not trigger notifications
	err := engine.ProcessStateChange(agentID, string(types.StateIdle), string(types.StateThinking))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	notifications, err := engine.GetUnconsumedNotifications()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(notifications) != 0 {
		t.Errorf("expected 0 notifications, got %d", len(notifications))
	}
}

func TestEngine_GetUnconsumedNotificationsByAgent(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	// Create notifications for multiple agents
	engine.ProcessStateChange("agent-1", string(types.StateIdle), string(types.StateWaitingPermission))
	engine.ProcessStateChange("agent-2", string(types.StateIdle), string(types.StateAskingQuestion))
	engine.ProcessStateChange("agent-1", string(types.StateThinking), string(types.StateFinished))

	// Get notifications for agent-1
	notifications, err := engine.GetUnconsumedNotificationsByAgent("agent-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(notifications) != 2 {
		t.Errorf("expected 2 notifications for agent-1, got %d", len(notifications))
	}

	// Get notifications for agent-2
	notifications, err = engine.GetUnconsumedNotificationsByAgent("agent-2")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(notifications) != 1 {
		t.Errorf("expected 1 notification for agent-2, got %d", len(notifications))
	}
}

func TestEngine_MarkAsConsumed(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	agentID := "agent-1"
	engine.ProcessStateChange(agentID, string(types.StateIdle), string(types.StateWaitingPermission))

	// Get the notification
	notifications, err := engine.GetUnconsumedNotifications()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}

	notif := notifications[0]

	// Mark as consumed
	err = engine.MarkAsConsumed(notif.AgentID, notif.NotifType, notif.Timestamp)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check that it's no longer unconsumed
	notifications, err = engine.GetUnconsumedNotifications()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(notifications) != 0 {
		t.Errorf("expected 0 unconsumed notifications, got %d", len(notifications))
	}
}

func TestEngine_MarkAsConsumed_NotFound(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	// Try to mark non-existent notification as consumed
	err := engine.MarkAsConsumed("agent-1", "permission_required", time.Now())
	if err == nil {
		t.Error("expected error for non-existent notification")
	}
}

func TestEngine_CountUnconsumedNotifications(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	count, err := engine.CountUnconsumedNotifications()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if count != 0 {
		t.Errorf("expected 0 notifications, got %d", count)
	}

	// Create some notifications
	engine.ProcessStateChange("agent-1", string(types.StateIdle), string(types.StateWaitingPermission))
	engine.ProcessStateChange("agent-2", string(types.StateIdle), string(types.StateAskingQuestion))

	count, err = engine.CountUnconsumedNotifications()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 notifications, got %d", count)
	}
}

func TestEngine_CountUnconsumedNotificationsByAgent(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	engine.ProcessStateChange("agent-1", string(types.StateIdle), string(types.StateWaitingPermission))
	engine.ProcessStateChange("agent-1", string(types.StateThinking), string(types.StateFinished))
	engine.ProcessStateChange("agent-2", string(types.StateIdle), string(types.StateAskingQuestion))

	count, err := engine.CountUnconsumedNotificationsByAgent("agent-1")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 notifications for agent-1, got %d", count)
	}

	count, err = engine.CountUnconsumedNotificationsByAgent("agent-2")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 notification for agent-2, got %d", count)
	}
}

func TestEngine_GetLastKnownState(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	agentID := "agent-1"

	// Initially no state
	_, exists := engine.GetLastKnownState(agentID)
	if exists {
		t.Error("expected no state initially")
	}

	// Process state change
	engine.ProcessStateChange(agentID, string(types.StateIdle), string(types.StateWaitingPermission))

	// Check last known state
	state, exists := engine.GetLastKnownState(agentID)
	if !exists {
		t.Error("expected state to exist")
	}

	if state != string(types.StateWaitingPermission) {
		t.Errorf("expected state %s, got %s", types.StateWaitingPermission, state)
	}
}

func TestEngine_UpdateLastKnownState(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	agentID := "agent-1"
	state := types.StateThinking

	engine.UpdateLastKnownState(agentID, string(state))

	retrievedState, exists := engine.GetLastKnownState(agentID)
	if !exists {
		t.Error("expected state to exist")
	}

	if retrievedState != string(state) {
		t.Errorf("expected state %s, got %s", state, retrievedState)
	}
}

func TestEngine_NotificationChannel(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	ch := engine.NotificationChannel()
	if ch == nil {
		t.Error("expected non-nil notification channel")
	}

	// Process state change in goroutine
	go func() {
		engine.ProcessStateChange("agent-1", string(types.StateIdle), string(types.StateWaitingPermission))
	}()

	// Wait for notification on channel
	select {
	case notif := <-ch:
		if notif.AgentID != "agent-1" {
			t.Errorf("expected agent-1, got %s", notif.AgentID)
		}
	case <-time.After(1 * time.Second):
		t.Error("timeout waiting for notification")
	}
}

func TestEngine_BuildMessage(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	tests := []struct {
		state    types.AgentState
		expected string
	}{
		{types.StateWaitingPermission, "Agent agent-1 is waiting for permission approval"},
		{types.StateAskingQuestion, "Agent agent-1 has a question that needs your answer"},
		{types.StateFinished, "Agent agent-1 has finished its task"},
		{types.StateError, "Agent agent-1 encountered an error"},
		{types.StateStopped, "Agent agent-1 has stopped"},
	}

	for _, tt := range tests {
		msg := engine.buildMessage("agent-1", string(tt.state))
		if msg != tt.expected {
			t.Errorf("for state %s, expected message %q, got %q", tt.state, tt.expected, msg)
		}
	}
}

func TestEngine_ShouldNotify(t *testing.T) {
	store := setupTestStore(t)
	engine := NewEngine(store)

	tests := []struct {
		state          types.AgentState
		shouldNotify   bool
		expectedTrigger NotificationTrigger
	}{
		{types.StateWaitingPermission, true, TriggerPermissionRequired},
		{types.StateAskingQuestion, true, TriggerQuestionAsked},
		{types.StateFinished, true, TriggerFinished},
		{types.StateError, true, TriggerError},
		{types.StateStopped, true, TriggerStopped},
		{types.StateIdle, false, ""},
		{types.StateThinking, false, ""},
		{types.StateExecuting, false, ""},
		{types.StateUnknown, false, ""},
	}

	for _, tt := range tests {
		trigger, shouldNotify := engine.shouldNotify(string(tt.state))
		if shouldNotify != tt.shouldNotify {
			t.Errorf("for state %s, expected shouldNotify=%v, got %v", tt.state, tt.shouldNotify, shouldNotify)
		}
		if shouldNotify && trigger != tt.expectedTrigger {
			t.Errorf("for state %s, expected trigger %s, got %s", tt.state, tt.expectedTrigger, trigger)
		}
	}
}
