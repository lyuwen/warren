package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServerRegistry(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create new registry
	registry, err := NewServerRegistry(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Should have local server by default
	if len(registry.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(registry.Servers))
	}

	if !registry.Servers[0].IsLocal() {
		t.Error("Expected first server to be local")
	}

	// Add a remote server
	remoteServer := &Server{
		Name: "test-remote",
		Host: "192.168.1.100",
		User: "testuser",
		Port: 22,
		Kind: ServerKindRemote,
	}

	if err := registry.Add(remoteServer); err != nil {
		t.Fatalf("Failed to add server: %v", err)
	}

	// Should have 2 servers now
	if len(registry.Servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(registry.Servers))
	}

	// Get the remote server
	retrieved, err := registry.Get("test-remote")
	if err != nil {
		t.Fatalf("Failed to get server: %v", err)
	}

	if retrieved.Host != "192.168.1.100" {
		t.Errorf("Expected host 192.168.1.100, got %s", retrieved.Host)
	}

	// Test persistence - create new registry from same directory
	registry2, err := NewServerRegistry(tmpDir)
	if err != nil {
		t.Fatalf("Failed to load registry: %v", err)
	}

	if len(registry2.Servers) != 2 {
		t.Errorf("Expected 2 servers after reload, got %d", len(registry2.Servers))
	}

	// Remove server
	if err := registry2.Remove("test-remote"); err != nil {
		t.Fatalf("Failed to remove server: %v", err)
	}

	if len(registry2.Servers) != 1 {
		t.Errorf("Expected 1 server after removal, got %d", len(registry2.Servers))
	}

	// Try to get removed server
	_, err = registry2.Get("test-remote")
	if err == nil {
		t.Error("Expected error when getting removed server")
	}
}

func TestServerRegistry_DuplicateName(t *testing.T) {
	tmpDir := t.TempDir()
	registry, err := NewServerRegistry(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	server1 := &Server{
		Name: "duplicate",
		Host: "host1",
		Kind: ServerKindRemote,
	}

	server2 := &Server{
		Name: "duplicate",
		Host: "host2",
		Kind: ServerKindRemote,
	}

	if err := registry.Add(server1); err != nil {
		t.Fatalf("Failed to add first server: %v", err)
	}

	// Should fail to add duplicate
	if err := registry.Add(server2); err == nil {
		t.Error("Expected error when adding duplicate server name")
	}
}

func TestServerRegistry_FileCreation(t *testing.T) {
	tmpDir := t.TempDir()

	registry, err := NewServerRegistry(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create registry: %v", err)
	}

	// Add a server to trigger save
	server := &Server{
		Name: "test",
		Host: "test.example.com",
		Kind: ServerKindRemote,
	}

	if err := registry.Add(server); err != nil {
		t.Fatalf("Failed to add server: %v", err)
	}

	// Check that file was created
	filePath := filepath.Join(tmpDir, "servers.yaml")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Expected servers.yaml to be created")
	}
}
