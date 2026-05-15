package core

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/lfu/warren/internal/events"
	"github.com/lfu/warren/internal/notifications"
	"github.com/lfu/warren/internal/parser"
	"github.com/lfu/warren/internal/state"
	"github.com/lfu/warren/internal/tmux"
)

// Warren is the main orchestrator that coordinates monitoring of agent sessions
type Warren struct {
	// Core components
	tmuxClient           *tmux.Client
	parser               *parser.ActivityParser
	stateDetector        *state.StateDetector
	conversationDetector *state.ConversationDetector
	conversationService  *ConversationService
	eventStore           *events.Store
	notifEngine          *notifications.Engine
	artifactManager      *ArtifactProfileManager

	// Configuration
	pollInterval  time.Duration
	minConfidence float64
	registryPath  string

	// Session tracking
	sessions        map[string]*MonitoredSession
	sessionRegistry *AgentSessionRegistry
	serverRegistry  *ServerRegistry
	mu              sync.RWMutex

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// MonitoredSession represents an agent session being monitored
type MonitoredSession struct {
	AgentID       string
	PaneID        string
	CurrentState  AgentState
	LastPollTime  time.Time
	LastContent   string
	ErrorCount    int
	ConsecutiveErrors int
}

// Config configures Warren behavior
type Config struct {
	PollInterval  time.Duration
	MinConfidence float64
	DBPath        string
	ConfigDir     string
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		PollInterval:  500 * time.Millisecond,
		MinConfidence: 0.7,
		DBPath:        "warren.db",
		ConfigDir:     ".warren",
	}
}

// NewWarren creates a new Warren orchestrator
func NewWarren(config *Config) (*Warren, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Set default ConfigDir if not provided
	if config.ConfigDir == "" {
		config.ConfigDir = ".warren"
	}

	// Initialize event store
	eventStore, err := events.NewStore(config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create event store: %w", err)
	}

	// Initialize components
	tmuxClient := tmux.NewClient(tmux.NewLocalExecutor())
	parser := parser.NewActivityParser()
	stateDetector := state.NewStateDetector()
	conversationDetector := state.NewConversationDetector()
	conversationService := NewConversationService()
	notifEngine := notifications.NewEngine(eventStore)
	artifactManager := NewArtifactProfileManager()

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize registries
	sessionRegistry := NewAgentSessionRegistry()

	// Load persisted registry from disk
	registryPath := filepath.Join(config.ConfigDir, "registry.json")
	if err := sessionRegistry.Load(registryPath); err != nil {
		// Log warning but don't fail - we can continue with empty registry
		fmt.Printf("Warning: failed to load registry from %s: %v\n", registryPath, err)
	}

	// Prune stale sessions
	pruned := sessionRegistry.Prune()
	if pruned > 0 {
		fmt.Printf("Pruned %d stale sessions from registry\n", pruned)
	}

	serverRegistry, err := NewServerRegistry(config.ConfigDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create server registry: %w", err)
	}

	return &Warren{
		tmuxClient:           tmuxClient,
		parser:               parser,
		stateDetector:        stateDetector,
		conversationDetector: conversationDetector,
		conversationService:  conversationService,
		eventStore:           eventStore,
		notifEngine:          notifEngine,
		artifactManager:      artifactManager,
		pollInterval:         config.PollInterval,
		minConfidence:        config.MinConfidence,
		registryPath:         registryPath,
		sessions:             make(map[string]*MonitoredSession),
		sessionRegistry:      sessionRegistry,
		serverRegistry:       serverRegistry,
		ctx:                  ctx,
		cancel:               cancel,
	}, nil
}

// AddSession registers an agent session for monitoring
func (w *Warren) AddSession(agentID, paneID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.sessions[agentID]; exists {
		return fmt.Errorf("session %s already registered", agentID)
	}

	w.sessions[agentID] = &MonitoredSession{
		AgentID:      agentID,
		PaneID:       paneID,
		CurrentState: StateUnknown,
		LastPollTime: time.Now(),
	}

	return nil
}

// RemoveSession unregisters an agent session
func (w *Warren) RemoveSession(agentID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.sessions[agentID]; !exists {
		return fmt.Errorf("session %s not found", agentID)
	}

	delete(w.sessions, agentID)
	return nil
}

// Start begins monitoring all registered sessions
func (w *Warren) Start() error {
	w.mu.RLock()
	sessionCount := len(w.sessions)
	w.mu.RUnlock()

	if sessionCount == 0 {
		return fmt.Errorf("no sessions registered")
	}

	// Start monitoring loop for each session
	w.mu.RLock()
	for agentID := range w.sessions {
		w.wg.Add(1)
		go w.monitorSession(agentID)
	}
	w.mu.RUnlock()

	return nil
}

// Stop gracefully stops all monitoring
func (w *Warren) Stop() error {
	w.cancel()
	w.wg.Wait()
	w.notifEngine.Close()
	return w.eventStore.Close()
}

// monitorSession is the main monitoring loop for a single session
func (w *Warren) monitorSession(agentID string) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			if err := w.pollSession(agentID); err != nil {
				w.handlePollError(agentID, err)
			}
		}
	}
}

