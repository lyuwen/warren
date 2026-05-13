package events

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// EventType represents the type of event
type EventType string

const (
	EventTypeActivity     EventType = "activity"
	EventTypeNotification EventType = "notification"
	EventTypeStateChange  EventType = "state_change"
)

// Event is the base event structure
type Event struct {
	ID        int64     `json:"id"`
	Type      EventType `json:"type"`
	AgentID   string    `json:"agent_id"`
	Timestamp time.Time `json:"timestamp"`
	Data      string    `json:"data"` // JSON-encoded event-specific data
}

// UnmarshalData unmarshals the event data into the provided struct
func (e *Event) UnmarshalData(v interface{}) error {
	return json.Unmarshal([]byte(e.Data), v)
}

// AgentActivityEvent represents an activity performed by an agent
type AgentActivityEvent struct {
	AgentID      string            `json:"agent_id"`
	ActivityType string            `json:"activity_type"` // "chat", "file", "tool", "prompt"
	Content      string            `json:"content"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
}

// NotificationEvent represents a notification that requires user attention
type NotificationEvent struct {
	AgentID      string            `json:"agent_id"`
	NotifType    string            `json:"notif_type"` // "permission_required", "question_asked", "finished", "error"
	Message      string            `json:"message"`
	Consumed     bool              `json:"consumed"`
	ConsumedAt   *time.Time        `json:"consumed_at,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
}

// StateChangeEvent represents a state transition
type StateChangeEvent struct {
	AgentID   string    `json:"agent_id"`
	FromState string    `json:"from_state"`
	ToState   string    `json:"to_state"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Store manages event persistence
type Store struct {
	db              *sql.DB
	pruningInterval time.Duration
	retentionPeriod time.Duration
	stopPruning     chan struct{}
}

// StoreConfig configures the event store
type StoreConfig struct {
	DBPath          string
	RetentionPeriod time.Duration // How long to keep events (default: 30 days)
	PruningInterval time.Duration // How often to run pruning (default: 24 hours)
}

// DefaultStoreConfig returns sensible defaults
func DefaultStoreConfig() *StoreConfig {
	return &StoreConfig{
		DBPath:          "warren.db",
		RetentionPeriod: 30 * 24 * time.Hour, // 30 days
		PruningInterval: 24 * time.Hour,      // daily
	}
}

// NewStore creates a new event store
func NewStore(dbPath string) (*Store, error) {
	config := DefaultStoreConfig()
	config.DBPath = dbPath
	return NewStoreWithConfig(config)
}

// NewStoreWithConfig creates a new event store with custom configuration
func NewStoreWithConfig(config *StoreConfig) (*Store, error) {
	if config == nil {
		config = DefaultStoreConfig()
	}

	// Validate configuration
	if config.RetentionPeriod <= 0 {
		config.RetentionPeriod = 30 * 24 * time.Hour
	}
	if config.PruningInterval <= 0 {
		config.PruningInterval = 24 * time.Hour
	}

	db, err := sql.Open("sqlite3", config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &Store{
		db:              db,
		retentionPeriod: config.RetentionPeriod,
		pruningInterval: config.PruningInterval,
		stopPruning:     make(chan struct{}),
	}

	if err := store.initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// initialize creates the database schema
func (s *Store) initialize() error {
	schema := `
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL,
		agent_id TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		data TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_events_agent_id ON events(agent_id);
	CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_agent_type ON events(agent_id, type);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

// Close closes the database connection
func (s *Store) Close() error {
	// Stop pruning job if running
	close(s.stopPruning)
	return s.db.Close()
}

// AppendActivity appends an activity event
func (s *Store) AppendActivity(event *AgentActivityEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = s.db.Exec(
		"INSERT INTO events (type, agent_id, timestamp, data) VALUES (?, ?, ?, ?)",
		EventTypeActivity,
		event.AgentID,
		event.Timestamp,
		string(data),
	)

	if err != nil {
		return fmt.Errorf("failed to insert activity event: %w", err)
	}

	return nil
}

// AppendNotification appends a notification event
func (s *Store) AppendNotification(event *NotificationEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = s.db.Exec(
		"INSERT INTO events (type, agent_id, timestamp, data) VALUES (?, ?, ?, ?)",
		EventTypeNotification,
		event.AgentID,
		event.Timestamp,
		string(data),
	)

	if err != nil {
		return fmt.Errorf("failed to insert notification event: %w", err)
	}

	return nil
}

// AppendStateChange appends a state change event
func (s *Store) AppendStateChange(event *StateChangeEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = s.db.Exec(
		"INSERT INTO events (type, agent_id, timestamp, data) VALUES (?, ?, ?, ?)",
		EventTypeStateChange,
		event.AgentID,
		event.Timestamp,
		string(data),
	)

	if err != nil {
		return fmt.Errorf("failed to insert state change event: %w", err)
	}

	return nil
}

// QueryOptions configures event queries
type QueryOptions struct {
	AgentID   string
	EventType EventType
	Since     *time.Time
	Until     *time.Time
	Limit     int
	Offset    int
}

// Query retrieves events based on options
func (s *Store) Query(opts QueryOptions) ([]*Event, error) {
	query := "SELECT id, type, agent_id, timestamp, data FROM events WHERE 1=1"
	args := []interface{}{}

	if opts.AgentID != "" {
		query += " AND agent_id = ?"
		args = append(args, opts.AgentID)
	}

	if opts.EventType != "" {
		query += " AND type = ?"
		args = append(args, opts.EventType)
	}

	if opts.Since != nil {
		query += " AND timestamp >= ?"
		args = append(args, opts.Since)
	}

	if opts.Until != nil {
		query += " AND timestamp <= ?"
		args = append(args, opts.Until)
	}

	query += " ORDER BY timestamp DESC"

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	if opts.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, opts.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	events := []*Event{}
	for rows.Next() {
		var event Event
		if err := rows.Scan(&event.ID, &event.Type, &event.AgentID, &event.Timestamp, &event.Data); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, &event)
	}

	return events, nil
}

