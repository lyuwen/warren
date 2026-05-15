# Warren Remote Server Configuration - Final Fix Summary

## All Issues Resolved ✅

### Issue 1: EventRetentionPeriod Error
**Error:** `Failed to create Warren orchestrator: invalid configuration: EventRetentionPeriod must be positive, got 0s`

**Fix:** Changed warren-web to use `DefaultConfig()` instead of creating a partial config struct.

### Issue 2: Remote Sessions Not Showing in warren-tui
**Problem:** warren-tui only showed local sessions even with remote servers configured.

**Fix:** Updated discovery to iterate through all servers in ServerRegistry instead of hardcoded localhost.

### Issue 3: Remote Servers Discovering Local Sessions
**Problem:** Remote server "foundry2" was discovering the exact same sessions as localhost.

**Fix:** Create separate tmux clients for each server:
- Local servers use `LocalExecutor` (runs commands directly)
- Remote servers use `RemoteExecutor` (runs commands via SSH)

### Issue 4: Web UI Not Showing Remote Servers
**Problem:** Web interface only showed localhost in server list.

**Fix:** Updated `handleGetServers` API to query ServerRegistry and count agents per server.

### Issue 5: SSH Argument Quoting Error
**Error:** `command list-sessions: -F expects an argument`

**Fix:** Changed RemoteExecutor to pass each argument separately to SSH instead of concatenating into a single string. This allows SSH to handle proper quoting of special characters in tmux format strings.

## How to Use

### 1. Build the Fixed Binaries

```bash
cd /home/lfu/git-projects/warren/.claude/worktrees/fix+warren-web-config
go build -o warren-web ./cmd/warren-web
go build -o warren-tui ./cmd/warren-tui
```

### 2. Create Server Configuration

```bash
mkdir -p ~/.warren
cat > ~/.warren/servers.yaml << 'EOF'
servers:
  - name: "localhost"
    host: "localhost"
    kind: "local"

  - name: "foundry2"
    host: "10.15.64.173"
    user: "your-username"
    port: 22
    kind: "remote"
EOF
```

**Important:** Replace `your-username` with your actual SSH username.

### 3. Test SSH Connectivity

Before running Warren, verify SSH works:

```bash
# Test basic SSH
ssh your-username@10.15.64.173

# Test tmux access
ssh your-username@10.15.64.173 tmux list-sessions
```

If you're prompted for a password, set up SSH key authentication:

```bash
# Generate key if you don't have one
ssh-keygen -t rsa -b 4096

# Copy to remote server
ssh-copy-id your-username@10.15.64.173
```

### 4. Run Warren

```bash
# Web interface
./warren-web

# Or TUI interface
./warren-tui
```

### 5. Verify It's Working

**Expected log output:**
```
Discovering sessions on server: localhost (localhost)
Registered agent session: localhost:0:0.0 (pane: %1, type: claude-code)
Registered agent session: localhost:0:6.0 (pane: %27, type: claude-code)
Discovering sessions on server: foundry2 (10.15.64.173)
Registered agent session: foundry2:dev:1.0 (pane: %5, type: claude-code)
Registered agent session: foundry2:prod:2.0 (pane: %12, type: claude-code)
Total sessions discovered: 4
```

**Key indicators it's working:**
- Different session names for localhost vs foundry2
- Different pane IDs (e.g., %1 vs %5)
- Sessions on foundry2 should match what you see when you SSH there

**Web UI (http://localhost:8080):**
- Server list shows both "localhost" and "foundry2"
- Each server has its own agent count
- All sessions are visible in the session list

## Technical Details

### Architecture Changes

**Before:**
```
Warren → Single tmuxClient (LocalExecutor) → All servers
```
Result: All discovery ran locally, even for remote servers.

**After:**
```
Warren → Per-server tmuxClient
  ├─ localhost → LocalExecutor → tmux commands
  └─ foundry2  → RemoteExecutor → ssh user@host tmux commands
```
Result: Each server uses the appropriate executor.

### SSH Command Construction

**Before (broken):**
```go
fullCmd := "tmux list-sessions -F #{session_name}"
ssh user@host fullCmd  // Shell parsing breaks on special chars
```

**After (fixed):**
```go
ssh user@host tmux list-sessions -F "#{session_name}"
// Each arg passed separately, SSH handles quoting
```

### Agent ID Format

Agent IDs now include server name: `server:session:window.pane`

Examples:
- `localhost:0:0.0` - Local session 0, window 0, pane 0
- `foundry2:dev:1.0` - Remote server "foundry2", session "dev", window 1, pane 0

This allows proper tracking and counting of agents per server.

## Files Changed

### Code Changes
- `cmd/warren-web/main.go` - Per-server client creation, tmux import
- `cmd/warren-tui/main.go` - Per-server client creation, tmux import
- `internal/web/api.go` - Query ServerRegistry, count agents per server
- `internal/core/warren.go` - Added GetServerRegistry() method
- `internal/tmux/executor.go` - Fixed SSH argument passing

### Documentation Added
- `docs/CONFIGURATION.md` - Comprehensive configuration guide
- `servers.yaml.example` - Configuration template with examples
- `QUICKSTART.md` - Step-by-step setup instructions

## Commits

Branch: `worktree-fix+warren-web-config`

1. `2be4b5b` - fix: warren-web config validation and add remote server support
2. `ff8c264` - fix: use correct executor for remote servers and show all servers in web UI
3. `579d4b9` - fix: properly pass tmux arguments through SSH

## Testing

All tests pass:
```bash
go test ./internal/core/... -v    # ✅ PASS
go test ./internal/tmux/... -v    # ✅ PASS
```

## Troubleshooting

### "Failed to discover topology on [server]"

**Check SSH connectivity:**
```bash
ssh user@host tmux list-sessions
```

If this fails, Warren will fail too. Common issues:
- SSH key not set up (prompts for password)
- Wrong username or hostname
- Firewall blocking SSH
- tmux not installed on remote server

### "No servers configured"

Create `~/.warren/servers.yaml` as shown above.

### Sessions still look the same

If localhost and remote show identical sessions, check:
1. Are you actually running tmux sessions on the remote server?
2. Does `ssh user@host tmux list-sessions` show different sessions?
3. Check the agent IDs - they should have different server prefixes

### SSH key permissions

If SSH fails with permission errors:
```bash
chmod 600 ~/.ssh/id_rsa
chmod 644 ~/.ssh/id_rsa.pub
chmod 700 ~/.ssh
```

## Next Steps

1. **Test with your actual remote server** - Verify sessions are discovered correctly
2. **Check web UI** - Confirm both servers appear and have correct agent counts
3. **Merge to main** - Once verified working:
   ```bash
   cd /home/lfu/git-projects/warren
   git checkout tech-debt/quick-wins
   git merge worktree-fix+warren-web-config
   ```
4. **Update main README** - Add link to CONFIGURATION.md

## Future Enhancements

1. **Connection pooling** - Reuse SSH connections instead of creating new ones
2. **Parallel discovery** - Discover servers concurrently
3. **Health checking** - Actually ping servers to check if they're online
4. **SSH config support** - Read from ~/.ssh/config for host aliases
5. **Better error messages** - Show specific SSH errors in web UI

## Documentation

See these files for more details:
- `docs/CONFIGURATION.md` - Full configuration reference
- `servers.yaml.example` - Configuration examples
- `QUICKSTART.md` - Quick setup guide
- `design-review.md` - Architecture overview
- `ROADMAP.md` - Implementation phases
