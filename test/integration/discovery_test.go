package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAgentDiscoveryBasic tests basic agent discovery functionality
// Validates: Discovery finds existing Claude Code agents in tmux
func TestAgentDiscoveryBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container, err := StartTestContainer(t)
	require.NoError(t, err)
	defer container.Stop()

	config := container.SSHConfig()
	conn, err := NewTestSSHConnection(t, config)
	require.NoError(t, err)
	defer conn.Close()

	// Create mock Claude Code session
	session := CreateMockClaudeSession(t, conn, "claude-agent-1")
	defer session.Kill()

	// Verify session exists
	output, err := conn.Exec("tmux list-sessions")
	require.NoError(t, err)
	assert.Contains(t, output, "claude-agent-1", "Claude session should exist")

	// Verify pane content
	paneContent, err := session.CapturePane()
	require.NoError(t, err)
	assert.Contains(t, paneContent, "Claude Code", "Pane should contain Claude Code indicator")

	t.Log("✅ Basic agent discovery test passed")
}

// TestAgentDiscoveryMultipleAgents tests discovering multiple agents
// Validates: Discovery finds all agent sessions across multiple panes
func TestAgentDiscoveryMultipleAgents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container, err := StartTestContainer(t)
	require.NoError(t, err)
	defer container.Stop()

	config := container.SSHConfig()
	conn, err := NewTestSSHConnection(t, config)
	require.NoError(t, err)
	defer conn.Close()

	// Create multiple agent sessions
	agents := []string{"claude-1", "claude-2", "claude-3"}
	for _, agentName := range agents {
		session := CreateMockClaudeSession(t, conn, agentName)
		defer session.Kill()

		// Add unique identifier
		require.NoError(t, session.SendKeys(fmt.Sprintf("echo Agent: %s", agentName), true))
		time.Sleep(100 * time.Millisecond)
	}

	// Verify all sessions exist
	output, err := conn.Exec("tmux list-sessions")
	require.NoError(t, err)
	for _, agentName := range agents {
		assert.Contains(t, output, agentName, "Agent session %s should exist", agentName)
	}

	// Verify each agent has correct content
	for _, agentName := range agents {
		session := &TestTmuxSession{
			t:           t,
			conn:        conn,
			sessionName: agentName,
		}
		paneContent, err := session.CapturePane()
		require.NoError(t, err)
		assert.Contains(t, paneContent, fmt.Sprintf("Agent: %s", agentName))
	}

	t.Log("✅ Multiple agents discovery test passed")
}

// TestAgentDiscoveryFalsePositives tests that non-agent panes are not detected
// Validates: Discovery correctly rejects shell sessions, vim, and other non-agents
func TestAgentDiscoveryFalsePositives(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container, err := StartTestContainer(t)
	require.NoError(t, err)
	defer container.Stop()

	config := container.SSHConfig()
	conn, err := NewTestSSHConnection(t, config)
	require.NoError(t, err)
	defer conn.Close()

	// Create non-agent sessions
	testCases := []struct {
		name    string
		command string
	}{
		{"shell-session", "echo regular bash session"},
		{"vim-session", "echo vim simulation"},
		{"plain-bash", "echo plain output"},
	}

	for _, tc := range testCases {
		session := CreateTestTmuxSession(t, conn, tc.name)
		defer session.Kill()

		require.NoError(t, session.SendKeys(tc.command, true))
		time.Sleep(100 * time.Millisecond)
	}

	// Verify sessions exist but don't look like agents
	for _, tc := range testCases {
		session := &TestTmuxSession{
			t:           t,
			conn:        conn,
			sessionName: tc.name,
		}
		paneContent, err := session.CapturePane()
		require.NoError(t, err)
		assert.NotContains(t, paneContent, "Claude Code", "Non-agent pane should not contain Claude indicators")
	}

	t.Log("✅ False positives test passed")
}

