package parser

import (
	"strings"
	"testing"
)

func TestActivityParser_ParseChat(t *testing.T) {
	parser := NewActivityParser()

	content := `
User: Hello, can you help me?
Assistant: Of course! What do you need?
User: I need to fix a bug
Claude: Let me analyze the code
`

	result, err := parser.Parse("test-agent", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should detect chat activities
	chatFound := false
	for _, activity := range result.Activities {
		if activity.ActivityType == "chat" {
			chatFound = true
			break
		}
	}

	if !chatFound {
		t.Error("Expected to find chat activities")
	}

	if !contains(result.DetectedTypes, "chat") {
		t.Error("Expected 'chat' in detected types")
	}
}

func TestActivityParser_ParseFileInteractions(t *testing.T) {
	parser := NewActivityParser()

	content := `
Reading file: /path/to/file.go
Editing file: /path/to/another.go
Writing file: /path/to/output.txt
`

	result, err := parser.Parse("test-agent", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should detect file activities
	fileCount := 0
	for _, activity := range result.Activities {
		if activity.ActivityType == "file" {
			fileCount++
		}
	}

	if fileCount == 0 {
		t.Error("Expected to find file activities")
	}

	if !contains(result.DetectedTypes, "file") {
		t.Error("Expected 'file' in detected types")
	}
}

func TestActivityParser_ParseToolUsage(t *testing.T) {
	parser := NewActivityParser()

	content := `
Executing command: ls -la
Running tests
Bash tool invoked
`

	result, err := parser.Parse("test-agent", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should detect tool activities
	toolFound := false
	for _, activity := range result.Activities {
		if activity.ActivityType == "tool" {
			toolFound = true
			break
		}
	}

	if !toolFound {
		t.Error("Expected to find tool activities")
	}

	if !contains(result.DetectedTypes, "tool") {
		t.Error("Expected 'tool' in detected types")
	}
}

func TestActivityParser_ParsePermissionPrompts(t *testing.T) {
	parser := NewActivityParser()

	content := `
Permission required to edit this file.
Approve or deny this action [y/n]?
`

	result, err := parser.Parse("test-agent", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should detect prompt activities
	permissionFound := false
	for _, activity := range result.Activities {
		if activity.ActivityType == "prompt" {
			if promptType, ok := activity.Metadata["prompt_type"]; ok && promptType == "permission" {
				permissionFound = true
				break
			}
		}
	}

	if !permissionFound {
		t.Error("Expected to find permission prompt")
	}

	if !contains(result.DetectedTypes, "prompt") {
		t.Error("Expected 'prompt' in detected types")
	}
}

func TestActivityParser_ParseQuestions(t *testing.T) {
	parser := NewActivityParser()

	content := `
Which approach should I use?
Do you want me to continue?
How should I handle this error?
`

	result, err := parser.Parse("test-agent", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should detect question prompts
	questionCount := 0
	for _, activity := range result.Activities {
		if activity.ActivityType == "prompt" {
			if promptType, ok := activity.Metadata["prompt_type"]; ok && promptType == "question" {
				questionCount++
			}
		}
	}

	if questionCount == 0 {
		t.Error("Expected to find question prompts")
	}
}

func TestActivityParser_Confidence(t *testing.T) {
	parser := NewActivityParser()

	tests := []struct {
		name        string
		content     string
		minConf     float64
		expectTypes int
	}{
		{
			name:        "no activities",
			content:     "just plain text",
			minConf:     0.0,
			expectTypes: 0,
		},
		{
			name: "single activity type",
			content: `
User: Hello
Assistant: Hi
`,
			minConf:     0.7,
			expectTypes: 1,
		},
		{
			name: "multiple activity types",
			content: `
User: Fix the bug
Reading file: test.go
Executing command: go test
Which approach should I use?
`,
			minConf:     0.9,
			expectTypes: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Parse("test-agent", tt.content)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if result.Confidence < tt.minConf {
				t.Errorf("Expected confidence >= %.2f, got %.2f", tt.minConf, result.Confidence)
			}

			if len(result.DetectedTypes) < tt.expectTypes {
				t.Errorf("Expected at least %d detected types, got %d", tt.expectTypes, len(result.DetectedTypes))
			}
		})
	}
}

func TestActivityParser_ExtractRecentChat(t *testing.T) {
	parser := NewActivityParser()

	content := `
Some other text
User: Message 1
Assistant: Response 1
User: Message 2
Assistant: Response 2
User: Message 3
More text
`

	messages := parser.ExtractRecentChat(content, 3)

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}

	// Should be in chronological order
	if !strings.Contains(messages[0], "Message 2") {
		t.Error("Expected first message to be 'Message 2'")
	}
}

func TestActivityParser_Metadata(t *testing.T) {
	parser := NewActivityParser()

	content := `
User: Hello
Reading file: /test/file.go
Bash tool invoked
Permission required
`

	result, err := parser.Parse("test-agent", content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Check that metadata is populated
	for _, activity := range result.Activities {
		if len(activity.Metadata) == 0 {
			t.Errorf("Expected metadata for activity type %s", activity.ActivityType)
		}

		switch activity.ActivityType {
		case "chat":
			if _, ok := activity.Metadata["role"]; !ok {
				t.Error("Expected 'role' in chat metadata")
			}
		case "file":
			if _, ok := activity.Metadata["operation"]; !ok {
				t.Error("Expected 'operation' in file metadata")
			}
		case "tool":
			if _, ok := activity.Metadata["tool_name"]; !ok {
				t.Error("Expected 'tool_name' in tool metadata")
			}
		case "prompt":
			if _, ok := activity.Metadata["prompt_type"]; !ok {
				t.Error("Expected 'prompt_type' in prompt metadata")
			}
		}
	}
}

func TestActivityParser_EmptyContent(t *testing.T) {
	parser := NewActivityParser()

	result, err := parser.Parse("test-agent", "")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(result.Activities) != 0 {
		t.Errorf("Expected 0 activities for empty content, got %d", len(result.Activities))
	}

	if result.Confidence != 0.0 {
		t.Errorf("Expected 0 confidence for empty content, got %.2f", result.Confidence)
	}
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
