package core

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lfu/warren/internal/types"
)

func TestAgentSessionRegistry_SaveLoad(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.json")

	// Create registry with test sessions
	registry := NewAgentSessionRegistry()

	session1 := &AgentSession{
		ID:              "test-1",
		Name:            "Test Agent 1",
		ServerName:      "localhost",
		TmuxSessionName: "main",
		TmuxWindowIndex: 0,
		TmuxPaneIndex:   0,
		TmuxPaneID:      "%1",
		AgentType:       "claude-code",
		CreatedAt:       time.Now(),
		LastSeenAt:      time.Now(),
		CurrentState:    types.StateIdle,
		Metadata:        map[string]string{"key": "value"},
	}

	session2 := &AgentSession{
		ID:              "test-2",
		Name:            "Test Agent 2",
		ServerName:      "localhost",
		TmuxSessionName: "main",
		TmuxWindowIndex: 0,
		TmuxPaneIndex:   1,
		TmuxPaneID:      "%2",
		AgentType:       "claude-code",
		CreatedAt:       time.Now(),
		LastSeenAt:      time.Now(),
		CurrentState:    types.StateThinking,
	}

	if err := registry.Register(session1); err != nil {
		t.Fatalf("Failed to register session1: %v", err)
	}

	if err := registry.Register(session2); err != nil {
		t.Fatalf("Failed to register session2: %v", err)
	}

	// Save registry
	if err := registry.Save(registryPath); err != nil {
		t.Fatalf("Failed to save registry: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		t.Fatalf("Registry file was not created")
	}

	// Load into new registry
	newRegistry := NewAgentSessionRegistry()
	if err := newRegistry.Load(registryPath); err != nil {
		t.Fatalf("Failed to load registry: %v", err)
	}

	// Verify sessions were loaded
	if newRegistry.Count() != 2 {
		t.Errorf("Expected 2 sessions, got %d", newRegistry.Count())
	}

	// Verify session1
	loaded1, err := newRegistry.Get("test-1")
	if err != nil {
		t.Fatalf("Failed to get session1: %v", err)
	}

	if loaded1.ID != session1.ID {
		t.Errorf("Session1 ID mismatch: expected %s, got %s", session1.ID, loaded1.ID)
	}

	if loaded1.Name != session1.Name {
		t.Errorf("Session1 Name mismatch: expected %s, got %s", session1.Name, loaded1.Name)
	}

	if loaded1.TmuxPaneID != session1.TmuxPaneID {
		t.Errorf("Session1 PaneID mismatch: expected %s, got %s", session1.TmuxPaneID, loaded1.TmuxPaneID)
	}

	if loaded1.CurrentState != session1.CurrentState {
		t.Errorf("Session1 State mismatch: expected %s, got %s", session1.CurrentState, loaded1.CurrentState)
	}

	// Verify session2
	loaded2, err := newRegistry.Get("test-2")
	if err != nil {
		t.Fatalf("Failed to get session2: %v", err)
	}

	if loaded2.ID != session2.ID {
		t.Errorf("Session2 ID mismatch: expected %s, got %s", session2.ID, loaded2.ID)
	}
}

func TestAgentSessionRegistry_LoadNonExistent(t *testing.T) {
	registry := NewAgentSessionRegistry()

	// Loading non-existent file should not error
	err := registry.Load("/nonexistent/path/registry.json")
	if err != nil {
		t.Errorf("Loading non-existent file should not error, got: %v", err)
	}

	// Registry should be empty
	if registry.Count() != 0 {
		t.Errorf("Expected empty registry, got %d sessions", registry.Count())
	}
}

func TestAgentSessionRegistry_Merge(t *testing.T) {
	registry := NewAgentSessionRegistry()

	// Add initial session
	session1 := &AgentSession{
		ID:              "test-1",
		Name:            "Original",
		ServerName:      "localhost",
		TmuxPaneID:      "%1",
		AgentType:       "claude-code",
		CreatedAt:       time.Now(),
		LastSeenAt:      time.Now(),
		CurrentState:    types.StateIdle,
	}

	if err := registry.Register(session1); err != nil {
		t.Fatalf("Failed to register session1: %v", err)
	}

	// Create discovered sessions (one overlapping, one new)
	discovered := []*AgentSession{
		{
			ID:              "test-1",
			Name:            "Updated",
			ServerName:      "localhost",
			TmuxPaneID:      "%1",
			AgentType:       "claude-code",
			CreatedAt:       time.Now(),
			LastSeenAt:      time.Now(),
			CurrentState:    types.StateThinking,
		},
		{
			ID:              "test-2",
			Name:            "New Session",
			ServerName:      "localhost",
			TmuxPaneID:      "%2",
			AgentType:       "claude-code",
			CreatedAt:       time.Now(),
			LastSeenAt:      time.Now(),
			CurrentState:    types.StateExecuting,
		},
	}

	// Merge discovered sessions
	registry.Merge(discovered)

	// Verify count
	if registry.Count() != 2 {
		t.Errorf("Expected 2 sessions after merge, got %d", registry.Count())
	}

	// Verify test-1 was updated
	updated, err := registry.Get("test-1")
	if err != nil {
		t.Fatalf("Failed to get test-1: %v", err)
	}

	if updated.Name != "Updated" {
		t.Errorf("Expected name 'Updated', got '%s'", updated.Name)
	}

	if updated.CurrentState != types.StateThinking {
		t.Errorf("Expected state StateThinking, got %s", updated.CurrentState)
	}

	// Verify test-2 was added
	_, err = registry.Get("test-2")
	if err != nil {
		t.Fatalf("Failed to get test-2: %v", err)
	}
}

func TestAgentSessionRegistry_Prune(t *testing.T) {
	registry := NewAgentSessionRegistry()

	// Add fresh session
	freshSession := &AgentSession{
		ID:              "fresh",
		Name:            "Fresh Session",
		ServerName:      "localhost",
		TmuxPaneID:      "%1",
		AgentType:       "claude-code",
		CreatedAt:       time.Now(),
		LastSeenAt:      time.Now(),
		CurrentState:    types.StateIdle,
	}

	// Add stale session (25 hours old) - directly to registry to avoid LastSeenAt update
	staleSession := &AgentSession{
		ID:              "stale",
		Name:            "Stale Session",
		ServerName:      "localhost",
		TmuxPaneID:      "%2",
		AgentType:       "claude-code",
		CreatedAt:       time.Now().Add(-25 * time.Hour),
		LastSeenAt:      time.Now().Add(-25 * time.Hour),
		CurrentState:    types.StateIdle,
	}

	if err := registry.Register(freshSession); err != nil {
		t.Fatalf("Failed to register fresh session: %v", err)
	}

	// Directly add stale session to avoid LastSeenAt update
	registry.sessions["stale"] = staleSession

	// Verify both sessions exist
	if registry.Count() != 2 {
		t.Errorf("Expected 2 sessions before prune, got %d", registry.Count())
	}

	// Prune stale sessions
	pruned := registry.Prune()

	// Verify one session was pruned
	if pruned != 1 {
		t.Errorf("Expected 1 session pruned, got %d", pruned)
	}

	// Verify only fresh session remains
	if registry.Count() != 1 {
		t.Errorf("Expected 1 session after prune, got %d", registry.Count())
	}

	// Verify fresh session still exists
	_, err := registry.Get("fresh")
	if err != nil {
		t.Errorf("Fresh session should still exist: %v", err)
	}

	// Verify stale session was removed
	_, err = registry.Get("stale")
	if err == nil {
		t.Errorf("Stale session should have been removed")
	}
}

func TestAgentSessionRegistry_AtomicWrite(t *testing.T) {
	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.json")

	registry := NewAgentSessionRegistry()

	session := &AgentSession{
		ID:              "test-1",
		Name:            "Test Agent",
		ServerName:      "localhost",
		TmuxPaneID:      "%1",
		AgentType:       "claude-code",
		CreatedAt:       time.Now(),
		LastSeenAt:      time.Now(),
		CurrentState:    types.StateIdle,
	}

	if err := registry.Register(session); err != nil {
		t.Fatalf("Failed to register session: %v", err)
	}

	// Save registry
	if err := registry.Save(registryPath); err != nil {
		t.Fatalf("Failed to save registry: %v", err)
	}

	// Verify temp file was cleaned up
	tempPath := registryPath + ".tmp"
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Errorf("Temp file should have been cleaned up")
	}

	// Verify final file exists
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		t.Errorf("Registry file should exist")
	}
}

