package core

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/lfu/warren/internal/claude"
	"github.com/lfu/warren/internal/tmux"
	"golang.org/x/crypto/ssh"
)

// ConversationService provides unified API for accessing conversation history
type ConversationService struct {
	sessionMapper      *claude.SessionMapper
	conversationReader *claude.ConversationReader
	cache              *conversationCache
	sshClients         map[string]*ssh.Client
	cacheTTL           time.Duration
	mu                 sync.RWMutex

	subMu         sync.Mutex
	subscriptions map[string]*subscription
	pollInterval  time.Duration
}

// subscription tracks an active polling watcher for a single agent.
type subscription struct {
	ch     chan string
	stopCh chan struct{}
	done   chan struct{}
}

// conversationCache caches parsed conversations to avoid re-reading unchanged files
type conversationCache struct {
	entries map[string]*cacheEntry
	mu      sync.RWMutex
}

type cacheEntry struct {
	messages   []*claude.Message
	modTime    time.Time
	expiry     time.Time
	filePath   string
}

// NewConversationService creates a new conversation service with default cache TTL
func NewConversationService() *ConversationService {
	return NewConversationServiceWithTTL(5 * time.Second)
}

// NewConversationServiceWithTTL creates a new conversation service with custom cache TTL.
// If cacheTTL is zero or negative, a default of 5 seconds is used.
func NewConversationServiceWithTTL(cacheTTL time.Duration) *ConversationService {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Second
	}
	return &ConversationService{
		sessionMapper:      claude.NewSessionMapper(),
		conversationReader: claude.NewConversationReader(),
		cache: &conversationCache{
			entries: make(map[string]*cacheEntry),
		},
		sshClients:    make(map[string]*ssh.Client),
		cacheTTL:      cacheTTL,
		subscriptions: make(map[string]*subscription),
		pollInterval:  3 * time.Second,
	}
}

// GetConversationHistory returns the full conversation history for an agent
func (cs *ConversationService) GetConversationHistory(session *AgentSession, server *Server, pane *tmux.Pane) ([]*claude.Message, error) {
	// Check if remote session - use remote reader for both session info and conversation
	if server.Kind == ServerKindRemote {
		sessionID, cwd, err := cs.getRemoteSessionInfo(server, pane)
		if err != nil {
			return nil, err
		}
		return cs.getRemoteConversation(server, sessionID, cwd)
	}

	// Local session - use local session mapper
	sessionID, cwd, err := cs.getSessionInfo(pane)
	if err != nil {
		return nil, err
	}
	return cs.getLocalConversation(sessionID, cwd)
}

// GetRecentMessages returns the N most recent messages for an agent
func (cs *ConversationService) GetRecentMessages(session *AgentSession, server *Server, pane *tmux.Pane, limit int) ([]*claude.Message, error) {
	messages, err := cs.GetConversationHistory(session, server, pane)
	if err != nil {
		return nil, err
	}

	if len(messages) <= limit {
		return messages, nil
	}

	return messages[len(messages)-limit:], nil
}

// GetUserAssistantMessages returns only user and assistant messages (filters out system messages)
func (cs *ConversationService) GetUserAssistantMessages(session *AgentSession, server *Server, pane *tmux.Pane) ([]*claude.Message, error) {
	messages, err := cs.GetConversationHistory(session, server, pane)
	if err != nil {
		return nil, err
	}

	return claude.FilterUserAssistant(messages), nil
}

// SetPollInterval changes the poll interval used by NEW subscriptions.
// Subscriptions already running keep their original interval. Default 3s.
// Intended for tests that need a faster cadence; values <= 0 are ignored.
func (cs *ConversationService) SetPollInterval(d time.Duration) {
	if d <= 0 {
		return
	}
	cs.subMu.Lock()
	defer cs.subMu.Unlock()
	cs.pollInterval = d
}

