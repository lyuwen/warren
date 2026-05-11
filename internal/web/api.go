package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/lfu/warren/internal/core"
)

// handleGetServers returns all registered servers
func (s *Server) handleGetServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// For now, return localhost as the only server
	// In the future, this will query the server registry
	servers := []map[string]interface{}{
		{
			"name":        "localhost",
			"host":        "localhost",
			"agent_count": len(s.warren.GetAllSessions()),
			"status":      "online",
		},
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
