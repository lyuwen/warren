package parser

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/lfu/warren/internal/events"
)

// ActivityParser parses captured pane content into structured activity events
type ActivityParser struct {
	// Patterns for detecting different activity types
	chatPatterns       []*regexp.Regexp
	filePatterns       []*regexp.Regexp
	toolPatterns       []*regexp.Regexp
	permissionPatterns []*regexp.Regexp
	questionPatterns   []*regexp.Regexp
}

// NewActivityParser creates a new activity parser
func NewActivityParser() *ActivityParser {
	return &ActivityParser{
		chatPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^user:`),
			regexp.MustCompile(`(?i)^assistant:`),
			regexp.MustCompile(`(?i)^claude:`),
			regexp.MustCompile(`(?i)>\s+.+`), // Prompt-style input
		},
		filePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)Read\s+tool.*?file_path`),
			regexp.MustCompile(`(?i)Edit\s+tool.*?file_path`),
			regexp.MustCompile(`(?i)Write\s+tool.*?file_path`),
			regexp.MustCompile(`(?i)reading\s+file:\s+(.+)`),
			regexp.MustCompile(`(?i)editing\s+file:\s+(.+)`),
			regexp.MustCompile(`(?i)writing\s+file:\s+(.+)`),
		},
		toolPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)Bash\s+tool`),
			regexp.MustCompile(`(?i)LSP\s+tool`),
			regexp.MustCompile(`(?i)WebSearch\s+tool`),
			regexp.MustCompile(`(?i)executing\s+command:`),
			regexp.MustCompile(`(?i)running\s+tests`),
		},
		permissionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)permission\s+required`),
			regexp.MustCompile(`(?i)approve\s+or\s+deny`),
			regexp.MustCompile(`(?i)waiting\s+for\s+approval`),
			regexp.MustCompile(`(?i)\[y/n\]`),
			regexp.MustCompile(`(?i)allow\s+this\s+action`),
		},
		questionPatterns: []*regexp.Regexp{
			// Look for Claude Code's AskUserQuestion tool usage
			regexp.MustCompile(`(?i)AskUserQuestion`),
			// Look for actual questions at end of output (standalone lines)
			// These patterns are much more specific to avoid false positives
			regexp.MustCompile(`(?m)^What would you like .*\?$`),
			regexp.MustCompile(`(?m)^Should I .*\?$`),
			regexp.MustCompile(`(?m)^Would you like .*\?$`),
			regexp.MustCompile(`(?m)^Do you want .*\?$`),
			regexp.MustCompile(`(?m)^How should I .*\?$`),
			regexp.MustCompile(`(?m)^Which .*would you prefer\?$`),
			// Multiple choice patterns (numbered options)
			regexp.MustCompile(`(?m)^\d+\.\s+.+$`), // "1. Option A"
		},
	}
}

// ParseResult contains the results of parsing
type ParseResult struct {
	Activities    []*events.AgentActivityEvent
	Confidence    float64
	DetectedTypes []string
}

// Parse analyzes captured content and extracts activity events
func (p *ActivityParser) Parse(agentID string, content string) (*ParseResult, error) {
	result := &ParseResult{
		Activities:    []*events.AgentActivityEvent{},
		DetectedTypes: []string{},
	}

	lines := strings.Split(content, "\n")
	timestamp := time.Now()

	// Parse chat messages
	chatActivities := p.parseChat(agentID, lines, timestamp)
	result.Activities = append(result.Activities, chatActivities...)
	if len(chatActivities) > 0 {
		result.DetectedTypes = append(result.DetectedTypes, "chat")
	}

	// Parse file interactions
	fileActivities := p.parseFileInteractions(agentID, content, timestamp)
	result.Activities = append(result.Activities, fileActivities...)
	if len(fileActivities) > 0 {
		result.DetectedTypes = append(result.DetectedTypes, "file")
	}

	// Parse tool usage
	toolActivities := p.parseToolUsage(agentID, content, timestamp)
	result.Activities = append(result.Activities, toolActivities...)
	if len(toolActivities) > 0 {
		result.DetectedTypes = append(result.DetectedTypes, "tool")
	}

	// Parse prompts (permissions and questions)
	promptActivities := p.parsePrompts(agentID, content, timestamp)
	result.Activities = append(result.Activities, promptActivities...)
	if len(promptActivities) > 0 {
		result.DetectedTypes = append(result.DetectedTypes, "prompt")
	}

	// Calculate confidence based on number of detected activities
	if len(result.Activities) > 0 {
		result.Confidence = 0.8 // Base confidence
		if len(result.DetectedTypes) > 2 {
			result.Confidence = 0.95 // High confidence with multiple activity types
		}
	}

	return result, nil
}

// parseChat extracts chat messages from content
func (p *ActivityParser) parseChat(agentID string, lines []string, timestamp time.Time) []*events.AgentActivityEvent {
	activities := []*events.AgentActivityEvent{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		for _, pattern := range p.chatPatterns {
			if pattern.MatchString(line) {
				role := "unknown"
				if strings.HasPrefix(strings.ToLower(line), "user:") {
					role = "user"
				} else if strings.HasPrefix(strings.ToLower(line), "assistant:") || strings.HasPrefix(strings.ToLower(line), "claude:") {
					role = "assistant"
				}

				activity := &events.AgentActivityEvent{
					AgentID:      agentID,
					ActivityType: "chat",
					Content:      line,
					Metadata: map[string]string{
						"role": role,
					},
					Timestamp: timestamp,
				}
				activities = append(activities, activity)
				break
			}
		}
	}

	return activities
}

