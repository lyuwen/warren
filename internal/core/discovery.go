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
func (d *AgentDiscovery) DiscoverInPane(serverName string, sessionName string, windowIndex int, paneID string) (*DiscoveryResult, error) {
	// Capture pane content
	result, err := d.tmuxClient.GetRecentContent(paneID, 200)
	if err != nil {
		return nil, fmt.Errorf("failed to capture pane content: %w", err)
	}

	content := result.Content

	// Try Claude Code detection
	if claudeResult := d.detectClaudeCode(content); claudeResult != nil {
		claudeResult.ServerName = serverName
		claudeResult.TmuxSessionName = sessionName
		claudeResult.TmuxWindowIndex = windowIndex
		claudeResult.TmuxPaneID = paneID
		return claudeResult, nil
	}

	// Try Copilot detection
	if copilotResult := d.detectCopilot(content); copilotResult != nil {
		copilotResult.ServerName = serverName
		copilotResult.TmuxSessionName = sessionName
		copilotResult.TmuxWindowIndex = windowIndex
		copilotResult.TmuxPaneID = paneID
		return copilotResult, nil
	}

	return nil, nil // No agent detected
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
				)

				if err != nil {
					// Log error but continue discovery
					continue
				}

				if result != nil && result.Confidence >= minConfidence {
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
	// Extract pane number from pane ID (e.g., "%2" -> "2")
	paneNum := strings.TrimPrefix(result.TmuxPaneID, "%")

	return fmt.Sprintf("%s:%s:%d.%s",
		result.ServerName,
		result.TmuxSessionName,
		result.TmuxWindowIndex,
		paneNum,
	)
}

// ToAgentSession converts a discovery result to an agent session
func (r *DiscoveryResult) ToAgentSession() *AgentSession {
	// Extract pane number from pane ID (e.g., "%2" -> "2")
	paneNum := strings.TrimPrefix(r.TmuxPaneID, "%")

	// Create human-readable name: hostname/session:window.pane
	// Include hostname for multi-server clarity
	humanName := fmt.Sprintf("%s/%s:%d.%s", r.ServerName, r.TmuxSessionName, r.TmuxWindowIndex, paneNum)

	return &AgentSession{
		ID:              GenerateSessionID(r),
		Name:            humanName,
		ServerName:      r.ServerName,
		TmuxSessionName: r.TmuxSessionName,
		TmuxWindowIndex: r.TmuxWindowIndex,
		TmuxPaneID:      r.TmuxPaneID,
		AgentType:       r.AgentType,
		CurrentState:    StateUnknown,
		Metadata: map[string]string{
			"discovery_confidence": fmt.Sprintf("%.2f", r.Confidence),
			"discovery_evidence":   strings.Join(r.Evidence, "; "),
		},
	}
}
