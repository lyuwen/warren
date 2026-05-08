# Warren - Brainstorming & Design Exploration

## Deep Dive: Key Design Challenges

### 1. Agent Identity & Naming

**Challenge**: How do we uniquely identify agents across the warren?

**Options:**
- **UUID-based**: `warren-a3f2b1c4` - unique but not human-friendly
- **Name-based**: `data-processor` - friendly but collision-prone
- **Hybrid**: `data-processor-a3f2` - balance of both
- **Hierarchical**: `prod1/data-processor` - includes server context

**Recommendation**: Hybrid approach
- User provides friendly name
- Warren generates short hash suffix for uniqueness
- Tmux session name: `warren-{name}-{hash}`
- Display name shows just the friendly part unless collision

**Edge cases:**
- What if user wants exact name control? (flag: `--exact-name`)
- How to handle agent migration between servers? (keep ID, update location)

### 2. State Management

**Challenge**: Where does Warren store what it knows?

**Local state** (`~/.warren/state.json`):
```json
{
  "agents": {
    "worker-1-a3f2": {
      "name": "worker-1",
      "server": "prod1",
      "tmux_session": "warren-worker-1-a3f2",
      "command": "python process.py",
      "working_dir": "/opt/agents",
      "created_at": "2026-05-08T10:30:00Z",
      "last_seen": "2026-05-08T11:45:00Z",
      "status": "running"
    }
  },
  "servers": {
    "prod1": {
      "host": "prod1.example.com",
      "last_connected": "2026-05-08T11:45:00Z"
    }
  }
}
```

**Remote state** (on each server):
- Actual tmux sessions (source of truth)
- Optional: `~/.warren-agent/metadata.json` per agent

**Sync strategy:**
- Local state is a cache
- `warren sync` reconciles with remote reality
- Auto-sync on most commands
- Handle drift gracefully (remote session died, manual tmux kill, etc.)

**Questions:**
- Should Warren own the tmux session exclusively? (yes, with escape hatch)
- What if user manually creates/kills tmux sessions? (detect and warn)
- Distributed state? (future: etcd/consul for multi-user scenarios)

### 3. Connection Management

**Challenge**: Managing SSH connections efficiently

**Strategies:**

**Option A: Connection per command**
```python
# Simple but slow
def run_command(server, cmd):
    ssh = connect(server)
    result = ssh.exec(cmd)
    ssh.close()
    return result
```

**Option B: Connection pooling**
```python
# Reuse connections
connection_pool = {}
def get_connection(server):
    if server not in connection_pool:
        connection_pool[server] = connect(server)
    return connection_pool[server]
```

**Option C: SSH ControlMaster**
```bash
# Let SSH handle multiplexing
ssh -o ControlMaster=auto -o ControlPath=~/.warren/ssh-%r@%h:%p
```

**Recommendation**: Start with Option C (ControlMaster)
- Leverages battle-tested SSH multiplexing
- No connection management code
- Fast subsequent connections
- Automatic cleanup

**Implementation:**
```python
ssh_config = {
    'ControlMaster': 'auto',
    'ControlPath': '~/.warren/ssh-%r@%h:%p',
    'ControlPersist': '10m'
}
```

### 4. Tmux Session Lifecycle

**Challenge**: Creating, managing, and cleaning up tmux sessions

**Session creation flow:**
```
1. SSH to server
2. Check if tmux session exists
3. If exists: error or attach?
4. Create new tmux session with name
5. Send command to session
6. Detach (session runs in background)
7. Record in local state
```

**Session attachment:**
```
1. Look up agent in local state
2. SSH to server
3. Attach to tmux session (interactive)
4. On detach: return to Warren CLI
```

**Session cleanup:**
```
1. Send kill signal to process in tmux
2. Wait for graceful shutdown
3. If timeout: force kill
4. Kill tmux session
5. Remove from local state
```

**Tmux commands:**
```bash
# Create session
tmux new-session -d -s warren-worker-1 'python process.py'

# Check if exists
tmux has-session -t warren-worker-1

# Attach
tmux attach-session -t warren-worker-1

# Send command
tmux send-keys -t warren-worker-1 'some command' C-m

# Kill session
tmux kill-session -t warren-worker-1

# List sessions
tmux list-sessions

# Capture pane (for logs)
tmux capture-pane -t warren-worker-1 -p
```

