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
	tmuxClient      *tmux.Client
	parser          *parser.ActivityParser
	stateDetector   *state.StateDetector
	eventStore      *events.Store
	notifEngine     *notifications.Engine
	artifactManager *ArtifactProfileManager

	// Configuration
	pollInterval  time.Duration
	minConfidence float64
	registryPath  string
	cacheTTL      time.Duration

	// Session tracking
	sessions        map[string]*MonitoredSession
	sessionRegistry *AgentSessionRegistry
	serverRegistry  *ServerRegistry
	mu              sync.RWMutex

	// Agent discovery
	agentDiscovery    *AgentDiscovery
	discoveryInterval time.Duration
	discoveryEnabled  bool

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// MonitoredSession represents an agent session being monitored.
//
// `PrevState` / `PrevTransitionTime` are used by the anti-flap debounce in
// `Warren.transitionTo` (Phase 2 audit Item #5): a transition `A → B` is
// rejected if the previous transition was `B → A` within the last 2
// seconds. This prevents `executing↔thinking` spam during a long tool call
// where partial captures briefly drop the spinner.
type MonitoredSession struct {
	AgentID            string
	PaneID             string
	CurrentState       AgentState
	LastPollTime       time.Time
	LastContent        string
	ErrorCount         int
	ConsecutiveErrors  int
	ServerName         string
	WorkingDir         string
	LastStateChange    time.Time
	PrevState          AgentState   // state immediately before CurrentState; populated by transitionTo
	PrevTransitionTime time.Time    // when the CurrentState transition occurred (i.e., when PrevState→CurrentState happened)
	TmuxClient         *tmux.Client // Per-session tmux client (local or remote)
}

// Config configures Warren behavior
type Config struct {
	PollInterval           time.Duration
	MinConfidence          float64
	DBPath                 string
	ConfigDir              string
	EventRetentionPeriod   time.Duration // How long to keep events (default: 30 days)
	EventPruningInterval   time.Duration // How often to prune events (default: 24 hours)
	CacheTTL               time.Duration // How long to cache conversation files (default: 5 seconds)
	RegistryPruneThreshold time.Duration // How old sessions must be to prune (default: 24 hours)

	// Agent discovery
	EnableAutoDiscovery bool          // Enable automatic agent discovery
	DiscoveryInterval   time.Duration // How often to run discovery (default: 5 minutes)
}

// DefaultConfig returns sensible defaults.
//
// DBPath and ConfigDir resolve under $HOME/.warren via DefaultDBPath() /
// DefaultConfigDir(). Prior to Phase 2 audit remediation this returned a
// cwd-relative "warren.db", which silently orphaned existing event data
// every time a user launched warren-web from a new directory.
func DefaultConfig() *Config {
	return &Config{
		PollInterval:           500 * time.Millisecond,
		MinConfidence:          0.7,
		DBPath:                 DefaultDBPath(),
		ConfigDir:              DefaultConfigDir(),
		EventRetentionPeriod:   30 * 24 * time.Hour, // 30 days
		EventPruningInterval:   24 * time.Hour,      // daily
		CacheTTL:               5 * time.Second,     // 5 seconds
		RegistryPruneThreshold: 24 * time.Hour,      // 24 hours
		EnableAutoDiscovery:    false,               // opt-in
		DiscoveryInterval:      5 * time.Minute,     // 5 minutes
	}
}