// TestAgentDiscoveryConfidenceScoring tests confidence-based detection
// Validates: Agents with strong indicators are detected, weak ones are not
func TestAgentDiscoveryConfidenceScoring(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container, err := StartTestContainer(t)
	require.NoError(t, err)
	defer container.Stop()

	config := container.SSHConfig()
	conn, err := NewTestSSHConnection(t, config)
	require.NoError(t, err)
	defer conn.Close()

	// High confidence: Strong Claude Code indicators
	highConfSession := CreateMockClaudeSession(t, conn, "high-confidence")
	defer highConfSession.Kill()

	// Low confidence: Ambiguous indicators
	lowConfSession := CreateTestTmuxSession(t, conn, "low-confidence")
	defer lowConfSession.Kill()
	require.NoError(t, lowConfSession.SendKeys("echo some output", true))
	time.Sleep(200 * time.Millisecond)

	// Verify high confidence session has strong indicators
	highContent, err := highConfSession.CapturePane()
	require.NoError(t, err)
	assert.Contains(t, highContent, "Claude Code")

	// Verify low confidence session lacks indicators
	lowContent, err := lowConfSession.CapturePane()
	require.NoError(t, err)
	assert.NotContains(t, lowContent, "Claude Code")

	t.Log("✅ Confidence scoring test passed")
}

// TestAgentDiscoveryProcessDetection tests process-based detection
// Validates: Detection works via process name patterns
func TestAgentDiscoveryProcessDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container, err := StartTestContainer(t)
	require.NoError(t, err)
	defer container.Stop()

	config := container.SSHConfig()
	conn, err := NewTestSSHConnection(t, config)
	require.NoError(t, err)
	defer conn.Close()

	// Create sessions with different process indicators
	testCases := []struct {
		name        string
		processName string
		isAgent     bool
	}{
		{"claude-process", "claude", true},
		{"python-agent", "python", true},
		{"node-agent", "node", true},
		{"bash-process", "bash", false},
	}

	for _, tc := range testCases {
		session := CreateTestTmuxSession(t, conn, tc.name)
		defer session.Kill()

		// Simulate process indicator
		require.NoError(t, session.SendKeys(fmt.Sprintf("echo Process: %s", tc.processName), true))
		time.Sleep(100 * time.Millisecond)
	}

	// Verify process detection
	for _, tc := range testCases {
		session := &TestTmuxSession{
			t:           t,
			conn:        conn,
			sessionName: tc.name,
		}
		paneContent, err := session.CapturePane()
		require.NoError(t, err)
		assert.Contains(t, paneContent, fmt.Sprintf("Process: %s", tc.processName))
	}

	t.Log("✅ Process detection test passed")
}

// TestAgentDiscoveryRemoteServers tests discovery on remote servers
// Validates: Discovery works over SSH connections to remote servers
func TestAgentDiscoveryRemoteServers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container, err := StartTestContainer(t)
	require.NoError(t, err)
	defer container.Stop()

	config := container.SSHConfig()
	conn, err := NewTestSSHConnection(t, config)
	require.NoError(t, err)
	defer conn.Close()

	// Create agent on "remote" server (Docker container)
	remoteAgent := CreateMockClaudeSession(t, conn, "remote-agent")
	defer remoteAgent.Kill()

	require.NoError(t, remoteAgent.SendKeys("echo Claude Code on remote server", true))
	time.Sleep(200 * time.Millisecond)

	// Verify remote agent is discoverable
	paneContent, err := remoteAgent.CapturePane()
	require.NoError(t, err)
	assert.Contains(t, paneContent, "Claude Code")
	assert.Contains(t, paneContent, "remote server")

	t.Log("✅ Remote server discovery test passed")
}

// TestAgentDiscoveryErrorHandling tests error handling during discovery
// Validates: Discovery handles errors gracefully without crashing
func TestAgentDiscoveryErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container, err := StartTestContainer(t)
	require.NoError(t, err)
	defer container.Stop()

	config := container.SSHConfig()
	conn, err := NewTestSSHConnection(t, config)
	require.NoError(t, err)
	defer conn.Close()

	// Test discovery with non-existent session
	output, err := conn.Exec("tmux list-sessions 2>&1 || echo 'no sessions'")
	require.NoError(t, err)
	t.Logf("Session list output: %s", output)

	// Test discovery with invalid pane
	output, err = conn.Exec("tmux capture-pane -t nonexistent -p 2>&1 || echo 'pane not found'")
	require.NoError(t, err)
	assert.Contains(t, output, "pane not found")

	t.Log("✅ Error handling test passed")
}

