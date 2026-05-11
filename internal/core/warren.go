package core

import (
	"context"
	"fmt"
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
	tmuxClient      *tmux.Client
	parser          *parser.ActivityParser
	stateDetector   *state.StateDetector
	eventStore      *events.Store
	notifEngine     *notifications.Engine
	artifactManager *ArtifactProfileManager

	// Configuration
	pollInterval time.Duration
	minConfidence float64

	// Session tracking
	sessions map[string]*MonitoredSession
	mu       sync.RWMutex

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
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	return &Config{
		PollInterval:  500 * time.Millisecond,
		MinConfidence: 0.7,
		DBPath:        "warren.db",
	}
}

// NewWarren creates a new Warren orchestrator
func NewWarren(config *Config) (*Warren, error) {
	if config == nil {
		config = DefaultConfig()
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
	notifEngine := notifications.NewEngine(eventStore)
	artifactManager := NewArtifactProfileManager()

	ctx, cancel := context.WithCancel(context.Background())

	return &Warren{
		tmuxClient:      tmuxClient,
		parser:          parser,
		stateDetector:   stateDetector,
		eventStore:      eventStore,
		notifEngine:     notifEngine,
		artifactManager: artifactManager,
		pollInterval:    config.PollInterval,
		minConfidence:   config.MinConfidence,
		sessions:        make(map[string]*MonitoredSession),
		ctx:             ctx,
		cancel:          cancel,
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
	w.mu.RLock()
	session, exists := w.sessions[agentID]
	w.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", agentID)
	}

	// Step 1: Capture pane content
	captureResult, err := w.tmuxClient.GetRecentContent(session.PaneID, 500)
	if err != nil {
		return fmt.Errorf("failed to capture pane: %w", err)
	}

	// Skip if content hasn't changed
	if captureResult.Content == session.LastContent {
		return nil
	}

	// Step 2: Parse activities
	parseResult, err := w.parser.Parse(agentID, captureResult.Content)
	if err != nil {
		return fmt.Errorf("failed to parse content: %w", err)
	}

	// Step 3: Store activities
	for _, activity := range parseResult.Activities {
		if err := w.eventStore.AppendActivity(activity); err != nil {
			return fmt.Errorf("failed to store activity: %w", err)
		}

		// Update artifact profile
		if err := w.artifactManager.ProcessActivity(activity); err != nil {
			// Log but don't fail on artifact processing errors
			continue
		}
	}

	// Step 4: Detect state from activities
	recentActivities, err := w.eventStore.GetRecentActivities(agentID, 20)
	if err != nil {
		return fmt.Errorf("failed to get recent activities: %w", err)
	}

	detectionResult := w.stateDetector.DetectFromActivities(recentActivities)

	// Step 5: Check for state transition
	if w.stateDetector.ShouldTransition(session.CurrentState, detectionResult, w.minConfidence) {
		oldState := session.CurrentState
		newState := detectionResult.State

		// Update session state
		w.mu.Lock()
		session.CurrentState = newState
		session.LastContent = captureResult.Content
		session.LastPollTime = time.Now()
		session.ConsecutiveErrors = 0
		w.mu.Unlock()

		// Process state change through notification engine (convert to string)
		if err := w.notifEngine.ProcessStateChange(agentID, string(oldState), string(newState)); err != nil {
			return fmt.Errorf("failed to process state change: %w", err)
		}
	} else {
		// No state transition, just update metadata
		w.mu.Lock()
		session.LastContent = captureResult.Content
		session.LastPollTime = time.Now()
		session.ConsecutiveErrors = 0
		w.mu.Unlock()
	}

	return nil
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