// SubscribeToUpdates starts a polling goroutine that watches the conversation
// history for the given agent and emits the agent ID on the returned channel
// whenever new messages are detected (either an increased message count or a
// newer last-message timestamp).
//
// The channel is buffered (size 10). Sends are non-blocking; if the buffer is
// full, the update is dropped — consumers that need every event should drain
// the channel promptly.
//
// Returns an error if agentID is already subscribed. Call Unsubscribe(agentID)
// or Close() to stop polling and close the channel.
func (cs *ConversationService) SubscribeToUpdates(
	agentID string,
	session *AgentSession,
	server *Server,
	pane *tmux.Pane,
) (<-chan string, error) {
	if agentID == "" {
		return nil, fmt.Errorf("agentID is required")
	}
	if server == nil {
		return nil, fmt.Errorf("server is required")
	}

	cs.subMu.Lock()
	if _, exists := cs.subscriptions[agentID]; exists {
		cs.subMu.Unlock()
		return nil, fmt.Errorf("agent %q is already subscribed", agentID)
	}
	interval := cs.pollInterval
	sub := &subscription{
		ch:     make(chan string, 10),
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
	cs.subscriptions[agentID] = sub
	cs.subMu.Unlock()

	go cs.pollLoop(agentID, session, server, pane, interval, sub)
	return sub.ch, nil
}

// Unsubscribe stops the polling goroutine for agentID and closes its channel.
// Safe to call multiple times and safe to call for an unknown agentID.
func (cs *ConversationService) Unsubscribe(agentID string) {
	cs.subMu.Lock()
	sub, ok := cs.subscriptions[agentID]
	if ok {
		delete(cs.subscriptions, agentID)
	}
	cs.subMu.Unlock()
	if !ok {
		return
	}
	cs.stopSubscription(sub)
}

// Close stops all active subscriptions and closes their channels. The
// ConversationService remains usable for one-shot reads after Close.
func (cs *ConversationService) Close() {
	cs.subMu.Lock()
	subs := cs.subscriptions
	cs.subscriptions = make(map[string]*subscription)
	cs.subMu.Unlock()
	for _, sub := range subs {
		cs.stopSubscription(sub)
	}
}

func (cs *ConversationService) stopSubscription(sub *subscription) {
	// stopCh may already be closed if the goroutine exited on its own; guard
	// against a double close.
	select {
	case <-sub.stopCh:
	default:
		close(sub.stopCh)
	}
	<-sub.done
	close(sub.ch)
}

// pollLoop is the per-subscription goroutine. It polls the agent's
// conversation history at the configured interval and emits agentID whenever
// the message count or last-message timestamp advances.
func (cs *ConversationService) pollLoop(
	agentID string,
	session *AgentSession,
	server *Server,
	pane *tmux.Pane,
	interval time.Duration,
	sub *subscription,
) {
	defer close(sub.done)

	var lastCount int
	var lastTimestamp time.Time

	// Prime the baseline so the first tick after creation only reports
	// *changes* and does not spuriously fire on initial state.
	if msgs, err := cs.GetConversationHistory(session, server, pane); err == nil {
		lastCount = len(msgs)
		if len(msgs) > 0 {
			lastTimestamp = msgs[len(msgs)-1].Timestamp
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-sub.stopCh:
			return
		case <-ticker.C:
			msgs, err := cs.GetConversationHistory(session, server, pane)
			if err != nil {
				// Transient read errors (file not yet created, SSH blip)
				// shouldn't kill the watcher — just try again next tick.
				continue
			}
			var newTimestamp time.Time
			if len(msgs) > 0 {
				newTimestamp = msgs[len(msgs)-1].Timestamp
			}
			if len(msgs) > lastCount || newTimestamp.After(lastTimestamp) {
				lastCount = len(msgs)
				lastTimestamp = newTimestamp
				select {
				case sub.ch <- agentID:
				default:
					// Channel full; consumer is slow. Drop this notification
					// rather than blocking the poll loop.
				}
			}
		}
	}
}

// getSessionInfo extracts session ID and CWD from pane (local only)
func (cs *ConversationService) getSessionInfo(pane *tmux.Pane) (string, string, error) {
	if pane == nil {
		return "", "", fmt.Errorf("no pane information")
	}

	pid := pane.PID
	if pid == 0 {
		return "", "", fmt.Errorf("no PID available")
	}

	// Get session ID
	sessionID, err := cs.sessionMapper.GetSessionID(pid)
	if err != nil {
		return "", "", fmt.Errorf("failed to get session ID: %w", err)
	}

	// Get CWD
	cwd, err := cs.sessionMapper.GetCWD(pid)
	if err != nil {
		return "", "", fmt.Errorf("failed to get CWD: %w", err)
	}

	return sessionID, cwd, nil
}

// getRemoteSessionInfo extracts session ID and CWD from a remote pane via SSH
func (cs *ConversationService) getRemoteSessionInfo(server *Server, pane *tmux.Pane) (string, string, error) {
	if pane == nil {
		return "", "", fmt.Errorf("no pane information")
	}

	pid := pane.PID
	if pid == 0 {
		return "", "", fmt.Errorf("no PID available")
	}

	cs.mu.RLock()
	client, ok := cs.sshClients[server.Host]
	cs.mu.RUnlock()

	if !ok {
		return "", "", fmt.Errorf("no SSH connection for host %s", server.Host)
	}

	remoteReader := claude.NewRemoteReader(client, server.Host)

	sessionID, err := remoteReader.GetSessionID(pid)
	if err != nil {
		return "", "", fmt.Errorf("failed to get remote session ID: %w", err)
	}

	cwd, err := remoteReader.GetCWD(pid)
	if err != nil {
		return "", "", fmt.Errorf("failed to get remote CWD: %w", err)
	}

	return sessionID, cwd, nil
}

// getLocalConversation reads conversation from local filesystem with caching
func (cs *ConversationService) getLocalConversation(sessionID, cwd string) ([]*claude.Message, error) {
	filePath := cs.conversationReader.GetConversationFile(sessionID, cwd)

	// Check cache
	if messages, ok := cs.cache.get(filePath); ok {
		return messages, nil
	}

	// Read from file
	messages, err := cs.conversationReader.ReadConversation(filePath)
	if err != nil {
		return nil, err
	}

	// Cache the result with configured TTL
	cs.cache.set(filePath, messages, cs.cacheTTL)

	return messages, nil
}

// getRemoteConversation reads conversation from remote server via SSH
func (cs *ConversationService) getRemoteConversation(server *Server, sessionID, cwd string) ([]*claude.Message, error) {
	cs.mu.RLock()
	client, ok := cs.sshClients[server.Host]
	cs.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no SSH connection for host %s", server.Host)
	}

	remoteReader := claude.NewRemoteReader(client, server.Host)
	return remoteReader.ReadConversation(sessionID, cwd)
}

// RegisterSSHClient registers an SSH client for a remote host
func (cs *ConversationService) RegisterSSHClient(host string, client *ssh.Client) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.sshClients[host] = client
}

// Cache methods

func (c *conversationCache) get(filePath string) ([]*claude.Message, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[filePath]
	if !ok {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.expiry) {
		return nil, false
	}

	// Check if file was modified
	info, err := os.Stat(filePath)
	if err != nil || !info.ModTime().Equal(entry.modTime) {
		return nil, false
	}

	return entry.messages, true
}

func (c *conversationCache) set(filePath string, messages []*claude.Message, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	info, err := os.Stat(filePath)
	if err != nil {
		return
	}

	c.entries[filePath] = &cacheEntry{
		messages: messages,
		modTime:  info.ModTime(),
		expiry:   time.Now().Add(ttl),
		filePath: filePath,
	}
}

func (c *conversationCache) invalidate(filePath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, filePath)
}

// ClearCache clears all cached conversations
func (cs *ConversationService) ClearCache() {
	cs.cache.mu.Lock()
	defer cs.cache.mu.Unlock()
	cs.cache.entries = make(map[string]*cacheEntry)
}
