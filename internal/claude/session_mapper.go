package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SessionMapper maps process PIDs to Claude session IDs
type SessionMapper struct {
	claudeDir string
}

// SessionInfo contains session metadata
type SessionInfo struct {
	PID       int    `json:"pid"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	StartedAt int64  `json:"startedAt"`
	Version   string `json:"version"`
	Status    string `json:"status"`
	Kind      string `json:"kind"`
}

// NewSessionMapper creates a new session mapper
func NewSessionMapper() *SessionMapper {
	home := os.Getenv("HOME")
	return &SessionMapper{
		claudeDir: filepath.Join(home, ".claude"),
	}
}

// GetSessionID returns the session ID for a given PID
// Handles both regular sessions and sub-agent sessions
func (sm *SessionMapper) GetSessionID(pid int) (string, error) {
	// Try to read session file first
	sessionFile := filepath.Join(sm.claudeDir, "sessions", fmt.Sprintf("%d.json", pid))
	data, err := os.ReadFile(sessionFile)
	if err == nil {
		var info SessionInfo
		if err := json.Unmarshal(data, &info); err == nil {
			return info.SessionID, nil
		}
	}

	// If session file doesn't exist, try to get parent session ID from process args
	return sm.getParentSessionFromProc(pid)
}

// GetSessionInfo returns full session information for a PID
func (sm *SessionMapper) GetSessionInfo(pid int) (*SessionInfo, error) {
	sessionFile := filepath.Join(sm.claudeDir, "sessions", fmt.Sprintf("%d.json", pid))
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("session file not found: %w", err)
	}

	var info SessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	return &info, nil
}

// getParentSessionFromProc extracts parent session ID from process arguments
// Used for sub-agent sessions that don't have their own session files
func (sm *SessionMapper) getParentSessionFromProc(pid int) (string, error) {
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return "", fmt.Errorf("process not found: %w", err)
	}

	args := strings.Split(string(cmdline), "\x00")
	for i, arg := range args {
		if arg == "--parent-session-id" && i+1 < len(args) {
			return args[i+1], nil
		}
	}

	return "", fmt.Errorf("no session ID found for PID %d", pid)
}

// GetCWD returns the working directory for a PID
func (sm *SessionMapper) GetCWD(pid int) (string, error) {
	// Try session file first
	info, err := sm.GetSessionInfo(pid)
	if err == nil {
		return info.CWD, nil
	}

	// Fallback to /proc/{pid}/cwd
	cwdLink := fmt.Sprintf("/proc/%d/cwd", pid)
	cwd, err := os.Readlink(cwdLink)
	if err != nil {
		return "", fmt.Errorf("failed to get cwd: %w", err)
	}

	return cwd, nil
}

// GetProjectSlug converts a working directory path to a project slug
// Example: /home/user/project → -home-user-project
func GetProjectSlug(cwd string) string {
	slug := strings.ReplaceAll(cwd, "/", "-")
	if !strings.HasPrefix(slug, "-") {
		slug = "-" + slug
	}
	return slug
}
