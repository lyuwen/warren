package state

import (
	"strings"
	"time"

	"github.com/lfu/warren/internal/claude"
	"github.com/lfu/warren/internal/types"
)

// ConversationDetector detects agent state from conversation history
// This is more accurate than parsing pane content because it uses the actual
// conversation messages from Claude's session files.
type ConversationDetector struct{}

// NewConversationDetector creates a new conversation-based state detector
func NewConversationDetector() *ConversationDetector {
	return &ConversationDetector{}
}

// DetectFromConversation determines agent state from conversation messages
func (d *ConversationDetector) DetectFromConversation(messages []*claude.Message) *DetectionResult {
	if len(messages) == 0 {
		return &DetectionResult{
			State:      types.StateUnknown,
			Confidence: 0.5,
			Signals:    []string{"no conversation history"},
			Timestamp:  time.Now(),
		}
	}

	// Get the last non-sidechain message
	var lastMessage *claude.Message
	for i := len(messages) - 1; i >= 0; i-- {
		if !messages[i].IsSidechain {
			lastMessage = messages[i]
			break
		}
	}

	if lastMessage == nil {
		return &DetectionResult{
			State:      types.StateUnknown,
			Confidence: 0.5,
			Signals:    []string{"no non-sidechain messages"},
			Timestamp:  time.Now(),
		}
	}

	// Detect state based on last message
	return d.detectFromLastMessage(lastMessage, messages)
}

// DetectFromConversationAndPane combines conversation history with pane content
// This provides more accurate real-time state detection
func (d *ConversationDetector) DetectFromConversationAndPane(messages []*claude.Message, paneContent string) *DetectionResult {
	// First check pane content for indicators
	paneResult := d.detectFromPaneContent(paneContent)

	// If pane shows clear state (high confidence), trust it
	if paneResult.Confidence >= 0.8 {
		return paneResult
	}

	// Otherwise use conversation history
	return d.DetectFromConversation(messages)
}

// detectFromPaneContent detects state from pane content indicators
func (d *ConversationDetector) detectFromPaneContent(content string) *DetectionResult {
	now := time.Now()
	lowerContent := strings.ToLower(content)

	// Check for active thinking/execution indicators (present tense, active)
	activeIndicators := []string{
		"cultivating",
		"thinking…",
		"reasoning…",
		"analyzing…",
		"processing…",
		"running 1 shell command",
		"running 1 tool",
		"executing tool",
		"⏳",
		"⌛",
		"◐", "◓", "◑", "◒", // Spinner characters
		"✶",
	}

	for _, indicator := range activeIndicators {
		if strings.Contains(lowerContent, indicator) {
			return &DetectionResult{
				State:      types.StateExecuting,
				Confidence: 0.95,
				Signals:    []string{"pane shows active indicator: " + indicator},
				Timestamp:  now,
			}
		}
	}

	// Check for tool execution indicators (present tense)
	if strings.Contains(lowerContent, "running 1 shell command") ||
		strings.Contains(lowerContent, "reading file") ||
		strings.Contains(lowerContent, "writing file") ||
		strings.Contains(lowerContent, "editing file") {
		return &DetectionResult{
			State:      types.StateExecuting,
			Confidence: 0.9,
			Signals:    []string{"pane shows active tool execution"},
			Timestamp:  now,
		}
	}

	// Check for permission/question indicators
	if strings.Contains(lowerContent, "waiting for permission") ||
		strings.Contains(lowerContent, "approve") ||
		strings.Contains(lowerContent, "deny") {
		return &DetectionResult{
			State:      types.StateWaitingPermission,
			Confidence: 0.95,
			Signals:    []string{"pane shows permission request"},
			Timestamp:  now,
		}
	}

	// Check for idle prompt (must be at the end, not in middle of output)
	lines := strings.Split(content, "\n")
	if len(lines) > 0 {
		// Check last few lines for prompt
		for i := len(lines) - 1; i >= 0 && i >= len(lines)-3; i-- {
			line := strings.TrimSpace(lines[i])
			// Check if line is a prompt
			if line == "❯" || line == ">" || strings.HasPrefix(line, "❯ ") {
				return &DetectionResult{
					State:      types.StateIdle,
					Confidence: 0.85,
					Signals:    []string{"pane shows idle prompt"},
					Timestamp:  now,
				}
			}
		}
	}

	// Check for completed indicators (past tense - these mean idle)
	completedIndicators := []string{
		"cogitated for",
		"cultivated for",
		"ran 1 shell command",
		"✻", // Completed marker
	}

	hasCompletedWork := false
	for _, indicator := range completedIndicators {
		if strings.Contains(lowerContent, indicator) {
			hasCompletedWork = true
			break
		}
	}

	// If we see completed work and no active indicators, it's idle
	if hasCompletedWork {
		return &DetectionResult{
			State:      types.StateIdle,
			Confidence: 0.85,
			Signals:    []string{"pane shows completed work, no active indicators"},
			Timestamp:  now,
		}
	}

	// No clear indicators from pane
	return &DetectionResult{
		State:      types.StateUnknown,
		Confidence: 0.3,
		Signals:    []string{"no clear indicators in pane"},
		Timestamp:  now,
	}
}

