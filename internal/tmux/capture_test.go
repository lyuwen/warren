package tmux

import (
	"strings"
	"testing"
)

func TestStripANSI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no ANSI codes",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "with color codes",
			input:    "\x1b[31mred text\x1b[0m",
			expected: "red text",
		},
		{
			name:     "with cursor movement",
			input:    "\x1b[2J\x1b[Hclear screen",
			expected: "clear screen",
		},
		{
			name:     "mixed content",
			input:    "normal \x1b[1mbold\x1b[0m normal",
			expected: "normal bold normal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripANSI(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTailContent(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"

	tests := []struct {
		name     string
		n        int
		expected string
	}{
		{
			name:     "last 2 lines",
			n:        2,
			expected: "line4\nline5",
		},
		{
			name:     "more than available",
			n:        10,
			expected: content,
		},
		{
			name:     "exact count",
			n:        5,
			expected: content,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TailContent(content, tt.n)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestHeadContent(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5"

	tests := []struct {
		name     string
		n        int
		expected string
	}{
		{
			name:     "first 2 lines",
			n:        2,
			expected: "line1\nline2",
		},
		{
			name:     "more than available",
			n:        10,
			expected: content,
		},
		{
			name:     "exact count",
			n:        5,
			expected: content,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HeadContent(content, tt.n)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDefaultCaptureOptions(t *testing.T) {
	opts := DefaultCaptureOptions()

	if opts.StartLine != -2000 {
		t.Errorf("Expected StartLine -2000, got %d", opts.StartLine)
	}

	if opts.EndLine != -1 {
		t.Errorf("Expected EndLine -1, got %d", opts.EndLine)
	}

	if !opts.StripANSI {
		t.Error("Expected StripANSI to be true")
	}

	if opts.JoinLines {
		t.Error("Expected JoinLines to be false")
	}
}

// Integration test - requires tmux to be running
func TestCapturePane_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	executor := NewLocalExecutor()
	client := NewClient(executor)

	// Try to list sessions - if this fails, tmux is not running
	sessions, err := client.ListSessions()
	if err != nil {
		t.Skip("Tmux not running, skipping integration test")
	}

	if len(sessions) == 0 {
		t.Skip("No tmux sessions found, skipping integration test")
	}

	// Get first pane from first session
	session := sessions[0]
	windows, err := client.ListWindows(session.Name)
	if err != nil || len(windows) == 0 {
		t.Skip("No windows found, skipping integration test")
	}

	window := windows[0]
	panes, err := client.ListPanes(session.Name, window.Index)
	if err != nil || len(panes) == 0 {
		t.Skip("No panes found, skipping integration test")
	}

	pane := panes[0]

	// Test capture
	result, err := client.GetVisibleContent(pane.ID)
	if err != nil {
		t.Fatalf("Failed to capture pane: %v", err)
	}

	if result.PaneID != pane.ID {
		t.Errorf("Expected pane ID %s, got %s", pane.ID, result.PaneID)
	}

	// Content should be a string (may be empty)
	if result.Content == "" {
		t.Log("Warning: captured content is empty")
	}

	// Test that ANSI codes are stripped
	if strings.Contains(result.Content, "\x1b[") {
		t.Error("ANSI codes not stripped from content")
	}
}
