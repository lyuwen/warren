# Warren - Layered Architecture

## Three-Layer Design

```
┌─────────────────────────────────────────────────┐
│           User Interaction Layer                │
│                                                 │
│  ┌──────────────┐         ┌─────────────────┐  │
│  │  Mobile App  │         │  Desktop TUI    │  │
│  │  (iOS/Android│         │  (Terminal UI)  │  │
│  │   or PWA)    │         │                 │  │
│  └──────┬───────┘         └────────┬────────┘  │
│         │                          │           │
└─────────┼──────────────────────────┼───────────┘
          │                          │
          │    API/IPC/Commands      │
          │                          │
┌─────────▼──────────────────────────▼───────────┐
│              Warren Core                        │
│                                                 │
│  - Agent registry management                   │
│  - Server configuration                        │
│  - SSH connection handling                     │
│  - Tmux session control                        │
│  - State synchronization                       │
│  - CLI commands (tunnel, dig, colony, etc.)    │
│                                                 │
└─────────────────┬───────────────────────────────┘
                  │
                  │ SSH
                  │
       ┌──────────┴──────────┬────────────┐
       │                     │            │
   ┌───▼────┐          ┌────▼───┐   ┌───▼────┐
   │Server A│          │Server B│   │Server C│
   │ tmux   │          │ tmux   │   │ tmux   │
   │ agents │          │ agents │   │ agents │
   └────────┘          └────────┘   └────────┘
```

## Layer 1: Warren Core (Foundation)

**What it is:**
- Python CLI tool/library
- Manages distributed workspace state
- Handles SSH + tmux operations
- Can be used standalone or as a library