### 5. Agent Discovery & Reconciliation

**Challenge**: What if Warren's state diverges from reality?

**Scenarios:**
- Warren thinks agent is running, but tmux session is dead
- Tmux session exists, but Warren doesn't know about it
- Server was rebooted, all sessions gone
- User manually killed tmux session

**Discovery mechanism:**
```bash
warren sync
# 1. For each server in config
# 2. List all tmux sessions matching 'warren-*'
# 3. Compare with local state
# 4. Report discrepancies
# 5. Offer to reconcile (adopt orphans, clean dead entries)
```

**Auto-discovery:**
```bash
warren discover prod1
# Find all warren-* sessions on prod1
# Import them into Warren state
# Useful for adopting existing agents
```

### 6. Logging & Output

**Challenge**: How to capture and view agent output?

**Options:**

**A. Tmux capture-pane**
```bash
# Pros: Simple, no extra setup
# Cons: Limited history (tmux buffer size)
tmux capture-pane -t warren-worker-1 -p -S -1000
```

**B. Redirect to file**
```bash
# Pros: Unlimited history, persistent
# Cons: Need to manage log files, rotation
python process.py > /var/log/warren/worker-1.log 2>&1
```

**C. Hybrid approach**
```bash
# Best of both worlds
python process.py 2>&1 | tee /var/log/warren/worker-1.log
```

**Recommendation**: Hybrid with log rotation
```python
command = f"{user_command} 2>&1 | tee -a ~/.warren-agent/{agent_id}.log"
tmux_cmd = f"tmux new-session -d -s {session_name} '{command}'"
```

**Log viewing:**
```bash
warren logs worker-1           # tail -f the log file
warren logs worker-1 --lines 100  # last 100 lines
warren logs worker-1 --follow     # follow mode
```

### 7. Agent Templates

**Challenge**: Make common agent patterns reusable

**Template definition** (`~/.warren/templates/python-worker.yaml`):
```yaml
name: python-worker
description: Python worker process with virtual environment
command: |
  source venv/bin/activate
  python {{ script }}
working_dir: /opt/agents
environment:
  PYTHONUNBUFFERED: "1"
  LOG_LEVEL: "INFO"
restart_policy: on-failure
health_check:
  command: "curl -f http://localhost:{{ port }}/health"
  interval: 30s
```

**Usage:**
```bash
warren dig my-worker --template python-worker \
  --var script=process.py \
  --var port=8080 \
  --server prod1
```

**Built-in templates:**
- `python-worker` - Python process with venv
- `node-service` - Node.js service
- `shell-script` - Simple bash script
- `docker-container` - Docker container runner
- `cron-job` - Periodic task

### 8. Multi-Agent Coordination

**Challenge**: How do agents work together?

**Use cases:**
- Sequential workflows (agent A → agent B → agent C)
- Parallel execution (fan-out, fan-in)
- Leader election (one primary, multiple standby)
- Shared state (distributed cache, message queue)

**Coordination primitives:**

**A. Simple dependencies**
```yaml
# Agent config
name: processor
depends_on:
  - fetcher  # Wait for fetcher to be running
```

**B. Message passing**
```bash
# Agent A sends message
warren send processor "data ready"

# Agent B receives
warren receive processor --wait
```

**C. Shared state**
```bash
# Set shared value
warren set-var "last_processed_id" "12345"

# Get shared value
warren get-var "last_processed_id"
```

**D. Workflow orchestration** (future)
```yaml
workflow: data-pipeline
steps:
  - name: fetch
    agent: fetcher
    server: prod1
  - name: process
    agent: processor
    server: prod2
    depends_on: [fetch]
  - name: upload
    agent: uploader
    server: prod3
    depends_on: [process]
```

### 9. Security & Access Control

**Challenge**: Keep the warren secure

**Authentication:**
- SSH key-based only (no passwords)
- Support for SSH agent forwarding
- Per-server key configuration
- Optional: SSH bastion/jump host support

**Authorization:**
- File permissions on `~/.warren/` (0700)
- Remote agent directories (0700)
- Log files (0600)

