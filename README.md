# Warren

Warren is a central hub for supervising and interacting with distributed coding-agent sessions that run inside tmux across multiple local and remote servers.

## Project Status

**Phase 2 Implementation Complete** - Warren now provides a complete monitoring solution with TUI and web interfaces for observing agent sessions in real-time.

### What's New in Phase 2

- ✅ **Event Store** - SQLite-based event storage for activity tracking
- ✅ **Activity Parser** - Extracts chat, file operations, and tool usage from agent output
- ✅ **State Detection** - Enhanced agent state inference (idle, thinking, waiting, error, etc.) with time-decay and improved question detection
- ✅ **Conversation History** - Full conversation display from Claude Code sessions (TUI: press 'c', Web: Conversation tab)
- ✅ **Artifact Profiles** - Tracks files touched and repositories accessed
- ✅ **Notification Engine** - Alerts on actionable states (permission required, questions, errors)
- ✅ **Terminal UI** - Keyboard-driven interface with Bubble Tea and conversation viewer
- ✅ **Web Interface** - Real-time web dashboard with WebSocket updates and conversation display
- ✅ **Topology View** - Hierarchical visualization of tmux infrastructure with agent state enrichment and filtering (TUI: press 't', Web: GET /api/topology)

### Phase 2 Follow-up Fixes

- ✅ **SSH ConnectionPool** - Now connects to remote servers using SSH agent and key-based auth, with `known_hosts` host-key verification required
- ✅ **Conversation Update Subscriptions** - `ConversationService.SubscribeToUpdates` is implemented via polling (default 3s interval); call `Unsubscribe(agentID)` to stop a watcher or `Close()` to stop all
- ✅ **Registry save failures** are now logged instead of silently swallowed
- ✅ **`findRepoRoot`** now validates `.git` is a real repository (file or directory containing `HEAD`), avoiding false matches on stale `.git` entries

## Quick Start

### Build

```bash
# Build all binaries
make

# Or manually with go
go build -o warren ./cmd/warren
go build -o warren-tui ./cmd/warren-tui
go build -o warren-web ./cmd/warren-web
```

### Usage

**Prerequisites:** Tmux running with Claude Code agents in panes.

**Start Web Interface (Recommended):**
```bash
make run-web
# Or: ./warren-web -addr :8080
# Open browser: http://localhost:8080
```

**Start Terminal UI:**
```bash
make run-tui
# Or: ./warren-tui
# Navigate with ↑/↓, Enter, Tab, q
# Press 'c' to view conversation history for selected agent
# Press 't' to view topology (servers, sessions, windows, panes)
# Press 'f' in topology view to filter by server or agent state
```

**See [Getting Started Guide](docs/getting-started.md) for detailed instructions.**

## Topology View

Warren provides a hierarchical visualization of your tmux infrastructure with real-time agent state information.

**Terminal UI:**
```bash
# Start warren-tui and press 't' to view topology
./warren-tui

# Navigation:
# ↑/↓ - Navigate nodes
# Enter - Expand/collapse
# f - Toggle filter mode
# s - Filter by server name (in filter mode)
# a - Filter by agent state (in filter mode)
# r - Refresh topology
# t - Toggle back to session list
```

**Web API:**
```bash
# Get complete topology
curl http://localhost:8080/api/topology | jq '.'

# Get topology for specific server
curl http://localhost:8080/api/topology/servers/localhost | jq '.'

# Get topology for specific session
curl http://localhost:8080/api/topology/sessions/\$0 | jq '.'
```

**Features:**
- 📡 Hierarchical tree view (servers → sessions → windows → panes)
- 🎯 Real-time agent state enrichment (idle, thinking, error, etc.)
- 🔍 Filter by server name or agent state
- 🔄 Live updates with refresh
- 📊 JSON API for programmatic access

See [TUI Topology Guide](docs/tui-topology-guide.md) and [Topology API Documentation](docs/api-topology.md) for details.

**See [Getting Started Guide](docs/getting-started.md) for detailed instructions.**

**Phase 1 - Tmux Interface:**
```bash
# Discover tmux topology
./warren topology

# Capture content from a pane
./warren capture <pane-id>

# Send text to a pane
./warren send <pane-id> "your text here"
```

The TUI provides keyboard-driven navigation:
- ↑/↓: Navigate sessions
- Enter: View details
- c: View conversation history (from agent detail view)
- t: View topology (hierarchical tree of servers/sessions/windows/panes)
- f: Toggle filter mode in topology view
- n: Notifications
- q: Quit

The web interface provides real-time updates at http://localhost:8080
- Click on any agent to view details
- Switch to "Conversation" tab to see full conversation history
- GET /api/topology for programmatic access to topology data

## Phase 1 Implementation

Phase 1 validates the tmux interface model. The following components are implemented:

### Core Components

- **Server Model** (`internal/core/`)
  - Server entity with local/remote support
  - Server registry with YAML persistence
  - Connection pool for SSH connections (agent + key-based auth, `known_hosts` verification)

### Tmux Interface (`internal/tmux/`)

- **Topology Discovery**
  - List sessions, windows, and panes
  - Build complete topology tree: Server → Session → Window → Pane
  - Support for both local and remote execution

- **Pane Capture**
  - Capture pane content with configurable scrollback
  - ANSI escape sequence stripping
  - Configurable capture options

- **Pane Control**
  - Send text to panes
  - Send keystroke sequences (Enter, Tab, Ctrl+C, etc.)
  - Pane validation before operations

