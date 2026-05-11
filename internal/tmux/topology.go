package tmux

import (
	"fmt"
	"strconv"
	"strings"
)

// TmuxSession represents a tmux session
type TmuxSession struct {
	Name      string
	ID        string
	Windows   []*Window
	Created   string
	Attached  bool
	ServerRef string // Reference to the server this session belongs to
}

// Window represents a tmux window within a session
type Window struct {
	Index     int
	Name      string
	ID        string
	Active    bool
	Panes     []*Pane
	SessionID string
}

// Pane represents a tmux pane within a window
type Pane struct {
	ID             string
	Index          int
	Title          string
	Width          int
	Height         int
	Active         bool
	WindowID       string
	PID            int    // Process ID running in the pane
	CurrentCommand string // Current command running in the pane
}

// Topology represents the complete tmux topology for a server
type Topology struct {
	ServerName string
	Sessions   []*TmuxSession
}

// Client provides methods to interact with tmux
type Client struct {
	executor CommandExecutor
}

// CommandExecutor defines the interface for executing commands
// This allows for local execution or remote SSH execution
type CommandExecutor interface {
	Execute(command string, args ...string) (string, error)
}

// NewClient creates a new tmux client
func NewClient(executor CommandExecutor) *Client {
	return &Client{
		executor: executor,
	}
}

// ListSessions returns all tmux sessions
func (c *Client) ListSessions() ([]*TmuxSession, error) {
	// Format: session_name:session_id:session_created:session_attached
	format := "#{session_name}:#{session_id}:#{session_created}:#{session_attached}"
	output, err := c.executor.Execute("tmux", "list-sessions", "-F", format)
	if err != nil {
		// No sessions is not an error
		if strings.Contains(err.Error(), "no server running") ||
		   strings.Contains(err.Error(), "no sessions") {
			return []*TmuxSession{}, nil
		}
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	sessions := make([]*TmuxSession, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}

		session := &TmuxSession{
			Name:     parts[0],
			ID:       parts[1],
			Created:  parts[2],
			Attached: parts[3] == "1",
			Windows:  []*Window{},
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// ListWindows returns all windows for a given session
func (c *Client) ListWindows(sessionName string) ([]*Window, error) {
	// Format: window_index:window_name:window_id:window_active
	format := "#{window_index}:#{window_name}:#{window_id}:#{window_active}"
	output, err := c.executor.Execute("tmux", "list-windows", "-t", sessionName, "-F", format)
	if err != nil {
		return nil, fmt.Errorf("failed to list windows for session %s: %w", sessionName, err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	windows := make([]*Window, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}

		index, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}

		window := &Window{
			Index:  index,
			Name:   parts[1],
			ID:     parts[2],
			Active: parts[3] == "1",
			Panes:  []*Pane{},
		}

		windows = append(windows, window)
	}

	return windows, nil
}

// ListPanes returns all panes for a given window
func (c *Client) ListPanes(sessionName string, windowIndex int) ([]*Pane, error) {
	target := fmt.Sprintf("%s:%d", sessionName, windowIndex)
	// Format: pane_id:pane_index:pane_title:pane_width:pane_height:pane_active:pane_pid:pane_current_command
	format := "#{pane_id}:#{pane_index}:#{pane_title}:#{pane_width}:#{pane_height}:#{pane_active}:#{pane_pid}:#{pane_current_command}"
	output, err := c.executor.Execute("tmux", "list-panes", "-t", target, "-F", format)
	if err != nil {
		return nil, fmt.Errorf("failed to list panes for %s: %w", target, err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	panes := make([]*Pane, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, ":")
		if len(parts) < 8 {
			continue
		}

		index, _ := strconv.Atoi(parts[1])
		width, _ := strconv.Atoi(parts[3])
		height, _ := strconv.Atoi(parts[4])
		pid, _ := strconv.Atoi(parts[6])

		pane := &Pane{
			ID:             parts[0],
			Index:          index,
			Title:          parts[2],
			Width:          width,
			Height:         height,
			Active:         parts[5] == "1",
			PID:            pid,
			CurrentCommand: parts[7],
		}

		panes = append(panes, pane)
	}

	return panes, nil
}

// DiscoverTopology discovers the complete tmux topology
func (c *Client) DiscoverTopology(serverName string) (*Topology, error) {
	sessions, err := c.ListSessions()
	if err != nil {
		return nil, err
	}

	// Populate windows and panes for each session
	for _, session := range sessions {
		session.ServerRef = serverName

		windows, err := c.ListWindows(session.Name)
		if err != nil {
			return nil, err
		}

		for _, window := range windows {
			window.SessionID = session.ID

			panes, err := c.ListPanes(session.Name, window.Index)
			if err != nil {
				return nil, err
			}

			for _, pane := range panes {
				pane.WindowID = window.ID
			}

			window.Panes = panes
		}

		session.Windows = windows
	}

	return &Topology{
		ServerName: serverName,
		Sessions:   sessions,
	}, nil
}

// FindPane finds a specific pane by ID across all sessions
func (t *Topology) FindPane(paneID string) (*Pane, *Window, *TmuxSession, error) {
	for _, session := range t.Sessions {
		for _, window := range session.Windows {
			for _, pane := range window.Panes {
				if pane.ID == paneID {
					return pane, window, session, nil
				}
			}
		}
	}
	return nil, nil, nil, fmt.Errorf("pane %s not found", paneID)
}
