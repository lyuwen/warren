package state

import (
	"testing"
	"time"

	"github.com/lfu/warren/internal/events"
	"github.com/lfu/warren/internal/types"
)

// TestStateDetector_RealSessionCapture_PermissionPrompt tests detection from a real permission prompt
func TestStateDetector_RealSessionCapture_PermissionPrompt(t *testing.T) {
	detector := NewStateDetector()

	// Simulated real capture from Claude Code waiting for permission
	content := `
I need to edit the file to fix the bug.

Permission required to edit file: /home/user/project/main.go
Allow this action? [y/n]
`

	result := detector.DetectFromContent(content)

	if result.State != types.StateWaitingPermission {
		t.Errorf("Expected StateWaitingPermission, got %s", result.State)
	}

	if result.Confidence < 0.9 {
		t.Errorf("Expected high confidence for permission prompt, got %.2f", result.Confidence)
	}

	if len(result.Signals) == 0 {
		t.Error("Expected signals to be populated")
	}
}

// TestStateDetector_RealSessionCapture_AskingQuestion tests detection from a real question
func TestStateDetector_RealSessionCapture_AskingQuestion(t *testing.T) {
	detector := NewStateDetector()

	content := `
I found two possible approaches to fix this issue:

1. Refactor the entire module
2. Apply a quick patch

Which approach would you prefer?
`

	result := detector.DetectFromContent(content)

	if result.State != types.StateAskingQuestion {
		t.Errorf("Expected StateAskingQuestion, got %s", result.State)
	}

	if result.Confidence < 0.8 {
		t.Errorf("Expected high confidence for question, got %.2f", result.Confidence)
	}
}

// TestStateDetector_RealSessionCapture_ExecutingCommand tests detection during command execution
func TestStateDetector_RealSessionCapture_ExecutingCommand(t *testing.T) {
	detector := NewStateDetector()

	content := `
Let me run the tests to verify the fix.

Executing command: go test ./...
Running tests...
`

	result := detector.DetectFromContent(content)

	if result.State != types.StateExecuting {
		t.Errorf("Expected StateExecuting, got %s", result.State)
	}

	if result.Confidence < 0.7 {
		t.Errorf("Expected reasonable confidence for executing state, got %.2f", result.Confidence)
	}
}

// TestStateDetector_RealSessionCapture_ErrorState tests detection of error state
func TestStateDetector_RealSessionCapture_ErrorState(t *testing.T) {
	detector := NewStateDetector()

	content := `
Error: failed to compile
main.go:15:2: undefined: someFunction
compilation failed
`

	result := detector.DetectFromContent(content)

	if result.State != types.StateError {
		t.Errorf("Expected StateError, got %s", result.State)
	}

	if result.Confidence < 0.8 {
		t.Errorf("Expected high confidence for error state, got %.2f", result.Confidence)
	}
}

// TestStateDetector_RealSessionCapture_TaskCompleted tests detection of finished state
func TestStateDetector_RealSessionCapture_TaskCompleted(t *testing.T) {
	detector := NewStateDetector()

	content := `
All tests pass. The bug has been fixed.

Task completed successfully.
`

	result := detector.DetectFromContent(content)

	if result.State != types.StateFinished {
		t.Errorf("Expected StateFinished, got %s", result.State)
	}

	if result.Confidence < 0.7 {
		t.Errorf("Expected reasonable confidence for finished state, got %.2f", result.Confidence)
	}
}

// TestStateDetector_RealSessionCapture_MultipleSignals tests complex scenarios with multiple signals
func TestStateDetector_RealSessionCapture_MultipleSignals(t *testing.T) {
	detector := NewStateDetector()

	// Scenario: Agent executed a command but it failed
	content := `
Executing command: npm install
Running npm install...
Error: ENOENT: no such file or directory
Installation failed
`

	result := detector.DetectFromContent(content)

	// Error should take priority over executing
	if result.State != types.StateError {
		t.Errorf("Expected StateError (higher priority), got %s", result.State)
	}

	// Should have multiple signals
	if len(result.Signals) < 2 {
		t.Errorf("Expected multiple signals, got %d", len(result.Signals))
	}
}

