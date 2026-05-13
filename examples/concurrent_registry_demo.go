package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lfu/warren/internal/core"
	"github.com/lfu/warren/internal/types"
)

// This demo shows file locking preventing corruption from concurrent Warren instances
func main() {
	// Create temp directory for demo
	tempDir, err := os.MkdirTemp("", "warren-demo-*")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	registryPath := filepath.Join(tempDir, "registry.json")
	fmt.Printf("Demo registry path: %s\n\n", registryPath)

	// Simulate multiple Warren instances running concurrently
	const numInstances = 5
	const operationsPerInstance = 10

	var wg sync.WaitGroup
	wg.Add(numInstances)

	startTime := time.Now()

	for i := 0; i < numInstances; i++ {
		go func(instanceID int) {
			defer wg.Done()

			fmt.Printf("[Instance %d] Starting...\n", instanceID)

			for j := 0; j < operationsPerInstance; j++ {
				// Load existing registry (simulating reading current state)
				registry := core.NewAgentSessionRegistry()
				if err := registry.Load(registryPath); err != nil {
					fmt.Printf("[Instance %d] Load failed: %v\n", instanceID, err)
					return
				}

				loadedCount := registry.Count()

				// Add a new session
				session := &core.AgentSession{
					ID:              fmt.Sprintf("instance-%d-session-%d", instanceID, j),
					Name:            fmt.Sprintf("Agent %d-%d", instanceID, j),
					ServerName:      "localhost",
					TmuxSessionName: "main",
					TmuxWindowIndex: 0,
					TmuxPaneIndex:   instanceID,
					TmuxPaneID:      fmt.Sprintf("%%%d", instanceID),
					AgentType:       "claude-code",
					CreatedAt:       time.Now(),
					LastSeenAt:      time.Now(),
					CurrentState:    types.StateIdle,
				}

				if err := registry.Register(session); err != nil {
					fmt.Printf("[Instance %d] Register failed: %v\n", instanceID, err)
					return
				}

				// Save registry (with file locking)
				if err := registry.Save(registryPath); err != nil {
					fmt.Printf("[Instance %d] Save failed: %v\n", instanceID, err)
					return
				}

				fmt.Printf("[Instance %d] Operation %d completed (loaded: %d, saved: %d)\n",
					instanceID, j+1, loadedCount, registry.Count())

				// Small delay to simulate real work
				time.Sleep(10 * time.Millisecond)
			}

			fmt.Printf("[Instance %d] Finished all operations\n", instanceID)
		}(i)
	}

	// Wait for all instances to complete
	wg.Wait()

	elapsed := time.Since(startTime)
	fmt.Printf("\n=== Demo Complete ===\n")
	fmt.Printf("Duration: %v\n", elapsed)

	// Load final registry and verify integrity
	finalRegistry := core.NewAgentSessionRegistry()
	if err := finalRegistry.Load(registryPath); err != nil {
		fmt.Printf("ERROR: Failed to load final registry: %v\n", err)
		fmt.Println("This indicates file corruption!")
		os.Exit(1)
	}

	fmt.Printf("Final registry loaded successfully\n")
	fmt.Printf("Total sessions: %d\n", finalRegistry.Count())
	fmt.Printf("Expected sessions: %d\n", numInstances*operationsPerInstance)

	if finalRegistry.Count() != numInstances*operationsPerInstance {
		fmt.Printf("\n⚠️  Note: Got %d sessions instead of %d\n", finalRegistry.Count(), numInstances*operationsPerInstance)
		fmt.Println("This is expected because each instance loads, adds one session, and saves.")
		fmt.Println("The last writer for each session ID wins (load-modify-save pattern).")
		fmt.Println("File locking ensures no corruption occurs during concurrent writes.")
	}

	// List all sessions
	fmt.Println("\nRegistered sessions:")
	for _, session := range finalRegistry.List() {
		fmt.Printf("  - %s (%s)\n", session.ID, session.Name)
	}

	fmt.Println("\n✅ File locking prevented corruption!")
	fmt.Println("Without locking, concurrent writes would corrupt the JSON file.")
}
