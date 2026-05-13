package core

// RegisterAgentSession registers an agent session in the AgentSessionRegistry
func (w *Warren) RegisterAgentSession(session *AgentSession) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.sessionRegistry == nil {
		w.sessionRegistry = NewAgentSessionRegistry()
	}

	if err := w.sessionRegistry.Register(session); err != nil {
		return err
	}

	// Auto-save registry after registration
	if w.registryPath != "" {
		if err := w.sessionRegistry.Save(w.registryPath); err != nil {
			// Log warning but don't fail
			// TODO: Add proper logging
		}
	}

	return nil
}

// GetAgentSessionRegistry returns the agent session registry
func (w *Warren) GetAgentSessionRegistry() *AgentSessionRegistry {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.sessionRegistry
}

// SaveRegistry saves the agent session registry to disk
func (w *Warren) SaveRegistry() error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.registryPath == "" {
		return nil // No registry path configured
	}

	if w.sessionRegistry == nil {
		return nil // No registry to save
	}

	return w.sessionRegistry.Save(w.registryPath)
}
