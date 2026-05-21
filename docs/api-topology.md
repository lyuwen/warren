# Topology API Documentation

## Overview

The Topology API provides access to Warren's tmux topology data, including servers, sessions, windows, and panes with agent state enrichment.

## Endpoints

### GET /api/topology

Returns the complete topology for all monitored tmux servers.

**URL:** `GET /api/topology`

**Response:** `200 OK`

**Response Body:**
```json
{
  "servers": [
    {
      "name": "localhost",
      "sessions": [
        {
          "id": "$0",
          "name": "main",
          "attached": true,
          "created": "1779070976",
          "windows": [
            {
              "id": "@0",
              "index": 0,
              "name": "editor",
              "active": true,
              "panes": [
                {
                  "id": "%1",
                  "index": 0,
                  "title": "vim",
                  "width": 204,
                  "height": 37,
                  "active": true,
                  "current_command": "vim",
                  "current_path": "/home/user/project",
                  "agent_id": "localhost:0:0.0",
                  "agent_state": "idle"
                }
              ]
            }
          ]
        }
      ]
    }
  ]
}
```

**Example:**
```bash
curl http://localhost:8080/api/topology | jq '.'
```

---

### GET /api/topology/servers/:name

Returns topology for a specific server.

**URL:** `GET /api/topology/servers/:name`

**Parameters:**
- `name` (path) - Server name (e.g., "localhost", "remote")

**Response:** `200 OK` or `404 Not Found`

**Example:**
```bash
curl http://localhost:8080/api/topology/servers/localhost | jq '.'
```

---

### GET /api/topology/sessions/:id

Returns topology for a specific session.

**URL:** `GET /api/topology/sessions/:id`

**Parameters:**
- `id` (path) - Session ID (e.g., "$0", "$1")

**Response:** `200 OK` or `404 Not Found`

**Example:**
```bash
curl http://localhost:8080/api/topology/sessions/\$0 | jq '.'
```

---

## Data Model

### Server
- `name` (string) - Server name
- `sessions` (array) - Array of Session objects

### Session
- `id` (string) - Session ID (e.g., "$0")
- `name` (string) - Session name
- `attached` (boolean) - Whether session is attached
- `created` (string) - Unix timestamp of creation
- `windows` (array) - Array of Window objects

### Window
- `id` (string) - Window ID (e.g., "@0")
- `index` (number) - Window index
- `name` (string) - Window name
- `active` (boolean) - Whether window is active
- `panes` (array) - Array of Pane objects

### Pane
- `id` (string) - Pane ID (e.g., "%1")
- `index` (number) - Pane index
- `title` (string) - Pane title
- `width` (number) - Pane width in characters
- `height` (number) - Pane height in characters
- `active` (boolean) - Whether pane is active
- `current_command` (string) - Current command running in pane
- `current_path` (string) - Current working directory
- `agent_id` (string, optional) - Agent ID if pane is monitored
- `agent_state` (string, optional) - Agent state if pane is monitored
  - Possible values: `"idle"`, `"thinking"`, `"executing"`, `"waiting_permission"`, `"asking_question"`, `"error"`, `"finished"`, `"stopped"`

---

## Agent State Enrichment

Panes that are monitored by Warren include additional fields:
- `agent_id` - Unique identifier for the agent
- `agent_state` - Current state of the agent

This allows you to see which panes are running Claude Code agents and their current status.

---

## Error Responses

### 400 Bad Request
Missing or invalid parameters.

```json
{
  "error": "server name is required"
}
```

### 404 Not Found
Server or session not found.

```json
{
  "error": "server not found: remote"
}
```

### 405 Method Not Allowed
Invalid HTTP method (only GET is supported).

```json
{
  "error": "method not allowed"
}
```

### 500 Internal Server Error
Server error while fetching topology.

```json
{
  "error": "failed to get topology: <error details>"
}
```

---

## Usage Examples

### Get all topology
```bash
curl http://localhost:8080/api/topology
```

### Get topology for specific server
```bash
curl http://localhost:8080/api/topology/servers/localhost
```

### Get topology for specific session
```bash
curl http://localhost:8080/api/topology/sessions/\$0
```

### Filter panes with agents
```bash
curl http://localhost:8080/api/topology | \
  jq '.servers[].sessions[].windows[].panes[] | select(.agent_id != null)'
```

### Count agents by state
```bash
curl http://localhost:8080/api/topology | \
  jq '[.servers[].sessions[].windows[].panes[] | select(.agent_state != null) | .agent_state] | group_by(.) | map({state: .[0], count: length})'
```

---

## Notes

- All endpoints return JSON
- Timestamps are Unix timestamps (seconds since epoch)
- Agent state enrichment only applies to monitored panes
- Topology data is fetched in real-time from tmux servers
