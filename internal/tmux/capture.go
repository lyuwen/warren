package tmux

import (
	"fmt"
	"regexp"
	"strings"
)

// CaptureOptions configures pane content capture
type CaptureOptions struct {
	// StartLine is the starting line to capture (negative for scrollback)
	StartLine int
	// EndLine is the ending line to capture (-1 for current visible area)
	EndLine int
	// StripANSI removes ANSI escape sequences if true
	StripANSI bool
	// JoinLines joins wrapped lines if true
	JoinLines bool
}

// DefaultCaptureOptions returns sensible defaults for capture
func DefaultCaptureOptions() *CaptureOptions {
	return &CaptureOptions{
		StartLine: -2000, // Capture 2000 lines of scrollback
		EndLine:   -1,    // Up to current visible area
		StripANSI: true,
		JoinLines: false,
	}
}

// CaptureResult contains the captured pane content and metadata
type CaptureResult struct {
	Content   string
	PaneID    string
	Lines     int
	Timestamp string
}

var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// CapturePane captures the content of a specific pane
func (c *Client) CapturePane(paneID string, opts *CaptureOptions) (*CaptureResult, error) {
	if opts == nil {
		opts = DefaultCaptureOptions()
	}

	args := []string{"capture-pane", "-p", "-t", paneID}

	// Add line range if specified
	if opts.StartLine != 0 || opts.EndLine != -1 {
		args = append(args, "-S", fmt.Sprintf("%d", opts.StartLine))
		if opts.EndLine != -1 {
			args = append(args, "-E", fmt.Sprintf("%d", opts.EndLine))
		}
	}

	// Join wrapped lines if requested
	if opts.JoinLines {
		args = append(args, "-J")
	}

	output, err := c.executor.Execute("tmux", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to capture pane %s: %w", paneID, err)
	}

	content := output

	// Strip ANSI escape sequences if requested
	if opts.StripANSI {
		content = stripANSI(content)
	}

	lines := strings.Count(content, "\n")

	return &CaptureResult{
		Content: content,
		PaneID:  paneID,
		Lines:   lines,
	}, nil
}

// stripANSI removes ANSI escape sequences from text
func stripANSI(text string) string {
	return ansiEscapeRegex.ReplaceAllString(text, "")
}

// GetVisibleContent captures only the currently visible content (no scrollback)
func (c *Client) GetVisibleContent(paneID string) (*CaptureResult, error) {
	opts := &CaptureOptions{
		StartLine: 0,
		EndLine:   -1,
		StripANSI: true,
		JoinLines: false,
	}
	return c.CapturePane(paneID, opts)
}

// GetRecentContent captures recent content including some scrollback
func (c *Client) GetRecentContent(paneID string, scrollbackLines int) (*CaptureResult, error) {
	opts := &CaptureOptions{
		StartLine: -scrollbackLines,
		EndLine:   -1,
		StripANSI: true,
		JoinLines: false,
	}
	return c.CapturePane(paneID, opts)
}

// TailContent returns the last N lines of captured content
func TailContent(content string, n int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= n {
		return content
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// HeadContent returns the first N lines of captured content
func HeadContent(content string, n int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= n {
		return content
	}
	return strings.Join(lines[:n], "\n")
}