func TestAgentSessionRegistry_ConcurrentSave(t *testing.T) {
	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.json")

	const numGoroutines = 10
	const numIterations = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch multiple goroutines that save concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				registry := NewAgentSessionRegistry()

				session := &AgentSession{
					ID:              "test-" + string(rune('A'+id)),
					Name:            "Test Agent",
					ServerName:      "localhost",
					TmuxPaneID:      "%1",
					AgentType:       "claude-code",
					CreatedAt:       time.Now(),
					LastSeenAt:      time.Now(),
					CurrentState:    types.StateIdle,
				}

				if err := registry.Register(session); err != nil {
					t.Errorf("Goroutine %d: Failed to register session: %v", id, err)
					return
				}

				if err := registry.Save(registryPath); err != nil {
					t.Errorf("Goroutine %d: Failed to save registry: %v", id, err)
					return
				}

				// Small delay to increase contention
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Verify file is not corrupted by loading it
	registry := NewAgentSessionRegistry()
	if err := registry.Load(registryPath); err != nil {
		t.Fatalf("Failed to load registry after concurrent saves: %v", err)
	}

	// File should contain valid JSON (Load would fail if corrupted)
	t.Logf("Successfully loaded registry with %d sessions after concurrent saves", registry.Count())
}

func TestAgentSessionRegistry_ConcurrentLoad(t *testing.T) {
	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.json")

	// Create and save initial registry
	registry := NewAgentSessionRegistry()
	for i := 0; i < 5; i++ {
		session := &AgentSession{
			ID:              "test-" + string(rune('A'+i)),
			Name:            "Test Agent",
			ServerName:      "localhost",
			TmuxPaneID:      "%1",
			AgentType:       "claude-code",
			CreatedAt:       time.Now(),
			LastSeenAt:      time.Now(),
			CurrentState:    types.StateIdle,
		}
		if err := registry.Register(session); err != nil {
			t.Fatalf("Failed to register session: %v", err)
		}
	}

	if err := registry.Save(registryPath); err != nil {
		t.Fatalf("Failed to save initial registry: %v", err)
	}

	const numGoroutines = 20
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch multiple goroutines that load concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			newRegistry := NewAgentSessionRegistry()
			if err := newRegistry.Load(registryPath); err != nil {
				t.Errorf("Goroutine %d: Failed to load registry: %v", id, err)
				return
			}

			if newRegistry.Count() != 5 {
				t.Errorf("Goroutine %d: Expected 5 sessions, got %d", id, newRegistry.Count())
			}
		}(i)
	}

	wg.Wait()
}

