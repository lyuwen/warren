package core

import (
	"errors"
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

var (
	ErrKnownHostsMissing    = errors.New("known_hosts is missing")
	ErrKnownHostsUnreadable = errors.New("known_hosts is unreadable")
	ErrKnownHostsMalformed  = errors.New("known_hosts is malformed")
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

// ConnectionPool manages SSH connections to remote servers.
//
// Get uses a double-checked-locking + per-key promise pattern so the pool's
// main mutex is never held across the (potentially slow) SSH dial+handshake.
// Concurrent Get calls for DIFFERENT servers proceed in parallel. Concurrent
// Get calls for the SAME server wait on a shared in-flight promise so the
// dial is performed exactly once per key (deduplication).
type ConnectionPool struct {
	mu                  sync.Mutex
	connections         map[string]*SSHClient
	pending             map[string]*connDial // in-flight dials, keyed by server.Name
	timeout             time.Duration
	insecureHostsWarned sync.Map // keyed by host:port; pool-scoped per Architect/API contract
}

// connDial represents an in-flight SSH dial. Callers that find an existing
// entry under p.mu wait on `ready`; the goroutine that installed the entry
// is responsible for performing the dial and closing `ready`.
type connDial struct {
	ready  chan struct{}
	client *SSHClient
	err    error
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(timeout time.Duration) *ConnectionPool {
	return &ConnectionPool{
		connections: make(map[string]*SSHClient),
		pending:     make(map[string]*connDial),
		timeout:     timeout,
	}
}

// Get retrieves or creates an SSH connection for the given server.
//
// The pool's main mutex is released before the SSH dial+handshake runs, so a
// slow or failing dial against one server does not block Get calls for other
// servers. Concurrent calls for the same server share a single in-flight
// dial via p.pending and observe the same result.
func (p *ConnectionPool) Get(server *Server) (*SSHClient, error) {
	if server.IsLocal() {
		return nil, fmt.Errorf("cannot create SSH connection for local server")
	}

	key := server.Name

	// Fast path + in-flight join under the lock; the dial itself is outside.
	p.mu.Lock()
	if conn, ok := p.connections[key]; ok {
		p.mu.Unlock()
		return conn, nil
	}
	if d, ok := p.pending[key]; ok {
		p.mu.Unlock()
		<-d.ready
		return d.client, d.err
	}
	d := &connDial{ready: make(chan struct{})}
	p.pending[key] = d
	p.mu.Unlock()

	// Perform the dial without holding p.mu so other servers proceed in
	// parallel. Concurrent same-key callers wait on d.ready above.
	client, err := p.dial(server)

	p.mu.Lock()
	delete(p.pending, key)
	d.client = client
	d.err = err
	if err == nil {
		p.connections[key] = client
	}
	p.mu.Unlock()

	close(d.ready)
	return client, err
}

// dial performs the SSH config build, TCP dial, and SSH handshake for a
// remote server. It must be called with p.mu released.
func (p *ConnectionPool) dial(server *Server) (*SSHClient, error) {
	cfg, err := p.buildClientConfig(server, p.timeout)
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
	return &SSHClient{client: client, server: server}, nil
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
func (p *ConnectionPool) buildClientConfig(server *Server, timeout time.Duration) (*ssh.ClientConfig, error) {
	user := server.User
	if user == "" {
		user = os.Getenv("USER")
	}

	auths := defaultAuthMethods()
	if len(auths) == 0 {
		return nil, fmt.Errorf("no ssh auth methods available (set SSH_AUTH_SOCK or provide ~/.ssh/id_ed25519 / id_rsa)")
	}

	hostKeyCallback, err := p.hostKeyCallback(server)
	if err != nil {
		return nil, err
	}

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

type knownHostsUnavailableError struct {
	sentinel error
	server   *Server
	detail   string
}

func (e *knownHostsUnavailableError) Error() string {
	return fmt.Sprintf(`SSH host key verification unavailable for %q (%s):
  %s

Warren refuses to connect because the server identity cannot be verified
(MITM vulnerable). To proceed, choose one:

  1. Populate known_hosts (recommended):
        ssh-keyscan -H %s >> ~/.ssh/known_hosts

  2. Connect with verification disabled (UNSAFE; dev/CI only):
        export WARREN_SSH_INSECURE_HOSTKEY=1`,
		e.server.Name,
		serverHostPort(e.server),
		e.detail,
		e.server.Host,
	)
}

func (e *knownHostsUnavailableError) Unwrap() error { return e.sentinel }

// hostKeyCallback returns a verifier backed by ~/.ssh/known_hosts.
//
// Strict by default: missing/unreadable/malformed known_hosts yields a wrapped
// sentinel error. The only path to ssh.InsecureIgnoreHostKey() is an explicit
// `WARREN_SSH_INSECURE_HOSTKEY=1` env opt-in.
//
// The signature accepts *Server so a future per-server `InsecureHostKey bool`
// field on Server can layer in without breaking callers.
func (p *ConnectionPool) hostKeyCallback(server *Server) (ssh.HostKeyCallback, error) {
	insecureOptIn := os.Getenv("WARREN_SSH_INSECURE_HOSTKEY") == "1"

	// TODO(phase3): if/when a per-server `insecure_host_key` field lands in
	// servers.yaml / Server, consult it here in addition to the env var.
	// Tracked in docs/phase2-technical-debt.md (P1 follow-up entry).

	sentinel, detail, cb := loadKnownHostsCallback()
	if sentinel == nil {
		return cb, nil
	}
	if !insecureOptIn {
		return nil, newKnownHostsUnavailableError(server, sentinel, detail)
	}

	hostPort := serverHostPort(server)
	if _, loaded := p.insecureHostsWarned.LoadOrStore(hostPort, struct{}{}); !loaded {
		log.Printf("WARN: accepting unverified host key for %s (WARREN_SSH_INSECURE_HOSTKEY=1; MITM-vulnerable)", hostPort)
	}
	return ssh.InsecureIgnoreHostKey(), nil
}

// loadKnownHostsCallback inspects ~/.ssh/known_hosts and returns:
//   - (nil, "", cb)                            — file present, parses cleanly
//   - (ErrKnownHostsMissing, detail, nil)      — home dir unresolvable OR file missing
//   - (ErrKnownHostsUnreadable, detail, nil)   — file exists but cannot be opened/read
//   - (ErrKnownHostsMalformed, detail, nil)    — file opens but knownhosts.New rejects it
func loadKnownHostsCallback() (sentinel error, detail string, cb ssh.HostKeyCallback) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		detail = "~/.ssh/known_hosts is missing"
		if err != nil {
			detail = fmt.Sprintf("~/.ssh/known_hosts is missing (%s)", err)
		}
		return ErrKnownHostsMissing, detail, nil
	}

	khPath := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(khPath); err != nil {
		switch {
		case os.IsNotExist(err):
			return ErrKnownHostsMissing, "~/.ssh/known_hosts is missing.", nil
		case os.IsPermission(err):
			return ErrKnownHostsUnreadable, fmt.Sprintf("~/.ssh/known_hosts is unreadable: %v.", err), nil
		default:
			return ErrKnownHostsUnreadable, fmt.Sprintf("~/.ssh/known_hosts is unreadable: %v.", err), nil
		}
	}

	f, err := os.Open(khPath)
	if err != nil {
		return ErrKnownHostsUnreadable, fmt.Sprintf("~/.ssh/known_hosts is unreadable: %v.", err), nil
	}
	_ = f.Close()

	cb, err = knownhosts.New(khPath)
	if err != nil {
		return ErrKnownHostsMalformed, fmt.Sprintf("~/.ssh/known_hosts is malformed: %v.", err), nil
	}
	return nil, "", cb
}

func newKnownHostsUnavailableError(server *Server, sentinel error, detail string) error {
	return &knownHostsUnavailableError{sentinel: sentinel, server: server, detail: detail}
}

// serverHostPort formats server.Host:port for logs and error messages,
// applying the default port of 22 when Server.Port is zero.
func serverHostPort(server *Server) string {
	port := server.Port
	if port == 0 {
		port = 22
	}
	return net.JoinHostPort(server.Host, fmt.Sprintf("%d", port))
}
