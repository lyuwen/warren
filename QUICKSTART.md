# Quick Setup Guide for Warren Remote Sessions

## Step 1: Build the Fixed Binaries

```bash
cd /home/lfu/git-projects/warren/.claude/worktrees/fix+warren-web-config
go build -o warren-web ./cmd/warren-web
go build -o warren-tui ./cmd/warren-tui
```

## Step 2: Create Configuration Directory

```bash
mkdir -p ~/.warren
```

## Step 3: Create Server Configuration

Create `~/.warren/servers.yaml`:

```yaml
servers:
  # Local machine
  - name: "localhost"
    host: "localhost"
    kind: "local"

  # Add your remote servers here
  - name: "my-remote-server"
    host: "remote.example.com"
    user: "your-username"
    port: 22
    kind: "remote"
    ssh_options:
      IdentityFile: "~/.ssh/id_rsa"
```

**Replace with your actual server details:**
- `name`: A friendly name for the server
- `host`: The hostname or IP address
- `user`: Your SSH username
- `port`: SSH port (usually 22)
- `IdentityFile`: Path to your SSH private key

## Step 4: Test SSH Connection

Before running Warren, make sure you can SSH to your remote server without a password:

```bash
ssh your-username@remote.example.com -p 22
```

If this prompts for a password, you need to set up SSH key authentication first.

## Step 5: Run Warren

### Option A: Web Interface

```bash
./warren-web
```

Then open http://localhost:8080 in your browser.

### Option B: TUI Interface

```bash
./warren-tui
```

## What You Should See

Warren will:
1. Load servers from `~/.warren/servers.yaml`
2. Connect to each server (local and remote)
3. Discover tmux sessions on each server
4. Detect Claude Code agent sessions
5. Display them in the interface

**Example output:**
```
Discovering sessions on server: localhost (localhost)
Registered agent session: localhost:main:0.0 (pane: %0, type: claude-code)
Discovering sessions on server: my-remote-server (remote.example.com)
Registered agent session: my-remote-server:dev:1.0 (pane: %5, type: claude-code)
Total sessions discovered: 2
```

## Troubleshooting

### "No servers configured"

Create `~/.warren/servers.yaml` as shown in Step 3.

### "Failed to discover topology on [server]"

1. Check SSH connectivity:
   ```bash
   ssh user@host -p port tmux list-sessions
   ```

2. Make sure tmux is installed on the remote server
3. Verify your SSH key has the correct permissions:
   ```bash
   chmod 600 ~/.ssh/id_rsa
   ```

### "EventRetentionPeriod must be positive"

You're using the old warren-web binary. Rebuild it (Step 1).

## Next Steps

- See `docs/CONFIGURATION.md` for detailed configuration options
- Check `servers.yaml.example` for more server configuration examples
- Read the Phase 2 documentation for feature details

## Merging the Fix

Once you've tested and confirmed it works:

```bash
cd /home/lfu/git-projects/warren
git checkout tech-debt/quick-wins
git merge worktree-fix+warren-web-config
go build -o warren-web ./cmd/warren-web
go build -o warren-tui ./cmd/warren-tui
```
