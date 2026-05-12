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
			minConfidence: 0.6, // Lower confidence without AskUserQuestion tool
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

// Test new idle detection features
func TestStateDetector_IdleDetection_PromptSuffix(t *testing.T) {
	detector := NewStateDetector()

	tests := []struct {
		name          string
		content       string
		expectedState types.AgentState
		minConfidence float64
	}{
		{
			name:          "bash prompt",
			content:       "user@host:~/project$ ",
			expectedState: types.StateIdle,
			minConfidence: 0.95,
		},
		{
			name:          "generic prompt",
			content:       "Ready for input\n> ",
			expectedState: types.StateIdle,
			minConfidence: 0.95,
		},
		{
			name:          "waiting for input indicator",
			content:       "Waiting for input from user",
			expectedState: types.StateIdle,
			minConfidence: 0.8,
		},
		{
			name:          "standing by indicator",
			content:       "Standing by for next command",
			expectedState: types.StateIdle,
			minConfidence: 0.8,
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
		})
	}
}

// Test reduced idle timeout (30s instead of 5min)
func TestStateDetector_IdleDetection_ReducedTimeout(t *testing.T) {
	detector := NewStateDetector()

	tests := []struct {
		name          string
		age           time.Duration
		expectedState types.AgentState
		minStrength   float64
	}{
		{
			name:          "35 seconds old - should be idle",
			age:           35 * time.Second,
			expectedState: types.StateIdle,
			minStrength:   0.7,
		},
		{
			name:          "1 minute old - higher confidence",
			age:           1 * time.Minute,
			expectedState: types.StateIdle,
			minStrength:   0.8,
		},
		{
			name:          "2 minutes old - very high confidence",
			age:           2 * time.Minute,
			expectedState: types.StateIdle,
			minStrength:   0.9,
		},
		{
			name:          "20 seconds old - not idle yet",
			age:           20 * time.Second,
			expectedState: types.StateThinking,
			minStrength:   0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activities := []*events.AgentActivityEvent{
				{
					AgentID:      "test-agent",
					ActivityType: "chat",
					Content:      "Last message",
					Metadata:     map[string]string{"role": "assistant"},
					Timestamp:    time.Now().Add(-tt.age),
				},
			}

			result := detector.DetectFromActivities(activities)

			if result.State != tt.expectedState {
				t.Errorf("Expected state %s, got %s", tt.expectedState, result.State)
			}
		})
	}
}

// Test stricter question detection
func TestStateDetector_StricterQuestionDetection(t *testing.T) {
	detector := NewStateDetector()

	tests := []struct {
		name          string
		content       string
		expectedState types.AgentState
		description   string
	}{
		{
			name:          "question with tool - high confidence",
			content:       "Should I proceed?\nAskUserQuestion tool called",
			expectedState: types.StateAskingQuestion,
			description:   "Question at end with tool indicator",
		},
		{
			name:          "question without tool - lower confidence",
			content:       "Should I proceed?",
			expectedState: types.StateAskingQuestion,
			description:   "Question at end without tool",
		},
		{
			name:          "question in middle - should not detect",
			content:       "I was wondering if this works?\nNow executing the command...",
			expectedState: types.StateExecuting,
			description:   "Question not in last 3 lines",
		},
		{
			name:          "code comment with question - should not detect",
			content:       "// Should we refactor this?\nExecuting tests",
			expectedState: types.StateExecuting,
			description:   "Question in code comment",
		},
		{
			name:          "rhetorical question - should not detect strongly",
			content:       "This is interesting, isn't it?\nExecuting the next step",
			expectedState: types.StateExecuting,
			description:   "Rhetorical question without proper pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFromContent(tt.content)

			if result.State != tt.expectedState {
				t.Errorf("%s: Expected state %s, got %s", tt.description, tt.expectedState, result.State)
			}
		})
	}
}

