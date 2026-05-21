package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// MockClaudeSession represents a mock Claude Code session for testing
type MockClaudeSession struct {
	*TestTmuxSession
}

// CreateMockClaudeSession creates a mock Claude Code session
func CreateMockClaudeSession(t *testing.T, conn *TestSSHConnection, sessionName string) *MockClaudeSession {
	t.Helper()

	session := CreateTestTmuxSession(t, conn, sessionName)

	// Simulate Claude Code startup
	session.SendKeys("echo Claude Code CLI v2.0.0", true)
	session.SendKeys("echo Session started", true)
	time.Sleep(200 * time.Millisecond)

	return &MockClaudeSession{
		TestTmuxSession: session,
	}
}

// SimulatePrompt simulates a Claude Code prompt
func (m *MockClaudeSession) SimulatePrompt() error {
	m.t.Helper()
	return m.SendKeys("echo claude>", true)
}

// SimulateUserInput simulates user input
func (m *MockClaudeSession) SimulateUserInput(input string) error {
	m.t.Helper()
	// Use printf to avoid issues with special characters
	if err := m.SendKeys(fmt.Sprintf("printf 'claude> %s\\n'", input), true); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	return nil
}

// SimulateToolCall simulates a tool call
func (m *MockClaudeSession) SimulateToolCall(tool, args string) error {
	m.t.Helper()
	// Use simple text without emojis to avoid encoding issues
	output := fmt.Sprintf("printf 'Tool: %s(%s)\\n'", tool, args)
	return m.SendKeys(output, true)
}

// SimulateResponse simulates Claude's response
func (m *MockClaudeSession) SimulateResponse(response string) error {
	m.t.Helper()
	return m.SendKeys(fmt.Sprintf("printf '%s\\n'", response), true)
}

// SimulateWaitingForPermission simulates waiting for permission
func (m *MockClaudeSession) SimulateWaitingForPermission(tool string) error {
	m.t.Helper()
	output := fmt.Sprintf("printf 'Waiting for permission: %s\\n'", tool)
	return m.SendKeys(output, true)
}

// SimulateError simulates an error
func (m *MockClaudeSession) SimulateError(errMsg string) error {
	m.t.Helper()
	output := fmt.Sprintf("printf 'Error: %s\\n'", errMsg)
	return m.SendKeys(output, true)
}

// SimulateComplete simulates task completion
func (m *MockClaudeSession) SimulateComplete() error {
	m.t.Helper()
	return m.SendKeys("printf 'Task complete\\n'", true)
}

// SimulateConversation simulates a full conversation
func (m *MockClaudeSession) SimulateConversation(steps []ConversationStep) error {
	m.t.Helper()

	for _, step := range steps {
		switch step.Type {
		case "user":
			if err := m.SimulateUserInput(step.Content); err != nil {
				return err
			}
		case "tool":
			if err := m.SimulateToolCall(step.Tool, step.Args); err != nil {
				return err
			}
		case "response":
			if err := m.SimulateResponse(step.Content); err != nil {
				return err
			}
		case "waiting":
			if err := m.SimulateWaitingForPermission(step.Tool); err != nil {
				return err
			}
		case "error":
			if err := m.SimulateError(step.Content); err != nil {
				return err
			}
		case "complete":
			if err := m.SimulateComplete(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown step type: %s", step.Type)
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// ConversationStep represents a step in a conversation
type ConversationStep struct {
	Type    string // user, tool, response, waiting, error, complete
	Content string
	Tool    string
	Args    string
}

// VerifyClaudeSession verifies that a session looks like Claude Code
func VerifyClaudeSession(t *testing.T, content string) bool {
	t.Helper()

	// Check for Claude Code indicators
	indicators := []string{
		"Claude Code",
		"claude>",
		"🔧", // Tool call emoji
		"✅", // Success emoji
		"❌", // Error emoji
	}

	for _, indicator := range indicators {
		if strings.Contains(content, indicator) {
			return true
		}
	}

	return false
}

// VerifyNotClaudeSession verifies that a session does NOT look like Claude Code
func VerifyNotClaudeSession(t *testing.T, content string) bool {
	t.Helper()

	// Check for non-Claude indicators
	nonClaudeIndicators := []string{
		"bash-",
		"zsh-",
		"$", // Shell prompt
		"#", // Root prompt
		"vim",
		"nano",
	}

	for _, indicator := range nonClaudeIndicators {
		if strings.Contains(content, indicator) {
			return true
		}
	}

	return false
}

// CreateMockShellSession creates a mock shell session (not Claude)
func CreateMockShellSession(t *testing.T, conn *TestSSHConnection, sessionName string) *TestTmuxSession {
	t.Helper()

	session := CreateTestTmuxSession(t, conn, sessionName)

	// Simulate shell prompt
	session.SendKeys("PS1='$ '", true)
	session.SendKeys("echo 'Shell session'", true)
	time.Sleep(200 * time.Millisecond)

	return session
}

// CreateMockVimSession creates a mock vim session (not Claude)
func CreateMockVimSession(t *testing.T, conn *TestSSHConnection, sessionName string) *TestTmuxSession {
	t.Helper()

	session := CreateTestTmuxSession(t, conn, sessionName)

	// Simulate vim
	session.SendKeys("vim test.txt", true)
	time.Sleep(200 * time.Millisecond)

	return session
}
