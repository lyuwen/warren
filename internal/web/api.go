package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/lfu/warren/internal/claude"
	"github.com/lfu/warren/internal/core"
	"github.com/lfu/warren/internal/tmux"
)

// handleGetServers returns all registered servers
func (s *Server) handleGetServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Query all servers from the registry
	allSessions := s.warren.GetAllSessions()
	serverAgentCounts := make(map[string]int)
	for _, sess := range allSessions {
		serverAgentCounts[sess.ServerName]++
	}

	registry := s.warren.GetServerRegistry()
	servers := make([]map[string]interface{}, 0)
	if registry != nil {
		for _, srv := range registry.List() {
			servers = append(servers, map[string]interface{}{
				"name":        srv.Name,
				"host":        srv.Host,
				"kind":        string(srv.Kind),
				"agent_count": serverAgentCounts[srv.Name],
				"status":      "online",
			})
		}
	}

	respondJSON(w, http.StatusOK, servers)
}

// handleGetAgents returns all agent sessions
func (s *Server) handleGetAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessions := s.warren.GetAllSessions()

	agents := make([]map[string]interface{}, 0, len(sessions))
	for _, session := range sessions {
		agents = append(agents, map[string]interface{}{
			"id":           session.AgentID,
			"pane_id":      session.PaneID,
			"state":        string(session.CurrentState),
			"last_poll":    session.LastPollTime,
			"error_count":  session.ErrorCount,
			"server":       session.ServerName,
			"working_dir":  session.WorkingDir,
		})
	}

	respondJSON(w, http.StatusOK, agents)
}

// handleGetAgent returns details for a specific agent
func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract agent ID from path: /api/agents/{id}
	agentID := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	if agentID == "" {
		http.Error(w, "Agent ID required", http.StatusBadRequest)
		return
	}

	// Get session state
	state, err := s.warren.GetSessionState(agentID)
	if err != nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Get session details
	sessions := s.warren.GetAllSessions()
	var session *core.MonitoredSession
	for _, s := range sessions {
		if s.AgentID == agentID {
			session = s
			break
		}
	}

	if session == nil {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Get artifact profile
	profile, err := s.warren.GetArtifactProfile(agentID)
	if err != nil {
		profile = nil // Profile may not exist yet
	}

	// Get recent activities
	store := s.warren.GetEventStore()
	activities, err := store.GetRecentActivities(agentID, 20)
	if err != nil {
		activities = nil
	}

	response := map[string]interface{}{
		"id":           session.AgentID,
		"pane_id":      session.PaneID,
		"state":        string(state),
		"last_poll":    session.LastPollTime,
		"error_count":  session.ErrorCount,
		"profile":      profile,
		"activities":   activities,
	}

	respondJSON(w, http.StatusOK, response)
}

// handleGetNotifications returns all unconsumed notifications
func (s *Server) handleGetNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	notifications, err := s.warren.GetUnconsumedNotifications()
	if err != nil {
		http.Error(w, "Failed to get notifications", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, notifications)
}

