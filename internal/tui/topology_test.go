package tui

import (
	"testing"

	"github.com/lfu/warren/internal/core"
	"github.com/lfu/warren/internal/tmux"
)

func TestFlattenTopology(t *testing.T) {
	// Create test topology
	topologies := []*tmux.Topology{
		{
			ServerName: "local",
			Sessions: []*tmux.TmuxSession{
				{
					ID:   "$0",
					Name: "main",
					Windows: []*tmux.Window{
						{
							Index: 0,
							Name:  "editor",
							Panes: []*tmux.Pane{
								{
									ID:    "%1",
									Title: "vim",
								},
							},
						},
					},
				},
			},
		},
	}

	// Create empty sessions map
	sessions := []*core.MonitoredSession{}

	// Create expanded map with server expanded
	expanded := map[string]bool{
		"server:local": true,
	}

	// Flatten topology
	nodes := flattenTopology(topologies, sessions, expanded)

	// Verify we got the server node
	if len(nodes) == 0 {
		t.Fatal("Expected at least one node, got 0")
	}

	// Check server node
	if nodes[0].nodeType != "server" {
		t.Errorf("Expected first node to be server, got %s", nodes[0].nodeType)
	}
	if nodes[0].name != "local" {
		t.Errorf("Expected server name 'local', got %s", nodes[0].name)
	}
	if !nodes[0].expanded {
		t.Error("Expected server to be expanded")
	}

	// Check session node
	if len(nodes) < 2 {
		t.Fatal("Expected at least 2 nodes (server + session)")
	}
	if nodes[1].nodeType != "session" {
		t.Errorf("Expected second node to be session, got %s", nodes[1].nodeType)
	}
	if nodes[1].name != "main" {
		t.Errorf("Expected session name 'main', got %s", nodes[1].name)
	}
}

func TestFlattenTopologyWithCollapsedServer(t *testing.T) {
	// Create test topology
	topologies := []*tmux.Topology{
		{
			ServerName: "local",
			Sessions: []*tmux.TmuxSession{
				{
					ID:   "$0",
					Name: "main",
					Windows: []*tmux.Window{
						{
							Index: 0,
							Name:  "editor",
							Panes: []*tmux.Pane{
								{
									ID:    "%1",
									Title: "vim",
								},
							},
						},
					},
				},
			},
		},
	}

	sessions := []*core.MonitoredSession{}

	// Server is NOT expanded
	expanded := map[string]bool{}

	// Flatten topology
	nodes := flattenTopology(topologies, sessions, expanded)

	// Should only have server node (no children shown)
	if len(nodes) != 1 {
		t.Errorf("Expected 1 node (collapsed server), got %d", len(nodes))
	}

	if nodes[0].nodeType != "server" {
		t.Errorf("Expected node to be server, got %s", nodes[0].nodeType)
	}
	if nodes[0].expanded {
		t.Error("Expected server to be collapsed")
	}
}

func TestFlattenTopologyWithMonitoredPane(t *testing.T) {
	// Create test topology
	topologies := []*tmux.Topology{
		{
			ServerName: "local",
			Sessions: []*tmux.TmuxSession{
				{
					ID:   "$0",
					Name: "main",
					Windows: []*tmux.Window{
						{
							Index: 0,
							Name:  "editor",
							Panes: []*tmux.Pane{
								{
									ID:    "%1",
									Title: "vim",
								},
							},
						},
					},
				},
			},
		},
	}

	// Create monitored session for pane %1
	sessions := []*core.MonitoredSession{
		{
			AgentID:      "agent-123",
			PaneID:       "%1",
			CurrentState: core.StateIdle,
			ServerName:   "local",
		},
	}

	// Expand all nodes
	expanded := map[string]bool{
		"server:local":       true,
		"session:local:$0":   true,
		"window:local:$0:0":  true,
	}

	// Flatten topology
	nodes := flattenTopology(topologies, sessions, expanded)

	// Find the pane node
	var paneNode *topologyNode
	for i := range nodes {
		if nodes[i].nodeType == "pane" {
			paneNode = &nodes[i]
			break
		}
	}

	if paneNode == nil {
		t.Fatal("Expected to find pane node")
	}

	// Verify agent state is set
	if paneNode.agentState != string(core.StateIdle) {
		t.Errorf("Expected agent state 'idle', got %s", paneNode.agentState)
	}
	if paneNode.agentID != "agent-123" {
		t.Errorf("Expected agent ID 'agent-123', got %s", paneNode.agentID)
	}
}

