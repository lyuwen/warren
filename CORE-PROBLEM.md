# Warren - Core Problem Statement

## The Real Problem

**You have a complex distributed workspace:**
- Multiple remote servers
- Multiple coding agents (Claude Code instances) running on these servers
- Each agent runs inside a tmux session for persistence/high availability
- Agents keep running even when you disconnect

**Current pain points:**
- Managing which agent is on which server
- Remembering tmux session names
- Switching between agents requires: remember server → SSH → remember session name → tmux attach
- No unified view of what's running where
- Manual bookkeeping of your workspace topology

## What Warren Actually Needs to Solve

**Warren is a workspace organizer for distributed tmux-based coding agents.**

Not about:
- ❌ Mobile-first design
- ❌ Web dashboards
- ❌ Complex orchestration
- ❌ Multi-user collaboration

Actually about:
- ✅ Organizing your remote workspace
- ✅ Quick access to any agent on any server
- ✅ Remembering what's running where
- ✅ Simple commands to jump between agents
- ✅ High availability through tmux persistence

## Core Use Cases

### 1. Jump to an agent
```bash
warren tunnel backend-api
# Warren knows: backend-api is on prod-server-2, session name is warren-backend-api
# Automatically: SSH + tmux attach in one command
```

### 2. See what's running
```bash
warren colony
# Shows all agents across all servers
# agent-1 → server-a → running
# agent-2 → server-b → running
# agent-3 → server-c → dead
```

### 3. Start a new agent
```bash
warren dig ml-trainer --server gpu-box
# Creates tmux session on gpu-box
# Launches Claude Code (or whatever agent)
# Records it in Warren's registry
```

### 4. Check agent status
```bash
warren status ml-trainer
# Is the tmux session alive?
# What server is it on?
# How long has it been running?
```

## Architecture (Simplified)

```
┌─────────────────────────────────────┐
│  Warren CLI (local machine)         │
│  - Config: which servers exist      │
│  - Registry: which agents are where │
│  - Commands: tunnel, dig, colony    │
└──────────────┬──────────────────────┘
               │
               │ SSH (standard)
               │
    ┌──────────┴──────────┬────────────┐
    │                     │            │
┌───▼────┐          ┌────▼───┐   ┌───▼────┐
│Server A│          │Server B│   │Server C│
│        │          │        │   │        │
│ tmux   │          │ tmux   │   │ tmux   │
│ └─agent│          │ └─agent│   │ └─agent│
│   └─agent│        │        │   │ └─agent│
└────────┘          └────────┘   └────────┘
```

**That's it.** No server component, no web UI, no API. Just a CLI that:
1. Knows your server topology
2. Tracks which tmux sessions are your agents
3. Makes it easy to jump between them

## Core Data Model

### Warren Registry (`~/.warren/registry.json`)
```json
{
  "agents": {
    "backend-api": {
      "server": "prod-2",
      "tmux_session": "warren-backend-api",
      "created": "2026-05-08T10:00:00Z"
    },
    "ml-trainer": {
      "server": "gpu-box",
      "tmux_session": "warren-ml-trainer",
      "created": "2026-05-08T11:30:00Z"
    }
  }
}
```

### Server Config (`~/.warren/servers.yaml`)
```yaml
servers:
  prod-2:
    host: prod2.example.com
    user: deploy
    
  gpu-box:
    host: gpu.example.com
    user: ml
    port: 2222
```

## Core Commands

```bash
# Setup
warren server add prod-2 prod2.example.com
warren server list

# Agent lifecycle
warren dig <name> --server <server>     # Create agent in tmux
warren tunnel <name>                    # SSH + tmux attach
warren colony                           # List all agents
warren status <name>                    # Check if alive
warren kill <name>                      # Kill tmux session

# Utility
warren sync                             # Reconcile registry with reality
warren which <name>                     # Show which server has this agent
```

## Implementation Focus

**Phase 1: Absolute minimum**
- Config file for servers
- Registry file for agents
- `warren tunnel` - jump to agent
- `warren colony` - list agents
- `warren dig` - create agent

**Phase 2: Polish**
- `warren sync` - detect drift
- Better error messages
- Tab completion
- Aliases/shortcuts

**Phase 3: Nice-to-have**
- Templates for common agent setups
- Log viewing without attaching
- Health checks

## Key Design Principles

1. **Simple local state** - Just JSON/YAML files, no database
2. **Leverage existing tools** - SSH, tmux, nothing fancy
3. **Fail gracefully** - If registry is wrong, detect and fix
4. **Fast** - Jumping to an agent should be instant
5. **Transparent** - User can always bypass Warren and use SSH/tmux directly

## What Warren Is NOT

- Not a process manager (systemd does that)
- Not a deployment tool (use rsync/git for that)
- Not a monitoring system (use proper monitoring)
- Not a multi-user system (single user, their workspace)
- Not a web service (just a CLI)

## Success Metric

Warren succeeds when:
- You stop manually SSHing and remembering session names
- `warren tunnel agent-name` becomes muscle memory
- You can see your entire distributed workspace at a glance
- Setting up a new agent takes one command instead of five

## Next Steps

1. Define exact config file formats
2. Implement basic SSH + tmux wrapper
3. Build registry management
4. Create core commands (tunnel, colony, dig)
5. Test with real Claude Code agents