// Validate checks if the configuration is valid and returns an error if not
func (c *Config) Validate() error {
	if c.PollInterval <= 0 {
		return fmt.Errorf("PollInterval must be positive, got %v", c.PollInterval)
	}
	if c.PollInterval < 100*time.Millisecond {
		return fmt.Errorf("PollInterval must be at least 100ms to avoid excessive CPU usage, got %v", c.PollInterval)
	}
	if c.MinConfidence < 0 || c.MinConfidence > 1 {
		return fmt.Errorf("MinConfidence must be between 0 and 1, got %v", c.MinConfidence)
	}
	if c.DBPath == "" {
		return fmt.Errorf("DBPath cannot be empty")
	}
	if c.ConfigDir == "" {
		return fmt.Errorf("ConfigDir cannot be empty")
	}
	if c.EventRetentionPeriod <= 0 {
		return fmt.Errorf("EventRetentionPeriod must be positive, got %v", c.EventRetentionPeriod)
	}
	if c.EventPruningInterval <= 0 {
		return fmt.Errorf("EventPruningInterval must be positive, got %v", c.EventPruningInterval)
	}
	if c.CacheTTL <= 0 {
		return fmt.Errorf("CacheTTL must be positive, got %v", c.CacheTTL)
	}
	if c.CacheTTL > 1*time.Hour {
		return fmt.Errorf("CacheTTL must be at most 1 hour to avoid stale data, got %v", c.CacheTTL)
	}
	if c.RegistryPruneThreshold <= 0 {
		return fmt.Errorf("RegistryPruneThreshold must be positive, got %v", c.RegistryPruneThreshold)
	}
	if c.RegistryPruneThreshold < 1*time.Hour {
		return fmt.Errorf("RegistryPruneThreshold must be at least 1 hour to avoid premature pruning, got %v", c.RegistryPruneThreshold)
	}
	return nil
}

