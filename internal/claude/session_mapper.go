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
	sessionID, err := sm.getParentSessionFromProc(pid)
	if err == nil {
		return sessionID, nil
	}

	// If still not found, search child processes (pane PID might be shell, Claude is child)
	return sm.searchChildProcesses(pid)
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

// searchChildProcesses searches child processes for Claude session files
// This handles cases where the pane PID is a shell and Claude is a child process
func (sm *SessionMapper) searchChildProcesses(parentPID int) (string, error) {
	// Read all session files
	sessionsDir := filepath.Join(sm.claudeDir, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read sessions directory: %w", err)
	}

	// Check each session file to see if it's a child of parentPID
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Extract PID from filename
		var childPID int
		if _, err := fmt.Sscanf(entry.Name(), "%d.json", &childPID); err != nil {
			continue
		}

		// Check if this process is a child of parentPID
		if sm.isChildProcess(childPID, parentPID) {
			// Found a child process with a session file
			sessionFile := filepath.Join(sessionsDir, entry.Name())
			data, err := os.ReadFile(sessionFile)
			if err != nil {
				continue
			}

			var info SessionInfo
			if err := json.Unmarshal(data, &info); err != nil {
				continue
			}

			return info.SessionID, nil
		}
	}

	return "", fmt.Errorf("no session ID found for PID %d or its children", parentPID)
}

// isChildProcess checks if childPID is a descendant of parentPID
func (sm *SessionMapper) isChildProcess(childPID, parentPID int) bool {
	// Read child's status to get its parent PID
	statusFile := fmt.Sprintf("/proc/%d/status", childPID)
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return false
	}

	// Parse PPid line
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "PPid:") {
			var ppid int
			if _, err := fmt.Sscanf(line, "PPid:\t%d", &ppid); err == nil {
				if ppid == parentPID {
					return true
				}
				// Recursively check if ppid is a child of parentPID
				if ppid > 1 {
					return sm.isChildProcess(ppid, parentPID)
				}
			}
			break
		}
	}

	return false
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
