package state

import (
	"testing"
	"time"

	"github.com/lfu/warren/internal/types"
	"github.com/lfu/warren/internal/events"
)

func TestStateDetector_DetectFromActivities_Empty(t *testing.T) {
	detector := NewStateDetector()

	result := detector.DetectFromActivities([]*events.AgentActivityEvent{})

	if result.State != types.StateUnknown {
		t.Errorf("Expected StateUnknown for empty activities, got %s", result.State)
	}

	if result.Confidence < 0.0 || result.Confidence > 1.0 {
		t.Errorf("Confidence should be between 0 and 1, got %.2f", result.Confidence)
	}
}

func TestStateDetector_DetectFromActivities_Permission(t *testing.T) {
	detector := NewStateDetector()

	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "prompt",
			Content:      "Permission required to edit file",
			Metadata: map[string]string{
				"prompt_type": "permission",
			},
			Timestamp: time.Now(),
		},
	}

	result := detector.DetectFromActivities(activities)

	if result.State != types.StateWaitingPermission {
		t.Errorf("Expected StateWaitingPermission, got %s", result.State)
	}

	if result.Confidence < 0.9 {
		t.Errorf("Expected high confidence for permission prompt, got %.2f", result.Confidence)
	}
}

func TestStateDetector_DetectFromActivities_Question(t *testing.T) {
	detector := NewStateDetector()

	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "prompt",
			Content:      "Which approach should I use?",
			Metadata: map[string]string{
				"prompt_type": "question",
			},
			Timestamp: time.Now(),
		},
	}

	result := detector.DetectFromActivities(activities)

	if result.State != types.StateAskingQuestion {
		t.Errorf("Expected StateAskingQuestion, got %s", result.State)
	}

	if result.Confidence < 0.8 {
		t.Errorf("Expected high confidence for question, got %.2f", result.Confidence)
	}
}

func TestStateDetector_DetectFromActivities_Executing(t *testing.T) {
	detector := NewStateDetector()

	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "tool",
			Content:      "Executing bash command",
			Metadata: map[string]string{
				"tool_name": "bash",
			},
			Timestamp: time.Now(),
		},
	}

	result := detector.DetectFromActivities(activities)

	if result.State != types.StateExecuting {
		t.Errorf("Expected StateExecuting, got %s", result.State)
	}
}

func TestStateDetector_DetectFromActivities_Error(t *testing.T) {
	detector := NewStateDetector()

	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "chat",
			Content:      "Error: failed to compile",
			Timestamp:    time.Now(),
		},
	}

	result := detector.DetectFromActivities(activities)

	if result.State != types.StateError {
		t.Errorf("Expected StateError, got %s", result.State)
	}
}

func TestStateDetector_DetectFromActivities_Finished(t *testing.T) {
	detector := NewStateDetector()

	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "chat",
			Content:      "Task completed successfully",
			Timestamp:    time.Now(),
		},
	}

	result := detector.DetectFromActivities(activities)

	if result.State != types.StateFinished {
		t.Errorf("Expected StateFinished, got %s", result.State)
	}
}

func TestStateDetector_DetectFromActivities_Idle(t *testing.T) {
	detector := NewStateDetector()

	// Old activity (more than 5 minutes ago)
	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "chat",
			Content:      "Last message",
			Timestamp:    time.Now().Add(-10 * time.Minute),
		},
	}

	result := detector.DetectFromActivities(activities)

	if result.State != types.StateIdle {
		t.Errorf("Expected StateIdle for old activity, got %s", result.State)
	}
}