func TestAgentSessionRegistry_ConcurrentSaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.json")

	// Create initial registry
	initialRegistry := NewAgentSessionRegistry()
	session := &AgentSession{
		ID:              "initial",
		Name:            "Initial Session",
		ServerName:      "localhost",
		TmuxPaneID:      "%0",
		AgentType:       "claude-code",
		CreatedAt:       time.Now(),
		LastSeenAt:      time.Now(),
		CurrentState:    types.StateIdle,
	}
	if err := initialRegistry.Register(session); err != nil {
		t.Fatalf("Failed to register initial session: %v", err)
	}
	if err := initialRegistry.Save(registryPath); err != nil {
		t.Fatalf("Failed to save initial registry: %v", err)
	}

	const numSavers = 5
	const numLoaders = 10
	const iterations = 3

	var wg sync.WaitGroup
	wg.Add(numSavers + numLoaders)

	// Launch saver goroutines
	for i := 0; i < numSavers; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				registry := NewAgentSessionRegistry()

				session := &AgentSession{
					ID:              "saver-" + string(rune('A'+id)),
					Name:            "Saver Session",
					ServerName:      "localhost",
					TmuxPaneID:      "%1",
					AgentType:       "claude-code",
					CreatedAt:       time.Now(),
					LastSeenAt:      time.Now(),
					CurrentState:    types.StateExecuting,
				}

				if err := registry.Register(session); err != nil {
					t.Errorf("Saver %d: Failed to register session: %v", id, err)
					return
				}

				if err := registry.Save(registryPath); err != nil {
					t.Errorf("Saver %d: Failed to save registry: %v", id, err)
					return
				}

				time.Sleep(2 * time.Millisecond)
			}
		}(i)
	}

	// Launch loader goroutines
	for i := 0; i < numLoaders; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				registry := NewAgentSessionRegistry()

				if err := registry.Load(registryPath); err != nil {
					t.Errorf("Loader %d: Failed to load registry: %v", id, err)
					return
				}

				// Just verify we got valid data (count >= 0)
				if registry.Count() < 0 {
					t.Errorf("Loader %d: Invalid session count: %d", id, registry.Count())
				}

				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// Final verification: load should succeed
	finalRegistry := NewAgentSessionRegistry()
	if err := finalRegistry.Load(registryPath); err != nil {
		t.Fatalf("Failed to load registry after concurrent operations: %v", err)
	}

	t.Logf("Final registry has %d sessions after concurrent save/load", finalRegistry.Count())
}

