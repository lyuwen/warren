# Agent Discovery Integration - Usage Guide

## Overview

Issue #3 adds automatic agent discovery to Warren. The discovery service can now:
- Automatically detect Claude Code and other agent sessions in tmux
- Register discovered agents for monitoring
- Run periodic re-discovery to find new agents
- Support both local and remote servers

## Quick Start

### Basic Usage

```go
// Create Warren
config := core.DefaultConfig()
warren, err := core.NewWarren(config)
if err != nil {
    log.Fatal(err)
}

// Enable discovery with defaults
discoveryConfig := core.DefaultDiscoveryConfig()
if err := warren.EnableDiscovery(discoveryConfig); err != nil {
    log.Fatal(err)
}

// Start with discovery loop
if err := warren.StartWithDiscovery(); err != nil {
    log.Fatal(err)
}
```

### Custom Configuration

```go
// Custom discovery settings
discoveryConfig := &core.DiscoveryConfig{
    DiscoveryInterval:   10 * time.Minute, // Re-scan every 10 minutes
    EnableAutoDiscovery: true,              // Run discovery on startup
}

warren.EnableDiscovery(discoveryConfig)
```

### Manual Discovery

```go
// Trigger discovery manually (e.g., from API endpoint)
if err := warren.RunDiscovery(); err != nil {
    log.Printf("Discovery failed: %v", err)
}
```

### Disable Periodic Discovery

```go
// Run initial discovery only, no periodic scanning
discoveryConfig := &core.DiscoveryConfig{
    DiscoveryInterval:   0,    // 0 = disabled
    EnableAutoDiscovery: true, // Run once on startup
}

warren.EnableDiscovery(discoveryConfig)
warren.Start() // Use regular Start(), not StartWithDiscovery()
```

## Configuration Options

### DiscoveryConfig

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `DiscoveryInterval` | `time.Duration` | `5 * time.Minute` | How often to scan for new agents (0 = disabled) |
| `EnableAutoDiscovery` | `bool` | `true` | Run discovery on startup |

### Validation Rules

- `DiscoveryInterval` must be ≥ 0 (0 = disabled)
- If enabled, `DiscoveryInterval` must be ≥ 1 minute (to avoid excessive scanning)

## How It Works

### Discovery Process

1. **Scan Servers**: Iterate through all registered servers (local + remote)
2. **Discover Topology**: Get tmux sessions, windows, and panes for each server
3. **Detect Agents**: Analyze each pane's process to identify agent sessions
4. **Register Agents**: Add discovered agents to session registry and start monitoring

### Agent Detection

The discovery service detects agents by examining:
- **Process name**: `claude`, `node`, `python`, `gh`, `aider`, `cursor`
- **Confidence threshold**: Only agents with confidence ≥ 0.7 are registered
- **Working directory**: Captured for context

### Supported Agent Types

- **Claude Code**: Detected via `claude` process
- **GitHub Copilot**: Detected via `gh` process
- **Python agents**: Detected via `python` process
- **Aider**: Detected via `aider` process
- **Cursor**: Detected via `cursor` process

## API Integration

### Web API Endpoint (Example)

```go
// Add to your web server
http.HandleFunc("/api/discover", func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    if err := warren.RunDiscovery(); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "discovery completed",
    })
})
```

## Implementation Notes

### Workaround for Linter Issues

Due to auto-linter conflicts, the discovery implementation uses:
- **Separate files**: `warren_discovery.go`, `discovery_config.go`, `warren_discovery_integration.go`
- **Package-level state map**: `discoveryStates` map stores discovery state per Warren instance
- **Opt-in design**: Discovery must be explicitly enabled via `EnableDiscovery()`

This approach avoids linter conflicts while maintaining clean separation of concerns.

### Thread Safety

- Discovery runs in a separate goroutine
- Uses Warren's existing mutex for session access
- Safe to call `RunDiscovery()` concurrently

### Error Handling

- Discovery failures are logged but don't stop Warren
- Individual server failures don't abort the entire discovery process
- Warnings are printed to stdout for debugging

## Testing

Run discovery tests:

```bash
go test ./internal/core -v -run TestDiscovery
go test ./internal/core -v -run "TestWarren.*Discovery"
```

## Future Enhancements

Potential improvements for future iterations:
1. Add discovery metrics (agents found, scan duration, errors)
2. Support custom agent detection patterns
3. Add discovery event notifications
4. Implement discovery result caching
5. Add API endpoint to list discovered agents
6. Support discovery filtering (exclude certain panes/sessions)

## Migration Guide

### Existing Code

If you have existing Warren code that manually registers sessions:

```go
// Old way - manual registration
warren.AddSession("agent-1", "%0")
warren.AddSession("agent-2", "%1")
warren.Start()
```

### With Discovery

```go
// New way - automatic discovery
warren.EnableDiscovery(core.DefaultDiscoveryConfig())
warren.StartWithDiscovery()
// Sessions are discovered and registered automatically
```

Both approaches can coexist - manually registered sessions are preserved.
