package state

import (
	"fmt"
	"strings"
	"time"

	"github.com/lfu/warren/internal/events"
	"github.com/lfu/warren/internal/types"
)

// StateDetector infers agent state from activity events
type StateDetector struct {
	// State priority: higher priority states override lower priority ones
	statePriority map[types.AgentState]int
}

// NewStateDetector creates a new state detector
func NewStateDetector() *StateDetector {
	return &StateDetector{
		statePriority: map[types.AgentState]int{
			types.StateError:             100, // Highest priority
			types.StateWaitingPermission: 90,
			types.StateAskingQuestion:    80,
			types.StateFinished:          70,
			types.StateStopped:           60,
			types.StateExecuting:         50,
			types.StateThinking:          40,
			types.StateIdle:              30,
			types.StateUnknown:           10, // Lowest priority
		},
	}
}

// DetectionResult contains the detected state and confidence
type DetectionResult struct {
	State      types.AgentState
	Confidence float64
	Signals    []string // Evidence for this state
	Timestamp  time.Time
}

// DetectFromActivities infers state from recent activity events
func (d *StateDetector) DetectFromActivities(activities []*events.AgentActivityEvent) *DetectionResult {
	if len(activities) == 0 {
		return &DetectionResult{
			State:      types.StateUnknown,
			Confidence: 0.5,
			Signals:    []string{"no recent activities"},
			Timestamp:  time.Now(),
		}
	}

	// Collect signals from activities
	signals := d.collectSignals(activities)

	// Determine state based on signals
	state, confidence := d.determineState(signals)

	return &DetectionResult{
		State:      state,
		Confidence: confidence,
		Signals:    d.formatSignals(signals),
		Timestamp:  time.Now(),
	}
}

// DetectFromContent infers state directly from captured pane content
func (d *StateDetector) DetectFromContent(content string) *DetectionResult {
	signals := d.collectSignalsFromContent(content)
	state, confidence := d.determineState(signals)

	return &DetectionResult{
		State:      state,
		Confidence: confidence,
		Signals:    d.formatSignals(signals),
		Timestamp:  time.Now(),
	}
}

// Signal represents a detected indicator of agent state
type Signal struct {
	State      types.AgentState
	Strength   float64 // 0.0 to 1.0
	Evidence   string
	Timestamp  time.Time
}

// collectSignals extracts state signals from activities
func (d *StateDetector) collectSignals(activities []*events.AgentActivityEvent) []*Signal {
	signals := []*Signal{}

	for _, activity := range activities {
		switch activity.ActivityType {
		case "prompt":
			if promptType, ok := activity.Metadata["prompt_type"]; ok {
				if promptType == "permission" {
					signals = append(signals, &Signal{
						State:      types.StateWaitingPermission,
						Strength:   0.95,
						Evidence:   "permission prompt detected",
						Timestamp:  activity.Timestamp,
					})
				} else if promptType == "question" {
					signals = append(signals, &Signal{
						State:      types.StateAskingQuestion,
						Strength:   0.9,
						Evidence:   "question detected",
						Timestamp:  activity.Timestamp,
					})
				}
			}

		case "tool":
			signals = append(signals, &Signal{
				State:      types.StateExecuting,
				Strength:   0.8,
				Evidence:   fmt.Sprintf("tool execution: %s", activity.Metadata["tool_name"]),
				Timestamp:  activity.Timestamp,
			})

		case "file":
			signals = append(signals, &Signal{
				State:      types.StateExecuting,
				Strength:   0.7,
				Evidence:   fmt.Sprintf("file operation: %s", activity.Metadata["operation"]),
				Timestamp:  activity.Timestamp,
			})

		case "chat":
			role := activity.Metadata["role"]
			if role == "user" {
				signals = append(signals, &Signal{
					State:      types.StateThinking,
					Strength:   0.6,
					Evidence:   "user message received",
					Timestamp:  activity.Timestamp,
				})
			} else if role == "assistant" {
				signals = append(signals, &Signal{
					State:      types.StateThinking,
					Strength:   0.5,
					Evidence:   "assistant responding",
					Timestamp:  activity.Timestamp,
				})
			}
		}
	}

	// Check for error signals
	for _, activity := range activities {
		contentLower := strings.ToLower(activity.Content)
		if strings.Contains(contentLower, "error") ||
			strings.Contains(contentLower, "failed") ||
			strings.Contains(contentLower, "exception") {
			signals = append(signals, &Signal{
				State:      types.StateError,
				Strength:   0.85,
				Evidence:   "error keyword detected",
				Timestamp:  activity.Timestamp,
			})
		}

		if strings.Contains(contentLower, "completed") ||
			strings.Contains(contentLower, "finished") ||
			strings.Contains(contentLower, "done") {
			signals = append(signals, &Signal{
				State:      types.StateFinished,
				Strength:   0.7,
				Evidence:   "completion keyword detected",
				Timestamp:  activity.Timestamp,
			})
		}
	}

	// Check for idle state (no recent activity)
	if len(activities) > 0 {
		lastActivity := activities[0]
		timeSinceLastActivity := time.Since(lastActivity.Timestamp)
		if timeSinceLastActivity > 5*time.Minute {
			signals = append(signals, &Signal{
				State:      types.StateIdle,
				Strength:   0.8,
				Evidence:   fmt.Sprintf("no activity for %v", timeSinceLastActivity.Round(time.Second)),
				Timestamp:  time.Now(),
			})
		}
	}

	return signals
}