**Secrets management:**
```bash
# Don't store secrets in Warren config
# Use environment variables or secret managers
warren dig worker --env-file .env.prod
warren dig worker --secret AWS_KEY=@aws-secret-manager:prod/api-key
```

**Audit logging:**
```bash
# Track all Warren operations
~/.warren/audit.log
2026-05-08T10:30:00Z user=lfu action=dig agent=worker-1 server=prod1
2026-05-08T10:35:00Z user=lfu action=tunnel agent=worker-1
2026-05-08T10:40:00Z user=lfu action=stop agent=worker-1
```

### 10. Error Handling & Resilience

**Challenge**: Things will go wrong

**Failure modes:**
- SSH connection fails
- Tmux not installed on remote
- Agent process crashes
- Server runs out of resources
- Network partition

**Resilience strategies:**

**Retry logic:**
```python
@retry(max_attempts=3, backoff=exponential)
def ssh_command(server, cmd):
    # Auto-retry transient failures
    pass
```

**Health checks:**
```bash
warren health
# Check each agent
# - Tmux session exists?
# - Process is running?
# - Custom health check passes?
# Report: healthy, degraded, dead
```

**Auto-restart:**
```yaml
agent:
  restart_policy: on-failure
  max_restarts: 3
  restart_delay: 10s
```

**Graceful degradation:**
- If can't reach server, show cached state
- Mark as "unknown" instead of failing
- Allow manual override/force commands

### 11. User Experience

**Challenge**: Make Warren delightful to use

**CLI design principles:**
- Commands should be intuitive (dig, tunnel, colony)
- Output should be beautiful (use `rich` for formatting)
- Feedback should be immediate
- Errors should be helpful

**Output examples:**

```bash
$ warren colony
┏━━━━━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━┓
┃ Agent      ┃ Server  ┃ Status  ┃ Uptime             ┃
┡━━━━━━━━━━━━╇━━━━━━━━━╇━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━┩
│ worker-1   │ prod1   │ ✓ Running│ 2d 3h 45m         │
│ processor  │ prod2   │ ✓ Running│ 1d 12h 30m        │
│ monitor    │ staging │ ⚠ Degraded│ 3h 15m           │
│ fetcher    │ prod1   │ ✗ Dead   │ -                 │
└────────────┴─────────┴─────────┴────────────────────┘
```

```bash
$ warren dig data-processor --server prod1
🐇 Digging burrow for data-processor on prod1...
   ✓ Connected to prod1
   ✓ Created tmux session warren-data-processor-a3f2
   ✓ Started process (PID 12345)
   ✓ Agent is running

🎉 data-processor is ready!
   Attach: warren tunnel data-processor
   Logs:   warren logs data-processor
   Stop:   warren stop data-processor
```

**Interactive mode:**
```bash
warren shell
warren> dig worker-1 prod1
warren> status worker-1
warren> tunnel worker-1
# ... interactive tmux session ...
warren> logs worker-1 --follow
warren> exit
```

**Progress indicators:**
- Spinners for long operations
- Progress bars for multi-step tasks
- Real-time status updates

### 12. Configuration Management

**Challenge**: Balance simplicity and flexibility

**Config file hierarchy:**
```
~/.warren/
├── config.yaml          # Global config
├── servers/
│   ├── prod1.yaml
│   ├── prod2.yaml
│   └── staging.yaml
├── templates/
│   ├── python-worker.yaml
│   └── node-service.yaml
├── state.json           # Runtime state
└── audit.log            # Audit trail
```

**Global config** (`~/.warren/config.yaml`):
```yaml
warren:
  home: ~/.warren
  log_level: INFO
  ssh:
    control_persist: 10m
    connect_timeout: 10s
  defaults:
    working_dir: /opt/agents
    restart_policy: never

servers:
  - prod1
  - prod2
  - staging

templates:
  - python-worker
  - node-service
```

**Server config** (`~/.warren/servers/prod1.yaml`):
```yaml
name: prod1
host: prod1.example.com
port: 22
user: deploy
ssh_key: ~/.ssh/id_rsa_prod
jump_host: bastion.example.com  # Optional
tags:
  - production
  - us-east-1
```

**Environment-specific overrides:**
```bash
# Use different config for different contexts
warren --config ~/.warren/work.yaml colony
warren --config ~/.warren/personal.yaml colony
```

### 13. Monitoring & Observability

