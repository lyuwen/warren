package notifications

import (
	"fmt"
	"sync"
	"time"

	"github.com/lfu/warren/internal/events"
)

// NotificationTrigger defines when to emit notifications
type NotificationTrigger string

const (
	TriggerPermissionRequired NotificationTrigger = "permission_required"
	TriggerQuestionAsked      NotificationTrigger = "question_asked"
	TriggerFinished           NotificationTrigger = "finished"
	TriggerError              NotificationTrigger = "error"
	TriggerStopped            NotificationTrigger = "stopped"
)

// Engine watches for state transitions and emits notifications
type Engine struct {
	store            *events.Store
	lastKnownStates  map[string]string
	mu               sync.RWMutex
	notificationChan chan *events.NotificationEvent
}

// NewEngine creates a new notification engine
func NewEngine(store *events.Store) *Engine {
	return &Engine{
		store:            store,
		lastKnownStates:  make(map[string]string),
		notificationChan: make(chan *events.NotificationEvent, 100),
	}
}

// ProcessStateChange checks if a state transition should trigger a notification
func (e *Engine) ProcessStateChange(agentID string, fromState, toState string) error {
	e.mu.Lock()
	e.lastKnownStates[agentID] = toState
	e.mu.Unlock()

	// Check if this state transition should trigger a notification
	trigger, shouldNotify := e.shouldNotify(toState)
	if !shouldNotify {
		return nil
	}

	// Create notification event
	notification := &events.NotificationEvent{
		AgentID:   agentID,
		NotifType: string(trigger),
		Message:   e.buildMessage(agentID, toState),
		Consumed:  false,
		Metadata: map[string]string{
			"from_state": fromState,
			"to_state":   toState,
		},
		Timestamp: time.Now(),
	}

	// Store notification
	if err := e.store.AppendNotification(notification); err != nil {
		return fmt.Errorf("failed to store notification: %w", err)
	}

	// Also emit to channel for real-time listeners
	select {
	case e.notificationChan <- notification:
	default:
		// Channel full, skip (notification is still in DB)
	}

	// Store state change event
	stateChange := &events.StateChangeEvent{
		AgentID:   agentID,
		FromState: fromState,
		ToState:   toState,
		Reason:    fmt.Sprintf("State transition triggered %s notification", trigger),
		Timestamp: time.Now(),
	}

	if err := e.store.AppendStateChange(stateChange); err != nil {
		return fmt.Errorf("failed to store state change: %w", err)
	}

	return nil
}

// shouldNotify determines if a state should trigger a notification
func (e *Engine) shouldNotify(state string) (NotificationTrigger, bool) {
	switch state {
	case "waiting_permission":
		return TriggerPermissionRequired, true
	case "asking_question":
		return TriggerQuestionAsked, true
	case "finished":
		return TriggerFinished, true
	case "error":
		return TriggerError, true
	case "stopped":
		return TriggerStopped, true
	default:
		return "", false
	}
}

// buildMessage creates a human-readable notification message
func (e *Engine) buildMessage(agentID string, state string) string {
	switch state {
	case "waiting_permission":
		return fmt.Sprintf("Agent %s is waiting for permission approval", agentID)
	case "asking_question":
		return fmt.Sprintf("Agent %s has a question that needs your answer", agentID)
	case "finished":
		return fmt.Sprintf("Agent %s has finished its task", agentID)
	case "error":
		return fmt.Sprintf("Agent %s encountered an error", agentID)
	case "stopped":
		return fmt.Sprintf("Agent %s has stopped", agentID)
	default:
		return fmt.Sprintf("Agent %s changed state to %s", agentID, state)
	}
}

// GetUnconsumedNotifications retrieves all unconsumed notifications
func (e *Engine) GetUnconsumedNotifications() ([]*events.NotificationEvent, error) {
	return e.store.GetUnconsumedNotifications()
}

// GetUnconsumedNotificationsByAgent retrieves unconsumed notifications for a specific agent
func (e *Engine) GetUnconsumedNotificationsByAgent(agentID string) ([]*events.NotificationEvent, error) {
	allNotifications, err := e.store.GetUnconsumedNotifications()
	if err != nil {
		return nil, err
	}

	filtered := make([]*events.NotificationEvent, 0)
	for _, notif := range allNotifications {
		if notif.AgentID == agentID {
			filtered = append(filtered, notif)
		}
	}

	return filtered, nil
}

// MarkAsConsumed marks a notification as consumed
// Note: This requires updating the notification in the event store
// Since events are immutable, we need to append a new version with Consumed=true
func (e *Engine) MarkAsConsumed(agentID string, notifType string, timestamp time.Time) error {
	// Query the specific notification
	eventList, err := e.store.Query(events.QueryOptions{
		AgentID:   agentID,
		EventType: events.EventTypeNotification,
		Limit:     1000,
	})

	if err != nil {
		return fmt.Errorf("failed to query notifications: %w", err)
	}

	// Find the matching notification and create a consumed version
	for _, event := range eventList {
		var notif events.NotificationEvent
		if err := event.UnmarshalData(&notif); err != nil {
			continue
		}

		if notif.AgentID == agentID &&
			notif.NotifType == notifType &&
			notif.Timestamp.Equal(timestamp) &&
			!notif.Consumed {

			// Create consumed version
			now := time.Now()
			consumedNotif := notif
			consumedNotif.Consumed = true
			consumedNotif.ConsumedAt = &now

			// Append as new event (immutable event store pattern)
			if err := e.store.AppendNotification(&consumedNotif); err != nil {
				return fmt.Errorf("failed to append consumed notification: %w", err)
			}

			return nil
		}
	}

	return fmt.Errorf("notification not found or already consumed")
}

// NotificationChannel returns a channel for real-time notification updates
func (e *Engine) NotificationChannel() <-chan *events.NotificationEvent {
	return e.notificationChan
}

// GetLastKnownState returns the last known state for an agent
func (e *Engine) GetLastKnownState(agentID string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state, exists := e.lastKnownStates[agentID]
	return state, exists
}

// UpdateLastKnownState updates the last known state for an agent
func (e *Engine) UpdateLastKnownState(agentID string, state string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.lastKnownStates[agentID] = state
}

// CountUnconsumedNotifications returns the count of unconsumed notifications
func (e *Engine) CountUnconsumedNotifications() (int, error) {
	notifications, err := e.GetUnconsumedNotifications()
	if err != nil {
		return 0, err
	}
	return len(notifications), nil
}

// CountUnconsumedNotificationsByAgent returns the count of unconsumed notifications for an agent
func (e *Engine) CountUnconsumedNotificationsByAgent(agentID string) (int, error) {
	notifications, err := e.GetUnconsumedNotificationsByAgent(agentID)
	if err != nil {
		return 0, err
	}
	return len(notifications), nil
}

// Close closes the notification engine
func (e *Engine) Close() {
	close(e.notificationChan)
}
