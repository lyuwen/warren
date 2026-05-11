package tmux

import (
	"bytes"
	"fmt"
	"os/exec"
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
	// Build the full command string
	fullCmd := command
	for _, arg := range args {
		// Simple quoting - may need to be more sophisticated
		if containsSpace(arg) {
			fullCmd += fmt.Sprintf(" '%s'", arg)
		} else {
			fullCmd += " " + arg
		}
	}

	// Execute via SSH
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

func containsSpace(s string) bool {
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '\n' {
			return true
		}
	}
	return false
}
