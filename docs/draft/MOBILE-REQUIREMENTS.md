# Warren - Mobile Use Case & Requirements

## The Real Problem

**Backend**: Multiple Claude Code agents running in tmux sessions across different remote servers

**Frontend**: 
- Desktop: SSH from various computers (works fine)
- Mobile: Phone with limited screen space (current pain point)

**User behavior**: Constantly on the move, needs to interact with coding agents from phone

## Why This Matters

Traditional tmux over SSH on mobile is **terrible**:
- Tiny terminal text
- No touch-friendly controls
- Keyboard shortcuts don't work well
- Hard to see agent status at a glance
- Difficult to read code/logs on small screen
- Switching between agents is clunky

## What Warren Needs to Be

Not just a CLI tool - Warren needs a **mobile-friendly interface** for managing remote coding agents.

### Core Mobile Use Cases

1. **Quick status check**: "Are my agents running? Any errors?"
2. **Read agent output**: "What did the agent just do?"
3. **Send commands**: "Tell agent-3 to run tests"
4. **Monitor progress**: "Is the build still running?"
5. **Emergency control**: "Stop agent-2, it's doing something wrong"
6. **Switch context**: "Jump to agent-5 on server-prod2"

### Mobile Interaction Patterns

**Dashboard view** (primary screen):
```
┌─────────────────────────┐
│ Warren 🐇              ⚙│
├─────────────────────────┤
│ ● agent-1    prod1   2h │
│   Building frontend...  │
│                         │
│ ● agent-2    prod2   5m │
│   Running tests...      │
│                         │
│ ⚠ agent-3    staging 1d │
│   Idle                  │
│                         │
│ ✗ agent-4    dev     -  │
│   Crashed               │
│                         │
│         [+ New Agent]   │
└─────────────────────────┘
```

**Agent detail view** (tap on agent):
```
┌─────────────────────────┐
│ ← agent-1          prod1│
├─────────────────────────┤
│ Status: ● Running       │
│ Uptime: 2h 34m          │
│ PID: 12345              │
├─────────────────────────┤
│ Recent Output:          │
│ ┌─────────────────────┐ │
│ │ ✓ Tests passed      │ │
│ │ Building...         │ │
│ │ [===>    ] 45%      │ │
│ └─────────────────────┘ │
├─────────────────────────┤
│ [View Full Logs]        │
│ [Send Command]          │
│ [Attach (SSH)]          │
│ [Restart]  [Stop]       │
└─────────────────────────┘
```

**Command input** (mobile-friendly):
```
┌─────────────────────────┐
│ Send to agent-1         │
├─────────────────────────┤
│ Quick Commands:         │
│ [Run Tests]  [Build]    │
│ [Deploy]     [Status]   │
│                         │
│ Or type custom:         │
│ ┌─────────────────────┐ │
│ │ /help              │ │
│ └─────────────────────┘ │
│                         │
│        [Send] [Cancel]  │
└─────────────────────────┘
```

## Architecture Revision

Warren needs **two components**:

### 1. Warren Server (runs on accessible host)
- Central coordination service
- Manages SSH connections to remote servers
- Maintains agent state
- Provides API for clients
- WebSocket for real-time updates

### 2. Warren Clients
- **CLI** (for desktop/laptop)
- **Web UI** (for mobile browser)
- **Mobile app** (future: native iOS/Android)

```
┌──────────────┐
│ Phone/Tablet │
│  (Web UI)    │
└──────┬───────┘
       │ HTTPS/WebSocket
       │
┌──────▼───────────────────┐
│   Warren Server          │
│   - API                  │
│   - WebSocket hub        │
│   - SSH connection pool  │
│   - State management     │
└──────┬───────────────────┘
       │ SSH
       │
   ┌───┴────┬────────┬──────┐
   │        │        │      │
┌──▼──┐ ┌──▼──┐ ┌───▼─┐ ┌──▼──┐
│prod1│ │prod2│ │stag │ │dev  │
│tmux │ │tmux │ │tmux │ │tmux │
│agent│ │agent│ │agent│ │agent│
└─────┘ └─────┘ └─────┘ └─────┘
```

## Technical Stack Revision

### Warren Server
- **Framework**: FastAPI (Python) - async, WebSocket support
- **SSH**: paramiko or asyncssh
- **Tmux**: libtmux
- **State**: SQLite or JSON file
- **Auth**: JWT tokens, optional OAuth

### Warren Web UI
- **Framework**: React or Vue (mobile-responsive)
- **UI Library**: Tailwind CSS + shadcn/ui
- **Real-time**: WebSocket client
- **PWA**: Installable on phone home screen

### Warren CLI
- **Framework**: Click (Python)
- **API Client**: httpx
- **Fallback**: Direct SSH if server unavailable

## API Design

### REST Endpoints

