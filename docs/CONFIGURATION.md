# Warren Configuration Guide

This guide explains how to configure Warren to monitor agent sessions across local and remote servers.

## Quick Start

1. **Create the Warren directory:**
   ```bash
   mkdir -p ~/.warren
   ```

2. **Create server configuration:**
   ```bash
   cp servers.yaml.example ~/.warren/servers.yaml
   # Edit ~/.warren/servers.yaml with your server details
   ```

3. **Start Warren:**
   ```bash
   # Web interface
   ./warren-web

   # TUI interface
   ./warren-tui
   ```

## Configuration Files

Warren uses the following configuration files in `~/.warren/`:

- **`servers.yaml`** - Server definitions (local and remote)
- **`warren.db`** - SQLite database for events and state
- **`registry.json`** - Agent session registry (auto-generated)

## Server Configuration

### File Location

`~/.warren/servers.yaml`

### Format

```yaml
servers:
  - name: "localhost"
    host: "localhost"
    kind: "local"

  - name: "production"
    host: "prod.example.com"
    user: "your-username"
    port: 22
    kind: "remote"
    ssh_options:
      IdentityFile: "~/.ssh/id_rsa"
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique identifier for the server |
| `host` | Yes | Hostname or IP address |
| `kind` | Yes | Either `"local"` or `"remote"` |
| `user` | Remote only | SSH username |
| `port` | Remote only | SSH port (default: 22) |
| `ssh_options` | No | Additional SSH options |

### Local Server

For monitoring tmux sessions on your local machine:

```yaml
servers:
  - name: "localhost"
    host: "localhost"
    kind: "local"
```

**Note:** If no `servers.yaml` exists, Warren automatically creates one with a local server entry.

### Remote Servers

For monitoring tmux sessions on remote machines via SSH:

```yaml
servers:
  - name: "production-server"
    host: "prod.example.com"
    user: "deploy"
    port: 22
    kind: "remote"
    ssh_options:
      IdentityFile: "~/.ssh/prod_key"
      StrictHostKeyChecking: "no"
```

**Requirements for remote servers:**
- SSH key-based authentication must be configured
- You must be able to SSH without password prompts
- The remote server must have tmux installed
- Your user must have permission to list and attach to tmux sessions

**Test your SSH connection:**
```bash
ssh user@host -p port tmux list-sessions
```

### SSH Options

Common SSH options you can configure:

```yaml
ssh_options:
  IdentityFile: "~/.ssh/custom_key"
  StrictHostKeyChecking: "no"
  UserKnownHostsFile: "/dev/null"
  ConnectTimeout: "10"
  ServerAliveInterval: "60"
```

## Command-Line Options

### warren-web

```bash
./warren-web [options]

Options:
  -addr string
        HTTP server address (default ":8080")
  -db string
        Database path (default "~/.warren/warren.db")
  -poll duration
        Polling interval (default 500ms)
  -confidence float
        Minimum confidence for state transitions (default 0.7)
```

**Example:**
```bash
./warren-web -addr :9090 -poll 1s
```

### warren-tui

```bash
./warren-tui

# Uses default configuration from ~/.warren/
```

## Configuration Validation

Warren validates configuration on startup and will report errors if:

- `EventRetentionPeriod` is not positive (default: 30 days)
- `PollInterval` is less than 100ms (default: 500ms)
- `MinConfidence` is not between 0 and 1 (default: 0.7)
- Server configuration is malformed

## Troubleshooting

### "No servers configured" message

**Problem:** Warren can't find `~/.warren/servers.yaml`

**Solution:**
```bash
mkdir -p ~/.warren
cp servers.yaml.example ~/.warren/servers.yaml
```

### "Failed to discover topology" on remote server

**Problem:** Can't connect to remote server or list tmux sessions

**Solutions:**
1. Test SSH connection:
   ```bash
   ssh user@host -p port
   ```

2. Test tmux access:
   ```bash
   ssh user@host -p port tmux list-sessions
   ```

3. Check SSH key permissions:
   ```bash
   chmod 600 ~/.ssh/id_rsa
   ```

4. Verify server configuration in `~/.warren/servers.yaml`

### "EventRetentionPeriod must be positive" error

**Problem:** Using old version of warren-web that doesn't set default config

**Solution:** Rebuild warren-web:
```bash
go build -o warren-web ./cmd/warren-web
```

### warren-tui only shows local sessions

**Problem:** Server configuration not loaded or remote discovery failing

**Solutions:**
1. Check `~/.warren/servers.yaml` exists and is valid
2. Check logs for discovery errors
3. Verify SSH connectivity to remote servers

## Default Configuration Values

Warren uses these defaults if not specified:

```go
PollInterval:           500ms
MinConfidence:          0.7
DBPath:                 ~/.warren/warren.db
ConfigDir:              ~/.warren
EventRetentionPeriod:   30 days
EventPruningInterval:   24 hours
CacheTTL:               5 seconds
RegistryPruneThreshold: 24 hours
```

## Security Considerations

1. **SSH Keys:** Use dedicated SSH keys for Warren with appropriate permissions
2. **Known Hosts:** Consider using `StrictHostKeyChecking: "yes"` in production
3. **User Permissions:** Warren runs with your user permissions - ensure appropriate access
4. **Database:** The SQLite database contains session metadata - protect `~/.warren/`

## Advanced Configuration

### Multiple Environments

You can configure multiple servers for different environments:

```yaml
servers:
  - name: "local"
    host: "localhost"
    kind: "local"

  - name: "dev-1"
    host: "dev1.example.com"
    user: "developer"
    port: 22
    kind: "remote"

  - name: "dev-2"
    host: "dev2.example.com"
    user: "developer"
    port: 22
    kind: "remote"

  - name: "staging"
    host: "staging.example.com"
    user: "deploy"
    port: 22
    kind: "remote"

  - name: "production"
    host: "prod.example.com"
    user: "deploy"
    port: 22
    kind: "remote"
```

### Custom Polling Intervals

For high-frequency monitoring:
```bash
./warren-web -poll 100ms
```

For low-overhead monitoring:
```bash
./warren-web -poll 2s
```

### Custom Database Location

```bash
./warren-web -db /var/lib/warren/warren.db
```

## Next Steps

- Read [ROADMAP.md](../ROADMAP.md) for planned features
- See [design-review.md](../design-review.md) for architecture details
- Check [Phase 2 Completion Report](phase2-completion-report.md) for current status
