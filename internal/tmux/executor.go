package tmux

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// LocalExecutor executes commands on the local machine
type LocalExecutor struct{}

// NewLocalExecutor creates a new local command executor
func NewLocalExecutor() *LocalExecutor {
	return &LocalExecutor{}
}

// Execute runs a command locally and returns its output
func (e *LocalExecutor) Execute(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// RemoteExecutor executes commands on a remote machine via SSH
type RemoteExecutor struct {
	host string
	user string
	port int
}

// NewRemoteExecutor creates a new remote command executor
func NewRemoteExecutor(user, host string, port int) *RemoteExecutor {
	return &RemoteExecutor{
		host: host,
		user: user,
		port: port,
	}
}

// Execute runs a command remotely via SSH and returns its output
func (e *RemoteExecutor) Execute(command string, args ...string) (string, error) {
	// Build the full command string with proper shell quoting.
	// Every argument is single-quoted to prevent remote shell interpretation
	// of special characters like #{}, $, etc.
	fullCmd := shellescape(command)
	for _, arg := range args {
		fullCmd += " " + shellescape(arg)
	}

	// Execute via SSH with connection multiplexing to avoid
	// overwhelming the remote SSH daemon with rapid connections.
	controlPath := fmt.Sprintf("/tmp/warren-ssh-%s@%s:%d", e.user, e.host, e.port)
	sshArgs := []string{
		"-o", "ControlMaster=auto",
		"-o", fmt.Sprintf("ControlPath=%s", controlPath),
		"-o", "ControlPersist=30",
		"-o", "ConnectTimeout=10",
		"-p", fmt.Sprintf("%d", e.port),
		fmt.Sprintf("%s@%s", e.user, e.host),
		fullCmd,
	}

	cmd := exec.Command("ssh", sshArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// shellescape wraps a string in single quotes for safe shell transport.
// Single quotes inside the string are handled by ending the quote, adding
// an escaped single quote, and reopening the quote.
func shellescape(s string) string {
	if s == "" {
		return "''"
	}
	// If the string is simple (no special chars), pass it through unquoted
	safe := true
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '/' || c == ':' || c == ',' || c == '+' || c == '=') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