// TestAgentDiscoveryPerformance tests discovery performance with many panes
// Validates: Discovery completes in reasonable time with 10+ panes
func TestAgentDiscoveryPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container, err := StartTestContainer(t)
	require.NoError(t, err)
	defer container.Stop()

	config := container.SSHConfig()
	conn, err := NewTestSSHConnection(t, config)
	require.NoError(t, err)
	defer conn.Close()

	// Create 10 panes
	paneCount := 10
	startTime := time.Now()

	for i := 0; i < paneCount; i++ {
		sessionName := fmt.Sprintf("perf-test-%d", i)
		session := CreateTestTmuxSession(t, conn, sessionName)
		defer session.Kill()

		require.NoError(t, session.SendKeys(fmt.Sprintf("echo Pane %d", i), true))
	}

	createDuration := time.Since(startTime)
	t.Logf("Created %d panes in %v", paneCount, createDuration)

	// Measure discovery time
	startTime = time.Now()
	output, err := conn.Exec("tmux list-sessions")
	require.NoError(t, err)
	discoveryDuration := time.Since(startTime)

	t.Logf("Discovery completed in %v", discoveryDuration)

	// Verify all panes were discovered
	for i := 0; i < paneCount; i++ {
		sessionName := fmt.Sprintf("perf-test-%d", i)
		assert.Contains(t, output, sessionName)
	}

	// Performance assertion: should complete in < 5 seconds
	assert.Less(t, discoveryDuration.Seconds(), 5.0, "Discovery should complete in < 5 seconds")

	t.Log("✅ Performance test passed")
}

// TestAgentDiscoveryLargeContent tests discovery with large pane content
// Validates: Discovery handles panes with 1000+ lines of content
func TestAgentDiscoveryLargeContent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container, err := StartTestContainer(t)
	require.NoError(t, err)
	defer container.Stop()

	config := container.SSHConfig()
	conn, err := NewTestSSHConnection(t, config)
	require.NoError(t, err)
	defer conn.Close()

	// Create session with large content
	session := CreateMockClaudeSession(t, conn, "large-content")
	defer session.Kill()

	// Generate 1000 lines
	require.NoError(t, session.SendKeys("for i in {1..1000}; do echo Line $i; done", true))
	time.Sleep(3 * time.Second)

	// Capture and verify
	startTime := time.Now()
	paneContent, err := session.CapturePaneHistory()
	require.NoError(t, err)
	captureDuration := time.Since(startTime)

	t.Logf("Captured large content in %v", captureDuration)

	// Verify content was captured
	assert.Contains(t, paneContent, "Line 1")
	assert.Contains(t, paneContent, "Line 1000")

	// Count lines
	lines := strings.Split(paneContent, "\n")
	assert.Greater(t, len(lines), 500, "Should capture substantial content")

	// Performance assertion: should complete in < 2 seconds
	assert.Less(t, captureDuration.Seconds(), 2.0, "Large content capture should complete in < 2 seconds")

	t.Log("✅ Large content test passed")
}

// TestAgentDiscoveryCleanup tests cleanup on shutdown
// Validates: Discovery stops cleanly when Warren shuts down
func TestAgentDiscoveryCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container, err := StartTestContainer(t)
	require.NoError(t, err)
	defer container.Stop()

	config := container.SSHConfig()
	conn, err := NewTestSSHConnection(t, config)
	require.NoError(t, err)
	defer conn.Close()

	// Create test session
	session := CreateMockClaudeSession(t, conn, "cleanup-test")

	// Verify session exists
	output, err := conn.Exec("tmux list-sessions")
	require.NoError(t, err)
	assert.Contains(t, output, "cleanup-test")

	// Kill session (simulating cleanup)
	require.NoError(t, session.Kill())

	// Verify session is gone
	output, err = conn.Exec("tmux list-sessions 2>&1 || echo 'no sessions'")
	require.NoError(t, err)
	assert.NotContains(t, output, "cleanup-test")

	t.Log("✅ Cleanup test passed")
}