```
GET    /api/agents                    # List all agents
GET    /api/agents/:id                # Get agent details
POST   /api/agents                    # Create new agent
DELETE /api/agents/:id                # Stop/remove agent
POST   /api/agents/:id/restart        # Restart agent
POST   /api/agents/:id/command        # Send command to agent
GET    /api/agents/:id/logs           # Get agent logs
GET    /api/agents/:id/logs/stream    # Stream logs (SSE)

GET    /api/servers                   # List servers
POST   /api/servers                   # Add server
GET    /api/servers/:id/health        # Server health check

GET    /api/templates                 # List templates
POST   /api/agents/from-template      # Create from template
```

### WebSocket Events

```javascript
// Client → Server
{
  "type": "subscribe",
  "agent_id": "worker-1"
}

{
  "type": "command",
  "agent_id": "worker-1",
  "command": "/test"
}

// Server → Client
{
  "type": "agent_status",
  "agent_id": "worker-1",
  "status": "running",
  "uptime": 7200
}

{
  "type": "agent_output",
  "agent_id": "worker-1",
  "output": "✓ Tests passed\n"
}

{
  "type": "agent_died",
  "agent_id": "worker-1",
  "reason": "process exited"
}
```

## Mobile-Specific Features

### 1. Notifications
- Push notifications when agent status changes
- "Agent crashed" alert
- "Build completed" notification
- Configurable per agent

### 2. Quick Actions
- Swipe gestures (swipe right to restart, left to stop)
- Long-press for context menu
- Shake to refresh (mobile app)

### 3. Optimized Display
- Syntax highlighting for code (but readable on small screen)
- Collapsible sections
- Infinite scroll for logs
- Pull-to-refresh

### 4. Offline Support
- Cache last known state
- Queue commands when offline
- Sync when connection restored

### 5. Voice Commands (future)
- "Warren, status of agent 1"
- "Warren, restart agent 3"
- "Warren, show me the logs"

## Deployment Options

### Option A: Self-hosted Warren Server
```bash
# On a server you control (e.g., home server, VPS)
warren-server start --host 0.0.0.0 --port 8080
# Access from anywhere: https://warren.yourdomain.com
```

### Option B: Local Warren Server + Tunnel
```bash
# On your laptop
warren-server start --local
# Use ngrok/tailscale for mobile access
ngrok http 8080
```

### Option C: Serverless Warren
- Deploy to Cloudflare Workers / Vercel
- State in Redis/Upstash
- SSH via WebSocket proxy

## Security Considerations

### Authentication
- Password + 2FA for web UI
- API tokens for CLI
- Biometric unlock on mobile

### Authorization
- Per-user agent access control
- Read-only vs full-control permissions
- Audit log of all actions

### Network Security
- HTTPS only
- WebSocket over TLS
- SSH keys never leave server
- Optional: VPN/Tailscale requirement

## User Flows

### Flow 1: Check status on phone
1. Open Warren web UI on phone
2. See dashboard with all agents
3. Green dots = running, red = problems
4. Tap agent to see details
5. Read recent output
6. Done (30 seconds)

### Flow 2: Respond to alert
1. Get push notification: "agent-3 crashed"
2. Tap notification → opens Warren
3. See error in logs
4. Tap "Restart" button
5. Confirm agent is running
6. Done (1 minute)

### Flow 3: Send command from phone
1. Open Warren
2. Tap agent-1
3. Tap "Send Command"
4. Type or select quick command
5. See command execute in real-time
6. Done (45 seconds)

### Flow 4: Monitor long-running task
1. Start build on agent-2
2. Subscribe to updates
3. Put phone in pocket
4. Get notification when complete
5. Review results later

## MVP Feature Set

**Phase 1: Core Server + Basic Web UI**
- [ ] Warren server with REST API
- [ ] Agent CRUD operations
- [ ] SSH connection management
- [ ] Tmux session control
- [ ] Basic web UI (list agents, view status)
- [ ] Real-time updates via WebSocket
- [ ] Mobile-responsive design

**Phase 2: Enhanced Mobile Experience**
- [ ] Log streaming
- [ ] Command sending
- [ ] Quick actions
- [ ] Push notifications
- [ ] PWA support (install on home screen)

**Phase 3: Advanced Features**
- [ ] Templates
- [ ] Multi-user support
- [ ] Agent coordination
- [ ] Metrics/monitoring
- [ ] CLI client

## Success Criteria

Warren succeeds when:
- You can check agent status from your phone in < 10 seconds
- You can restart a crashed agent from your phone in < 30 seconds
- You can read agent output comfortably on mobile
- You get notified immediately when something needs attention
- You rarely need to SSH from your phone anymore

## Open Questions

1. **Where to host Warren server?**
   - Home server? VPS? Laptop with tunnel?
   
2. **Authentication strategy?**
   - Simple password? OAuth? Tailscale auth?
   
3. **How much history to keep?**
   - Full logs? Last N lines? Time-based retention?
   
4. **Multi-device sync?**
   - Should CLI and web UI share state seamlessly?
   
5. **Agent interaction model?**
   - Send raw commands? Predefined actions? Both?

6. **Real-time requirements?**
   - How fast do updates need to be? (1s? 5s? 30s?)

7. **Offline capabilities?**
   - What should work without connection?
