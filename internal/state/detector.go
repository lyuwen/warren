package state

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lfu/warren/internal/events"
	"github.com/lfu/warren/internal/types"
)

// Real Claude Code v2.1.x UI patterns compiled once at package level.
var (
	// Spinner patterns: ✢ Fluttering… / ✢ Churning… / ✢ Thinking… etc.
	// These indicate an actively running agent.
	reSpinner = regexp.MustCompile(`✢\s+\S+…`)

	// Completion timer: ✻ Worked for 7m 23s / ✻ Brewed for 1m 58s / ✻ Churned for 31s
	reCompletionTimer = regexp.MustCompile(`✻\s+\S+\s+for\s+[\dm]+\s*[\ds]*`)

	// Recap line: ※ recap: ...
	reRecap = regexp.MustCompile(`※\s+recap:`)

	// Tool invocations: ● Bash(...) / ● Read(...) / ● Edit(...) / ● Write(...)
	// ● Skill(...) / ● Agent(...)
	reToolCall = regexp.MustCompile(`●\s+(Bash|Read|Edit|Write|Skill|Agent|LSP|WebSearch|WebFetch|Grep|Glob|NotebookEdit)\(`)

	// Tool result: ⎿  (indented result from a tool)
	reToolResult = regexp.MustCompile(`⎿\s+`)

	// Status bar model info: Opus 4.6 (1M context) or Sonnet 4.6 etc.
	reStatusBar = regexp.MustCompile(`(Opus|Sonnet|Haiku)\s+[\d.]+\s+\([\dMk]+\s+context\)`)

	// Status bar progress: ░ or █ progress bars with percentage
	reProgressBar = regexp.MustCompile(`[░█]+\s+\d+%`)

	// Permission prompt: "Esc to cancel" or "Tab to amend" at the end
	rePermissionFooter = regexp.MustCompile(`Esc to cancel\s*·?\s*Tab to amend`)

	// Numbered choice selector: ❯ N. Option text
	reChoiceSelector = regexp.MustCompile(`❯\s+\d+\.\s+`)

	// Claude Code prompt: ❯ followed by text or empty
	rePrompt = regexp.MustCompile(`(?m)^.*❯\s*$`)

	// Claude Code prompt with user input after it: ❯ <text>
	// Claude Code prompt with user input: line starting with ❯ followed by text
	rePromptWithInput = regexp.MustCompile(`^\s*❯[\s\x{00a0}]+\S+`)

	// Claude Code welcome box (idle at start): "Welcome back!" or "Claude Code v2"
	reWelcomeBox = regexp.MustCompile(`(?i)claude code v[\d.]+`)

	// Agent finished indicators: "N agents finished" or "Done (N tool uses"
	reAgentsDone = regexp.MustCompile(`\d+\s+agents?\s+finished`)

	// Auto mode indicator in status bar
	reAutoMode = regexp.MustCompile(`⏵⏵\s+auto mode`)

	// Session label in status bar (divider line with label)
	reSessionLabel = regexp.MustCompile(`─{3,}.*─{3,}`)

	// Tool execution status: ⎿  Running… or ⎿  Waiting…
	reToolRunning = regexp.MustCompile(`⎿\s+(Running|Waiting)…`)
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

// collectSignalsFromContent extracts signals directly from pane content captured
// from a real Claude Code v2.1.x session via tmux capture-pane.
//
// The patterns here are grounded in real captures stored in internal/testdata/.
// Key Claude Code UI elements:
//   - ● text       → Claude output / action
//   - ● Tool(args) → tool invocation (Bash, Read, Edit, Write, Skill, Agent, …)
//   - ⎿  result    → tool result
//   - ✢ Verb…      → active spinner (running)
//   - ✻ Verb for T → completion timer (finished)
//   - ※ recap:     → post-task recap (idle/finished)
//   - ❯ text       → user prompt / empty prompt
//   - ❯ N. Option  → permission/choice selector
//   - Esc to cancel · Tab to amend → permission prompt footer
//   - Status bar   → model name, progress bar, auto mode, branch
func (d *StateDetector) collectSignalsFromContent(content string) []*Signal {
	signals := []*Signal{}
	now := time.Now()

	// Split into lines; collect last N non-empty lines for recency checks.
	lines := strings.Split(content, "\n")
	lastNonEmpty := collectLastNonEmptyLines(lines, 15)
	bottom10 := collectLastNonEmptyLines(lines, 10)

	// Pre-scan bottom lines for key UI elements
	// Normalize non-breaking spaces (U+00A0) to regular spaces in bottom lines
	// Claude Code's terminal output sometimes uses NBSP after ❯
	normalizedBottom10 := make([]string, len(bottom10))
	for i, line := range bottom10 {
		normalizedBottom10[i] = strings.ReplaceAll(line, "\u00a0", " ")
	}
	hasEmptyPrompt := false
	hasPromptWithInput := false
	hasStatusBar := false
	hasSpinnerInBottom := false
	hasPermissionFooterInBottom := false
	hasAutoMode := false

	// Scan all bottom lines for UI indicators
	for _, line := range normalizedBottom10 {
		if reStatusBar.MatchString(line) || reAutoMode.MatchString(line) || reProgressBar.MatchString(line) {
			hasStatusBar = true
		}
		if reSpinner.MatchString(line) {
			hasSpinnerInBottom = true
		}
		if reToolRunning.MatchString(line) {
			hasSpinnerInBottom = true // treat as equivalent to spinner
		}

		if rePermissionFooter.MatchString(strings.TrimSpace(line)) {
			hasPermissionFooterInBottom = true
		}
	}

	// Find the first ❯ line from the bottom — that's the current prompt
	for _, line := range normalizedBottom10 {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "❯") {
			if trimmed == "❯" || trimmed == "❯ " {
				hasEmptyPrompt = true
			} else if !reChoiceSelector.MatchString(trimmed) {
				hasPromptWithInput = true
			}
			break
		}
	}

	// Detect auto mode: check for status bar indicator or auto-approval text.
	// "Allowed by auto mode" appears after auto-approval in the tool output.
	// We check bottom 20 lines (recent output) rather than full scrollback to
	// avoid false positives from text discussing auto mode.
	bottom20 := collectLastNonEmptyLines(lines, 20)
	for _, line := range bottom20 {
		if reAutoMode.MatchString(line) ||
			strings.Contains(line, "Allowed by auto mode") {
			hasAutoMode = true
			break
		}
	}

	contentLower := strings.ToLower(content)

	// --- 1. Permission / Choice prompt (only at the very bottom) ---
	// Must have BOTH the "Esc to cancel" footer AND numbered options nearby.
	// This avoids false positives from code output containing the footer text.
	hasChoiceOptionsInBottom := false
	for _, line := range normalizedBottom10 {
		if regexp.MustCompile(`^\s*(â¯\s+)?\d+\.\s+\S+`).MatchString(strings.TrimSpace(line)) {
			hasChoiceOptionsInBottom = true
			break
		}
	}
	isRealPermissionPrompt := hasPermissionFooterInBottom && hasChoiceOptionsInBottom

	if isRealPermissionPrompt {
		if hasAutoMode {
			signals = append(signals, &Signal{
				State:    types.StateExecuting,
				Strength: 0.90,
				Evidence: "permission prompt in auto mode (auto-approved)",
				Timestamp: now,
			})
		} else {
			signals = append(signals, &Signal{
				State:    types.StateWaitingPermission,
				Strength: 0.95,
				Evidence: "permission prompt footer (Esc to cancel · Tab to amend)",
				Timestamp: now,
			})
		}
	}


	// Legacy permission patterns (only in recent output)
	for _, line := range bottom10 {
		lineLower := strings.ToLower(line)
		if strings.Contains(lineLower, "permission required") ||
			strings.Contains(lineLower, "approve or deny") ||
			strings.Contains(lineLower, "[y/n]") {
			signals = append(signals, &Signal{
				State:    types.StateWaitingPermission,
				Strength: 0.90,
				Evidence: "legacy permission prompt keywords",
				Timestamp: now,
			})
			break
		}
	}

	// --- 2. Active spinner → Running ---
	if hasSpinnerInBottom {
		signals = append(signals, &Signal{
			State:    types.StateExecuting,
			Strength: 0.95,
			Evidence: "active spinner (✢) in recent output",
			Timestamp: now,
		})
	}

	// Tool invocations in progress (only in recent output)
	var toolMatchesInBottom []string
	for _, line := range bottom10 {
		if reToolCall.MatchString(line) {
			toolMatchesInBottom = append(toolMatchesInBottom, reToolCall.FindString(line))
		}
	}
	if len(toolMatchesInBottom) > 0 {
		matches := toolMatchesInBottom
		toolName := "unknown"
		if len(matches) > 0 {
			last := matches[len(matches)-1]
			// Extract tool name from "● ToolName("
			parts := strings.Fields(last)
			if len(parts) >= 2 {
				toolName = strings.TrimSuffix(parts[1], "(")
			}
		}
		signals = append(signals, &Signal{
			State:    types.StateExecuting,
			Strength: 0.70,
			Evidence: fmt.Sprintf("tool invocation: %s", toolName),
			Timestamp: now,
		})
	}

	// --- 3. Completion timer → Finished (only when NOT at a prompt) ---
	// If the session has an empty prompt or prompt with input, completion
	// markers are historical — the session is idle, not finished.
	if !hasEmptyPrompt && !hasPromptWithInput {
		if reCompletionTimer.MatchString(content) {
			signals = append(signals, &Signal{
				State:    types.StateFinished,
				Strength: 0.90,
				Evidence: "completion timer (✻ Verb for T)",
				Timestamp: now,
			})
		}

		if reRecap.MatchString(content) {
			signals = append(signals, &Signal{
				State:    types.StateFinished,
				Strength: 0.85,
				Evidence: "recap line (※ recap:)",
				Timestamp: now,
			})
		}
	}

	// --- 4. Idle / waiting at prompt ---
	// Spinner overrides idle — if the agent is actively processing,
	// the empty prompt is just the input area, not an idle indicator.
	if !hasSpinnerInBottom {
		if hasEmptyPrompt && hasStatusBar {
			signals = append(signals, &Signal{
				State:    types.StateIdle,
				Strength: 0.95,
				Evidence: "empty â¯ prompt with status bar",
				Timestamp: now,
			})
		} else if hasEmptyPrompt {
			signals = append(signals, &Signal{
				State:    types.StateIdle,
				Strength: 0.80,
				Evidence: "empty â¯ prompt",
				Timestamp: now,
			})
		}
	}
	// Prompt with text after it (user typed something, session hasn't started)
	if hasPromptWithInput && !hasSpinnerInBottom {
		signals = append(signals, &Signal{
			State:    types.StateIdle,
			Strength: 0.90,
			Evidence: "❯ prompt with user input",
			Timestamp: now,
		})
	}

	// Welcome box → idle (fresh session)
	if reWelcomeBox.MatchString(content) && hasEmptyPrompt {
		signals = append(signals, &Signal{
			State:    types.StateIdle,
			Strength: 0.90,
			Evidence: "welcome screen with empty prompt",
			Timestamp: now,
		})
	}

	// Legacy idle indicators (backward compatibility)
	if strings.Contains(contentLower, "waiting for input") ||
		strings.Contains(contentLower, "ready for next") ||
		strings.Contains(contentLower, "standing by") {
		signals = append(signals, &Signal{
			State:    types.StateIdle,
			Strength: 0.75,
			Evidence: "legacy idle status indicator",
			Timestamp: now,
		})
	}

	// Legacy prompt suffix (bash/shell prompt)
	if len(lines) > 0 {
		lastLine := lines[len(lines)-1]
		if strings.HasSuffix(lastLine, "> ") || strings.HasSuffix(lastLine, "$ ") {
			signals = append(signals, &Signal{
				State:    types.StateIdle,
				Strength: 0.85,
				Evidence: "waiting at shell prompt",
				Timestamp: now,
			})
		}
	}

	// --- 5. Asking question ---
	// Claude asks a question and then shows ✻ timer + empty ❯ prompt.
	// The question is a ● line ending with "?" in the recent output.
	hasQuestionBullet := false
	questionText := ""
	for _, line := range lastNonEmpty {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "●") && strings.HasSuffix(trimmed, "?") {
			hasQuestionBullet = true
			questionText = trimmed
			break
		}
	}

	if hasQuestionBullet && hasEmptyPrompt {
		signals = append(signals, &Signal{
			State:    types.StateAskingQuestion,
			Strength: 0.95,
			Evidence: fmt.Sprintf("Claude question with empty prompt: %s", questionText),
			Timestamp: now,
		})
	} else if hasQuestionBullet {
		signals = append(signals, &Signal{
			State:    types.StateAskingQuestion,
			Strength: 0.70,
			Evidence: fmt.Sprintf("Claude question bullet: %s", questionText),
			Timestamp: now,
		})
	}

	// Stricter general question detection - must be in last few lines
	if !hasQuestionBullet {
		hasQuestionInLastLines := false
		for _, line := range lastNonEmpty {
			lineLower := strings.ToLower(line)
			// Skip code blocks, comments, and tool output
			if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "//") ||
				strings.HasPrefix(line, "#") || strings.HasPrefix(strings.TrimSpace(line), "⎿") {
				continue
			}
			if strings.HasSuffix(strings.TrimSpace(line), "?") {
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

		hasAskUserQuestionTool := strings.Contains(content, "AskUserQuestion") ||
			strings.Contains(content, "asking for input") ||
			strings.Contains(content, "multiple choice")

		if hasQuestionInLastLines && hasAskUserQuestionTool {
			signals = append(signals, &Signal{
				State:    types.StateAskingQuestion,
				Strength: 0.90,
				Evidence: "question with tool call in recent output",
				Timestamp: now,
			})
		} else if hasQuestionInLastLines {
			signals = append(signals, &Signal{
				State:    types.StateAskingQuestion,
				Strength: 0.60,
				Evidence: "question pattern in recent output",
				Timestamp: now,
			})
		}
	}

	// --- 6. Errors (only in recent output, not scrollback) ---
	hasErrorInBottom := false
	for _, line := range bottom10 {
		lineLower := strings.ToLower(line)
		if strings.Contains(lineLower, "error:") ||
			strings.Contains(lineLower, "failed:") ||
			strings.Contains(lineLower, "exception") {
			hasErrorInBottom = true
			break
		}
	}
	if hasErrorInBottom {
		signals = append(signals, &Signal{
			State:    types.StateError,
			Strength: 0.85,
			Evidence: "error keyword in content",
			Timestamp: now,
		})
	}

	// --- 7. Legacy execution indicators (backward compat) ---
	if strings.Contains(contentLower, "executing") {
		signals = append(signals, &Signal{
			State:    types.StateExecuting,
			Strength: 0.70,
			Evidence: "legacy execution keyword",
			Timestamp: now,
		})
	}

	// --- 8. Legacy completion indicators (backward compat) ---
	if strings.Contains(contentLower, "completed successfully") ||
		strings.Contains(contentLower, "task finished") ||
		strings.Contains(contentLower, "all done") {
		signals = append(signals, &Signal{
			State:    types.StateFinished,
			Strength: 0.75,
			Evidence: "legacy completion keyword",
			Timestamp: now,
		})
	}

	return signals
}

// collectLastNonEmptyLines returns the last n non-empty lines from the bottom,
// preserving bottom-to-top order (index 0 = closest to bottom).
func collectLastNonEmptyLines(lines []string, n int) []string {
	result := []string{}
	for i := len(lines) - 1; i >= 0 && len(result) < n; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			result = append(result, lines[i]) // Keep original indentation
		}
	}
	return result
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
