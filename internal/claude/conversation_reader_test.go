package claude

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConversationReader_ParseConversation(t *testing.T) {
	// Create a temporary test conversation file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Write test JSONL data
	testData := `{"type":"user","uuid":"test-1","timestamp":"2026-05-12T10:00:00Z","message":{"role":"user","content":"Hello"}}
{"type":"assistant","uuid":"test-2","timestamp":"2026-05-12T10:00:01Z","message":{"role":"assistant","content":"Hi there"}}
{"type":"ai-title","aiTitle":"Test conversation"}
`
	if err := os.WriteFile(testFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	reader := NewConversationReader()
	messages, err := reader.ReadConversation(testFile)
	if err != nil {
		t.Fatalf("Failed to read conversation: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}

	// Check first message
	if messages[0].Type != "user" {
		t.Errorf("Expected first message type 'user', got %q", messages[0].Type)
	}
	if messages[0].Role != "user" {
		t.Errorf("Expected first message role 'user', got %q", messages[0].Role)
	}
	if messages[0].Content != "Hello" {
		t.Errorf("Expected first message content 'Hello', got %q", messages[0].Content)
	}

	// Check timestamp parsing
	expectedTime := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	if !messages[0].Timestamp.Equal(expectedTime) {
		t.Errorf("Expected timestamp %v, got %v", expectedTime, messages[0].Timestamp)
	}
}

func TestConversationReader_ParseToolCalls(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Test data with tool use
	testData := `{"type":"assistant","uuid":"test-1","timestamp":"2026-05-12T10:00:00Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"Read","input":{"file_path":"/test.txt"}}]}}
`
	if err := os.WriteFile(testFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	reader := NewConversationReader()
	messages, err := reader.ReadConversation(testFile)
	if err != nil {
		t.Fatalf("Failed to read conversation: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	msg := messages[0]
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(msg.ToolCalls))
	}

	toolCall := msg.ToolCalls[0]
	if toolCall.Name != "Read" {
		t.Errorf("Expected tool name 'Read', got %q", toolCall.Name)
	}
	if toolCall.ID != "tool-1" {
		t.Errorf("Expected tool ID 'tool-1', got %q", toolCall.ID)
	}
}

func TestConversationReader_GetRecentMessages(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Write 5 messages
	testData := `{"type":"user","uuid":"1","timestamp":"2026-05-12T10:00:00Z","message":{"role":"user","content":"1"}}
{"type":"user","uuid":"2","timestamp":"2026-05-12T10:00:01Z","message":{"role":"user","content":"2"}}
{"type":"user","uuid":"3","timestamp":"2026-05-12T10:00:02Z","message":{"role":"user","content":"3"}}
{"type":"user","uuid":"4","timestamp":"2026-05-12T10:00:03Z","message":{"role":"user","content":"4"}}
{"type":"user","uuid":"5","timestamp":"2026-05-12T10:00:04Z","message":{"role":"user","content":"5"}}
`
	if err := os.WriteFile(testFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	reader := NewConversationReader()
	messages, err := reader.GetRecentMessages(testFile, 3)
	if err != nil {
		t.Fatalf("Failed to get recent messages: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}

	// Should get the last 3 messages (3, 4, 5)
	if messages[0].Content != "3" {
		t.Errorf("Expected first message content '3', got %q", messages[0].Content)
	}
	if messages[2].Content != "5" {
		t.Errorf("Expected last message content '5', got %q", messages[2].Content)
	}
}

func TestFilterUserAssistant(t *testing.T) {
	messages := []*Message{
		{Type: "user", Role: "user"},
		{Type: "assistant", Role: "assistant"},
		{Type: "system"},
		{Type: "ai-title"},
		{Type: "user", Role: "user"},
	}

	filtered := FilterUserAssistant(messages)

	if len(filtered) != 3 {
		t.Errorf("Expected 3 filtered messages, got %d", len(filtered))
	}

	for _, msg := range filtered {
		if msg.Type != "user" && msg.Type != "assistant" {
			t.Errorf("Unexpected message type in filtered results: %q", msg.Type)
		}
	}
}

func TestConversationReader_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.jsonl")

	// Write invalid JSON (should be skipped)
	testData := `{"type":"user","uuid":"1","message":{"role":"user","content":"valid"}}
invalid json line
{"type":"user","uuid":"2","message":{"role":"user","content":"also valid"}}
`
	if err := os.WriteFile(testFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	reader := NewConversationReader()
	messages, err := reader.ReadConversation(testFile)
	if err != nil {
		t.Fatalf("Failed to read conversation: %v", err)
	}

	// Should skip invalid line and return 2 valid messages
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages (invalid line skipped), got %d", len(messages))
	}
}
