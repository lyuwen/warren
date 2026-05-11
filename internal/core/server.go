package core

import (
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

// ServerKind represents whether a server is local or remote
type ServerKind string

const (
	ServerKindLocal  ServerKind = "local"
	ServerKindRemote ServerKind = "remote"
)

// Server represents a machine that hosts tmux sessions
type Server struct {
	Name       string            `yaml:"name" json:"name"`
	Host       string            `yaml:"host" json:"host"`
	User       string            `yaml:"user" json:"user"`
	Port       int               `yaml:"port" json:"port"`
	SSHOptions map[string]string `yaml:"ssh_options,omitempty" json:"ssh_options,omitempty"`
	Kind       ServerKind        `yaml:"kind" json:"kind"`
}

// IsLocal returns true if this server represents the local machine
func (s *Server) IsLocal() bool {
	return s.Kind == ServerKindLocal
}

// Address returns the SSH connection address for remote servers
func (s *Server) Address() string {
	if s.IsLocal() {
		return "localhost"
	}
	return fmt.Sprintf("%s@%s:%d", s.User, s.Host, s.Port)
}

// SSHClient represents an SSH connection to a remote server
type SSHClient struct {
	client *ssh.Client
	server *Server
}

// Close closes the SSH connection
func (c *SSHClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// ConnectionPool manages SSH connections to remote servers
type ConnectionPool struct {
	connections map[string]*SSHClient
	timeout     time.Duration
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(timeout time.Duration) *ConnectionPool {
	return &ConnectionPool{
		connections: make(map[string]*SSHClient),
		timeout:     timeout,
	}
}

// Get retrieves or creates an SSH connection for the given server
func (p *ConnectionPool) Get(server *Server) (*SSHClient, error) {
	if server.IsLocal() {
		return nil, fmt.Errorf("cannot create SSH connection for local server")
	}

	key := server.Name
	if conn, exists := p.connections[key]; exists {
		return conn, nil
	}

	// TODO: Implement actual SSH connection
	// For now, return placeholder
	return nil, fmt.Errorf("SSH connection not yet implemented")
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	for _, conn := range p.connections {
		if err := conn.Close(); err != nil {
			return err
		}
	}
	p.connections = make(map[string]*SSHClient)
	return nil
}
