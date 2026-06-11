package web

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lfu/warren/internal/core"
)

//go:embed static/*
var staticFiles embed.FS

// DefaultBindAddr is the loopback-only address Warren binds by default. It
// mirrors the SSH host-key strict-by-default posture from Batch 1: surface
// access must be explicit rather than network-wide.
const DefaultBindAddr = "127.0.0.1:8080"

// EnvBindAddr is the environment variable override for the bind address.
// The CLI --bind flag takes precedence over this variable.
const EnvBindAddr = "WARREN_WEB_BIND"

// EnvCORSOrigins is the environment variable override for the CORS allow-list.
// Value is a comma-separated list of origins (e.g. "https://app.example.com").
// Entries are added to (not replacing) the default loopback origins.
const EnvCORSOrigins = "WARREN_WEB_CORS_ORIGINS"

// defaultCORSOrigins is the strict default CORS allow-list. Only loopback
// origins are permitted unless the operator overrides via WARREN_WEB_CORS_ORIGINS.
var defaultCORSOrigins = []string{
	"http://localhost:8080",
	"http://127.0.0.1:8080",
}

// Server is the HTTP server for the Warren web interface
type Server struct {
	warren              *core.Warren
	conversationService *core.ConversationService
	httpServer          *http.Server
	addr                string
	wsHub               *Hub
}

// Config configures the web server
type Config struct {
	Addr   string
	Warren *core.Warren
	// CORSOrigins, if non-nil, overrides the env-resolved allow-list. Mainly
	// used by tests; production callers should rely on WARREN_WEB_CORS_ORIGINS.
	CORSOrigins []string
}

// NewServer creates a new web server
func NewServer(config *Config) *Server {
	if config.Addr == "" {
		config.Addr = DefaultBindAddr
	}

	wsHub := NewHub()

	server := &Server{
		warren:              config.Warren,
		conversationService: core.NewConversationServiceWithTTL(config.Warren.CacheTTL()),
		addr:                config.Addr,
		wsHub:               wsHub,
	}

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/servers", server.handleGetServers)
	mux.HandleFunc("/api/agents", server.handleGetAgents)
	mux.HandleFunc("/api/agents/", server.handleGetAgent)
	mux.HandleFunc("/api/notifications", server.handleGetNotifications)
	mux.HandleFunc("/api/notifications/consume", server.handleConsumeNotification)
	mux.HandleFunc("/api/notifications/clear", server.handleClearNotifications)

	// Topology routes
	mux.HandleFunc("/api/topology", server.handleGetTopology)
	mux.HandleFunc("/api/topology/servers/", server.handleGetTopologyServer)
	mux.HandleFunc("/api/topology/sessions/", server.handleGetTopologySession)

	// Conversation route
	mux.HandleFunc("/api/conversation/", server.handleGetConversation)

	// WebSocket route
	mux.HandleFunc("/ws", server.handleWebSocket)

	// Static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("failed to create static filesystem: %v", err))
	}
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	// Wrap the mux in CORS middleware. The allow-list is loopback-only by
	// default; WARREN_WEB_CORS_ORIGINS adds extra origins (e.g. when the UI
	// is served from a different host). If the operator bound to a non-default
	// loopback port, expand the allow-list to include that port too so the
	// bundled UI keeps working without extra env wiring.
	origins := config.CORSOrigins
	if origins == nil {
		origins = resolveCORSOrigins(os.Getenv(EnvCORSOrigins))
		origins = append(origins, originHostsForBind(config.Addr)...)
	}
	handler := corsMiddleware(origins, mux)

	server.httpServer = &http.Server{
		Addr:         config.Addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server
}

// resolveCORSOrigins returns the default loopback allow-list plus any extra
// origins supplied via WARREN_WEB_CORS_ORIGINS (comma-separated). Whitespace
// is trimmed; empty entries are skipped.
func resolveCORSOrigins(envValue string) []string {
	origins := append([]string{}, defaultCORSOrigins...)
	if envValue == "" {
		return origins
	}
	for _, o := range strings.Split(envValue, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

// corsMiddleware enforces a strict allow-list for cross-origin requests.
//
// Same-origin requests (no Origin header) pass through untouched. Requests
// whose Origin appears in the allow-list receive the matching CORS headers.
// All other cross-origin requests proceed without CORS headers — the browser
// will block the response. CORS preflights (OPTIONS) from in-policy origins
// short-circuit with 204 No Content.
func corsMiddleware(allowedOrigins []string, next http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := allowed[origin]; ok {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Vary", "Origin")
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			}
		}
		if r.Method == http.MethodOptions {
			// Preflight: respond 204 only when in-policy (Allow-Origin set).
			// Out-of-policy preflights fall through to next so the handler's
			// usual method-not-allowed response is preserved.
			if w.Header().Get("Access-Control-Allow-Origin") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isLoopbackBind reports whether addr binds exclusively to a loopback
// interface. Empty host (":8080"), "0.0.0.0", and any non-loopback address
// return false so the caller can warn appropriately.
func isLoopbackBind(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// If we can't parse it, treat as non-loopback to err on the safe
		// side of WARNing.
		return false
	}
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// originHostsForBind returns the loopback-style origins implied by a custom
// bind port — used by NewServer to keep the default CORS allow-list useful
// when the operator binds to a non-default loopback port. Returns nil for
// the default port (already covered by defaultCORSOrigins) and for any
// non-loopback host (operator must opt in via WARREN_WEB_CORS_ORIGINS).
func originHostsForBind(addr string) []string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return nil
	}
	// Skip when the bind port matches the default — defaultCORSOrigins
	// already covers it. Derive the default port from DefaultBindAddr so a
	// future change to that constant doesn't leave this early-out stale.
	if _, defaultPort, derr := net.SplitHostPort(DefaultBindAddr); derr == nil && port == defaultPort {
		return nil
	}
	// Only auto-expand for loopback binds; non-loopback callers must opt in
	// via WARREN_WEB_CORS_ORIGINS explicitly.
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return []string{"http://localhost:" + port, "http://127.0.0.1:" + port}
	default:
		return nil
	}
}

// Start starts the web server and WebSocket hub
func (s *Server) Start() error {
	// Start WebSocket hub
	go s.wsHub.Run()

	// Start state change monitor
	go s.monitorStateChanges()

	// WARN if the operator opted out of loopback-only binding — this exposes
	// the REST API on all matching network interfaces and relies entirely on
	// the CORS allow-list and network reachability for protection.
	if !isLoopbackBind(s.addr) {
		log.Printf("WARN: warren-web bound to non-loopback address %q; REST API is reachable from the network. Ensure WARREN_WEB_CORS_ORIGINS is set and any reverse-proxy auth is in place.", s.addr)
	}

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
