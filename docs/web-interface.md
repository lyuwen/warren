# Warren Web Interface

A real-time web interface for monitoring AI agent sessions across tmux panes.

## Features

- **Real-time Monitoring**: WebSocket-based live updates of agent state changes
- **REST API**: Full REST API for programmatic access
- **Responsive Design**: Mobile-friendly interface that works on all devices
- **Multiple Views**:
  - Agent list with state indicators
  - Detailed agent view with activity history and artifact profiles
  - Notification inbox with actionable alerts
  - Server overview

## Quick Start

### Prerequisites

- Go 1.21 or later
- Running tmux sessions with AI agents
- Warren database (created automatically)

### Installation

```bash
# Install dependencies
go get github.com/gorilla/websocket

# Build the web server
go build -o warren-web ./cmd/warren-web

# Run the server
./warren-web
```

### Command-Line Options

```bash
./warren-web [options]

Options:
  -addr string
        HTTP server address (default ":8080")
  -db string
        Database path (default "warren.db")
  -poll duration
        Polling interval (default 500ms)
  -confidence float
        Minimum confidence for state transitions (default 0.7)
```

### Example

```bash
# Start web interface on port 3000 with 1-second polling
./warren-web -addr :3000 -poll 1s

# Access the interface
open http://localhost:3000
```

## REST API

### Endpoints

#### `GET /api/servers`
List all registered servers.

**Response:**
```json
[
  {
    "name": "localhost",
    "host": "localhost",
    "agent_count": 3,
    "status": "online"
  }
]
```

#### `GET /api/agents`
List all agent sessions.

**Response:**
```json
[
  {
    "id": "agent-1",
    "pane_id": "%0",
    "state": "executing",
    "last_poll": "2024-05-11T10:30:00Z",
    "error_count": 0
  }
]
```

#### `GET /api/agents/:id`
Get detailed information about a specific agent.

**Response:**
```json
{
  "id": "agent-1",
  "pane_id": "%0",
  "state": "executing",
  "last_poll": "2024-05-11T10:30:00Z",
  "error_count": 0,
  "profile": {
    "files_visited": ["/path/to/file.go"],
    "total_reads": 5,
    "total_writes": 2,
    "repos_touched": ["/path/to/repo"]
  },
  "activities": [
    {
      "activity_type": "tool",
      "content": "Read file: /path/to/file.go",
      "timestamp": "2024-05-11T10:29:55Z"
    }
  ]
}
```

#### `GET /api/notifications`
Get all unconsumed notifications.

**Response:**
```json
[
  {
    "agent_id": "agent-1",
    "notif_type": "permission_required",
    "message": "Agent agent-1 is waiting for permission approval",
    "consumed": false,
    "timestamp": "2024-05-11T10:30:00Z",
    "metadata": {
      "from_state": "executing",
      "to_state": "waiting_permission"
    }
  }
]
```

#### `POST /api/notifications/consume`
Mark a notification as consumed.

**Request:**
```json
{
  "agent_id": "agent-1",
  "notif_type": "permission_required",
  "timestamp": "2024-05-11T10:30:00Z"
}
```

**Response:**
```json
{
  "status": "ok"
}
```

## WebSocket API

### Connection

Connect to `ws://localhost:8080/ws` for real-time updates.

### Message Types

#### State Change
Sent when an agent's state changes.

```json
{
  "type": "state_change",
  "agent_id": "agent-1",
  "from_state": "executing",
  "to_state": "waiting_permission",
  "timestamp": "2024-05-11T10:30:00Z"
}
```

#### Notification
Sent when new notifications arrive.

```json
{
  "type": "notification",
  "count": 1,
  "notifications": [...],
  "timestamp": "2024-05-11T10:30:00Z"
}
```

### Auto-Reconnect

The WebSocket client automatically reconnects if the connection is lost. Reconnection attempts occur every 3 seconds.

## Agent States

The interface displays agents in various states:

- **unknown** - Initial state, no data yet
- **idle** - Agent is idle, waiting for input
- **thinking** - Agent is processing/reasoning
- **executing** - Agent is executing tools or commands
- **waiting_permission** - Agent needs user approval (actionable)
- **asking_question** - Agent has a question (actionable)
- **finished** - Agent completed its task (actionable)
- **error** - Agent encountered an error (actionable)
- **stopped** - Agent has stopped

Actionable states trigger notifications and are highlighted in the UI.

## Architecture

```
┌─────────────────┐
│   Browser       │
│  (HTML/CSS/JS)  │
└────────┬────────┘
         │ HTTP/WebSocket
         ▼
┌─────────────────┐
│  Web Server     │
│  (Go)           │
├─────────────────┤
│ • REST API      │
│ • WebSocket Hub │
│ • Static Files  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Warren          │
│ Orchestrator    │
├─────────────────┤
│ • State Monitor │
│ • Event Store   │
│ • Notifications │
└─────────────────┘
```

## Development

### Project Structure

```
internal/web/
├── server.go          # HTTP server and WebSocket hub
├── api.go             # REST API handlers
├── websocket.go       # WebSocket client management
└── static/
    ├── index.html     # Main HTML page
    ├── styles.css     # Responsive CSS
    └── app.js         # JavaScript application

cmd/warren-web/
└── main.go            # Entry point
```

### Building

```bash
# Build for current platform
go build -o warren-web ./cmd/warren-web

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o warren-web-linux ./cmd/warren-web

# Build for macOS
GOOS=darwin GOARCH=amd64 go build -o warren-web-macos ./cmd/warren-web
```

### Testing

The web interface requires manual testing with a browser:

1. Start the web server: `./warren-web`
2. Open browser to `http://localhost:8080`
3. Verify all views load correctly
4. Check WebSocket connection status (green dot)
5. Test navigation between views
6. Verify real-time updates when agent states change
7. Test notification consumption
8. Verify mobile responsiveness (resize browser)

## Troubleshooting

### WebSocket Connection Failed

- Check that the web server is running
- Verify firewall settings allow WebSocket connections
- Check browser console for errors

### No Agents Displayed

- Ensure tmux sessions are running
- Check that Warren discovered the sessions (see server logs)
- Verify database path is correct

### Notifications Not Updating

- Check WebSocket connection status (should be green)
- Verify agents are in actionable states
- Check browser console for JavaScript errors

## Security Considerations

**Current Implementation:**
- No authentication/authorization
- WebSocket accepts all origins
- Intended for local development only

**Production Recommendations:**
- Add authentication (JWT, OAuth, etc.)
- Restrict WebSocket origins
- Use HTTPS/WSS
- Add rate limiting
- Implement CORS policies

## License

Part of the Warren project.
