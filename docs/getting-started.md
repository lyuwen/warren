# Getting Started with Warren

This guide will help you start using Warren to monitor your Claude Code agent sessions.

## Prerequisites

Before using Warren, you need:

1. **Tmux running** with active sessions
2. **Claude Code agents** running in tmux panes
3. **Warren binaries** built from source

### Check Prerequisites

```bash
# Verify tmux is running
tmux list-sessions

# You should see output like:
# 0: 3 windows (created Sat May 11 21:00:00 2026)

# Verify Claude Code agents are running
tmux list-panes -a -F "#{pane_id} #{pane_title}"

# Look for panes with "claude" in the title
```

## Building Warren

```bash
# Clone the repository
git clone https://github.com/lfu/warren.git
cd warren

# Build the binaries
go build -o warren-web ./cmd/warren-web
go build -o warren-tui ./cmd/warren-tui

# Verify builds
./warren-web --help
./warren-tui --help
```

## Starting Warren

### Option 1: Web Interface (Recommended)

The web interface provides a browser-based view of your agents.

```bash
# Start the web server
./warren-web -addr :8080

# Output:
# Discovering agent sessions...
# Registered agent session: localhost-0-%2 (pane: %2, type: claude-code)
# Starting monitoring for 1 agent sessions...
# Warren web interface available at http://localhost:8080
```

**Open your browser:** http://localhost:8080

You'll see:
- **Agents page**: List of all detected agents with states
- **Agent details**: Click any agent to view detailed information
- **Conversation tab**: View full conversation history from Claude Code sessions
- **Notifications page**: Alerts for agents needing attention
- **Servers page**: Overview of monitored servers

### Option 2: Terminal UI

The TUI provides a terminal-based interface.

```bash
# Start the TUI
./warren-tui

# The TUI will:
# 1. Discover agent sessions automatically
# 2. Start monitoring
# 3. Display the agent list
```

**TUI Navigation:**
- `↑/↓` - Navigate agent list
- `Enter` - View agent details
- `c` - View conversation history (from agent detail view)
- `j/k` - Scroll conversation up/down
- `Tab` - Switch between views (Agents, Notifications, Servers)
- `Esc` - Go back
- `q` - Quit

## What to Expect

### First Launch

When you start Warren, it will:

1. **Discover agents** - Scan tmux for Claude Code sessions
2. **Register sessions** - Add discovered agents to the registry
3. **Start monitoring** - Begin polling agents every 500ms
4. **Detect states** - Identify what each agent is doing

### Agent States

Warren detects these states:

- **idle** - Agent waiting for input (green)
- **thinking** - Agent processing (blue)
- **executing** - Agent running commands (cyan)
- **waiting_permission** - Agent needs approval (yellow)
- **asking_question** - Agent has a question (yellow)
- **finished** - Agent completed task (green)
- **error** - Agent encountered error (red)
- **unknown** - State unclear (gray)

### Notifications

Warren generates notifications when agents need attention:

- **permission_required** - Agent waiting for approval
- **question_asked** - Agent has a question
- **error** - Agent encountered an error
- **finished** - Agent completed a task

## Common Scenarios

### Scenario 1: No Agents Detected

**Symptom:** Warren shows "No agent sessions found"

**Causes:**
1. No tmux sessions running
2. No Claude Code agents in tmux
3. Agent panes don't match detection patterns

**Solutions:**
```bash
# Check tmux sessions
tmux list-sessions

# Check pane titles
tmux list-panes -a -F "#{pane_id} #{pane_title}"

# Start a Claude Code agent in tmux
tmux new-session -s test
# In the new session, start Claude Code
claude

# Restart Warren
./warren-web
```

### Scenario 2: Agents Stuck in "Unknown" State

**Symptom:** Agents show as "unknown" state

**Causes:**
1. Agent pane content is ambiguous
2. No clear state indicators in output
3. Agent is idle with no recent activity

