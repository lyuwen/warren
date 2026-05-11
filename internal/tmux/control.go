package tmux

import (
	"fmt"
	"strings"
)

// SendTextOptions configures how text is sent to a pane
type SendTextOptions struct {
	// Literal sends text literally without interpretation
	Literal bool
	// Enter sends an Enter key after the text
	Enter bool
}

// SendKeysOptions configures how keys are sent to a pane
type SendKeysOptions struct {
	// Literal sends keys literally
	Literal bool
}

// SendText sends text to a specific pane
func (c *Client) SendText(paneID string, text string, opts *SendTextOptions) error {
	if opts == nil {
		opts = &SendTextOptions{}
	}

	args := []string{"send-keys", "-t", paneID}

	if opts.Literal {
		args = append(args, "-l")
	}

	args = append(args, text)

	if opts.Enter {
		args = append(args, "Enter")
	}

	_, err := c.executor.Execute("tmux", args...)
	if err != nil {
		return fmt.Errorf("failed to send text to pane %s: %w", paneID, err)
	}

	return nil
}

// SendKeys sends keystroke sequences to a specific pane
func (c *Client) SendKeys(paneID string, keys ...string) error {
	args := []string{"send-keys", "-t", paneID}
	args = append(args, keys...)

	_, err := c.executor.Execute("tmux", args...)
	if err != nil {
		return fmt.Errorf("failed to send keys to pane %s: %w", paneID, err)
	}

	return nil
}

// SendEnter sends an Enter key to a pane
func (c *Client) SendEnter(paneID string) error {
	return c.SendKeys(paneID, "Enter")
}

// SendTab sends a Tab key to a pane
func (c *Client) SendTab(paneID string) error {
	return c.SendKeys(paneID, "Tab")
}

// SendShiftTab sends Shift+Tab to a pane
func (c *Client) SendShiftTab(paneID string) error {
	return c.SendKeys(paneID, "S-Tab")
}

// SendCtrlC sends Ctrl+C to a pane
func (c *Client) SendCtrlC(paneID string) error {
	return c.SendKeys(paneID, "C-c")
}

// SendCtrlD sends Ctrl+D to a pane
func (c *Client) SendCtrlD(paneID string) error {
	return c.SendKeys(paneID, "C-d")
}

// ValidatePane checks if a pane exists and is accessible
func (c *Client) ValidatePane(paneID string) error {
	// Try to get pane info
	format := "#{pane_id}"
	output, err := c.executor.Execute("tmux", "display-message", "-p", "-t", paneID, "-F", format)
	if err != nil {
		return fmt.Errorf("pane %s not found or not accessible: %w", paneID, err)
	}

	if strings.TrimSpace(output) != paneID {
		return fmt.Errorf("pane validation failed: expected %s, got %s", paneID, strings.TrimSpace(output))
	}

	return nil
}

// GetPaneInfo retrieves detailed information about a pane
func (c *Client) GetPaneInfo(paneID string) (*Pane, error) {
	// Format: pane_id:pane_index:pane_title:pane_width:pane_height:pane_active:pane_pid
	format := "#{pane_id}:#{pane_index}:#{pane_title}:#{pane_width}:#{pane_height}:#{pane_active}:#{pane_pid}"
	output, err := c.executor.Execute("tmux", "display-message", "-p", "-t", paneID, "-F", format)
	if err != nil {
		return nil, fmt.Errorf("failed to get pane info for %s: %w", paneID, err)
	}

	parts := strings.Split(strings.TrimSpace(output), ":")
	if len(parts) < 7 {
		return nil, fmt.Errorf("unexpected pane info format: %s", output)
	}

	var index, width, height, pid int
	fmt.Sscanf(parts[1], "%d", &index)
	fmt.Sscanf(parts[3], "%d", &width)
	fmt.Sscanf(parts[4], "%d", &height)
	fmt.Sscanf(parts[6], "%d", &pid)

	return &Pane{
		ID:     parts[0],
		Index:  index,
		Title:  parts[2],
		Width:  width,
		Height: height,
		Active: parts[5] == "1",
		PID:    pid,
	}, nil
}