// NewWarren creates a new Warren orchestrator
func NewWarren(config *Config) (*Warren, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Ensure the config directory exists so the event store, server registry,
	// and session registry can all open files under it. Idempotent.
	if err := EnsureConfigDir(config.ConfigDir); err != nil {
		return nil, fmt.Errorf("failed to ensure config dir %s: %w", config.ConfigDir, err)
	}

	// Initialize event store with retention configuration
	storeConfig := &events.StoreConfig{
		DBPath:          config.DBPath,
		RetentionPeriod: config.EventRetentionPeriod,
		PruningInterval: config.EventPruningInterval,
	}
	eventStore, err := events.NewStoreWithConfig(storeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create event store: %w", err)
	}

	// Start background pruning job
	eventStore.StartPruningJob()

	// Initialize components
	tmuxClient := tmux.NewClient(tmux.NewLocalExecutor())
	parser := parser.NewActivityParser()
	stateDetector := state.NewStateDetector()
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
	pruned := sessionRegistry.PruneWithThreshold(config.RegistryPruneThreshold)
	if pruned > 0 {
		fmt.Printf("Pruned %d stale sessions from registry\n", pruned)
	}

	serverRegistry, err := NewServerRegistry(config.ConfigDir)
	if err != nil {
		// Cancel the context we created above so callers do not leak a
		// goroutine waiting on it when NewWarren itself fails.
		cancel()
		return nil, fmt.Errorf("failed to create server registry: %w", err)
	}

	return &Warren{
		tmuxClient:      tmuxClient,
		parser:          parser,
		stateDetector:   stateDetector,
		eventStore:      eventStore,
		notifEngine:     notifEngine,
		artifactManager: artifactManager,
		pollInterval:    config.PollInterval,
		minConfidence:   config.MinConfidence,
		registryPath:    registryPath,
		cacheTTL:        config.CacheTTL,
		sessions:        make(map[string]*MonitoredSession),
		sessionRegistry: sessionRegistry,
		serverRegistry:  serverRegistry,
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
	client := session.TmuxClient
	if client == nil {
		client = w.tmuxClient
	}
	captureResult, err := client.GetRecentContent(session.PaneID, 500)
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

	// Step 4: Detect state from content (primary) and activities (secondary)
	// Content-based detection is more accurate for Claude Code UI patterns
	contentResult := w.stateDetector.DetectFromContent(captureResult.Content)

	// Fall back to activity-based detection if content detection is low confidence
	detectionResult := contentResult
	if contentResult.Confidence < 0.6 {
		recentActivities, err := w.eventStore.GetRecentActivities(agentID, 20)
		if err == nil && len(recentActivities) > 0 {
			activityResult := w.stateDetector.DetectFromActivities(recentActivities)
			if activityResult.Confidence > contentResult.Confidence {
				detectionResult = activityResult
			}
		}
	}

	// Step 5: Check for state transition with debounce.
	// Don't transition from an active state (executing, thinking) to idle
	// within 10 seconds — prevents flickering when polls capture brief
	// moments between tool calls in an active session.
	shouldTransition := detectionResult.State != session.CurrentState && detectionResult.Confidence >= w.minConfidence
	if shouldTransition && detectionResult.State == StateIdle {
		activeStates := map[AgentState]bool{
			StateExecuting: true,
			StateThinking:  true,
		}
		if activeStates[session.CurrentState] && time.Since(session.LastStateChange) < 10*time.Second {
			shouldTransition = false
		}
	}

	if shouldTransition {
		// Build the reason string based on whether the target state is
		// notify-worthy. See Item #5-D (planval): the previous "State
		// transition triggered <X> notification" phrasing overclaimed for
		// non-notify targets; persist-all-transitions requires accurate
		// per-transition reasons.
		reason := w.transitionReason(detectionResult)
		if err := w.transitionTo(agentID, detectionResult.State, reason, detectionResult.Confidence); err != nil {
			return fmt.Errorf("failed to record state transition: %w", err)
		}
		// transitionTo updates CurrentState/LastStateChange; we still need
		// to refresh LastContent/LastPollTime/ConsecutiveErrors.
		w.mu.Lock()
		session.LastContent = captureResult.Content
		session.LastPollTime = time.Now()
		session.ConsecutiveErrors = 0
		w.mu.Unlock()
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

// handlePollError handles errors during polling.
//
// After ≥10 consecutive errors, the session transitions to StateError via
// the same `transitionTo` helper used by the success path — so the error
// transition is persisted as a state-change event with reason
// "consecutive poll errors: N" (Item #5-C, #5-D from the planval brief).
// Phase 2 Item #5 fixed the bug where this path mutated `CurrentState`
// directly and dropped the transition entirely from the events table.
func (w *Warren) handlePollError(agentID string, err error) {
	w.mu.Lock()
	session, exists := w.sessions[agentID]
	if !exists {
		w.mu.Unlock()
		return
	}

	session.ErrorCount++
	session.ConsecutiveErrors++
	consecutive := session.ConsecutiveErrors
	currentState := session.CurrentState
	w.mu.Unlock()

	// After many consecutive errors, mark session as error state.
	// This is recoverable — a successful poll will clear it.
	// We avoid duplicate transitions if we're already in StateError.
	if consecutive >= 10 && currentState != StateError {
		reason := fmt.Sprintf("consecutive poll errors: %d", consecutive)
		// confidence 1.0: the error state is observed directly (we counted
		// the poll failures), not inferred from a regex tier.
		_ = w.transitionTo(agentID, StateError, reason, 1.0)
	}
}

// transitionReason builds the Reason field stored on the StateChangeEvent
// for transitions detected from content/activities (Item #5-D).
//
// Notify-worthy targets carry "notification: <trigger>" so the audit
// trail self-describes the user-visible side effect. Non-notify targets
// carry "state detection (confidence X.XX)" so timeline consumers can
// distinguish "Warren saw a transition the user wouldn't be paged for"
// from "Warren sent a notification."
func (w *Warren) transitionReason(detection *state.DetectionResult) string {
	if trigger, notifyWorthy := notifyWorthyTrigger(detection.State); notifyWorthy {
		return fmt.Sprintf("notification: %s", trigger)
	}
	return fmt.Sprintf("state detection (confidence %.2f)", detection.Confidence)
}

// transitionTo is the ONLY writer of `session.CurrentState` outside session
// construction (Item #5-C). It enforces the anti-flap debounce, persists
// EVERY accepted transition to the event store (not just notify-worthy
// ones — the bug behind Item #5), and forwards notify-worthy transitions
// to the notification engine.
//
// Returns nil even when a transition is dropped by debounce so callers can
// treat "no-op" the same as "applied." Returns a non-nil error only when
// event-store persistence fails.
func (w *Warren) transitionTo(agentID string, newState AgentState, reason string, confidence float64) error {
	w.mu.Lock()
	session, exists := w.sessions[agentID]
	if !exists {
		w.mu.Unlock()
		return fmt.Errorf("session %s not found", agentID)
	}
	oldState := session.CurrentState
	if oldState == newState {
		w.mu.Unlock()
		return nil
	}
	// Anti-flap debounce (Item #5-B): reject A→B if previous transition
	// was B→A within the last 2 seconds. At 500ms poll cadence this
	// corresponds to 4 consecutive poll cycles of evidence — enough to
	// distinguish a real switch from a flicker between captures.
	if session.PrevState == newState && time.Since(session.PrevTransitionTime) < 2*time.Second {
		w.mu.Unlock()
		return nil
	}
	now := time.Now()
	session.PrevState = oldState
	session.PrevTransitionTime = now
	session.CurrentState = newState
	session.LastStateChange = now
	w.mu.Unlock()

	// Persist the state-change event for EVERY accepted transition. This
	// is the core of Item #5 — previously this lived in
	// notifications.Engine.ProcessStateChange and only fired for the 5/9
	// notify-worthy states.
	stateChange := &events.StateChangeEvent{
		AgentID:    agentID,
		FromState:  string(oldState),
		ToState:    string(newState),
		Reason:     reason,
		Timestamp:  now,
		Confidence: confidence,
	}
	if err := w.eventStore.AppendStateChange(stateChange); err != nil {
		return fmt.Errorf("failed to append state change: %w", err)
	}

	// Forward notify-worthy transitions to the notification engine. The
	// engine's own AppendStateChange call has been removed — see
	// notifications/engine.go.
	if _, notifyWorthy := notifyWorthyTrigger(newState); notifyWorthy {
		if err := w.notifEngine.ProcessStateChange(agentID, string(oldState), string(newState)); err != nil {
			// Log via return rather than panic; the state change is
			// already persisted, so the audit trail is intact.
			return fmt.Errorf("failed to process state change notification: %w", err)
		}
	}
	return nil
}

// notifyWorthyTrigger returns the notification trigger string for states
// that should generate a user-visible notification, and `(_, false)` for
// states that are persisted but silent (idle/thinking/executing/unknown).
// Mirrors `notifications.Engine.shouldNotify` — kept here to decouple
// `transitionReason` and `transitionTo` from the engine's internal API.
func notifyWorthyTrigger(s AgentState) (string, bool) {
	switch s {
	case StateWaitingPermission:
		return "permission_required", true
	case StateAskingQuestion:
		return "question_asked", true
	case StateFinished:
		return "finished", true
	case StateError:
		return "error", true
	case StateStopped:
		return "stopped", true
	default:
		return "", false
	}
}

// GetSessionState returns the current state of a session
// CacheTTL returns the configured cache TTL for conversation history.
func (w *Warren) CacheTTL() time.Duration {
	return w.cacheTTL
}

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

// AddSessionWithClient registers an agent session with a specific tmux client
func (w *Warren) AddSessionWithClient(agentID, paneID, serverName, workingDir string, client *tmux.Client) error {
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
		ServerName:   serverName,
		WorkingDir:   workingDir,
		TmuxClient:   client,
	}

	return nil
}

// TmuxClientForServer creates a tmux client for the given server
func TmuxClientForServer(server *Server) *tmux.Client {
	if server.IsLocal() {
		return tmux.NewClient(tmux.NewLocalExecutor())
	}
	port := server.Port
	if port == 0 {
		port = 22
	}
	return tmux.NewClient(tmux.NewRemoteExecutor(server.User, server.Host, port))
}
