package core

import (
	"fmt"
	"time"
)

// DiscoveryConfig holds configuration for agent discovery
type DiscoveryConfig struct {
	DiscoveryInterval   time.Duration // How often to run agent discovery (default: 5 minutes, 0 = disabled)
	EnableAutoDiscovery bool          // Enable automatic agent discovery on startup (default: true)
}

// DefaultDiscoveryConfig returns sensible defaults for discovery
func DefaultDiscoveryConfig() *DiscoveryConfig {
	return &DiscoveryConfig{
		DiscoveryInterval:   5 * time.Minute, // 5 minutes
		EnableAutoDiscovery: true,            // enabled by default
	}
}

// ValidateDiscoveryConfig checks if discovery configuration is valid
func ValidateDiscoveryConfig(config *DiscoveryConfig) error {
	if config.DiscoveryInterval < 0 {
		return fmt.Errorf("DiscoveryInterval must be non-negative (0 = disabled), got %v", config.DiscoveryInterval)
	}
	if config.DiscoveryInterval > 0 && config.DiscoveryInterval < 1*time.Minute {
		return fmt.Errorf("DiscoveryInterval must be at least 1 minute to avoid excessive scanning, got %v", config.DiscoveryInterval)
	}
	return nil
}

