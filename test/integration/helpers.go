package integration

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// TestConfig holds configuration for integration tests
type TestConfig struct {
	SSHHost     string
	SSHPort     int
	SSHUser     string
	SSHPassword string
	SSHKeyPath  string
}

// DefaultTestConfig returns default test configuration
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		SSHHost:     getEnv("WARREN_TEST_SSH_HOST", "localhost"),
		SSHPort:     getEnvInt("WARREN_TEST_SSH_PORT", 2222),
		SSHUser:     getEnv("WARREN_TEST_SSH_USER", "testuser"),
		SSHPassword: getEnv("WARREN_TEST_SSH_PASS", "testpass"),
		SSHKeyPath:  getEnv("WARREN_TEST_SSH_KEY", ""),
	}
}

// TestSSHConnection wraps an SSH connection for testing
type TestSSHConnection struct {
	t      *testing.T
	client *ssh.Client
	config *TestConfig
}

// NewTestSSHConnection creates a new test SSH connection
func NewTestSSHConnection(t *testing.T, config *TestConfig) (*TestSSHConnection, error) {
	t.Helper()

	var authMethods []ssh.AuthMethod

	// Try key-based auth first if key path provided
	if config.SSHKeyPath != "" {
		key, err := os.ReadFile(config.SSHKeyPath)
		if err == nil {
			signer, err := ssh.ParsePrivateKey(key)
			if err == nil {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	// Add password auth
	if config.SSHPassword != "" {
		authMethods = append(authMethods, ssh.Password(config.SSHPassword))
	}

	sshConfig := &ssh.ClientConfig{
		User:            config.SSHUser,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // OK for testing
		Timeout:         10 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", config.SSHHost, config.SSHPort)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}

	t.Logf("Connected to SSH server at %s", addr)

	return &TestSSHConnection{
		t:      t,
		client: client,
		config: config,
	}, nil
}

// Exec executes a command on the remote server
func (c *TestSSHConnection) Exec(command string) (string, error) {
	c.t.Helper()

	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(command)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// ExecWithTimeout executes a command with a timeout
func (c *TestSSHConnection) ExecWithTimeout(command string, timeout time.Duration) (string, error) {
	c.t.Helper()

	type result struct {
		output string
		err    error
	}

	resultCh := make(chan result, 1)

	go func() {
		output, err := c.Exec(command)
		resultCh <- result{output, err}
	}()

	select {
	case res := <-resultCh:
		return res.output, res.err
	case <-time.After(timeout):
		return "", fmt.Errorf("command timed out after %v", timeout)
	}
}

// Close closes the SSH connection
func (c *TestSSHConnection) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// TestTmuxSession represents a tmux session for testing
type TestTmuxSession struct {
	t          *testing.T
	conn       *TestSSHConnection
	sessionName string
}

// CreateTestTmuxSession creates a new tmux session for testing
func CreateTestTmuxSession(t *testing.T, conn *TestSSHConnection, sessionName string) *TestTmuxSession {
	t.Helper()

	// Create tmux session
	cmd := fmt.Sprintf("tmux new-session -d -s %s", sessionName)
	_, err := conn.Exec(cmd)
	if err != nil {
		t.Fatalf("Failed to create tmux session: %v", err)
	}

	t.Logf("Created tmux session: %s", sessionName)

	return &TestTmuxSession{
		t:           t,
		conn:        conn,
		sessionName: sessionName,
	}
}

// SendKeys sends keys to the tmux session
func (s *TestTmuxSession) SendKeys(keys string, enter bool) error {
	s.t.Helper()

	// Escape single quotes in keys by replacing ' with '\''
	escapedKeys := strings.ReplaceAll(keys, "'", "'\\''")

	cmd := fmt.Sprintf("tmux send-keys -t %s '%s'", s.sessionName, escapedKeys)
	if enter {
		cmd += " C-m"
	}

	_, err := s.conn.Exec(cmd)
	if err != nil {
		return fmt.Errorf("failed to send keys: %w", err)
	}

	// Give tmux time to process
	time.Sleep(100 * time.Millisecond)

	return nil
}

// CapturePane captures the content of the tmux pane
func (s *TestTmuxSession) CapturePane() (string, error) {
	s.t.Helper()

	cmd := fmt.Sprintf("tmux capture-pane -t %s -p", s.sessionName)
	output, err := s.conn.Exec(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to capture pane: %w", err)
	}

	return output, nil
}

// CapturePaneHistory captures the full pane history
func (s *TestTmuxSession) CapturePaneHistory() (string, error) {
	s.t.Helper()

	cmd := fmt.Sprintf("tmux capture-pane -t %s -p -S -10000", s.sessionName)
	output, err := s.conn.Exec(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to capture pane history: %w", err)
	}

	return output, nil
}

// NewWindow creates a new window in the session
func (s *TestTmuxSession) NewWindow(windowName string) error {
	s.t.Helper()

	cmd := fmt.Sprintf("tmux new-window -t %s -n %s", s.sessionName, windowName)
	_, err := s.conn.Exec(cmd)
	if err != nil {
		return fmt.Errorf("failed to create window: %w", err)
	}

	s.t.Logf("Created window: %s in session %s", windowName, s.sessionName)
	return nil
}

// Kill kills the tmux session
func (s *TestTmuxSession) Kill() error {
	s.t.Helper()

	cmd := fmt.Sprintf("tmux kill-session -t %s", s.sessionName)
	_, err := s.conn.Exec(cmd)
	if err != nil {
		// Session might already be dead, log but don't fail
		s.t.Logf("Warning: failed to kill session %s: %v", s.sessionName, err)
		return nil
	}

	s.t.Logf("Killed tmux session: %s", s.sessionName)
	return nil
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// WaitForCondition waits for a condition to be true
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, message string) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("Timeout waiting for: %s", message)
	return false
}