// handleConsumeNotification marks a notification as consumed
func (s *Server) handleConsumeNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		AgentID   string `json:"agent_id"`
		NotifType string `json:"notif_type"`
		Timestamp string `json:"timestamp"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Parse timestamp
	timestamp, err := parseTimestamp(req.Timestamp)
	if err != nil {
		http.Error(w, "Invalid timestamp", http.StatusBadRequest)
		return
	}

	// Mark as consumed
	engine := s.warren.GetNotificationEngine()
	if err := engine.MarkAsConsumed(req.AgentID, req.NotifType, timestamp); err != nil {
		http.Error(w, "Failed to mark notification as consumed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleClearNotifications marks all unconsumed notifications as consumed
func (s *Server) handleClearNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	engine := s.warren.GetNotificationEngine()
	count, err := engine.MarkAllAsConsumed()
	if err != nil {
		http.Error(w, "Failed to clear notifications", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "cleared": count})
}

// handleGetConversation returns conversation history for an agent
func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract agent ID from path: /api/conversation/{id}
	agentID := strings.TrimPrefix(r.URL.Path, "/api/conversation/")
	if agentID == "" {
		http.Error(w, "Agent ID required", http.StatusBadRequest)
		return
	}

	// Parse query parameters
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if n, err := parseIntParam(limitStr); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	offset := 0
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if n, err := parseIntParam(offsetStr); err == nil && n >= 0 {
			offset = n
		}
	}

	// Get agent session from Warren
	session, err := s.warren.GetSession(agentID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get session: %v", err), http.StatusNotFound)
		return
	}

	// Get server info
	server, err := s.warren.GetServer(session.ServerName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get server: %v", err), http.StatusInternalServerError)
		return
	}

	// Get pane info
	pane, err := s.warren.GetPane(session, server)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get pane: %v", err), http.StatusInternalServerError)
		return
	}

	// Load conversation from Claude session files
	messages, err := s.conversationService.GetRecentMessages(session, server, pane, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load conversation: %v", err), http.StatusInternalServerError)
		return
	}

	// Apply offset if specified
	if offset > 0 && offset < len(messages) {
		messages = messages[offset:]
	} else if offset >= len(messages) {
		messages = []*claude.Message{}
	}

	// Build response
	response := map[string]interface{}{
		"agent_id": agentID,
		"messages": messages,
		"total":    len(messages),
		"limit":    limit,
		"offset":   offset,
		"status":   "ok",
	}

	respondJSON(w, http.StatusOK, response)
}

// parseIntParam parses an integer parameter from a string
func parseIntParam(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// parseTimestamp parses a timestamp string
func parseTimestamp(s string) (time.Time, error) {
	// Try RFC3339 format first
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}

	// Try RFC3339Nano format
	return time.Parse(time.RFC3339Nano, s)
}

// handleGetTopology returns the complete topology for all servers
func (s *Server) handleGetTopology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	topologies, err := s.warren.GetTopology()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get topology: %v", err), http.StatusInternalServerError)
		return
	}

	// Get all monitored sessions to enrich topology with agent states
	sessions := s.warren.GetAllSessions()
	sessionMap := make(map[string]*core.MonitoredSession)
	for _, sess := range sessions {
		sessionMap[sess.PaneID] = sess
	}

	// Build response with hierarchical structure
	response := map[string]interface{}{
		"servers": buildTopologyResponse(topologies, sessionMap),
	}

	respondJSON(w, http.StatusOK, response)
}

// handleGetTopologyServer returns topology for a specific server
func (s *Server) handleGetTopologyServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract server name from path: /api/topology/servers/{name}
	serverName := strings.TrimPrefix(r.URL.Path, "/api/topology/servers/")
	if serverName == "" {
		http.Error(w, "Server name required", http.StatusBadRequest)
		return
	}

	topologies, err := s.warren.GetTopology()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get topology: %v", err), http.StatusInternalServerError)
		return
	}

	// Find the requested server
	var serverTopology *tmux.Topology
	for _, topo := range topologies {
		if topo.ServerName == serverName {
			serverTopology = topo
			break
		}
	}

	if serverTopology == nil {
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	// Get monitored sessions for this server
	sessions := s.warren.GetAllSessions()
	sessionMap := make(map[string]*core.MonitoredSession)
	for _, sess := range sessions {
		if sess.ServerName == serverName {
			sessionMap[sess.PaneID] = sess
		}
	}

	// Build response
	response := buildServerTopologyResponse(serverTopology, sessionMap)

	respondJSON(w, http.StatusOK, response)
}

// handleGetTopologySession returns topology for a specific session
func (s *Server) handleGetTopologySession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from path: /api/topology/sessions/{id}
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/topology/sessions/")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Get the agent session
	agentSession, err := s.warren.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Get topology for the server
	topologies, err := s.warren.GetTopology()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get topology: %v", err), http.StatusInternalServerError)
		return
	}

	// Find the session in topology
	var foundSession *tmux.TmuxSession
	for _, topo := range topologies {
		if topo.ServerName == agentSession.ServerName {
			for _, sess := range topo.Sessions {
				if sess.Name == agentSession.TmuxSessionName {
					foundSession = sess
					break
				}
			}
		}
	}

	if foundSession == nil {
		http.Error(w, "Session not found in topology", http.StatusNotFound)
		return
	}

	// Get monitored sessions for enrichment
	sessions := s.warren.GetAllSessions()
	sessionMap := make(map[string]*core.MonitoredSession)
	for _, sess := range sessions {
		sessionMap[sess.PaneID] = sess
	}

	// Build response
	response := map[string]interface{}{
		"id":          foundSession.ID,
		"name":        foundSession.Name,
		"server":      agentSession.ServerName,
		"windows":     buildWindowsResponse(foundSession.Windows, agentSession.ServerName, sessionMap),
		"agent_state": string(agentSession.CurrentState),
	}

	respondJSON(w, http.StatusOK, response)
}

// buildTopologyResponse builds the topology response structure
func buildTopologyResponse(topologies []*tmux.Topology, sessionMap map[string]*core.MonitoredSession) []map[string]interface{} {
	servers := make([]map[string]interface{}, 0, len(topologies))

	for _, topo := range topologies {
		servers = append(servers, buildServerTopologyResponse(topo, sessionMap))
	}

	return servers
}

// buildServerTopologyResponse builds a server topology response
func buildServerTopologyResponse(topo *tmux.Topology, sessionMap map[string]*core.MonitoredSession) map[string]interface{} {
	sessions := make([]map[string]interface{}, 0, len(topo.Sessions))

	for _, sess := range topo.Sessions {
		sessions = append(sessions, map[string]interface{}{
			"id":       sess.ID,
			"name":     sess.Name,
			"created":  sess.Created,
			"attached": sess.Attached,
			"windows":  buildWindowsResponse(sess.Windows, topo.ServerName, sessionMap),
		})
	}

	return map[string]interface{}{
		"name":     topo.ServerName,
		"sessions": sessions,
	}
}

// buildWindowsResponse builds windows response with panes
func buildWindowsResponse(windows []*tmux.Window, serverName string, sessionMap map[string]*core.MonitoredSession) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(windows))

	for _, win := range windows {
		panes := make([]map[string]interface{}, 0, len(win.Panes))

		for _, pane := range win.Panes {
			paneData := map[string]interface{}{
				"id":              pane.ID,
				"index":           pane.Index,
				"title":           pane.Title,
				"width":           pane.Width,
				"height":          pane.Height,
				"active":          pane.Active,
				"current_command": pane.CurrentCommand,
				"current_path":    pane.CurrentPath,
			}

			// Add agent state if this pane is monitored
			if sess, ok := sessionMap[pane.ID]; ok {
				paneData["agent_state"] = string(sess.CurrentState)
				paneData["agent_id"] = sess.AgentID
			}

			panes = append(panes, paneData)
		}

		result = append(result, map[string]interface{}{
			"index":  win.Index,
			"name":   win.Name,
			"id":     win.ID,
			"active": win.Active,
			"panes":  panes,
		})
	}

	return result
}
