package core

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
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

// Client returns the underlying *ssh.Client (may be nil for placeholder values).
func (c *SSHClient) Client() *ssh.Client {
	return c.client
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
	mu          sync.Mutex
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

	p.mu.Lock()
	defer p.mu.Unlock()

	key := server.Name
	if conn, exists := p.connections[key]; exists {
		return conn, nil
	}

	cfg, err := buildClientConfig(server, p.timeout)
	if err != nil {
		return nil, fmt.Errorf("build ssh config for %s: %w", server.Name, err)
	}

	port := server.Port
	if port == 0 {
		port = 22
	}
	addr := net.JoinHostPort(server.Host, fmt.Sprintf("%d", port))

	dialer := net.Dialer{Timeout: p.timeout}
	netConn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(netConn, addr, cfg)
	if err != nil {
		netConn.Close()
		return nil, fmt.Errorf("ssh handshake with %s: %w", addr, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	wrapped := &SSHClient{client: client, server: server}
	p.connections[key] = wrapped
	return wrapped, nil
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	for _, conn := range p.connections {
		if err := conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	p.connections = make(map[string]*SSHClient)
	return firstErr
}

// buildClientConfig assembles an *ssh.ClientConfig for the given server using
// agent + key-file auth and known_hosts-based host key verification.
func buildClientConfig(server *Server, timeout time.Duration) (*ssh.ClientConfig, error) {
	user := server.User
	if user == "" {
		user = os.Getenv("USER")
	}

	auths := defaultAuthMethods()
	if len(auths) == 0 {
		return nil, fmt.Errorf("no ssh auth methods available (set SSH_AUTH_SOCK or provide ~/.ssh/id_ed25519 / id_rsa)")
	}

	hostKeyCallback := defaultHostKeyCallback()

	return &ssh.ClientConfig{
		User:            user,
		Auth:            auths,
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	}, nil
}

// defaultAuthMethods returns SSH auth methods derived from the environment:
// the SSH agent (if SSH_AUTH_SOCK is set) followed by the default user key
// files. Missing or unreadable items are skipped silently.
func defaultAuthMethods() []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			ag := agent.NewClient(conn)
			methods = append(methods, ssh.PublicKeysCallback(ag.Signers))
		}
	}

	home, err := os.UserHomeDir()
	if err == nil {
		keyFiles := []string{
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_rsa"),
		}
		var signers []ssh.Signer
		for _, kf := range keyFiles {
			data, err := os.ReadFile(kf)
			if err != nil {
				continue
			}
			signer, err := ssh.ParsePrivateKey(data)
			if err != nil {
				// Encrypted or invalid key — skip silently.
				continue
			}
			signers = append(signers, signer)
		}
		if len(signers) > 0 {
			methods = append(methods, ssh.PublicKeys(signers...))
		}
	}

	return methods
}

// defaultHostKeyCallback returns a host key verifier backed by
// ~/.ssh/known_hosts. If the file is unavailable, it falls back to an
// insecure-accept callback with a logged warning so connections still work
// in environments without provisioned known_hosts (development, CI).
func defaultHostKeyCallback() ssh.HostKeyCallback {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("warren/ssh: cannot resolve home directory for known_hosts: %v; falling back to InsecureIgnoreHostKey", err)
		return ssh.InsecureIgnoreHostKey()
	}
	khPath := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(khPath); err != nil {
		log.Printf("warren/ssh: known_hosts unavailable at %s (%v); falling back to InsecureIgnoreHostKey", khPath, err)
		return ssh.InsecureIgnoreHostKey()
	}
	cb, err := knownhosts.New(khPath)
	if err != nil {
		log.Printf("warren/ssh: failed to load known_hosts %s: %v; falling back to InsecureIgnoreHostKey", khPath, err)
		return ssh.InsecureIgnoreHostKey()
	}
	return cb
}
