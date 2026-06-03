package core

import (
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestServer_IsLocal(t *testing.T) {
	tests := []struct {
		name     string
		server   *Server
		expected bool
	}{
		{
			name: "local server",
			server: &Server{
				Name: "localhost",
				Kind: ServerKindLocal,
			},
			expected: true,
		},
		{
			name: "remote server",
			server: &Server{
				Name: "remote-host",
				Host: "192.168.1.100",
				User: "user",
				Port: 22,
				Kind: ServerKindRemote,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.server.IsLocal(); got != tt.expected {
				t.Errorf("IsLocal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestServer_Address(t *testing.T) {
	tests := []struct {
		name     string
		server   *Server
		expected string
	}{
		{
			name: "local server",
			server: &Server{
				Name: "localhost",
				Kind: ServerKindLocal,
			},
			expected: "localhost",
		},
		{
			name: "remote server",
			server: &Server{
				Name: "remote-host",
				Host: "192.168.1.100",
				User: "testuser",
				Port: 22,
				Kind: ServerKindRemote,
			},
			expected: "testuser@192.168.1.100:22",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.server.Address(); got != tt.expected {
				t.Errorf("Address() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConnectionPool(t *testing.T) {
	pool := NewConnectionPool(0)
	defer pool.Close()

	localServer := &Server{
		Name: "localhost",
		Kind: ServerKindLocal,
	}

	// Should error for local server
	_, err := pool.Get(localServer)
	if err == nil {
		t.Error("Expected error for local server, got nil")
	}
}

// TestConnectionPool_LocalServerRejected verifies Get() refuses local servers.
func TestConnectionPool_LocalServerRejected(t *testing.T) {
	pool := NewConnectionPool(5 * time.Second)
	defer pool.Close()

	local := &Server{Name: "localhost", Kind: ServerKindLocal}
	client, err := pool.Get(local)
	if err == nil {
		t.Fatal("expected error for local server, got nil")
	}
	if client != nil {
		t.Errorf("expected nil client for local server, got %v", client)
	}
}

// TestConnectionPool_ConcurrentGet exercises the pool from many goroutines
// to catch races (run with -race). All calls target an unreachable server so
// they should all return an error without corrupting internal state.
func TestConnectionPool_ConcurrentGet(t *testing.T) {
	// Opt in to insecure host-key mode so the test exercises the dial path
	// regardless of whether the test machine has a populated
	// ~/.ssh/known_hosts. Without this, hostKeyCallbackForServer would
	// refuse before dial on a fresh CI runner and the test would still pass
	// — but for the wrong reason (it asserts no race, not a specific error
	// path, and we want the race coverage to actually reach dial).
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")

	pool := NewConnectionPool(100 * time.Millisecond)
	defer pool.Close()

	// Unreachable address — connection will fail fast but should not race.
	server := &Server{
		Name: "unreachable-test",
		Host: "127.0.0.1",
		User: "nobody",
		Port: 1, // port 1 is almost never open
		Kind: ServerKindRemote,
	}

	const N = 25
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			// We don't care if it succeeds or fails — only that it doesn't race/panic.
			_, _ = pool.Get(server)
		}()
	}
	wg.Wait()
}

// TestConnectionPool_Close_MapCleared verifies Close clears the connection map.
func TestConnectionPool_Close_MapCleared(t *testing.T) {
	pool := NewConnectionPool(time.Second)

	// Seed a fake entry directly into the map to simulate a pooled connection
	// without requiring a real SSH server.
	pool.connections["fake-server"] = &SSHClient{
		client: nil, // Close() handles nil safely
		server: &Server{Name: "fake-server", Kind: ServerKindRemote},
	}

	if len(pool.connections) != 1 {
		t.Fatalf("expected 1 connection before Close, got %d", len(pool.connections))
	}

	if err := pool.Close(); err != nil {
		t.Fatalf("Close() returned unexpected error: %v", err)
	}

	if len(pool.connections) != 0 {
		t.Errorf("expected empty connection map after Close, got %d entries", len(pool.connections))
	}
}

// TestConnectionPool_LiveSSH attempts a real SSH connection. Gated behind
// WARREN_TEST_SSH so it doesn't run in CI. To run:
//
//	WARREN_TEST_SSH=1 \
//	WARREN_TEST_SSH_HOST=example.com \
//	WARREN_TEST_SSH_USER=myuser \
//	WARREN_TEST_SSH_PORT=22 \
//	go test ./internal/core/ -run TestConnectionPool_LiveSSH
func TestConnectionPool_LiveSSH(t *testing.T) {
	if os.Getenv("WARREN_TEST_SSH") == "" {
		t.Skip("set WARREN_TEST_SSH=1 to run live SSH connection test")
	}

	host := os.Getenv("WARREN_TEST_SSH_HOST")
	user := os.Getenv("WARREN_TEST_SSH_USER")
	if host == "" || user == "" {
		t.Skip("WARREN_TEST_SSH_HOST and WARREN_TEST_SSH_USER must be set")
	}

	port := 22
	if p := os.Getenv("WARREN_TEST_SSH_PORT"); p != "" {
		if pp, err := strconv.Atoi(p); err == nil {
			port = pp
		}
	}

	pool := NewConnectionPool(10 * time.Second)
	defer pool.Close()

	server := &Server{
		Name: "live-test",
		Host: host,
		User: user,
		Port: port,
		Kind: ServerKindRemote,
	}

	client, err := pool.Get(server)
	if err != nil {
		t.Fatalf("expected successful SSH connection, got error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}

	// Second Get should return the same cached client.
	client2, err := pool.Get(server)
	if err != nil {
		t.Fatalf("second Get returned error: %v", err)
	}
	if client2 != client {
		t.Error("expected cached client to be returned on second Get")
	}
}