// pollSession performs a single poll cycle for a session
func (w *Warren) pollSession(agentID string) error {
	// Get session from registry
	session, err := w.sessionRegistry.Get(agentID)
	if err != nil {
		return fmt.Errorf("session %s not found: %w", agentID, err)
	}

	// Get server info
	server, err := w.GetServer(session.ServerName)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	// Get pane info
	pane, err := w.GetPane(session, server)
	if err != nil {
		return fmt.Errorf("failed to get pane: %w", err)
	}

	// Create appropriate tmux client for this server
	var tmuxClient *tmux.Client
	if server.IsLocal() {
		tmuxClient = w.tmuxClient
	} else {
		if server.Port == 0 {
			server.Port = 22
		}
		tmuxClient = tmux.NewClient(tmux.NewRemoteExecutor(server.User, server.Host, server.Port))
	}

	// Capture pane content for real-time state detection
	var paneContent string
	captureResult, err := tmuxClient.GetRecentContent(session.TmuxPaneID, 100)
	if err == nil {
		paneContent = captureResult.Content
	}

	// Get conversation history (last 10 messages should be enough)
	messages, err := w.conversationService.GetRecentMessages(session, server, pane, 10)
	if err != nil {
		// If we can't get conversation history, fall back to pane-only detection
		if paneContent != "" {
			detectionResult := w.conversationDetector.DetectFromConversationAndPane(nil, paneContent)
			w.updateSessionStateIfChanged(agentID, detectionResult.State, detectionResult.Confidence)
		} else {
			w.updateSessionState(agentID, StateUnknown, 0.3)
		}
		return nil
	}

	// Detect state from conversation + pane content
	detectionResult := w.conversationDetector.DetectFromConversationAndPane(messages, paneContent)

	// Update state if confidence is high enough
	w.updateSessionStateIfChanged(agentID, detectionResult.State, detectionResult.Confidence)

	return nil
}

// updateSessionStateIfChanged updates session state if it changed and confidence is high enough
func (w *Warren) updateSessionStateIfChanged(agentID string, newState AgentState, confidence float64) {
	w.mu.RLock()
	oldSession, exists := w.sessions[agentID]
	w.mu.RUnlock()

	if !exists {
		// Create monitored session if it doesn't exist
		w.mu.Lock()
		w.sessions[agentID] = &MonitoredSession{
			AgentID:      agentID,
			PaneID:       "",
			CurrentState: newState,
			LastPollTime: time.Now(),
		}
		w.mu.Unlock()

		// Update registry
		w.sessionRegistry.UpdateState(agentID, newState)
		return
	}

	oldState := oldSession.CurrentState

	// Update state if it changed and confidence is high enough
	if oldState != newState && confidence >= w.minConfidence {
		// Update session state
		w.mu.Lock()
		oldSession.CurrentState = newState
		oldSession.LastPollTime = time.Now()
		oldSession.ConsecutiveErrors = 0
		w.mu.Unlock()

		// Update registry
		if err := w.sessionRegistry.UpdateState(agentID, newState); err != nil {
			// Log but don't fail
			return
		}

		// Process state change through notification engine
		if err := w.notifEngine.ProcessStateChange(agentID, string(oldState), string(newState)); err != nil {
			// Log but don't fail
			return
		}
	} else {
		// No state transition, just update metadata
		w.mu.Lock()
		oldSession.LastPollTime = time.Now()
		oldSession.ConsecutiveErrors = 0
		w.mu.Unlock()
	}
}

// updateSessionState is a helper to update session state
func (w *Warren) updateSessionState(agentID string, state AgentState, confidence float64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if session, exists := w.sessions[agentID]; exists {
		session.CurrentState = state
		session.LastPollTime = time.Now()
	}
}

// handlePollError handles errors during polling
func (w *Warren) handlePollError(agentID string, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	session, exists := w.sessions[agentID]
	if !exists {
		return
	}

	session.ErrorCount++
	session.ConsecutiveErrors++

	// If too many consecutive errors, mark session as error state
	if session.ConsecutiveErrors >= 5 {
		oldState := session.CurrentState
		session.CurrentState = StateError

		// Emit error notification (convert to string)
		w.notifEngine.ProcessStateChange(agentID, string(oldState), string(StateError))
	}
}

// GetSessionState returns the current state of a session
func (w *Warren) GetSessionState(agentID string) (AgentState, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	session, exists := w.sessions[agentID]
	if !exists {
		return StateUnknown, fmt.Errorf("session %s not found", agentID)
	}

	return session.CurrentState, nil
}

// GetAllSessions returns all monitored sessions in sorted order
func (w *Warren) GetAllSessions() []*MonitoredSession {
	w.mu.RLock()
	defer w.mu.RUnlock()

	sessions := make([]*MonitoredSession, 0, len(w.sessions))
	for _, session := range w.sessions {
		sessions = append(sessions, session)
	}

	// Sort for stable, predictable ordering
	sort.Slice(sessions, func(i, j int) bool {
		// Primary: Agent ID (which includes server:session:window.pane)
		return sessions[i].AgentID < sessions[j].AgentID
	})

	return sessions
}

// GetUnconsumedNotifications returns all unconsumed notifications
func (w *Warren) GetUnconsumedNotifications() ([]*events.NotificationEvent, error) {
	return w.notifEngine.GetUnconsumedNotifications()
}

// GetArtifactProfile returns the artifact profile for an agent
func (w *Warren) GetArtifactProfile(agentID string) (*ArtifactProfile, error) {
	return w.artifactManager.GetProfile(agentID)
}

// GetEventStore returns the event store for direct queries
func (w *Warren) GetEventStore() *events.Store {
	return w.eventStore
}

// GetNotificationEngine returns the notification engine
func (w *Warren) GetNotificationEngine() *notifications.Engine {
	return w.notifEngine
}

// GetTmuxClient returns the tmux client
func (w *Warren) GetTmuxClient() *tmux.Client {
	return w.tmuxClient
}

// GetServerRegistry returns the server registry
func (w *Warren) GetServerRegistry() *ServerRegistry {
	return w.serverRegistry
}

