package core

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lfu/warren/internal/tmux"
)

// AgentDiscovery handles automatic discovery of agent sessions in tmux panes
type AgentDiscovery struct {
	tmuxClient *tmux.Client
}

// NewAgentDiscovery creates a new agent discovery service
func NewAgentDiscovery(tmuxClient *tmux.Client) *AgentDiscovery {
	return &AgentDiscovery{
		tmuxClient: tmuxClient,
	}
}

// DiscoveryResult contains information about a discovered agent
type DiscoveryResult struct {
	ServerName      string
	TmuxSessionName string
	TmuxWindowIndex int
	TmuxPaneID      string
	TmuxPaneIndex   int      // Pane index (0, 1, 2...) for display
	AgentType       string
	Confidence      float64 // 0.0 to 1.0
	Evidence        []string
}

// ClaudeCodeDetector detects Claude Code sessions
var claudeCodePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)claude\s+code`),
	regexp.MustCompile(`(?i)claude-code`),
	regexp.MustCompile(`@anthropic-ai/sdk`),
	regexp.MustCompile(`(?i)claude\s+\d+\.\d+`),
	regexp.MustCompile(`(?i)anthropic`),
	regexp.MustCompile(`tool_use`),
	regexp.MustCompile(`<thinking>`),
}

// CopilotDetector detects GitHub Copilot CLI sessions
var copilotPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)github\s+copilot`),
	regexp.MustCompile(`(?i)gh\s+copilot`),
	regexp.MustCompile(`@github/copilot`),
}

// DiscoverInPane analyzes a pane to detect if it contains an agent session
func (d *AgentDiscovery) DiscoverInPane(serverName string, sessionName string, windowIndex int, paneID string, currentCommand string) (*DiscoveryResult, error) {
	// CRITICAL: Check current command FIRST to avoid false positives
	// If the command is a known agent, trust it and return immediately
	if isAgentCommand(currentCommand) {
		// Determine agent type from command
		agentType := "unknown"
		confidence := 0.95 // High confidence since we verified the process

		if strings.HasPrefix(currentCommand, "claude") {
			agentType = "claude-code"
		} else if strings.HasPrefix(currentCommand, "node") {
			agentType = "claude-code" // Assume node is Claude Code
		} else if strings.HasPrefix(currentCommand, "gh") {
			agentType = "copilot"
		} else if strings.HasPrefix(currentCommand, "python") {
			agentType = "python-agent"
		} else if strings.HasPrefix(currentCommand, "aider") {
			agentType = "aider"
		} else if strings.HasPrefix(currentCommand, "cursor") {
			agentType = "cursor"
		}

		return &DiscoveryResult{
			ServerName:      serverName,
			TmuxSessionName: sessionName,
			TmuxWindowIndex: windowIndex,
			TmuxPaneID:      paneID,
			AgentType:       agentType,
			Confidence:      confidence,
			Evidence:        []string{fmt.Sprintf("process: %s", currentCommand)},
		}, nil
	}

	return nil, nil // Not an agent process
}

// isAgentCommand checks if the command is a known agent process
func isAgentCommand(command string) bool {
	// Known agent commands
	agentCommands := []string{
		"claude",      // Claude Code CLI
		"node",        // Node.js (for Claude Code and other agents)
		"python",      // Python agents
		"gh",          // GitHub Copilot CLI
		"aider",       // Aider AI
		"cursor",      // Cursor AI
	}

	// Check if command matches any known agent
	for _, agentCmd := range agentCommands {
		if strings.HasPrefix(command, agentCmd) {
			return true
		}
	}

	return false
}

func (d *AgentDiscovery) detectClaudeCode(content string) *DiscoveryResult {
	evidence := []string{}
	matchCount := 0

	for _, pattern := range claudeCodePatterns {
		if pattern.MatchString(content) {
			matchCount++
			evidence = append(evidence, fmt.Sprintf("matched pattern: %s", pattern.String()))
		}
	}

	if matchCount == 0 {
		return nil
	}

	// Calculate confidence based on number of matches
	confidence := float64(matchCount) / float64(len(claudeCodePatterns))
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Boost confidence if we see strong indicators
	if strings.Contains(content, "claude code") || strings.Contains(content, "claude-code") {
		confidence = 0.95
	}

	return &DiscoveryResult{
		AgentType:  "claude-code",
		Confidence: confidence,
		Evidence:   evidence,
	}
}

func (d *AgentDiscovery) detectCopilot(content string) *DiscoveryResult {
	evidence := []string{}
	matchCount := 0

	for _, pattern := range copilotPatterns {
		if pattern.MatchString(content) {
			matchCount++
			evidence = append(evidence, fmt.Sprintf("matched pattern: %s", pattern.String()))
		}
	}

	if matchCount == 0 {
		return nil
	}

	confidence := float64(matchCount) / float64(len(copilotPatterns))
	if confidence > 1.0 {
		confidence = 1.0
	}

	if strings.Contains(content, "github copilot") || strings.Contains(content, "gh copilot") {
		confidence = 0.95
	}

	return &DiscoveryResult{
		AgentType:  "copilot",
		Confidence: confidence,
		Evidence:   evidence,
	}
}

// DiscoverAll discovers all agent sessions across a topology
func (d *AgentDiscovery) DiscoverAll(topology *tmux.Topology, minConfidence float64) ([]*DiscoveryResult, error) {
	results := []*DiscoveryResult{}

	for _, session := range topology.Sessions {
		for _, window := range session.Windows {
			for _, pane := range window.Panes {
				result, err := d.DiscoverInPane(
					topology.ServerName,
					session.Name,
					window.Index,
					pane.ID,
					pane.CurrentCommand, // Pass current command for filtering
				)

				if err != nil {
					// Log error but continue discovery
					continue
				}

				if result != nil && result.Confidence >= minConfidence {
					// Store the pane index for display purposes
					result.TmuxPaneIndex = pane.Index
					results = append(results, result)
				}
			}
		}
	}

	return results, nil
}

// GenerateSessionID generates a unique session ID from discovery result
// Format: server:session:window.pane (e.g., localhost:0:10.0)
func GenerateSessionID(result *DiscoveryResult) string {
	return fmt.Sprintf("%s:%s:%d.%d",
		result.ServerName,
		result.TmuxSessionName,
		result.TmuxWindowIndex,
		result.TmuxPaneIndex,
	)
}

// ToAgentSession converts a discovery result to an agent session
func (r *DiscoveryResult) ToAgentSession() *AgentSession {
	// Create human-readable name: hostname/session:window.pane
	// Include hostname for multi-server clarity
	humanName := fmt.Sprintf("%s/%s:%d.%d", r.ServerName, r.TmuxSessionName, r.TmuxWindowIndex, r.TmuxPaneIndex)

	return &AgentSession{
		ID:              GenerateSessionID(r),
		Name:            humanName,
		ServerName:      r.ServerName,
		TmuxSessionName: r.TmuxSessionName,
		TmuxWindowIndex: r.TmuxWindowIndex,
		TmuxPaneIndex:   r.TmuxPaneIndex,
		TmuxPaneID:      r.TmuxPaneID,
		AgentType:       r.AgentType,
		CurrentState:    StateUnknown,
		Metadata: map[string]string{
			"discovery_confidence": fmt.Sprintf("%.2f", r.Confidence),
			"discovery_evidence":   strings.Join(r.Evidence, "; "),
		},
	}
}