// collectSignalsFromContent extracts signals directly from content
func (d *StateDetector) collectSignalsFromContent(content string) []*Signal {
	signals := []*Signal{}
	contentLower := strings.ToLower(content)
	now := time.Now()

	// Permission prompts
	if strings.Contains(contentLower, "permission required") ||
		strings.Contains(contentLower, "approve or deny") ||
		strings.Contains(contentLower, "[y/n]") {
		signals = append(signals, &Signal{
			State:      types.StateWaitingPermission,
			Strength:   0.95,
			Evidence:   "permission prompt in content",
			Timestamp:  now,
		})
	}

	// Questions
	if strings.Contains(contentLower, "should i") ||
		strings.Contains(contentLower, "do you want") ||
		strings.Contains(contentLower, "which") && strings.Contains(contentLower, "?") {
		signals = append(signals, &Signal{
			State:      types.StateAskingQuestion,
			Strength:   0.85,
			Evidence:   "question in content",
			Timestamp:  now,
		})
	}

	// Errors
	if strings.Contains(contentLower, "error:") ||
		strings.Contains(contentLower, "failed:") ||
		strings.Contains(contentLower, "exception") {
		signals = append(signals, &Signal{
			State:      types.StateError,
			Strength:   0.9,
			Evidence:   "error in content",
			Timestamp:  now,
		})
	}

	// Execution
	if strings.Contains(contentLower, "executing") ||
		strings.Contains(contentLower, "running") {
		signals = append(signals, &Signal{
			State:      types.StateExecuting,
			Strength:   0.75,
			Evidence:   "execution indicator in content",
			Timestamp:  now,
		})
	}

	// Completion
	if strings.Contains(contentLower, "completed successfully") ||
		strings.Contains(contentLower, "task finished") ||
		strings.Contains(contentLower, "all done") {
		signals = append(signals, &Signal{
			State:      types.StateFinished,
			Strength:   0.8,
			Evidence:   "completion indicator in content",
			Timestamp:  now,
		})
	}

	return signals
}

// determineState selects the most likely state from signals
func (d *StateDetector) determineState(signals []*Signal) (types.AgentState, float64) {
	if len(signals) == 0 {
		return types.StateUnknown, 0.5
	}

	// Group signals by state
	stateScores := make(map[types.AgentState]float64)
	for _, signal := range signals {
		stateScores[signal.State] += signal.Strength
	}

	// Find highest priority state with sufficient confidence
	var bestState types.AgentState
	var bestScore float64
	highestPriority := -1

	for state, score := range stateScores {
		priority := d.statePriority[state]
		if priority > highestPriority || (priority == highestPriority && score > bestScore) {
			bestState = state
			bestScore = score
			highestPriority = priority
		}
	}

	// Normalize confidence
	confidence := bestScore
	if confidence > 1.0 {
		confidence = 1.0
	}

	return bestState, confidence
}

// formatSignals converts signals to human-readable strings
func (d *StateDetector) formatSignals(signals []*Signal) []string {
	formatted := make([]string, len(signals))
	for i, signal := range signals {
		formatted[i] = fmt.Sprintf("%s (%.2f): %s", signal.State, signal.Strength, signal.Evidence)
	}
	return formatted
}

// ShouldTransition determines if a state transition should occur
func (d *StateDetector) ShouldTransition(currentState types.AgentState, newResult *DetectionResult, minConfidence float64) bool {
	// Always transition if confidence is high enough and state is different
	if newResult.State != currentState && newResult.Confidence >= minConfidence {
		// Check priority - only transition to higher or equal priority states
		currentPriority := d.statePriority[currentState]
		newPriority := d.statePriority[newResult.State]
		return newPriority >= currentPriority
	}
	return false
}
