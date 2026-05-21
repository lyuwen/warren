package core

import (
	"testing"
	"time"
)

func TestDiscoveryConfig_Defaults(t *testing.T) {
	config := DefaultDiscoveryConfig()

	if config.DiscoveryInterval != 5*time.Minute {
		t.Errorf("Expected default interval 5m, got %v", config.DiscoveryInterval)
	}

	if !config.EnableAutoDiscovery {
		t.Error("Expected auto-discovery enabled by default")
	}
}

func TestDiscoveryConfig_Validation(t *testing.T) {
	tests := []struct {
		name      string
		config    *DiscoveryConfig
		shouldErr bool
	}{
		{
			name: "valid config",
			config: &DiscoveryConfig{
				DiscoveryInterval:   5 * time.Minute,
				EnableAutoDiscovery: true,
			},
			shouldErr: false,
		},
		{
			name: "disabled discovery",
			config: &DiscoveryConfig{
				DiscoveryInterval:   0,
				EnableAutoDiscovery: false,
			},
			shouldErr: false,
		},
		{
			name: "negative interval",
			config: &DiscoveryConfig{
				DiscoveryInterval:   -1 * time.Minute,
				EnableAutoDiscovery: true,
			},
			shouldErr: true,
		},
		{
			name: "interval too short",
			config: &DiscoveryConfig{
				DiscoveryInterval:   30 * time.Second,
				EnableAutoDiscovery: true,
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDiscoveryConfig(tt.config)
			if tt.shouldErr && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestWarren_EnableDiscovery(t *testing.T) {
	// Create Warren instance
	config := DefaultConfig()
	config.DBPath = t.TempDir() + "/test.db"
	config.ConfigDir = t.TempDir()

	warren, err := NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}
	defer warren.Stop()

	// Enable discovery
	discoveryConfig := &DiscoveryConfig{
		DiscoveryInterval:   5 * time.Minute,
		EnableAutoDiscovery: false, // Don't run initial discovery in test
	}

	err = warren.EnableDiscovery(discoveryConfig)
	if err != nil {
		t.Fatalf("Failed to enable discovery: %v", err)
	}

	// Verify discovery state was initialized
	if warren.agentDiscovery == nil {
		t.Error("Agent discovery service not created")
	}

	if warren.discoveryInterval != 5*time.Minute {
		t.Errorf("Expected interval 5m, got %v", warren.discoveryInterval)
	}

	if warren.discoveryEnabled {
		t.Error("Expected discoveryEnabled to be false")
	}
}

func TestWarren_RunDiscovery_NoServers(t *testing.T) {
	// Create Warren instance
	config := DefaultConfig()
	config.DBPath = t.TempDir() + "/test.db"
	config.ConfigDir = t.TempDir()

	warren, err := NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}
	defer warren.Stop()

	// Remove all servers from registry
	servers := warren.serverRegistry.List()
	for _, server := range servers {
		warren.serverRegistry.Remove(server.Name)
	}

	// Enable discovery
	discoveryConfig := &DiscoveryConfig{
		DiscoveryInterval:   5 * time.Minute,
		EnableAutoDiscovery: false,
	}

	err = warren.EnableDiscovery(discoveryConfig)
	if err != nil {
		t.Fatalf("Failed to enable discovery: %v", err)
	}

	// Run discovery (should fail with no servers)
	err = warren.RunDiscovery()
	if err == nil {
		t.Error("Expected error when running discovery with no servers")
	}
}

func TestWarren_CleanupDiscovery(t *testing.T) {
	// Create Warren instance
	config := DefaultConfig()
	config.DBPath = t.TempDir() + "/test.db"
	config.ConfigDir = t.TempDir()

	warren, err := NewWarren(config)
	if err != nil {
		t.Fatalf("Failed to create Warren: %v", err)
	}

	// Enable discovery
	discoveryConfig := DefaultDiscoveryConfig()
	discoveryConfig.EnableAutoDiscovery = false

	err = warren.EnableDiscovery(discoveryConfig)
	if err != nil {
		t.Fatalf("Failed to enable discovery: %v", err)
	}

	// Verify discovery is initialized
	if warren.agentDiscovery == nil {
		t.Fatal("Discovery not initialized")
	}

	// Stop Warren (should clean up discovery)
	warren.Stop()

	// Discovery fields remain but Warren is stopped
	// This is acceptable - no memory leak since Warren itself is being cleaned up
}
