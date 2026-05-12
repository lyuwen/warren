package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// RemoteReader reads Claude session data from remote servers via SSH
type RemoteReader struct {
	client *ssh.Client
	host   string
}

// NewRemoteReader creates a new remote reader for a given SSH connection
func NewRemoteReader(client *ssh.Client, host string) *RemoteReader {
	return &RemoteReader{
		client: client,
		host:   host,
	}
}

// GetSessionID returns the session ID for a remote PID
func (rr *RemoteReader) GetSessionID(pid int) (string, error) {
	// Try session file first
	sessionFile := fmt.Sprintf("~/.claude/sessions/%d.json", pid)
	content, err := rr.readRemoteFile(sessionFile)
	if err == nil {
		// Parse session info
		var info SessionInfo
		if err := json.Unmarshal([]byte(content), &info); err == nil {
			return info.SessionID, nil
		}
	}

	// Try to get parent session ID from process args
	cmd := fmt.Sprintf("cat /proc/%d/cmdline 2>/dev/null | tr '\\0' '\\n' | grep -A1 '^--parent-session-id$' | tail -1", pid)
	output, err := rr.runCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("no session ID found for remote PID %d: %w", pid, err)
	}

	sessionID := strings.TrimSpace(output)
	if sessionID == "" {
		return "", fmt.Errorf("no session ID found for remote PID %d", pid)
	}

	return sessionID, nil
}

// GetCWD returns the working directory for a remote PID
func (rr *RemoteReader) GetCWD(pid int) (string, error) {
	// Try session file first
	sessionFile := fmt.Sprintf("~/.claude/sessions/%d.json", pid)
	content, err := rr.readRemoteFile(sessionFile)
	if err == nil {
		var info SessionInfo
		if err := json.Unmarshal([]byte(content), &info); err == nil {
			return info.CWD, nil
		}
	}

	// Fallback to /proc/{pid}/cwd
	cmd := fmt.Sprintf("readlink /proc/%d/cwd 2>/dev/null", pid)
	output, err := rr.runCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get remote cwd: %w", err)
	}

	return strings.TrimSpace(output), nil
}

// ReadConversation reads a conversation file from the remote server
func (rr *RemoteReader) ReadConversation(sessionID, cwd string) ([]*Message, error) {
	projectSlug := GetProjectSlug(cwd)
	remotePath := fmt.Sprintf("~/.claude/projects/%s/%s.jsonl", projectSlug, sessionID)

	content, err := rr.readRemoteFile(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote conversation file: %w", err)
	}

	// Parse JSONL content
	lines := strings.Split(content, "\n")
	var messages []*Message

	cr := NewConversationReader()
	for _, line := range lines {
		if line == "" {
			continue
		}

		var msg Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		// Parse timestamp
		if msg.TimestampRaw != "" {
			ts, err := time.Parse(time.RFC3339, msg.TimestampRaw)
			if err == nil {
				msg.Timestamp = ts
			}
		}

		// Parse message content
		if msg.Type == "user" || msg.Type == "assistant" {
			if err := cr.parseMessageContent(&msg); err != nil {
				continue
			}
		}

		messages = append(messages, &msg)
	}

	return messages, nil
}

// readRemoteFile reads a file from the remote server
func (rr *RemoteReader) readRemoteFile(path string) (string, error) {
	// Expand ~ to home directory
	cmd := fmt.Sprintf("cat %s 2>/dev/null", path)
	return rr.runCommand(cmd)
}

// runCommand executes a command on the remote server and returns output
func (rr *RemoteReader) runCommand(cmd string) (string, error) {
	session, err := rr.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// CopyConversationToLocal copies a remote conversation file to local cache
// This can improve performance for repeated reads
func (rr *RemoteReader) CopyConversationToLocal(sessionID, cwd, localCacheDir string) (string, error) {
	projectSlug := GetProjectSlug(cwd)
	remotePath := fmt.Sprintf("~/.claude/projects/%s/%s.jsonl", projectSlug, sessionID)

	content, err := rr.readRemoteFile(remotePath)
	if err != nil {
		return "", err
	}

	// Create local cache directory
	cacheDir := filepath.Join(localCacheDir, "remote-conversations", rr.host)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Write to local cache
	localPath := filepath.Join(cacheDir, fmt.Sprintf("%s.jsonl", sessionID))
	if err := os.WriteFile(localPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write cache file: %w", err)
	}

	return localPath, nil
}