**Challenge**: Know what's happening in the warren

**Metrics to track:**
- Agent count (total, running, dead)
- Uptime per agent
- Resource usage (CPU, memory, disk)
- Error rates
- Command execution times

**Monitoring commands:**
```bash
warren health              # Overall health check
warren top                 # Resource usage (like htop)
warren stats               # Aggregate statistics
warren events              # Recent events/changes
```

**Integration with monitoring systems:**
```bash
# Export metrics for Prometheus
warren metrics --format prometheus > /var/lib/node_exporter/warren.prom

# Send to StatsD
warren metrics --statsd localhost:8125

# Webhook notifications
warren watch --webhook https://hooks.slack.com/... \
  --on agent_down,agent_restart
```

**Dashboard** (future):
```bash
warren dashboard
# Launch web UI at http://localhost:8080
# Real-time view of all agents
# Interactive controls
```

### 14. Testing Strategy

**Challenge**: How do we test Warren itself?

**Unit tests:**
- Config parsing
- State management
- Command generation
- Error handling

**Integration tests:**
- SSH connection mocking
- Tmux command execution
- End-to-end workflows

**Test environment:**
```bash
# Use Docker containers as fake servers
docker-compose up -d
# Creates test-server-1, test-server-2, test-server-3
# Each with SSH and tmux installed

# Run integration tests
pytest tests/integration/
```

**Manual testing:**
```bash
# Vagrant for local testing
vagrant up
warren server add test1 localhost:2222
warren dig test-worker --server test1
```

## Open Design Questions

### Q1: Should Warren support non-tmux agents?
**Scenario**: User wants to manage systemd services, Docker containers, or bare processes

**Options:**
- A: Tmux-only (keep it simple)
- B: Pluggable backends (tmux, systemd, docker, screen)
- C: Tmux as wrapper (run everything through tmux)

**Leaning towards**: C (tmux as universal wrapper)
- Consistent interface
- Unified logging/attachment
- Simpler implementation

### Q2: Multi-user support?
**Scenario**: Team wants to share Warren state

**Options:**
- A: Single-user only (current design)
- B: Shared state file with locking
- C: Central state server (API)
- D: Git-based state sync

**Leaning towards**: A for v1, D for future
- Start simple
- Git sync is familiar to developers
- Can evolve to C if needed

### Q3: Agent code deployment?
**Scenario**: User wants Warren to deploy code, not just run commands

**Options:**
- A: Out of scope (use rsync/git separately)
- B: Built-in deployment (`warren deploy`)
- C: Integration with deployment tools (Ansible, Fabric)

**Leaning towards**: A for v1, B for v2
- Focus on agent management first
- Deployment is a separate concern
- Can add later without breaking changes

### Q4: Windows support?
**Scenario**: User has Windows servers

**Options:**
- A: Linux/macOS only
- B: WSL support
- C: Native Windows (PowerShell remoting)

**Leaning towards**: A for v1
- Tmux is Unix-only
- WSL is possible but adds complexity
- Revisit if demand exists

### Q5: Agent communication protocol?
**Scenario**: Agents need to talk to each other

**Options:**
- A: No direct communication (orchestrate via Warren)
- B: Message queue (Redis, RabbitMQ)
- C: HTTP API per agent
- D: Shared filesystem

**Leaning towards**: A for v1, B for v2
- Keep agents simple
- Warren as coordinator
- Add messaging when use case is clear

## Next Steps in Brainstorming

1. **Prototype the core loop**: SSH → tmux → command execution
2. **Design the config file format** in detail
3. **Sketch the CLI command tree** with all flags/options
4. **Define the state schema** precisely
5. **Plan the error messages** (what does user see when things fail?)
6. **Create user stories** (day in the life of Warren user)
7. **Performance considerations** (how many agents can Warren handle?)
8. **Migration path** (how to adopt Warren for existing setups?)

## Ideas for Future Exploration

- **Warren Cloud**: Managed service for Warren state/coordination
- **Warren UI**: Web dashboard for visual management
- **Warren Plugins**: Extend Warren with custom commands
- **Warren Marketplace**: Share templates and workflows
- **Warren Agent SDK**: Library for building Warren-aware agents
- **Warren Swarm**: Multi-warren coordination (warren of warrens!)