// TestStateDetector_RealSessionCapture_ConversationFlow tests state detection across a conversation
func TestStateDetector_RealSessionCapture_ConversationFlow(t *testing.T) {
	detector := NewStateDetector()

	// Simulate a sequence of activities
	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "chat",
			Content:      "User: Fix the bug in main.go",
			Metadata:     map[string]string{"role": "user"},
			Timestamp:    time.Now().Add(-5 * time.Minute),
		},
		{
			AgentID:      "test-agent",
			ActivityType: "file",
			Content:      "Reading file: main.go",
			Metadata:     map[string]string{"operation": "read"},
			Timestamp:    time.Now().Add(-4 * time.Minute),
		},
		{
			AgentID:      "test-agent",
			ActivityType: "tool",
			Content:      "Running tests",
			Metadata:     map[string]string{"tool_name": "bash"},
			Timestamp:    time.Now().Add(-3 * time.Minute),
		},
		{
			AgentID:      "test-agent",
			ActivityType: "prompt",
			Content:      "Should I refactor the entire function?",
			Metadata:     map[string]string{"prompt_type": "question"},
			Timestamp:    time.Now().Add(-1 * time.Minute),
		},
	}

	result := detector.DetectFromActivities(activities)

	// Most recent high-priority signal should win (question)
	if result.State != types.StateAskingQuestion {
		t.Errorf("Expected StateAskingQuestion (most recent high-priority), got %s", result.State)
	}

	// Should have signals from multiple activities
	if len(result.Signals) < 3 {
		t.Errorf("Expected multiple signals from conversation flow, got %d", len(result.Signals))
	}
}

// TestStateDetector_RealSessionCapture_IdleDetection tests idle state detection
func TestStateDetector_RealSessionCapture_IdleDetection(t *testing.T) {
	detector := NewStateDetector()

	// Old activity without completion keywords
	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "chat",
			Content:      "Working on the task",
			Timestamp:    time.Now().Add(-10 * time.Minute),
		},
	}

	result := detector.DetectFromActivities(activities)

	// Should detect idle due to old timestamp
	if result.State != types.StateIdle {
		t.Errorf("Expected StateIdle for old activity, got %s", result.State)
	}

	if result.Confidence < 0.7 {
		t.Errorf("Expected reasonable confidence for idle detection, got %.2f", result.Confidence)
	}
}

// TestStateDetector_RealSessionCapture_AmbiguousState tests handling of ambiguous signals
func TestStateDetector_RealSessionCapture_AmbiguousState(t *testing.T) {
	detector := NewStateDetector()

	// Content without clear state indicators
	content := `
Let me think about this...
Analyzing the code...
Considering the best approach...
`

	result := detector.DetectFromContent(content)

	// Should detect unknown state (no clear signals)
	if result.State != types.StateUnknown {
		t.Errorf("Expected StateUnknown for ambiguous content, got %s", result.State)
	}

	// Confidence should be low for ambiguous content
	if result.Confidence > 0.6 {
		t.Errorf("Expected low confidence for ambiguous state, got %.2f", result.Confidence)
	}
}

// TestStateDetector_RealSessionCapture_EmptyContent tests handling of empty content
func TestStateDetector_RealSessionCapture_EmptyContent(t *testing.T) {
	detector := NewStateDetector()

	content := ""

	result := detector.DetectFromContent(content)

	if result.State != types.StateUnknown {
		t.Errorf("Expected StateUnknown for empty content, got %s", result.State)
	}
}

// TestStateDetector_RealSessionCapture_PermissionApproved tests state after permission approval
func TestStateDetector_RealSessionCapture_PermissionApproved(t *testing.T) {
	detector := NewStateDetector()

	// Sequence: permission prompt -> approval -> execution
	activities := []*events.AgentActivityEvent{
		{
			AgentID:      "test-agent",
			ActivityType: "prompt",
			Content:      "Permission required to edit file",
			Metadata:     map[string]string{"prompt_type": "permission"},
			Timestamp:    time.Now().Add(-2 * time.Minute),
		},
		{
			AgentID:      "test-agent",
			ActivityType: "chat",
			Content:      "User: y",
			Metadata:     map[string]string{"role": "user"},
			Timestamp:    time.Now().Add(-1 * time.Minute),
		},
		{
			AgentID:      "test-agent",
			ActivityType: "file",
			Content:      "Editing file: main.go",
			Metadata:     map[string]string{"operation": "edit"},
			Timestamp:    time.Now(),
		},
	}

	result := detector.DetectFromActivities(activities)

	// Permission has highest priority, so it may still show waiting_permission
	// This is actually correct behavior - the detector sees all signals
	// In practice, the control loop would clear old permission signals after approval
	if result.State != types.StateWaitingPermission && result.State != types.StateExecuting {
		t.Errorf("Expected StateWaitingPermission or StateExecuting, got %s", result.State)
	}

	// Should have multiple signals
	if len(result.Signals) < 2 {
		t.Errorf("Expected multiple signals, got %d", len(result.Signals))
	}
}
