package web

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/lfu/warren/internal/core"
)

//go:embed static/*
var staticFiles embed.FS

// Server is the HTTP server for the Warren web interface
type Server struct {
	warren     *core.Warren
	httpServer *http.Server
	addr       string
	wsHub      *Hub
}

// Config configures the web server
type Config struct {
	Addr   string
	Warren *core.Warren
}

// NewServer creates a new web server
func NewServer(config *Config) *Server {
	if config.Addr == "" {
		config.Addr = ":8080"
	}

	wsHub := NewHub()

	server := &Server{
		warren: config.Warren,
		addr:   config.Addr,
		wsHub:  wsHub,
	}

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/servers", server.handleGetServers)
	mux.HandleFunc("/api/agents", server.handleGetAgents)
	mux.HandleFunc("/api/agents/", server.handleGetAgent)
	mux.HandleFunc("/api/notifications", server.handleGetNotifications)
	mux.HandleFunc("/api/notifications/consume", server.handleConsumeNotification)

	// WebSocket route
	mux.HandleFunc("/ws", server.handleWebSocket)

	// Static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("failed to create static filesystem: %v", err))
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	server.httpServer = &http.Server{
		Addr:         config.Addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server
}

// Start starts the web server and WebSocket hub
func (s *Server) Start() error {
	// Start WebSocket hub
	go s.wsHub.Run()

	// Start state change monitor
	go s.monitorStateChanges()

	// Start HTTP server
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	fmt.Printf("Warren web interface started on %s\n", s.addr)
	return nil
}

// Stop gracefully stops the web server
func (s *Server) Stop(ctx context.Context) error {
	s.wsHub.Stop()
	return s.httpServer.Shutdown(ctx)
}

// monitorStateChanges watches for state changes and broadcasts to WebSocket clients
func (s *Server) monitorStateChanges() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastStates := make(map[string]string)

	for range ticker.C {
		sessions := s.warren.GetAllSessions()

		for _, session := range sessions {
			currentState := string(session.CurrentState)
			lastState, exists := lastStates[session.AgentID]

			if !exists || lastState != currentState {
				// State changed, broadcast to WebSocket clients
				s.wsHub.Broadcast(StateChangeMessage{
					Type:      "state_change",
					AgentID:   session.AgentID,
					FromState: lastState,
					ToState:   currentState,
					Timestamp: time.Now(),
				})

				lastStates[session.AgentID] = currentState
			}
		}

		// Also check for new notifications
		notifications, err := s.warren.GetUnconsumedNotifications()
		if err == nil && len(notifications) > 0 {
			s.wsHub.Broadcast(NotificationMessage{
				Type:          "notification",
				Count:         len(notifications),
				Notifications: notifications,
				Timestamp:     time.Now(),
			})
		}
	}
}

// StateChangeMessage is sent when an agent's state changes
type StateChangeMessage struct {
	Type      string    `json:"type"`
	AgentID   string    `json:"agent_id"`
	FromState string    `json:"from_state"`
	ToState   string    `json:"to_state"`
	Timestamp time.Time `json:"timestamp"`
}

// NotificationMessage is sent when new notifications arrive
type NotificationMessage struct {
	Type          string      `json:"type"`
	Count         int         `json:"count"`
	Notifications interface{} `json:"notifications"`
	Timestamp     time.Time   `json:"timestamp"`
}