func TestRenderTopologyNode(t *testing.T) {
	tests := []struct {
		name     string
		node     topologyNode
		selected bool
		want     string
	}{
		{
			name: "server node expanded",
			node: topologyNode{
				nodeType:    "server",
				name:        "local",
				level:       0,
				expanded:    true,
				hasChildren: true,
			},
			selected: false,
			want:     "📡 ▼ local",
		},
		{
			name: "server node collapsed",
			node: topologyNode{
				nodeType:    "server",
				name:        "local",
				level:       0,
				expanded:    false,
				hasChildren: true,
			},
			selected: false,
			want:     "📡 ▶ local",
		},
		{
			name: "pane with agent state",
			node: topologyNode{
				nodeType:   "pane",
				name:       "%1",
				level:      3,
				agentState: "idle",
				agentID:    "agent-123",
			},
			selected: false,
			want:     "      └─ 🔲 %1 [idle]",
		},
		{
			name: "selected node",
			node: topologyNode{
				nodeType: "session",
				name:     "main",
				level:    1,
			},
			selected: true,
			want:     "  └─ 📋 > main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := renderTopologyNode(tt.node, tt.selected)
			if got != tt.want {
				t.Errorf("renderTopologyNode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterTopologyByServer(t *testing.T) {
	nodes := []topologyNode{
		{nodeType: "server", name: "local", serverName: "local"},
		{nodeType: "session", name: "main", serverName: "local"},
		{nodeType: "server", name: "remote", serverName: "remote"},
		{nodeType: "session", name: "test", serverName: "remote"},
	}

	m := &Model{
		topologyFilterServer: "local",
		topologyFilterState:  "all",
	}

	filtered := m.filterTopology(nodes)

	// Should only have local server and session
	if len(filtered) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(filtered))
	}

	for _, node := range filtered {
		if node.serverName != "local" {
			t.Errorf("Expected server name 'local', got %s", node.serverName)
		}
	}
}

func TestFilterTopologyByState(t *testing.T) {
	nodes := []topologyNode{
		{nodeType: "server", name: "local", serverName: "local"},
		{nodeType: "pane", name: "%1", serverName: "local", agentState: "idle"},
		{nodeType: "pane", name: "%2", serverName: "local", agentState: "thinking"},
		{nodeType: "pane", name: "%3", serverName: "local", agentState: "error"},
	}

	m := &Model{
		topologyFilterServer: "",
		topologyFilterState:  "idle",
	}

	filtered := m.filterTopology(nodes)

	// Should have server and idle pane
	foundIdle := false
	for _, node := range filtered {
		if node.nodeType == "pane" {
			if node.agentState != "idle" {
				t.Errorf("Expected only idle panes, got %s", node.agentState)
			}
			foundIdle = true
		}
	}

	if !foundIdle {
		t.Error("Expected to find idle pane")
	}
}

func TestFilterTopologyCombined(t *testing.T) {
	nodes := []topologyNode{
		{nodeType: "server", name: "local", serverName: "local"},
		{nodeType: "pane", name: "%1", serverName: "local", agentState: "idle"},
		{nodeType: "pane", name: "%2", serverName: "local", agentState: "thinking"},
		{nodeType: "server", name: "remote", serverName: "remote"},
		{nodeType: "pane", name: "%3", serverName: "remote", agentState: "idle"},
	}

	m := &Model{
		topologyFilterServer: "local",
		topologyFilterState:  "idle",
	}

	filtered := m.filterTopology(nodes)

	// Should only have local server and local idle pane
	for _, node := range filtered {
		if node.serverName != "local" {
			t.Errorf("Expected server name 'local', got %s", node.serverName)
		}
		if node.nodeType == "pane" && node.agentState != "idle" {
			t.Errorf("Expected agent state 'idle', got %s", node.agentState)
		}
	}
}

func TestFilterTopologyNoFilters(t *testing.T) {
	nodes := []topologyNode{
		{nodeType: "server", name: "local", serverName: "local"},
		{nodeType: "pane", name: "%1", serverName: "local", agentState: "idle"},
		{nodeType: "server", name: "remote", serverName: "remote"},
	}

	m := &Model{
		topologyFilterServer: "",
		topologyFilterState:  "all",
	}

	filtered := m.filterTopology(nodes)

	// Should return all nodes
	if len(filtered) != len(nodes) {
		t.Errorf("Expected %d nodes, got %d", len(nodes), len(filtered))
	}
}
