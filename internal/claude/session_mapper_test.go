package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSessionMapper_GetSessionID(t *testing.T) {
	// This test requires a real session file to exist
	// Skip if not in a test environment with Claude sessions
	home := os.Getenv("HOME")
	sessionsDir := filepath.Join(home, ".claude", "sessions")

	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Skip("No .claude/sessions directory found")
	}

	mapper := NewSessionMapper()

	// Test with a non-existent PID
	_, err := mapper.GetSessionID(999999)
	if err == nil {
		t.Error("Expected error for non-existent PID")
	}
}

func TestSessionMapper_GetProjectSlug(t *testing.T) {
	tests := []struct {
		cwd      string
		expected string
	}{
		{
			cwd:      "/home/user/project",
			expected: "-home-user-project",
		},
		{
			cwd:      "/var/log",
			expected: "-var-log",
		},
		{
			cwd:      "-already-prefixed",
			expected: "-already-prefixed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.cwd, func(t *testing.T) {
			result := GetProjectSlug(tt.cwd)
			if result != tt.expected {
				t.Errorf("GetProjectSlug(%q) = %q, want %q", tt.cwd, result, tt.expected)
			}
		})
	}
}

func TestSessionMapper_GetCWD(t *testing.T) {
	mapper := NewSessionMapper()

	// Test with current process
	pid := os.Getpid()
	cwd, err := mapper.GetCWD(pid)
	if err != nil {
		t.Fatalf("Failed to get CWD for current process: %v", err)
	}

	// Should return a valid directory
	if _, err := os.Stat(cwd); os.IsNotExist(err) {
		t.Errorf("CWD does not exist: %s", cwd)
	}
}
