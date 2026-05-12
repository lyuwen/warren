package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ConversationReader reads and parses Claude conversation files
type ConversationReader struct {
	claudeDir string
}

// Message represents a conversation message
type Message struct {
	Type        string          `json:"type"`
	UUID        string          `json:"uuid"`
	ParentUUID  string          `json:"parentUuid"`
	Timestamp   time.Time       `json:"timestamp"`
	TimestampRaw string         `json:"-"`
	Role        string          `json:"role,omitempty"` // Extracted from message.role
	Content     string          `json:"content,omitempty"` // Simplified content
	Model       string          `json:"model,omitempty"` // Model used (for assistant messages)
	ToolCalls   []ToolCall      `json:"toolCalls,omitempty"` // Tool calls in this message
	RawMessage  json.RawMessage `json:"message,omitempty"`
	IsSidechain bool            `json:"isSidechain"`
}

// ToolCall represents a tool invocation
type ToolCall struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Input  json.RawMessage `json:"input"`
	Result string          `json:"result,omitempty"`
}

// messageContent is used for parsing the message field
type messageContent struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	Model      string          `json:"model,omitempty"`
	StopReason string          `json:"stop_reason,omitempty"`
}

// contentBlock represents a content block (text, thinking, tool_use, tool_result)
type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	Thinking  string          `json:"thinking,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ID        string          `json:"id,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
}

// NewConversationReader creates a new conversation reader
func NewConversationReader() *ConversationReader {
	home := os.Getenv("HOME")
	return &ConversationReader{
		claudeDir: filepath.Join(home, ".claude"),
	}
}

// GetConversationFile returns the path to the conversation file for a session
func (cr *ConversationReader) GetConversationFile(sessionID, cwd string) string {
	projectSlug := GetProjectSlug(cwd)
	return filepath.Join(cr.claudeDir, "projects", projectSlug, sessionID+".jsonl")
}

// ReadConversation reads and parses a conversation file
func (cr *ConversationReader) ReadConversation(filePath string) ([]*Message, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open conversation file: %w", err)
	}
	defer file.Close()

	var messages []*Message
	scanner := bufio.NewScanner(file)

	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			// Skip invalid lines silently
			continue
		}

		// Parse timestamp
		if msg.TimestampRaw != "" {
			ts, err := time.Parse(time.RFC3339, msg.TimestampRaw)
			if err == nil {
				msg.Timestamp = ts
			}
		}

		// Parse message content for user/assistant messages
		if msg.Type == "user" || msg.Type == "assistant" {
			if err := cr.parseMessageContent(&msg); err != nil {
				// Skip messages we can't parse
				continue
			}
		}

		messages = append(messages, &msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return messages, nil
}

// parseMessageContent extracts role, content, and tool calls from the message field
func (cr *ConversationReader) parseMessageContent(msg *Message) error {
	var mc messageContent
	if err := json.Unmarshal(msg.RawMessage, &mc); err != nil {
		return err
	}

	msg.Role = mc.Role
	msg.Model = mc.Model

	// Parse content - can be string or array of blocks
	var contentStr string
	if err := json.Unmarshal(mc.Content, &contentStr); err == nil {
		// Simple string content
		msg.Content = contentStr
		return nil
	}

	// Array of content blocks
	var blocks []contentBlock
	if err := json.Unmarshal(mc.Content, &blocks); err != nil {
		return err
	}

	// Extract text, thinking, and tool calls
	var textParts []string
	for _, block := range blocks {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "thinking":
			// Include thinking as part of content
			textParts = append(textParts, "[thinking] "+block.Thinking)
		case "tool_use":
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
			textParts = append(textParts, fmt.Sprintf("[tool: %s]", block.Name))
		case "tool_result":
			// Find matching tool call and add result
			for i := range msg.ToolCalls {
				if msg.ToolCalls[i].ID == block.ToolUseID {
					msg.ToolCalls[i].Result = block.Content
					break
				}
			}
		}
	}

	if len(textParts) > 0 {
		msg.Content = textParts[0] // Use first text part as primary content
	}

	return nil
}

// GetRecentMessages returns the N most recent messages from a conversation
func (cr *ConversationReader) GetRecentMessages(filePath string, limit int) ([]*Message, error) {
	messages, err := cr.ReadConversation(filePath)
	if err != nil {
		return nil, err
	}

	if len(messages) <= limit {
		return messages, nil
	}

	return messages[len(messages)-limit:], nil
}

// FilterByType returns only messages of specific types
func FilterByType(messages []*Message, types ...string) []*Message {
	typeMap := make(map[string]bool)
	for _, t := range types {
		typeMap[t] = true
	}

	var filtered []*Message
	for _, msg := range messages {
		if typeMap[msg.Type] {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

// FilterUserAssistant returns only user and assistant messages
func FilterUserAssistant(messages []*Message) []*Message {
	return FilterByType(messages, "user", "assistant")
}
