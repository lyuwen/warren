package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/lfu/warren/internal/core"
	"github.com/lfu/warren/internal/tmux"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "topology":
		cmdTopology()
	case "capture":
		cmdCapture()
	case "send":
		cmdSend()
	case "test-loop":
		cmdTestLoop()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Warren Phase 1 Test CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  warren topology              - Discover and display tmux topology")
	fmt.Println("  warren capture <pane-id>     - Capture content from a pane")
	fmt.Println("  warren send <pane-id> <text> - Send text to a pane")
	fmt.Println("  warren test-loop <pane-id>   - Test control loop on a pane")
}

func cmdTopology() {
	// Get config directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %v", err)
	}
	configDir := filepath.Join(homeDir, ".warren")

	// Create registry
	registry, err := core.NewServerRegistry(configDir)
	if err != nil {
		log.Fatalf("Failed to create registry: %v", err)
	}

	// Get local server
	servers := registry.List()
	if len(servers) == 0 {
		log.Fatal("No servers found in registry")
	}

	localServer := servers[0]
	fmt.Printf("Discovering topology for server: %s\n\n", localServer.Name)

	// Create tmux client
	executor := tmux.NewLocalExecutor()
	client := tmux.NewClient(executor)

	// Discover topology
	topology, err := client.DiscoverTopology(localServer.Name)
	if err != nil {
		log.Fatalf("Failed to discover topology: %v", err)
	}

	// Display topology
	fmt.Printf("Server: %s\n", topology.ServerName)
	fmt.Printf("Sessions: %d\n\n", len(topology.Sessions))

	for _, session := range topology.Sessions {
		fmt.Printf("  Session: %s (ID: %s)\n", session.Name, session.ID)
		fmt.Printf("    Attached: %v\n", session.Attached)
		fmt.Printf("    Windows: %d\n", len(session.Windows))

		for _, window := range session.Windows {
			activeMarker := ""
			if window.Active {
				activeMarker = " *"
			}
			fmt.Printf("      Window %d: %s (ID: %s)%s\n", window.Index, window.Name, window.ID, activeMarker)
			fmt.Printf("        Panes: %d\n", len(window.Panes))

			for _, pane := range window.Panes {
				activeMarker := ""
				if pane.Active {
					activeMarker = " *"
				}
				fmt.Printf("          Pane %d: %s (%dx%d) PID:%d%s\n",
					pane.Index, pane.ID, pane.Width, pane.Height, pane.PID, activeMarker)
				if pane.Title != "" {
					fmt.Printf("            Title: %s\n", pane.Title)
				}
			}
		}
		fmt.Println()
	}
}

func cmdCapture() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: warren capture <pane-id>")
		os.Exit(1)
	}

	paneID := os.Args[2]

	executor := tmux.NewLocalExecutor()
	client := tmux.NewClient(executor)

	// Validate pane
	if err := client.ValidatePane(paneID); err != nil {
		log.Fatalf("Pane validation failed: %v", err)
	}

	// Capture content
	result, err := client.GetRecentContent(paneID, 100)
	if err != nil {
		log.Fatalf("Capture failed: %v", err)
	}

	fmt.Printf("Captured %d lines from pane %s:\n", result.Lines, result.PaneID)
	fmt.Println("---")
	fmt.Println(result.Content)
	fmt.Println("---")
}

func cmdSend() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: warren send <pane-id> <text>")
		os.Exit(1)
	}

	paneID := os.Args[2]
	text := os.Args[3]

	executor := tmux.NewLocalExecutor()
	client := tmux.NewClient(executor)

	// Validate pane
	if err := client.ValidatePane(paneID); err != nil {
		log.Fatalf("Pane validation failed: %v", err)
	}

	// Send text
	if err := client.SendText(paneID, text, &tmux.SendTextOptions{Literal: true, Enter: true}); err != nil {
		log.Fatalf("Send failed: %v", err)
	}

	fmt.Printf("Sent text to pane %s\n", paneID)
}

func cmdTestLoop() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: warren test-loop <pane-id>")
		os.Exit(1)
	}

	paneID := os.Args[2]

	executor := tmux.NewLocalExecutor()
	client := tmux.NewClient(executor)
	loop := tmux.NewControlLoop(client)

	// Validate pane
	if err := client.ValidatePane(paneID); err != nil {
		log.Fatalf("Pane validation failed: %v", err)
	}

	fmt.Printf("Testing control loop on pane %s\n", paneID)

	// Capture initial state
	before, err := client.GetRecentContent(paneID, 50)
	if err != nil {
		log.Fatalf("Initial capture failed: %v", err)
	}

	fmt.Println("\nInitial content (last 10 lines):")
	fmt.Println("---")
	fmt.Println(tmux.TailContent(before.Content, 10))
	fmt.Println("---")

	// Send a test command
	fmt.Println("\nSending test command: echo 'Warren test'")
	if err := client.SendText(paneID, "echo 'Warren test'", &tmux.SendTextOptions{Literal: true, Enter: true}); err != nil {
		log.Fatalf("Send failed: %v", err)
	}

	// Wait and capture again
	fmt.Println("\nWaiting for state change...")
	_, afterState, err := loop.DetectStateChange(paneID, 100*time.Millisecond, 5*time.Second)
	if err != nil {
		log.Fatalf("State change detection failed: %v", err)
	}

	fmt.Println("\nState changed!")
	fmt.Println("\nAfter content (last 10 lines):")
	fmt.Println("---")
	fmt.Println(tmux.TailContent(afterState, 10))
	fmt.Println("---")

	fmt.Println("\n✓ Control loop test completed successfully")
}