// detectFromLastMessage determines state from the last message
func (d *ConversationDetector) detectFromLastMessage(lastMsg *claude.Message, allMessages []*claude.Message) *DetectionResult {
	now := time.Now()
	age := now.Sub(lastMsg.Timestamp)

	switch lastMsg.Type {
	case "user":
		// User just sent a message - agent should be thinking/executing
		if age < 5*time.Second {
			return &DetectionResult{
				State:      types.StateThinking,
				Confidence: 0.95,
				Signals:    []string{"user message received recently"},
				Timestamp:  now,
			}
		} else if age < 30*time.Second {
			return &DetectionResult{
				State:      types.StateExecuting,
				Confidence: 0.9,
				Signals:    []string{"user message received, agent processing"},
				Timestamp:  now,
			}
		} else {
			// User message is old but no assistant response - might be stuck
			return &DetectionResult{
				State:      types.StateError,
				Confidence: 0.7,
				Signals:    []string{"user message sent but no response for >30s"},
				Timestamp:  now,
			}
		}

	case "assistant":
		// Check if assistant is still working (has tool calls pending)
		if len(lastMsg.ToolCalls) > 0 {
			// Check if tool calls have results
			hasUnfinishedTools := false
			for _, tool := range lastMsg.ToolCalls {
				if tool.Result == "" {
					hasUnfinishedTools = true
					break
				}
			}

			if hasUnfinishedTools {
				return &DetectionResult{
					State:      types.StateExecuting,
					Confidence: 0.95,
					Signals:    []string{"assistant has pending tool calls"},
					Timestamp:  now,
				}
			}
		}

		// Assistant finished responding - agent is idle
		return &DetectionResult{
			State:      types.StateIdle,
			Confidence: 0.95,
			Signals:    []string{"assistant finished responding"},
			Timestamp:  now,
		}

	case "system":
		// System messages can indicate permission requests or questions
		content := strings.ToLower(lastMsg.Content)

		if strings.Contains(content, "permission") || strings.Contains(content, "approve") {
			return &DetectionResult{
				State:      types.StateWaitingPermission,
				Confidence: 0.95,
				Signals:    []string{"system message requesting permission"},
				Timestamp:  now,
			}
		}

		if strings.Contains(content, "question") || strings.Contains(content, "?") {
			return &DetectionResult{
				State:      types.StateAskingQuestion,
				Confidence: 0.9,
				Signals:    []string{"system message with question"},
				Timestamp:  now,
			}
		}

		// System message might be a summary - check if it's recent
		if age < 10*time.Second {
			// Recent system message, agent might still be working
			return &DetectionResult{
				State:      types.StateIdle,
				Confidence: 0.6,
				Signals:    []string{"recent system message (likely summary)"},
				Timestamp:  now,
			}
		}

		// Old system message - agent is idle
		return &DetectionResult{
			State:      types.StateIdle,
			Confidence: 0.8,
			Signals:    []string{"old system message, agent idle"},
			Timestamp:  now,
		}

	case "attachment":
		// Attachment messages don't indicate state by themselves
		// Fall back to previous message
		if len(allMessages) > 1 {
			for i := len(allMessages) - 2; i >= 0; i-- {
				if !allMessages[i].IsSidechain {
					return d.detectFromLastMessage(allMessages[i], allMessages[:i+1])
				}
			}
		}
		return &DetectionResult{
			State:      types.StateUnknown,
			Confidence: 0.5,
			Signals:    []string{"last message is attachment"},
			Timestamp:  now,
		}

	default:
		return &DetectionResult{
			State:      types.StateUnknown,
			Confidence: 0.5,
			Signals:    []string{"unknown message type: " + lastMsg.Type},
			Timestamp:  now,
		}
	}
}
