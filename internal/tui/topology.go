package tui

import (
	"fmt"
	"strings"

	"github.com/lfu/warren/internal/core"
	"github.com/lfu/warren/internal/tmux"
)

// topologyNode represents a node in the flattened topology tree
type topologyNode struct {
	nodeType   string // "server", "session", "window", "pane"
	id         string // Unique identifier for this node
	name       string // Display name
	level      int    // Indentation level (0=server, 1=session, 2=window, 3=pane)
	expanded   bool   // Whether this node is expanded
	hasChildren bool  // Whether this node has children
	agentState string // Agent state if this is a monitored pane
	agentID    string // Agent ID if this is a monitored pane
	serverName string // Server name (for all nodes)
}

// flattenTopology converts hierarchical topology to a flat list for rendering
func flattenTopology(topologies []*tmux.Topology, sessions []*core.MonitoredSession, expanded map[string]bool) []topologyNode {
	nodes := make([]topologyNode, 0)

	// Create session map for quick lookup
	sessionMap := make(map[string]*core.MonitoredSession)
	for _, sess := range sessions {
		sessionMap[sess.PaneID] = sess
	}

	// Process each server
	for _, topo := range topologies {
		serverID := fmt.Sprintf("server:%s", topo.ServerName)
		serverNode := topologyNode{
			nodeType:    "server",
			id:          serverID,
			name:        topo.ServerName,
			level:       0,
			expanded:    expanded[serverID],
			hasChildren: len(topo.Sessions) > 0,
			serverName:  topo.ServerName,
		}
		nodes = append(nodes, serverNode)

		// Only show sessions if server is expanded
		if !serverNode.expanded {
			continue
		}

		// Process each session
		for _, sess := range topo.Sessions {
			sessionID := fmt.Sprintf("session:%s:%s", topo.ServerName, sess.ID)
			sessionNode := topologyNode{
				nodeType:    "session",
				id:          sessionID,
				name:        sess.Name,
				level:       1,
				expanded:    expanded[sessionID],
				hasChildren: len(sess.Windows) > 0,
				serverName:  topo.ServerName,
			}
			nodes = append(nodes, sessionNode)

			// Only show windows if session is expanded
			if !sessionNode.expanded {
				continue
			}

			// Process each window
			for _, win := range sess.Windows {
				windowID := fmt.Sprintf("window:%s:%s:%d", topo.ServerName, sess.ID, win.Index)
				windowNode := topologyNode{
					nodeType:    "window",
					id:          windowID,
					name:        fmt.Sprintf("%d: %s", win.Index, win.Name),
					level:       2,
					expanded:    expanded[windowID],
					hasChildren: len(win.Panes) > 0,
					serverName:  topo.ServerName,
				}
				nodes = append(nodes, windowNode)

				// Only show panes if window is expanded
				if !windowNode.expanded {
					continue
				}

				// Process each pane
				for _, pane := range win.Panes {
					paneID := fmt.Sprintf("pane:%s:%s:%d:%s", topo.ServerName, sess.ID, win.Index, pane.ID)

					// Check if this pane is monitored
					agentState := ""
					agentID := ""
					if monSess, ok := sessionMap[pane.ID]; ok {
						agentState = string(monSess.CurrentState)
						agentID = monSess.AgentID
					}

					paneNode := topologyNode{
						nodeType:    "pane",
						id:          paneID,
						name:        pane.ID,
						level:       3,
						expanded:    false, // Panes don't have children
						hasChildren: false,
						agentState:  agentState,
						agentID:     agentID,
						serverName:  topo.ServerName,
					}
					nodes = append(nodes, paneNode)
				}
			}
		}
	}

	return nodes
}

