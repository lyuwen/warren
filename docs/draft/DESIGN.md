# Warren - Design Document

## Big Picture

Warren is a central management system for distributed remote agents that live in tmux sessions across multiple servers. Think of it as a control plane for your agent infrastructure.

## Problem Statement

**Current pain points:**
- Managing multiple remote agents across different servers is tedious
- SSH + tmux commands are repetitive and error-prone
- No unified view of all running agents
- Hard to coordinate multi-agent workflows
- Agent state is scattered across servers

**What Warren solves:**
- Single command-line interface to manage all agents
- Automatic tmux session management
- Centralized monitoring and status
- Easy deployment and lifecycle management
- Persistent agent sessions that survive disconnections

## Architecture Overview

```
┌─────────────────────────────────────────┐
│         Warren CLI (Local)              │
│  - Command interface                    │
│  - Configuration management             │
│  - State tracking                       │
└──────────────┬──────────────────────────┘
               │
               │ SSH connections
               │
    ┌──────────┴──────────┬───────────────┐
    │                     │               │
┌───▼────┐          ┌────▼───┐      ┌───▼────┐
│Server 1│          │Server 2│      │Server 3│
│        │          │        │      │        │
│ tmux   │          │ tmux   │      │ tmux   │
│ ├─agent│          │ ├─agent│      │ ├─agent│
│ ├─agent│          │ └─agent│      │ ├─agent│
│ └─agent│          │        │      │ └─agent│
└────────┘          └────────┘      └────────┘
```

## Core Components

### 1. Warren CLI (Local Control Plane)
- **Configuration**: Server definitions, agent templates, credentials
- **State Management**: Track which agents are running where
- **Command Router**: Dispatch commands to appropriate servers
- **Session Manager**: Handle SSH connections and tmux interactions

### 2. Remote Agents (Distributed Workers)
- **Tmux Sessions**: Each agent runs in isolated tmux session
- **Agent Process**: The actual workload (script, service, daemon)
- **Lifecycle**: Start, stop, restart, monitor
- **Logs**: Captured within tmux for inspection

### 3. Communication Layer
- **SSH**: Secure connections to remote servers
- **Tmux Protocol**: Session creation, attachment, command execution
- **Status Polling**: Health checks and monitoring

## Key Features

### Phase 1: Core Functionality
- [ ] Define servers in configuration
- [ ] Deploy agent to remote server in tmux
- [ ] List all agents across colony
- [ ] Attach to agent tmux session
- [ ] Stop/kill agent
- [ ] View agent status

### Phase 2: Enhanced Management
- [ ] Agent templates (predefined configurations)
- [ ] Log aggregation and viewing
- [ ] Health monitoring and alerts
- [ ] Restart policies (auto-restart on failure)
- [ ] Resource usage tracking

### Phase 3: Coordination
- [ ] Multi-agent workflows
- [ ] Agent-to-agent communication
- [ ] Dependency management
- [ ] Distributed task execution

## Data Model

### Server
```yaml
name: production-1
host: prod1.example.com
port: 22
user: deploy
ssh_key: ~/.ssh/id_rsa
```

### Agent
```yaml
id: worker-1
name: data-processor
server: production-1
tmux_session: warren-worker-1
command: python process.py
working_dir: /opt/agents
status: running
pid: 12345
started_at: 2026-05-08T10:30:00Z
```

### Warren Config
```yaml
warren_home: ~/.warren
servers:
  - production-1
  - production-2
  - staging-1
agents:
  - worker-1
  - worker-2
  - monitor-1
```

## Command Interface

```bash
# Server management
warren server add <name> <host>
warren server list
warren server remove <name>

# Agent lifecycle
warren dig <agent-name> <server>        # Create/deploy new agent
warren tunnel <agent-name>              # Attach to agent tmux session
warren status [agent-name]              # Show agent status
warren colony                           # List all agents
warren stop <agent-name>                # Stop agent
warren restart <agent-name>             # Restart agent
warren burrow <agent-name> <command>    # Deploy agent with command

# Monitoring
warren logs <agent-name>                # View agent logs
warren health                           # Health check all agents
warren top                              # Resource usage overview
```

## Technical Decisions

### Language
**Python** - Good SSH/subprocess libraries, easy scripting, wide adoption

### Key Libraries
- `paramiko` or `fabric` - SSH connections
- `libtmux` - Tmux session management
- `click` - CLI framework
- `pyyaml` - Configuration parsing
- `rich` - Terminal UI/formatting

### Storage
- **Local config**: `~/.warren/config.yaml`
- **State file**: `~/.warren/state.json`
- **Logs**: `~/.warren/logs/<agent-name>.log`

### Security
- SSH key-based authentication
- No password storage
- SSH agent forwarding support
- Per-server credential isolation

## Open Questions

1. **Agent discovery**: Should Warren auto-discover existing tmux sessions?
2. **Conflict resolution**: What if tmux session name already exists?
3. **Network failures**: How to handle SSH disconnections gracefully?
4. **Agent communication**: Do agents need to talk to each other directly?
5. **Scaling**: How many agents/servers should Warren reasonably support?
6. **Monitoring backend**: Should we integrate with existing monitoring (Prometheus, etc.)?

## Success Metrics

Warren is successful when:
- Deploying an agent takes one command instead of 5+ manual steps
- You can see all agent status at a glance
- Attaching to any agent is instant and intuitive
- Agent management feels playful and delightful, not tedious
- The system is reliable enough to trust for production workloads

## Next Steps

1. Define the configuration file format
2. Build basic SSH connection manager
3. Implement tmux session creation/attachment
4. Create core CLI commands (dig, tunnel, colony, status)
5. Add state persistence
6. Build monitoring and health checks
