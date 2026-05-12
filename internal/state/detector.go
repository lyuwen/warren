package state

import (
	"fmt"
	"strings"
	"time"

	"github.com/lfu/warren/internal/events"
	"github.com/lfu/warren/internal/types"
)

// StateDetector infers agent state from activity events
//
// Design Decisions:
//
// 1. Idle Timeout (30 seconds):
//    - Reduced from 5 minutes to 30 seconds for faster idle detection
//    - Rationale: Claude Code agents typically respond within seconds. If no activity
//      for 30s, the agent is likely waiting for input, not actively working.
//    - Graduated strength: 0.7 at 30s, 0.8 at 1min, 0.9 at 2min+ for increasing confidence
//    - Balances responsiveness (show idle quickly) vs false positives (brief pauses during work)
//
// 2. Time-Decay (100% → 50% → 20%):
//    - Fresh signals (0-30s): 100% strength - recent activity is most relevant
//    - Medium age (30s-2min): 50% strength - still relevant but fading
//    - Old signals (>2min): 20% strength - historical context only
//    - Prevents stale signals from dominating current state detection
//
// 3. Thread-Safety:
//    - StateDetector is stateless except for the priority map (read-only after construction)
//    - Safe for concurrent use by multiple goroutines
//    - Signal structs are not mutated during detection (time-decay applied to copies)
//    - Each detection call creates new Signal instances and result
//
// 4. Priority System:
//    - Higher priority states (error, permission) override lower priority (idle, unknown)
//    - Exception: Lower priority states with 2x confidence can override (prevents false positives)
//    - Idle priority 35 (raised from 30) allows strong idle signals to beat weak thinking signals
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
			// Idle priority raised from 30 to 35 to reduce false positives.
			// Previously, weak thinking signals (0.5 strength) from old assistant
			// messages would override strong idle signals (0.7+ strength) due to
			// priority alone. With priority 35 and the 2x confidence override rule,
			// idle can now win when it has much stronger evidence (e.g., 0.7 vs 0.25
			// after time-decay), preventing agents that are clearly idle from showing
			// as "thinking" indefinitely.
			types.StateIdle:              35,
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

	// Check for idle state (no recent activity) - enhanced with time-decay
	if len(activities) > 0 {
		lastActivity := activities[0]
		timeSinceLastActivity := time.Since(lastActivity.Timestamp)
		if timeSinceLastActivity > 30*time.Second {
			// Base strength 0.7, increases with time
			strength := 0.7
			if timeSinceLastActivity > 2*time.Minute {
				strength = 0.9 // Very confident after 2 minutes
			} else if timeSinceLastActivity > 1*time.Minute {
				strength = 0.8 // More confident after 1 minute
			}
			signals = append(signals, &Signal{
				State:      types.StateIdle,
				Strength:   strength,
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

	// Split into lines for better analysis
	lines := strings.Split(content, "\n")
	lastLines := []string{}
	for i := len(lines) - 1; i >= 0 && len(lastLines) < 3; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			lastLines = append(lastLines, trimmed)
		}
	}

	// Signal 1: Prompt detection (strongest idle signal)
	if len(lines) > 0 {
		lastLine := lines[len(lines)-1]
		if strings.HasSuffix(lastLine, "> ") || strings.HasSuffix(lastLine, "$ ") {
			signals = append(signals, &Signal{
				State:      types.StateIdle,
				Strength:   0.95,
				Evidence:   "waiting at prompt",
				Timestamp:  now,
			})
		}
	}

	// Signal 2: Idle status indicators
	if strings.Contains(contentLower, "waiting for input") ||
		strings.Contains(contentLower, "ready for next") ||
		strings.Contains(contentLower, "standing by") {
		signals = append(signals, &Signal{
			State:      types.StateIdle,
			Strength:   0.8,
			Evidence:   "idle status indicator",
			Timestamp:  now,
		})
	}

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

	// Stricter question detection - must be in last 3 lines
	hasQuestionInLastLines := false
	for _, line := range lastLines {
		lineLower := strings.ToLower(line)
		// Skip code blocks and comments
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}
		// Must have question mark at end
		if strings.HasSuffix(strings.TrimSpace(line), "?") {
			// Check for actual question patterns
			if strings.Contains(lineLower, "should i") ||
				strings.Contains(lineLower, "do you want") ||
				strings.Contains(lineLower, "would you like") ||
				strings.Contains(lineLower, "which") ||
				strings.Contains(lineLower, "how") ||
				strings.Contains(lineLower, "what") ||
				strings.Contains(lineLower, "where") ||
				strings.Contains(lineLower, "when") {
				hasQuestionInLastLines = true
				break
			}
		}
	}

	// Also check for AskUserQuestion tool indicator
	hasAskUserQuestionTool := strings.Contains(content, "AskUserQuestion") ||
		strings.Contains(content, "asking for input") ||
		strings.Contains(content, "multiple choice")

	if hasQuestionInLastLines && hasAskUserQuestionTool {
		signals = append(signals, &Signal{
			State:      types.StateAskingQuestion,
			Strength:   0.9,
			Evidence:   "question with tool call in recent output",
			Timestamp:  now,
		})
	} else if hasQuestionInLastLines {
		// Lower confidence without tool confirmation
		signals = append(signals, &Signal{
			State:      types.StateAskingQuestion,
			Strength:   0.6,
			Evidence:   "question pattern in recent output",
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

	now := time.Now()

	// Group signals by state with time-decay applied
	stateScores := make(map[types.AgentState]float64)
	for _, signal := range signals {
		// Calculate decayed strength without mutating the signal
		decayedStrength := signal.Strength
		age := now.Sub(signal.Timestamp)
		if age > 2*time.Minute {
			decayedStrength *= 0.2 // 20% strength after 2 minutes
		} else if age > 30*time.Second {
			decayedStrength *= 0.5 // 50% strength after 30 seconds
		}
		// else: 100% strength (0-30 seconds old)

		stateScores[signal.State] += decayedStrength
	}

	// Find highest priority state with sufficient confidence
	var bestState types.AgentState
	var bestScore float64
	highestPriority := -1

	// First pass: find the highest priority state
	for state, score := range stateScores {
		priority := d.statePriority[state]
		if priority > highestPriority {
			bestState = state
			bestScore = score
			highestPriority = priority
		} else if priority == highestPriority && score > bestScore {
			bestState = state
			bestScore = score
		}
	}

	// Second pass: check if a lower priority state has much higher confidence
	for state, score := range stateScores {
		priority := d.statePriority[state]
		if priority < highestPriority && score > bestScore*2.0 {
			// Lower priority but much higher confidence (2x) can override
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
