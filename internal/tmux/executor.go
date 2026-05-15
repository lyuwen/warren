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
	// Build the full command string with proper quoting
	// We need to pass the entire command as a single string to SSH
	// to prevent the remote shell from interpreting special characters
	fullCmd := command
	for _, arg := range args {
		// Quote arguments that contain special characters or spaces
		if needsQuoting(arg) {
			fullCmd += fmt.Sprintf(" \"%s\"", escapeQuotes(arg))
		} else {
			fullCmd += " " + arg
		}
	}

	// Execute via SSH with the full command as a single argument
	sshArgs := []string{
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

// needsQuoting returns true if the argument contains characters that need quoting
func needsQuoting(s string) bool {
	// Check for special shell characters
	specialChars := " \t\n#{}[]()$`\"'\\|&;<>*?!"
	for _, c := range s {
		for _, special := range specialChars {
			if c == special {
				return true
			}
		}
	}
	return false
}

// escapeQuotes escapes double quotes in a string
func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, "\"", "\\\"")
}