**Responsibilities:**
- Server configuration management
- Agent registry (what's running where)
- SSH connection pooling
- Tmux session lifecycle (create, attach, kill)
- State synchronization with remote reality
- Command execution

**Interface:**
```python
# As a library
from warren import Warren

warren = Warren()
warren.dig_agent("backend-api", server="prod-2")
warren.tunnel_agent("backend-api")
agents = warren.list_agents()
status = warren.get_agent_status("backend-api")
```

```bash
# As a CLI
warren dig backend-api --server prod-2
warren tunnel backend-api
warren colony
warren status backend-api
```

**Storage:**
- `~/.warren/config.yaml` - server definitions
- `~/.warren/registry.json` - agent state
- `~/.warren/ssh-control/` - SSH connection sockets

## Layer 2a: Desktop TUI (Terminal Interface)

**What it is:**
- Rich terminal UI for desktop/laptop use
- Interactive, keyboard-driven
- Runs in your terminal

**Technology:**
- Python with `textual` or `rich` + `prompt_toolkit`
- Full-screen terminal application
- Keyboard shortcuts and mouse support

**Features:**

**Main dashboard:**
```
┌─ Warren 🐇 ────────────────────────────────────────────────┐
│                                                             │
│  Servers (3)              Agents (5)                        │
│  ┌─────────────────┐     ┌──────────────────────────────┐  │
│  │ ● prod-1        │     │ ● backend-api    prod-1  2d  │  │
│  │ ● prod-2        │     │ ● ml-trainer     gpu-box 5h  │  │
│  │ ● gpu-box       │     │ ● frontend       prod-2  1d  │  │
│  └─────────────────┘     │ ⚠ worker-3       prod-1  12h │  │
│                          │ ✗ scraper        prod-2  -   │  │
│                          └──────────────────────────────┘  │
│                                                             │
│  Recent Activity                                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ 10:30  Created agent ml-trainer on gpu-box          │  │
│  │ 10:25  Attached to backend-api                      │  │
│  │ 10:20  Agent scraper died on prod-2                 │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  [t]unnel [d]ig [k]ill [r]efresh [q]uit                   │
└─────────────────────────────────────────────────────────────┘
```

**Agent detail view:**
```
┌─ Agent: backend-api ───────────────────────────────────────┐
│                                                             │
│  Server: prod-1                    Status: ● Running        │
│  Session: warren-backend-api       Uptime: 2d 3h 45m       │
│  PID: 12345                        CPU: 15%  Mem: 2.3GB    │
│                                                             │
│  Recent Output (last 20 lines)                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ [10:45:23] Processing request...                     │  │
│  │ [10:45:24] ✓ Request completed                       │  │
│  │ [10:45:25] Waiting for next task...                  │  │
│  │                                                       │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  [a]ttach [l]ogs [r]estart [k]ill [←]back                 │
└─────────────────────────────────────────────────────────────┘
```

**Keyboard shortcuts:**
- `t` - Tunnel to selected agent
- `d` - Dig new agent
- `k` - Kill selected agent
- `r` - Refresh status
- `l` - View logs
- `s` - Sync with remote
- `q` - Quit
- `↑↓` - Navigate
- `Enter` - Select/drill down

**Launch:**
```bash
warren tui
# or just
warren
# (TUI as default interface)
```

## Layer 2b: Mobile App (On-the-Go Interface)

**What it is:**
- Native mobile app or PWA
- Touch-friendly interface
- For checking status and basic control while mobile

**Technology Options:**

**Option A: Progressive Web App (PWA)**
- React/Vue + Tailwind
- Installable on home screen
- Works offline (cached state)
- No app store needed

**Option B: React Native**
- True native app
- Better performance
- Push notifications
- App store distribution

**Option C: Flutter**
- Cross-platform (iOS + Android)
- Single codebase
- Native performance

**Recommendation**: Start with PWA, migrate to React Native if needed

**Features:**

**Dashboard (mobile):**
```
┌─────────────────────────┐
│ Warren 🐇          [⚙]  │
├─────────────────────────┤
│                         │
│ ● backend-api           │
│   prod-1 • 2d           │
│   Last: Processing...   │
│                         │
│ ● ml-trainer            │
│   gpu-box • 5h          │
│   Last: Training epoch  │
│                         │
│ ⚠ worker-3              │
│   prod-1 • 12h          │
│   Last: Idle            │
│                         │
│ ✗ scraper               │
│   prod-2 • crashed      │
│   Last: Error 500       │
│                         │
│         [+ New]         │
│                         │
└─────────────────────────┘
```

**Agent detail (mobile):**
```
┌─────────────────────────┐
│ ← backend-api           │
├─────────────────────────┤
│ Server: prod-1          │
│ Status: ● Running       │
│ Uptime: 2d 3h 45m       │
│                         │
│ Recent Output:          │
│ ┌─────────────────────┐ │
│ │ Processing...       │ │
│ │ ✓ Completed         │ │
│ │ Waiting...          │ │
│ └─────────────────────┘ │
│                         │
│ [View Full Logs]        │
│                         │
│ [Restart]    [Kill]     │
│                         │
└─────────────────────────┘
```

**Mobile-specific features:**
- Pull to refresh
- Swipe actions (swipe left to kill, right to restart)
- Push notifications (agent crashed, etc.)
- Quick actions widget
- Offline mode (show cached state)

## Communication Between Layers

### Desktop TUI ↔ Warren Core
**Direct Python API calls** (same process)
```python
# TUI imports Warren as library
from warren import Warren
warren = Warren()
agents = warren.list_agents()  # Direct function call
```

### Mobile App ↔ Warren Core
**Two options:**

**Option 1: Warren Server (API wrapper)**
```
Mobile App → HTTPS/WebSocket → Warren Server → Warren Core → SSH → Tmux
```

Warren Server is a thin FastAPI wrapper around Warren Core:
```python
from fastapi import FastAPI
from warren import Warren

app = FastAPI()
warren = Warren()

@app.get("/agents")
def list_agents():
    return warren.list_agents()

@app.post("/agents/{name}/tunnel")
def tunnel_agent(name: str):
    # Can't actually tunnel from mobile
    # But can show logs, send commands, etc.
    return warren.get_agent_output(name)
```

**Option 2: SSH + Warren CLI (no server)**
```
Mobile App → SSH → Remote machine → warren CLI → Warren Core
```

Mobile app uses SSH client to run Warren commands remotely:
```javascript
// Mobile app
const ssh = new SSHClient();
ssh.connect(warrenHost);
const result = ssh.exec('warren colony --json');
const agents = JSON.parse(result);
```

**Recommendation**: Option 1 (Warren Server) for better mobile UX
- Real-time updates via WebSocket
- No need for SSH client on mobile
- Can add authentication/authorization
- Better for push notifications

## Deployment Scenarios

### Scenario 1: Personal laptop + mobile
```
Laptop: Warren Core + TUI (primary interface)
Phone: Mobile app → Warren Server (running on laptop or VPS)
```

### Scenario 2: Multiple computers + mobile
```
Desktop: Warren Core + TUI
Laptop: Warren Core + TUI
Phone: Mobile app → Warren Server (on VPS)

All share state via:
- Git sync of ~/.warren/ directory, OR
- Warren Server as central state store
```

### Scenario 3: Mobile-only (rare)
```
Phone: Mobile app → Warren Server (on VPS)
Warren Server manages all agents
```

## Development Phases

### Phase 1: Warren Core (Foundation)
- [ ] Server configuration
- [ ] Agent registry
- [ ] SSH connection management
- [ ] Tmux session control
- [ ] Core commands (dig, tunnel, colony, status, kill)
- [ ] State sync

### Phase 2: Desktop TUI
- [ ] Main dashboard
- [ ] Agent list view
- [ ] Agent detail view
- [ ] Keyboard navigation
- [ ] Real-time updates
- [ ] Log viewer

### Phase 3: Warren Server (API Layer)
- [ ] REST API for agent operations
- [ ] WebSocket for real-time updates
- [ ] Authentication
- [ ] State management

### Phase 4: Mobile App
- [ ] Agent list view
- [ ] Agent detail view
- [ ] Basic controls (restart, kill)
- [ ] Log viewing
- [ ] Push notifications
- [ ] Offline support

## Technology Stack Summary

| Layer | Technology | Why |
|-------|-----------|-----|
| Warren Core | Python 3.11+ | Rich ecosystem for SSH/tmux |
| | paramiko/asyncssh | SSH connections |
| | libtmux | Tmux control |
| | click | CLI framework |
| | pydantic | Config validation |
| Desktop TUI | textual | Modern Python TUI framework |
| | rich | Beautiful terminal output |
| Warren Server | FastAPI | Async, WebSocket, fast |
| | uvicorn | ASGI server |
| Mobile App | React + Vite | Fast, modern |
| | Tailwind CSS | Utility-first styling |
| | Capacitor | PWA → native wrapper |

## Open Questions

1. **State synchronization**: How do multiple Warren instances (laptop + desktop) share state?
   - Git sync of ~/.warren/?
   - Central Warren Server as source of truth?
   - Conflict resolution strategy?

2. **Mobile app deployment**: Where does Warren Server run?
   - User's laptop (with tunnel)?
   - VPS?
   - Home server?

3. **Authentication**: How does mobile app authenticate?
   - API tokens?
   - OAuth?
   - SSH key-based?

4. **Real-time updates**: How fast do we need updates?
   - TUI: instant (local)
   - Mobile: 5-10 seconds acceptable?

5. **Offline capabilities**: What works without connection?
   - TUI: everything (direct SSH)
   - Mobile: view cached state only?