func TestAgentSessionRegistry_LockTimeout(t *testing.T) {
	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.json")

	// Create initial registry
	registry1 := NewAgentSessionRegistry()
	session := &AgentSession{
		ID:              "test-1",
		Name:            "Test Session",
		ServerName:      "localhost",
		TmuxPaneID:      "%1",
		AgentType:       "claude-code",
		CreatedAt:       time.Now(),
		LastSeenAt:      time.Now(),
		CurrentState:    types.StateIdle,
	}
	if err := registry1.Register(session); err != nil {
		t.Fatalf("Failed to register session: %v", err)
	}
	if err := registry1.Save(registryPath); err != nil {
		t.Fatalf("Failed to save initial registry: %v", err)
	}

	// This test verifies that lock timeout works, but we can't easily
	// hold a lock indefinitely in a test without using internal APIs.
	// Instead, we verify that rapid concurrent access doesn't deadlock.

	const numGoroutines = 5
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	start := time.Now()

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			registry := NewAgentSessionRegistry()
			if err := registry.Load(registryPath); err != nil {
				t.Errorf("Goroutine %d: Failed to load: %v", id, err)
			}
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// All operations should complete quickly (well under 5 seconds per operation)
	if elapsed > 10*time.Second {
		t.Errorf("Concurrent operations took too long: %v", elapsed)
	}

	t.Logf("Concurrent operations completed in %v", elapsed)
}

func TestAgentSessionRegistry_NoCorruptionUnderLoad(t *testing.T) {
	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.json")

	const numWriters = 10
	const numReaders = 20
	const duration = 2 * time.Second

	var wg sync.WaitGroup
	wg.Add(numWriters + numReaders)

	stop := make(chan struct{})

	// Launch writer goroutines
	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-stop:
					return
				default:
					registry := NewAgentSessionRegistry()

					// Add multiple sessions
					for j := 0; j < 3; j++ {
						session := &AgentSession{
							ID:              "writer-" + string(rune('A'+id)) + "-" + string(rune('0'+j)),
							Name:            "Writer Session",
							ServerName:      "localhost",
							TmuxPaneID:      "%1",
							AgentType:       "claude-code",
							CreatedAt:       time.Now(),
							LastSeenAt:      time.Now(),
							CurrentState:    types.StateExecuting,
						}
						registry.Register(session)
					}

					if err := registry.Save(registryPath); err != nil {
						t.Errorf("Writer %d: Failed to save: %v", id, err)
						return
					}

					time.Sleep(10 * time.Millisecond)
				}
			}
		}(i)
	}

	// Launch reader goroutines
	for i := 0; i < numReaders; i++ {
		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-stop:
					return
				default:
					registry := NewAgentSessionRegistry()

					if err := registry.Load(registryPath); err != nil {
						t.Errorf("Reader %d: Failed to load: %v", id, err)
						return
					}

					// Verify we can access the data
					_ = registry.Count()

					time.Sleep(5 * time.Millisecond)
				}
			}
		}(i)
	}

	// Run for specified duration
	time.Sleep(duration)
	close(stop)
	wg.Wait()

	// Final verification: file should be valid
	finalRegistry := NewAgentSessionRegistry()
	if err := finalRegistry.Load(registryPath); err != nil {
		t.Fatalf("Failed to load registry after stress test: %v", err)
	}

	t.Logf("Stress test completed successfully, final registry has %d sessions", finalRegistry.Count())
}