// GetRecentActivities retrieves recent activity events for an agent
func (s *Store) GetRecentActivities(agentID string, limit int) ([]*AgentActivityEvent, error) {
	events, err := s.Query(QueryOptions{
		AgentID:   agentID,
		EventType: EventTypeActivity,
		Limit:     limit,
	})

	if err != nil {
		return nil, err
	}

	activities := make([]*AgentActivityEvent, 0, len(events))
	for _, event := range events {
		var activity AgentActivityEvent
		if err := json.Unmarshal([]byte(event.Data), &activity); err != nil {
			continue // Skip malformed events
		}
		activities = append(activities, &activity)
	}

	return activities, nil
}

// GetUnconsumedNotifications retrieves unconsumed notifications
func (s *Store) GetUnconsumedNotifications() ([]*NotificationEvent, error) {
	events, err := s.Query(QueryOptions{
		EventType: EventTypeNotification,
		Limit:     1000, // Reasonable limit
	})

	if err != nil {
		return nil, err
	}

	// Group notifications by (agentID, notifType, timestamp) to handle consumed versions
	// Key: agentID + "|" + notifType + "|" + timestamp
	notifMap := make(map[string]*NotificationEvent)

	for _, event := range events {
		var notif NotificationEvent
		if err := json.Unmarshal([]byte(event.Data), &notif); err != nil {
			continue
		}

		key := fmt.Sprintf("%s|%s|%d", notif.AgentID, notif.NotifType, notif.Timestamp.Unix())

		// Keep the latest version (consumed versions are appended later)
		if _, exists := notifMap[key]; !exists {
			notifMap[key] = &notif
		} else if notif.Consumed {
			// If we see a consumed version, replace the unconsumed one
			notifMap[key] = &notif
		}
	}

	// Return only unconsumed notifications
	notifications := make([]*NotificationEvent, 0)
	for _, notif := range notifMap {
		if !notif.Consumed {
			notifications = append(notifications, notif)
		}
	}

	return notifications, nil
}

// CleanupOldEvents removes events older than the retention period
func (s *Store) CleanupOldEvents(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	result, err := s.db.Exec("DELETE FROM events WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old events: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// Count returns the total number of events
func (s *Store) Count() (int64, error) {
	var count int64
	err := s.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count events: %w", err)
	}
	return count, nil
}

// PruneOldEvents removes events older than the configured retention period
// Returns the number of events deleted
func (s *Store) PruneOldEvents(olderThan time.Duration) (int, error) {
	cutoff := time.Now().Add(-olderThan)

	result, err := s.db.Exec("DELETE FROM events WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to prune old events: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

// StartPruningJob starts a background goroutine that periodically prunes old events
// The job runs at the configured pruning interval and deletes events older than the retention period
func (s *Store) StartPruningJob() {
	go func() {
		ticker := time.NewTicker(s.pruningInterval)
		defer ticker.Stop()

		// Run initial pruning immediately
		s.runPruning()

		for {
			select {
			case <-ticker.C:
				s.runPruning()
			case <-s.stopPruning:
				return
			}
		}
	}()
}

// runPruning executes a single pruning cycle with logging
func (s *Store) runPruning() {
	startTime := time.Now()

	deleted, err := s.PruneOldEvents(s.retentionPeriod)
	if err != nil {
		fmt.Printf("[EventStore] Pruning failed: %v\n", err)
		return
	}

	duration := time.Since(startTime)

	if deleted > 0 {
		fmt.Printf("[EventStore] Pruned %d events older than %v (took %v)\n",
			deleted, s.retentionPeriod, duration)
	}
}
