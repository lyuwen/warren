package core

// RegisterAgentSession registers an agent session in the AgentSessionRegistry
func (w *Warren) RegisterAgentSession(session *AgentSession) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.sessionRegistry == nil {
		w.sessionRegistry = NewAgentSessionRegistry()
	}

	return w.sessionRegistry.Register(session)
}

// GetAgentSessionRegistry returns the agent session registry
func (w *Warren) GetAgentSessionRegistry() *AgentSessionRegistry {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.sessionRegistry
}