// renderTopologyTree renders the topology tree as a string
func renderTopologyTree(nodes []topologyNode, cursor int, width int, height int) string {
	var b strings.Builder

	// Header
	b.WriteString("┌─ Warren Topology ─────────────────────────────────────┐\n")
	b.WriteString("│                                                        │\n")

	// Calculate visible range (simple scrolling)
	visibleHeight := height - 6 // Account for header and footer
	startIdx := 0
	endIdx := len(nodes)

	if cursor >= visibleHeight {
		startIdx = cursor - visibleHeight + 1
	}
	if endIdx > startIdx+visibleHeight {
		endIdx = startIdx + visibleHeight
	}

	// Render nodes
	for i := startIdx; i < endIdx && i < len(nodes); i++ {
		node := nodes[i]

		// Build line
		line := renderTopologyNode(node, i == cursor)

		// Truncate if too long
		if len(line) > width-4 {
			line = line[:width-7] + "..."
		}

		// Pad to width
		padding := width - len(line) - 4
		if padding < 0 {
			padding = 0
		}

		b.WriteString("│ ")
		b.WriteString(line)
		b.WriteString(strings.Repeat(" ", padding))
		b.WriteString(" │\n")
	}

	// Fill remaining space
	for i := endIdx - startIdx; i < visibleHeight; i++ {
		b.WriteString("│")
		b.WriteString(strings.Repeat(" ", width-2))
		b.WriteString("│\n")
	}

	// Footer
	b.WriteString("│                                                        │\n")
	b.WriteString("│ [↑↓] Navigate [Enter] Expand/Collapse [t] Toggle      │\n")
	b.WriteString("│ [r] Refresh [q] Quit                                   │\n")
	b.WriteString("└────────────────────────────────────────────────────────┘\n")

	return b.String()
}

// renderTopologyNode renders a single topology node
func renderTopologyNode(node topologyNode, selected bool) string {
	var b strings.Builder

	// Indentation
	indent := strings.Repeat("  ", node.level)
	b.WriteString(indent)

	// Tree characters
	if node.level > 0 {
		b.WriteString("└─ ")
	}

	// Icon based on node type
	switch node.nodeType {
	case "server":
		b.WriteString("📡 ")
	case "session":
		b.WriteString("📋 ")
	case "window":
		b.WriteString("🪟 ")
	case "pane":
		b.WriteString("🔲 ")
	}

	// Expand/collapse indicator
	if node.hasChildren {
		if node.expanded {
			b.WriteString("▼ ")
		} else {
			b.WriteString("▶ ")
		}
	}

	// Name
	if selected {
		b.WriteString("> ")
	}
	b.WriteString(node.name)

	// Agent state for panes
	if node.nodeType == "pane" && node.agentState != "" {
		b.WriteString(fmt.Sprintf(" [%s]", node.agentState))
	}

	return b.String()
}

// refreshTopologyData refreshes the topology data from Warren
func (m *Model) refreshTopologyData() {
	// Get topology from Warren
	topologies, err := m.warren.GetTopology()
	if err != nil {
		m.err = err
		return
	}

	// Get monitored sessions
	sessions := m.warren.GetAllSessions()

	// Initialize expanded map if needed
	if m.topologyExpanded == nil {
		m.topologyExpanded = make(map[string]bool)

		// Expand first server by default
		if len(topologies) > 0 {
			serverID := fmt.Sprintf("server:%s", topologies[0].ServerName)
			m.topologyExpanded[serverID] = true
		}
	}

	// Flatten topology
	m.topologyNodes = flattenTopology(topologies, sessions, m.topologyExpanded)
}

// toggleTopologyNode toggles the expanded state of the current node
func (m *Model) toggleTopologyNode() {
	if m.topologyCursor < 0 || m.topologyCursor >= len(m.topologyNodes) {
		return
	}

	node := m.topologyNodes[m.topologyCursor]
	if !node.hasChildren {
		return
	}

	// Toggle expanded state
	m.topologyExpanded[node.id] = !m.topologyExpanded[node.id]

	// Refresh topology to reflect changes
	m.refreshTopologyData()
}