func TestStateDetector_DetectFromContent(t *testing.T) {
	detector := NewStateDetector()

	tests := []struct {
		name          string
		content       string
		expectedState types.AgentState
		minConfidence float64
	}{
		{
			name:          "permission prompt",
			content:       "Permission required to proceed [y/n]",
			expectedState: types.StateWaitingPermission,
			minConfidence: 0.9,
		},
		{
			name:          "question",
			content:       "Should I continue with this approach?",
			expectedState: types.StateAskingQuestion,
			minConfidence: 0.8,
		},
		{
			name:          "error",
			content:       "Error: connection failed",
			expectedState: types.StateError,
			minConfidence: 0.8,
		},
		{
			name:          "executing",
			content:       "Executing command: npm test",
			expectedState: types.StateExecuting,
			minConfidence: 0.7,
		},
		{
			name:          "finished",
			content:       "Task finished successfully",
			expectedState: types.StateFinished,
			minConfidence: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFromContent(tt.content)

			if result.State != tt.expectedState {
				t.Errorf("Expected state %s, got %s", tt.expectedState, result.State)
			}

			if result.Confidence < tt.minConfidence {
				t.Errorf("Expected confidence >= %.2f, got %.2f", tt.minConfidence, result.Confidence)
			}

			if len(result.Signals) == 0 {
				t.Error("Expected signals to be populated")
			}
		})
	}
}

func TestStateDetector_StatePriority(t *testing.T) {
	detector := NewStateDetector()

	// Error should have higher priority than executing
	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "tool",
			Content:      "Executing command",
			Metadata:     map[string]string{"tool_name": "bash"},
			Timestamp:    time.Now(),
		},
		{
			AgentID:      "test-agent",
			ActivityType: "chat",
			Content:      "Error occurred",
			Timestamp:    time.Now(),
		},
	}

	result := detector.DetectFromActivities(activities)

	// Error should win due to higher priority
	if result.State != types.StateError {
		t.Errorf("Expected StateError to have priority, got %s", result.State)
	}
}

func TestStateDetector_ShouldTransition(t *testing.T) {
	detector := NewStateDetector()

	tests := []struct {
		name           string
		currentState   types.AgentState
		newState       types.AgentState
		confidence     float64
		minConfidence  float64
		shouldTransit  bool
	}{
		{
			name:          "high confidence, different state, higher priority",
			currentState:  types.StateIdle,
			newState:      types.StateWaitingPermission,
			confidence:    0.95,
			minConfidence: 0.8,
			shouldTransit: true,
		},
		{
			name:          "low confidence",
			currentState:  types.StateIdle,
			newState:      types.StateThinking,
			confidence:    0.5,
			minConfidence: 0.8,
			shouldTransit: false,
		},
		{
			name:          "same state",
			currentState:  types.StateIdle,
			newState:      types.StateIdle,
			confidence:    0.95,
			minConfidence: 0.8,
			shouldTransit: false,
		},
		{
			name:          "lower priority state",
			currentState:  types.StateError,
			newState:      types.StateIdle,
			confidence:    0.95,
			minConfidence: 0.8,
			shouldTransit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &DetectionResult{
				State:      tt.newState,
				Confidence: tt.confidence,
			}

			shouldTransit := detector.ShouldTransition(tt.currentState, result, tt.minConfidence)

			if shouldTransit != tt.shouldTransit {
				t.Errorf("Expected shouldTransition=%v, got %v", tt.shouldTransit, shouldTransit)
			}
		})
	}
}

func TestStateDetector_MultipleSignals(t *testing.T) {
	detector := NewStateDetector()

	// Multiple activities of different types
	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "chat",
			Content:      "User: fix the bug",
			Metadata:     map[string]string{"role": "user"},
			Timestamp:    time.Now().Add(-2 * time.Minute),
		},
		{
			AgentID:      "test-agent",
			ActivityType: "file",
			Content:      "Reading file: test.go",
			Metadata:     map[string]string{"operation": "read"},
			Timestamp:    time.Now().Add(-1 * time.Minute),
		},
		{
			AgentID:      "test-agent",
			ActivityType: "tool",
			Content:      "Running tests",
			Metadata:     map[string]string{"tool_name": "bash"},
			Timestamp:    time.Now(),
		},
	}

	result := detector.DetectFromActivities(activities)

	// Should detect executing state (most recent and highest priority among these)
	if result.State != types.StateExecuting {
		t.Errorf("Expected StateExecuting, got %s", result.State)
	}

	// Should have multiple signals
	if len(result.Signals) < 2 {
		t.Errorf("Expected multiple signals, got %d", len(result.Signals))
	}
}
