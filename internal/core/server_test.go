package core

import (
	"testing"
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