// parseFileInteractions extracts file read/edit/write operations
func (p *ActivityParser) parseFileInteractions(agentID string, content string, timestamp time.Time) []*events.AgentActivityEvent {
	activities := []*events.AgentActivityEvent{}

	for _, pattern := range p.filePatterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			operation := "unknown"
			filePath := ""

			if strings.Contains(strings.ToLower(match[0]), "read") {
				operation = "read"
			} else if strings.Contains(strings.ToLower(match[0]), "edit") {
				operation = "edit"
			} else if strings.Contains(strings.ToLower(match[0]), "write") {
				operation = "write"
			}

			if len(match) > 1 {
				filePath = match[1]
			}

			activity := &events.AgentActivityEvent{
				AgentID:      agentID,
				ActivityType: "file",
				Content:      match[0],
				Metadata: map[string]string{
					"operation": operation,
					"file_path": filePath,
				},
				Timestamp: timestamp,
			}
			activities = append(activities, activity)
		}
	}

	return activities
}

// parseToolUsage extracts tool execution activities
func (p *ActivityParser) parseToolUsage(agentID string, content string, timestamp time.Time) []*events.AgentActivityEvent {
	activities := []*events.AgentActivityEvent{}

	for _, pattern := range p.toolPatterns {
		matches := pattern.FindAllString(content, -1)
		for _, match := range matches {
			toolName := "unknown"
			if strings.Contains(strings.ToLower(match), "bash") {
				toolName = "bash"
			} else if strings.Contains(strings.ToLower(match), "lsp") {
				toolName = "lsp"
			} else if strings.Contains(strings.ToLower(match), "websearch") {
				toolName = "websearch"
			}

			activity := &events.AgentActivityEvent{
				AgentID:      agentID,
				ActivityType: "tool",
				Content:      match,
				Metadata: map[string]string{
					"tool_name": toolName,
				},
				Timestamp: timestamp,
			}
			activities = append(activities, activity)
		}
	}

	return activities
}

// parsePrompts extracts permission prompts and questions
func (p *ActivityParser) parsePrompts(agentID string, content string, timestamp time.Time) []*events.AgentActivityEvent {
	activities := []*events.AgentActivityEvent{}

	// Check for permission prompts
	for _, pattern := range p.permissionPatterns {
		if pattern.MatchString(content) {
			activity := &events.AgentActivityEvent{
				AgentID:      agentID,
				ActivityType: "prompt",
				Content:      pattern.FindString(content),
				Metadata: map[string]string{
					"prompt_type": "permission",
				},
				Timestamp: timestamp,
			}
			activities = append(activities, activity)
			break // Only one permission prompt per parse
		}
	}

	// Check for questions - much more strict to avoid false positives
	// Only detect questions that appear at the END of content (last non-empty lines)
	lines := strings.Split(content, "\n")

	// Get last 10 non-empty lines (where real questions and multiple choice appear)
	lastLines := []string{}
	for i := len(lines) - 1; i >= 0 && len(lastLines) < 10; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			lastLines = append([]string{line}, lastLines...)
		}
	}

	// Check for multiple choice questions (numbered options)
	// Look for pattern: multiple consecutive lines starting with "1.", "2.", "3."
	multipleChoiceCount := 0
	for _, line := range lastLines {
		if regexp.MustCompile(`^\d+\.\s+.+$`).MatchString(line) {
			multipleChoiceCount++
		}
	}

	// If we have 2+ numbered options, it's a multiple choice question
	if multipleChoiceCount >= 2 {
		// Collect all the options
		optionsText := []string{}
		for _, line := range lastLines {
			if regexp.MustCompile(`^\d+\.\s+.+$`).MatchString(line) {
				optionsText = append(optionsText, line)
			}
		}

		activity := &events.AgentActivityEvent{
			AgentID:      agentID,
			ActivityType: "prompt",
			Content:      strings.Join(optionsText, "\n"),
			Metadata: map[string]string{
				"prompt_type":    "question",
				"question_type":  "multiple_choice",
				"option_count":   fmt.Sprintf("%d", multipleChoiceCount),
			},
			Timestamp: timestamp,
		}
		activities = append(activities, activity)
		return activities // Return early, we found a multiple choice question
	}

	// Check if any of the last lines match question patterns (for "?" questions)
	for _, line := range lastLines {
		// Skip lines that are clearly not questions to user
		if strings.HasPrefix(line, "//") || // Code comments
			strings.HasPrefix(line, "#") || // Comments
			strings.Contains(line, "```") || // Code blocks
			strings.HasPrefix(line, "*") || // Markdown lists
			strings.HasPrefix(line, "-") { // Markdown lists
			continue
		}

		for _, pattern := range p.questionPatterns {
			if pattern.MatchString(line) {
				activity := &events.AgentActivityEvent{
					AgentID:      agentID,
					ActivityType: "prompt",
					Content:      line,
					Metadata: map[string]string{
						"prompt_type": "question",
					},
					Timestamp: timestamp,
				}
				activities = append(activities, activity)
				break // Only one question per line
			}
		}
	}

	return activities
}

// ExtractRecentChat extracts the most recent chat messages
func (p *ActivityParser) ExtractRecentChat(content string, maxMessages int) []string {
	lines := strings.Split(content, "\n")
	messages := []string{}

	for i := len(lines) - 1; i >= 0 && len(messages) < maxMessages; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		for _, pattern := range p.chatPatterns {
			if pattern.MatchString(line) {
				messages = append([]string{line}, messages...) // Prepend to maintain order
				break
			}
		}
	}

	return messages
}
