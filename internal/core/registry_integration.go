package core

import "log"

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

	// Auto-save registry after registration. Save failures are logged but do
	// not fail the registration itself — the in-memory registry is still
	// valid, and the persistence layer can be retried later.
	if w.registryPath != "" {
		if err := w.sessionRegistry.Save(w.registryPath); err != nil {
			log.Printf("warren: failed to auto-save session registry to %s: %v", w.registryPath, err)
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
