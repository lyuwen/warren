package core

// EnableDiscovery initializes and enables agent discovery for Warren
// This should be called after Warren is created but before Start()
func (w *Warren) EnableDiscovery(config *DiscoveryConfig) error {
	if config == nil {
		config = DefaultDiscoveryConfig()
	}

	// Validate discovery config
	if err := ValidateDiscoveryConfig(config); err != nil {
		return err
	}

	// Initialize discovery
	w.initializeDiscovery(config)

	// Run initial discovery if enabled
	if config.EnableAutoDiscovery {
		if err := w.runDiscovery(); err != nil {
			// Log warning but don't fail - discovery is optional
			return err
		}
	}

	return nil
}

// StartWithDiscovery starts Warren with automatic discovery enabled
// This is a convenience method that combines Start() with discovery loop
func (w *Warren) StartWithDiscovery() error {
	// Start discovery loop
	w.startDiscoveryLoop()

	// Start normal monitoring
	return w.Start()
}