**Solutions:**
- This is normal for idle agents
- Interact with the agent to trigger state changes
- Check agent detail view for activity history

### Scenario 3: TUI Shows Empty Screen

**Symptom:** TUI launches but shows no content

**Causes:**
1. Terminal too small
2. No agents discovered
3. TTY not available

**Solutions:**
```bash
# Resize terminal (minimum 80x24)
# Check for agents
tmux list-panes -a

# Try web interface instead
./warren-web
```

## Configuration

### Command-Line Flags

**warren-web:**
```bash
./warren-web \
  -addr :8080 \              # HTTP server address
  -db warren.db \            # Database path
  -poll 500ms \              # Polling interval
  -confidence 0.7            # Min confidence for state detection
```

**warren-tui:**
```bash
# TUI uses default config
# Database: ~/.warren/warren.db
# Poll interval: 500ms
# Confidence: 0.7
```

### Database Location

- **warren-web**: `./warren.db` (current directory)
- **warren-tui**: `~/.warren/warren.db` (home directory)

**Note:** Web and TUI use separate databases by default.

## Troubleshooting

### "Failed to create Warren directory"

**Solution:**
```bash
# Create directory manually
mkdir -p ~/.warren
chmod 755 ~/.warren
```

### "Failed to discover topology"

**Causes:**
- Tmux not running
- Tmux socket not accessible

**Solution:**
```bash
# Verify tmux is running
tmux list-sessions

# Check tmux socket
ls -la /tmp/tmux-*/default
```

### "Error running TUI: could not open a new TTY"

**Causes:**
- Running in non-interactive environment
- No TTY available

**Solution:**
- Use web interface instead
- Run in a real terminal (not via script)

### Web Interface Not Accessible

**Causes:**
- Port already in use
- Firewall blocking connection

**Solution:**
```bash
# Check if port is in use
lsof -i :8080

# Try different port
./warren-web -addr :8090

# Check firewall
sudo ufw status
```

## Security Notes

**IMPORTANT:** Warren is designed for localhost-only use.

- Web interface has NO authentication
- Web interface has NO encryption
- Only bind to localhost (127.0.0.1 or ::1)
- Never expose Warren to the internet

See `docs/security.md` for details.

## Next Steps

Once Warren is running:

1. **Explore the interface** - Browse agents, check states
2. **Monitor notifications** - Watch for agents needing attention
3. **Check activity** - View agent chat history and file operations
4. **Test state detection** - Interact with agents and watch states update

## Getting Help

- **Documentation**: See `docs/` directory
- **API Reference**: See `docs/web-interface.md`
- **Security**: See `docs/security.md`
- **Issues**: Report at https://github.com/lfu/warren/issues

## Quick Reference

```bash
# Start web interface
./warren-web -addr :8080

# Start TUI
./warren-tui

# Check agents via API
curl http://localhost:8080/api/agents

# Check notifications
curl http://localhost:8080/api/notifications

# View agent details
curl http://localhost:8080/api/agents/<agent-id>
```

## Example Session

```bash
# Terminal 1: Start Claude Code agents in tmux
tmux new-session -s agents
claude  # Start first agent

# Terminal 2: Start Warren
cd warren
./warren-web -addr :8080

# Output:
# Discovering agent sessions...
# Registered agent session: localhost-agents-%0 (pane: %0, type: claude-code)
# Starting monitoring for 1 agent sessions...
# Warren web interface available at http://localhost:8080

# Terminal 3: Open browser
firefox http://localhost:8080

# You should see your agent listed with its current state!
```

## Tips

- **Multiple agents**: Warren automatically discovers all Claude Code agents in tmux
- **Real-time updates**: States update every 500ms (configurable)
- **Notifications**: Check the Notifications page for agents needing attention
- **Activity history**: Click an agent to see its chat history and file operations
- **Performance**: Warren is lightweight and has minimal impact on agent performance

Happy monitoring! 🎉
