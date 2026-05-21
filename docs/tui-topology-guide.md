# Warren TUI - Topology View Usage Guide

## Overview

The Topology View provides a hierarchical tree visualization of your tmux infrastructure, showing servers, sessions, windows, and panes with real-time agent state information.

## Accessing Topology View

Press `t` from any view to toggle the topology view.

```
┌─ Warren Topology ─────────────────────────────────────┐
│                                                        │
│ 📡 ▼ localhost                                         │
│   └─ 📋 ▼ main                                         │
│       └─ 🪟 ▼ editor                                   │
│           └─ 🔲 %1 [idle]                              │
│                                                        │
│ [↑↓] Navigate [Enter] Expand/Collapse [f] Filter      │
│ [t] Toggle View [r] Refresh [q] Quit                   │
└────────────────────────────────────────────────────────┘
```

## Navigation

### Basic Navigation
- `↑` / `↓` - Move cursor up/down
- `Enter` - Expand/collapse selected node
- `t` - Toggle between topology view and session list
- `r` - Refresh topology data
- `q` - Quit warren-tui

### Node Types and Icons
- 📡 **Server** - Tmux server (local or remote)
- 📋 **Session** - Tmux session
- 🪟 **Window** - Tmux window
- 🔲 **Pane** - Tmux pane

### Expand/Collapse Indicators
- `▶` - Node is collapsed (children hidden)
- `▼` - Node is expanded (children visible)

## Agent State Display

Monitored panes show their agent state in brackets:

```
🔲 %1 [idle]           - Agent is idle
🔲 %2 [thinking]       - Agent is thinking
🔲 %3 [executing]      - Agent is executing a command
🔲 %4 [error]          - Agent encountered an error
🔲 %5 [asking_question] - Agent is asking a question
```

## Filtering

### Entering Filter Mode

Press `f` to toggle filter mode.

```
┌─ Warren Topology ─────────────────────────────────────┐
│ FILTER: Server=local State=idle                       │
│ 📡 ▼ localhost                                         │
│   └─ 📋 ▼ main                                         │
│       └─ 🪟 ▼ editor                                   │
│           └─ 🔲 %1 [idle]                              │
│                                                        │
│ [s] Server [a] State [f] Exit Filter [Esc] Clear      │
│ [t] Toggle View [r] Refresh [q] Quit                   │
└────────────────────────────────────────────────────────┘
```

### Filter Controls

When in filter mode:
- `s` - Cycle server name filter (empty → "local" → empty)
- `a` - Cycle agent state filter (all → idle → thinking → error → all)
- `f` - Exit filter mode (clears filters)
- `Esc` - Clear all filters

### Filter Types

**Server Name Filter:**
- Case-insensitive partial match
- Shows only nodes from matching servers
- Example: "local" matches "localhost"

**Agent State Filter:**
- Shows only panes with matching agent state
- Options: all, idle, thinking, error
- Non-monitored panes are hidden when filtering by state

### Combined Filters

You can apply both server and state filters simultaneously:
- Press `s` to filter by server
- Press `a` to filter by agent state
- Both conditions must match for a node to be shown

### No Results

When filters return no matches:

```
┌─ Warren Topology ─────────────────────────────────────┐
│ FILTER: Server=remote State=error                     │
│                                                        │
│                   No results found                     │
│                                                        │
│ [s] Server [a] State [f] Exit Filter [Esc] Clear      │
│ [t] Toggle View [r] Refresh [q] Quit                   │
└────────────────────────────────────────────────────────┘
```

## Usage Examples

### Example 1: Browse All Servers

1. Start warren-tui: `warren-tui`
2. Press `t` to enter topology view
3. Use `↑`/`↓` to navigate
4. Press `Enter` to expand/collapse nodes
5. View the complete hierarchy

### Example 2: Find Idle Agents

1. Press `t` to enter topology view
2. Press `f` to enter filter mode
3. Press `a` repeatedly until "idle" is selected
4. View only panes with idle agents
5. Press `f` to exit filter mode

### Example 3: View Specific Server

1. Press `t` to enter topology view
2. Press `f` to enter filter mode
3. Press `s` to filter by server name
4. View only nodes from that server
5. Press `f` to exit filter mode

### Example 4: Find Agents with Errors

1. Press `t` to enter topology view
2. Press `f` to enter filter mode
3. Press `a` repeatedly until "error" is selected
4. View only panes with agents in error state
5. Navigate to the error pane for details

## Tips

- **Refresh regularly:** Press `r` to get the latest topology data
- **Use filters:** Narrow down large topologies with filters
- **Expand strategically:** Collapse servers you're not interested in
- **Check agent states:** Monitor agent health at a glance
- **Combine filters:** Use server + state filters for precise results

## Keyboard Reference

| Key | Action |
|-----|--------|
| `t` | Toggle topology view |
| `↑` | Move cursor up |
| `↓` | Move cursor down |
| `Enter` | Expand/collapse node |
| `r` | Refresh topology |
| `f` | Toggle filter mode |
| `s` | Cycle server filter (in filter mode) |
| `a` | Cycle agent state filter (in filter mode) |
| `Esc` | Clear filters (in filter mode) |
| `q` | Quit |

## Troubleshooting

### Topology view is empty
- Check that Warren is monitoring tmux sessions
- Press `r` to refresh topology data
- Verify tmux servers are running

### Agent states not showing
- Ensure panes are being monitored by Warren
- Check that agents are running in the panes
- Refresh with `r` to update agent states

### Filters not working
- Verify you're in filter mode (press `f`)
- Check the filter status line at the top
- Try clearing filters with `Esc` and reapplying

### Navigation not responding
- Ensure you're in topology view (press `t`)
- Check that there are nodes to navigate
- Try refreshing with `r`

## See Also

- [API Documentation](api-topology.md) - Topology API endpoints
- [Warren README](../README.md) - General Warren documentation
