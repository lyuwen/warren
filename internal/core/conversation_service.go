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

// NewConversationServiceWithTTL creates a new conversation service with custom cache TTL
func NewConversationServiceWithTTL(cacheTTL time.Duration) *ConversationService {
	return &ConversationService{
		sessionMapper:      claude.NewSessionMapper(),
		conversationReader: claude.NewConversationReader(),
		cache: &conversationCache{
			entries: make(map[string]*cacheEntry),
		},
		sshClients: make(map[string]*ssh.Client),
		cacheTTL:   cacheTTL,
	}
}

// GetConversationHistory returns the full conversation history for an agent
func (cs *ConversationService) GetConversationHistory(session *AgentSession, server *Server, pane *tmux.Pane) ([]*claude.Message, error) {
	// Get session ID and CWD
	sessionID, cwd, err := cs.getSessionInfo(pane)
	if err != nil {
		return nil, err
	}

	// Check if remote session
	if server.Kind == ServerKindRemote {
		return cs.getRemoteConversation(server, sessionID, cwd)
	}

	// Local session
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

// SubscribeToUpdates returns a channel that receives updates when conversation changes
// The channel receives the agent ID when new messages are detected
func (cs *ConversationService) SubscribeToUpdates(agentID string) <-chan string {
	ch := make(chan string, 10)
	// TODO: Implement file watching or polling
	// For now, return empty channel
	return ch
}

// getSessionInfo extracts session ID and CWD from pane
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