- **Control Loop**
  - High-level capture → validate → action → verify workflow
  - Built-in validators and verifiers
  - Helper methods for common operations (approve permission, answer questions)
  - State change detection

## Architecture

```
warren/
├── cmd/
│   ├── warren/              # CLI tool for Phase 1 testing
│   ├── warren-tui/          # Terminal UI (Phase 2)
│   └── warren-web/          # Web interface (Phase 2)
├── internal/
│   ├── core/                # Server model, agent sessions, artifact profiles
│   ├── events/              # Event store (SQLite)
│   ├── notifications/       # Notification engine
│   ├── parser/              # Activity parser
│   ├── state/               # State detection
│   ├── tmux/                # Tmux interface layer
│   ├── tui/                 # Terminal UI (Bubble Tea)
│   ├── types/               # Shared types (AgentState)
│   └── web/                 # Web server and API
└── docs/
    ├── CLAUDE.md            # Project guide
    ├── design-review.md     # Design specification
    ├── ROADMAP.md           # Implementation roadmap
    ├── phase2-status.md     # Phase 2 completion report
    └── security.md          # Security documentation
```

## Phase 2 Components

### Event Store (`internal/events/`)
- SQLite-based immutable event log
- Stores agent activity, state changes, and notifications
- Efficient querying by agent, time range, and event type

### Activity Parser (`internal/parser/`)
- Extracts structured data from agent output
- Detects chat messages, file operations, tool usage
- Identifies permission prompts and questions

### State Detection (`internal/state/`)
- Infers agent state from activity patterns
- States: idle, thinking, executing, waiting_permission, asking_question, finished, error, stopped
- Priority-based resolution for conflicting signals

### Artifact Profiles (`internal/core/`)
- Tracks files visited and edited by each agent
- Detects Git repositories automatically
- Provides statistics (reads, edits, writes)

### Notification Engine (`internal/notifications/`)
- Triggers on actionable states
- Immutable event pattern for consumption tracking
- Real-time notification channel

### Terminal UI (`internal/tui/`)
- Built with Bubble Tea framework
- Three views: session list, agent detail, notifications
- Keyboard-driven navigation
- Real-time updates (500ms polling)

### Web Interface (`internal/web/`)
- REST API with 5 endpoints
- WebSocket for real-time updates
- Responsive HTML/CSS/JavaScript frontend
- Localhost-only security (CORS validated)

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run integration tests (requires tmux)
go test -v ./internal/tmux/...

# Run only unit tests (skip integration)
go test -short ./...
```

**Test Coverage:** 130+ tests across all packages

## Security

Warren is designed for **localhost-only deployment**. The web interface has no authentication and should never be exposed to a network.

See `docs/security.md` for detailed security documentation.

**Safe deployment:**
```bash
warren-web --bind 127.0.0.1:8080
```

**UNSAFE - Do not expose to network:**
```bash
warren-web --bind 0.0.0.0:8080  # DO NOT USE
```

## Next Steps

Phase 2 is complete. Future work (Phase 3+) will focus on:

- **Network Deployment Support**
  - Authentication & authorization
  - HTTPS/TLS support
  - CSRF protection
  - Rate limiting

- **Enhanced Monitoring**
  - Historical state tracking
  - Performance metrics
  - Custom alert rules

- **Multi-Server Support**
  - Remote server monitoring via SSH
  - Distributed deployment
  - Cross-server session management

See `ROADMAP.md` for detailed planning.

## Known Limitations

**Phase 2 Limitations:**

- **Multi-Server Discovery**: Multi-server agent discovery has not been tested at scale. Current validation covers localhost only. (Planned for Phase 3)

- **SSH Host Keys**: Remote SSH connections require the target host to be present in your `known_hosts` file. Unknown hosts are rejected — add them via `ssh-keyscan` or a one-time interactive `ssh` connection before pointing Warren at them.

- **E2E Test Coverage**: The following components have not been validated in end-to-end testing:
  - TUI interface (manual testing required)
  - WebSocket real-time updates (REST API validated)
  - Artifact profile detail views (activity tracking validated)

These limitations do not affect core Phase 2 functionality: users can see all agent sessions, their states, and recent activity without SSHing to tmux.

## Phase 1 Validation Results

✅ **Server Model**: Local server detection and registry persistence working
✅ **Topology Discovery**: Successfully discovers sessions, windows, and panes
✅ **Pane Capture**: Captures content with ANSI stripping and scrollback support
✅ **Pane Control**: Sends text and keystrokes reliably
✅ **Control Loop**: Capture → validate → action → verify workflow validated

**Conclusion**: The tmux interface is clean and reliable enough to build Phase 2 on top of it.

## Next Steps

Phase 2 will implement:
- Agent session registry and discovery
- Event store for activities and notifications
- Activity parser for Claude Code sessions
- State detection (idle, thinking, waiting_permission, etc.)
- Basic TUI and web interface

## Documentation

- [CLAUDE.md](CLAUDE.md) - Project guide and working instructions
- [design-review.md](design-review.md) - Complete design specification
- [ROADMAP.md](ROADMAP.md) - Implementation roadmap with all phases
- [Getting Started Guide](docs/getting-started.md) - Detailed setup and usage instructions
- [Topology API Documentation](docs/api-topology.md) - REST API endpoints for topology data
- [TUI Topology Guide](docs/tui-topology-guide.md) - Terminal UI topology view usage

## License

MIT License - see [LICENSE](LICENSE) file for details.
