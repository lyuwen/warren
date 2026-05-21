package integration

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestContainer represents a Docker test container
type TestContainer struct {
	t           *testing.T
	name        string
	running     bool
	KeepAlive   bool // If true, don't stop container on cleanup
}

// StartTestContainer starts the Docker SSH test container
func StartTestContainer(t *testing.T) (*TestContainer, error) {
	t.Helper()

	container := &TestContainer{
		t:    t,
		name: "warren-ssh-test",
	}

	// Check if container is already running
	if container.IsRunning() {
		t.Logf("Container %s is already running", container.name)
		container.running = true
		return container, nil
	}

	// Start container using docker compose
	t.Logf("Starting container %s...", container.name)
	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = "../../test/docker"
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w\nOutput: %s", err, output)
	}

	// Wait for container to be ready
	if !WaitForCondition(t, container.IsRunning, 30*time.Second, "container to start") {
		return nil, fmt.Errorf("container failed to start within 30 seconds")
	}

	// Wait for SSH to be ready
	if !WaitForCondition(t, container.IsSSHReady, 30*time.Second, "SSH to be ready") {
		return nil, fmt.Errorf("SSH failed to be ready within 30 seconds")
	}

	container.running = true
	t.Logf("Container %s is ready", container.name)

	return container, nil
}

// IsRunning checks if the container is running
func (c *TestContainer) IsRunning() bool {
	cmd := exec.Command("docker", "ps", "--filter", fmt.Sprintf("name=%s", c.name), "--format", "{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == c.name
}

// IsSSHReady checks if SSH is ready in the container
func (c *TestContainer) IsSSHReady() bool {
	cmd := exec.Command("docker", "exec", c.name, "pgrep", "sshd")
	err := cmd.Run()
	return err == nil
}

// Stop stops the container
func (c *TestContainer) Stop() error {
	if !c.running {
		return nil
	}

	if c.KeepAlive {
		c.t.Logf("KeepAlive is true, not stopping container %s", c.name)
		return nil
	}

	c.t.Logf("Stopping container %s...", c.name)
	cmd := exec.Command("docker", "compose", "down")
	cmd.Dir = "../../test/docker"
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop container: %w\nOutput: %s", err, output)
	}

	c.running = false
	c.t.Logf("Container %s stopped", c.name)
	return nil
}

// Exec executes a command in the container
func (c *TestContainer) Exec(command string) (string, error) {
	cmd := exec.Command("docker", "exec", c.name, "bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}
	return string(output), nil
}

// ExecAsUser executes a command as a specific user
func (c *TestContainer) ExecAsUser(user, command string) (string, error) {
	cmd := exec.Command("docker", "exec", "-u", user, c.name, "bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}
	return string(output), nil
}

// SSHConfig returns SSH configuration for the container
func (c *TestContainer) SSHConfig() *TestConfig {
	return DefaultTestConfig()
}

// Logs returns container logs
func (c *TestContainer) Logs() (string, error) {
	cmd := exec.Command("docker", "logs", c.name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to get logs: %w", err)
	}
	return string(output), nil
}

// Restart restarts the container
func (c *TestContainer) Restart() error {
	c.t.Logf("Restarting container %s...", c.name)
	cmd := exec.Command("docker", "restart", c.name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart container: %w\nOutput: %s", err, output)
	}

	// Wait for SSH to be ready again
	if !WaitForCondition(c.t, c.IsSSHReady, 30*time.Second, "SSH to be ready after restart") {
		return fmt.Errorf("SSH failed to be ready after restart")
	}

	c.t.Logf("Container %s restarted", c.name)
	return nil
}

// KillSSH kills the SSH process in the container (for testing reconnection)
func (c *TestContainer) KillSSH() error {
	c.t.Logf("Killing SSH process in container %s...", c.name)
	_, err := c.Exec("pkill sshd")
	return err
}

// RestartSSH restarts the SSH service
func (c *TestContainer) RestartSSH() error {
	c.t.Logf("Restarting SSH service in container %s...", c.name)
	_, err := c.Exec("service ssh restart")
	if err != nil {
		return err
	}

	// Wait for SSH to be ready
	if !WaitForCondition(c.t, c.IsSSHReady, 10*time.Second, "SSH to be ready after restart") {
		return fmt.Errorf("SSH failed to be ready after restart")
	}

	return nil
}

// CreateTmuxSession creates a tmux session in the container
func (c *TestContainer) CreateTmuxSession(sessionName string) error {
	c.t.Logf("Creating tmux session %s in container...", sessionName)
	_, err := c.ExecAsUser("testuser", fmt.Sprintf("tmux new-session -d -s %s", sessionName))
	return err
}

// ListTmuxSessions lists tmux sessions in the container
func (c *TestContainer) ListTmuxSessions() (string, error) {
	return c.ExecAsUser("testuser", "tmux list-sessions")
}

// KillTmuxSession kills a tmux session in the container
func (c *TestContainer) KillTmuxSession(sessionName string) error {
	c.t.Logf("Killing tmux session %s in container...", sessionName)
	_, err := c.ExecAsUser("testuser", fmt.Sprintf("tmux kill-session -t %s", sessionName))
	return err
}

// CleanupTmuxSessions kills all tmux sessions in the container
func (c *TestContainer) CleanupTmuxSessions() error {
	c.t.Logf("Cleaning up all tmux sessions in container...")
	_, err := c.ExecAsUser("testuser", "tmux kill-server 2>/dev/null || true")
	return err
}