// Test time-decay for signals
func TestStateDetector_TimeDecay(t *testing.T) {
	detector := NewStateDetector()

	tests := []struct {
		name        string
		signalAge   time.Duration
		baseStrength float64
		expectedDecay float64
	}{
		{
			name:          "fresh signal (10s) - no decay",
			signalAge:     10 * time.Second,
			baseStrength:  0.9,
			expectedDecay: 1.0,
		},
		{
			name:          "medium age (45s) - 50% decay",
			signalAge:     45 * time.Second,
			baseStrength:  0.9,
			expectedDecay: 0.5,
		},
		{
			name:          "old signal (3min) - 20% decay",
			signalAge:     3 * time.Minute,
			baseStrength:  0.9,
			expectedDecay: 0.2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signals := []*Signal{
				{
					State:     types.StateAskingQuestion,
					Strength:  tt.baseStrength,
					Evidence:  "test signal",
					Timestamp: time.Now().Add(-tt.signalAge),
				},
			}

			state, confidence := detector.determineState(signals)

			expectedConfidence := tt.baseStrength * tt.expectedDecay
			tolerance := 0.05

			if state != types.StateAskingQuestion {
				t.Errorf("Expected StateAskingQuestion, got %s", state)
			}

			if confidence < expectedConfidence-tolerance || confidence > expectedConfidence+tolerance {
				t.Errorf("Expected confidence ~%.2f (%.2f * %.2f), got %.2f",
					expectedConfidence, tt.baseStrength, tt.expectedDecay, confidence)
			}
		})
	}
}

// Test state priority adjustment (idle raised from 30 to 35)
func TestStateDetector_IdlePriorityRaised(t *testing.T) {
	detector := NewStateDetector()

	// Idle should now have priority 35
	if detector.statePriority[types.StateIdle] != 35 {
		t.Errorf("Expected idle priority 35, got %d", detector.statePriority[types.StateIdle])
	}

	// Test that idle signal is strong enough to be detected
	// Even with a weak thinking signal present
	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "chat",
			Content:      "Working on it",
			Metadata:     map[string]string{"role": "assistant"},
			Timestamp:    time.Now().Add(-2 * time.Minute), // Old enough for strong idle signal
		},
	}

	result := detector.DetectFromActivities(activities)

	// Should detect idle due to age and strong idle signal (0.9)
	// The thinking signal (0.5) will decay to 0.1 (20% after 2min)
	// Idle signal (0.9) should win even though thinking has higher priority (40 vs 35)
	// because the confidence difference is large enough
	if result.State != types.StateIdle {
		t.Errorf("Expected StateIdle with strong signal, got %s (confidence: %.2f)", result.State, result.Confidence)
	}
}

// Integration test: idle agent should show idle within 5 seconds
func TestStateDetector_Integration_IdleWithin5Seconds(t *testing.T) {
	detector := NewStateDetector()

	// Simulate an agent that finished responding 35 seconds ago
	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "chat",
			Content:      "Here's the implementation. Let me know if you need changes.",
			Metadata:     map[string]string{"role": "assistant"},
			Timestamp:    time.Now().Add(-35 * time.Second),
		},
	}

	result := detector.DetectFromActivities(activities)

	if result.State != types.StateIdle {
		t.Errorf("Expected StateIdle for agent idle >30s, got %s", result.State)
	}

	if result.Confidence < 0.7 {
		t.Errorf("Expected confidence >= 0.7 for idle detection, got %.2f", result.Confidence)
	}
}

// Test that old question signals don't cause false positives
func TestStateDetector_OldQuestionSignalsDecay(t *testing.T) {
	detector := NewStateDetector()

	// Old question (3 minutes ago) + recent idle indicator
	content := "Should I proceed with this approach?\n\n[... 3 minutes of output ...]\n\nReady for input\n> "

	result := detector.DetectFromContent(content)

	// Should detect idle (prompt suffix) over old question
	if result.State != types.StateIdle {
		t.Errorf("Expected StateIdle (prompt detected), got %s", result.State)
	}
}
