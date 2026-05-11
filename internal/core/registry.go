package core

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ServerRegistry manages the collection of known servers
type ServerRegistry struct {
	Servers  []*Server `yaml:"servers"`
	filePath string
}

// NewServerRegistry creates a new server registry
func NewServerRegistry(configDir string) (*ServerRegistry, error) {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	filePath := filepath.Join(configDir, "servers.yaml")
	registry := &ServerRegistry{
		Servers:  make([]*Server, 0),
		filePath: filePath,
	}

	// Try to load existing registry
	if err := registry.Load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		// File doesn't exist yet, initialize with local server
		registry.AddLocalServer()
	}

	return registry, nil
}

// Load reads the registry from disk
func (r *ServerRegistry) Load() error {
	data, err := os.ReadFile(r.filePath)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, r)
}

// Save writes the registry to disk
func (r *ServerRegistry) Save() error {
	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	return os.WriteFile(r.filePath, data, 0644)
}

// AddLocalServer adds the local machine as a server
func (r *ServerRegistry) AddLocalServer() {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}

	local := &Server{
		Name: hostname,
		Host: "localhost",
		Kind: ServerKindLocal,
	}

	r.Servers = append(r.Servers, local)
}

// Add adds a server to the registry
func (r *ServerRegistry) Add(server *Server) error {
	// Check for duplicate names
	for _, s := range r.Servers {
		if s.Name == server.Name {
			return fmt.Errorf("server with name %q already exists", server.Name)
		}
	}

	r.Servers = append(r.Servers, server)
	return r.Save()
}

// Get retrieves a server by name
func (r *ServerRegistry) Get(name string) (*Server, error) {
	for _, s := range r.Servers {
		if s.Name == name {
			return s, nil
		}
	}
	return nil, fmt.Errorf("server %q not found", name)
}

// Remove removes a server from the registry
func (r *ServerRegistry) Remove(name string) error {
	for i, s := range r.Servers {
		if s.Name == name {
			r.Servers = append(r.Servers[:i], r.Servers[i+1:]...)
			return r.Save()
		}
	}
	return fmt.Errorf("server %q not found", name)
}

// List returns all servers in the registry
func (r *ServerRegistry) List() []*Server {
	return r.Servers
}
