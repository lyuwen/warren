# Warren - Central Agent Hub Vision

## The Real Vision

Warren is not just a session manager - it's a **central hub for monitoring and interacting with multiple coding agents**.

**Inspiration**: Agent Manager in Antigravity app

## Core Concept

Instead of jumping between tmux sessions, Warren brings all agent activity to you in one place:
- See what each agent is doing right now
- View recent chat history from each agent
- See which files each agent is working on
- Interact with agents without attaching to tmux
- Respond to permission prompts remotely
- View and edit files agents are working on

**Mental model**: Mission control for your distributed coding agents

## Two-Phase Approach

### Phase 1: Passive Monitoring (Read-Only Hub)

**Goal**: Aggregate and display agent activity from all tmux sessions

**What Warren captures from each tmux session:**

1. **Tmux pane content snapshot**
   - Current visible content in the tmux window
   - Last N lines of scrollback buffer
   - Refresh periodically (every 5-10 seconds)

2. **Parsed agent activity**
   - Chat messages (user prompts + agent responses)
   - Files being read/written/edited
   - Commands being executed
   - Tool calls being made
   - Permission prompts waiting for response

3. **Agent state**
   - Current task/context
   - Is agent idle, thinking, or executing?
   - Any errors or warnings
   - Last activity timestamp

**How to capture:**
```bash
# Capture tmux pane content
tmux capture-pane -t warren-agent-1 -p -S -1000

# Parse the output to extract:
# - Claude Code chat messages
# - File paths mentioned
# - Tool calls (Read, Write, Edit, Bash, etc.)
# - Permission prompts
# - Task status
```

**Display in Warren hub:**
```
┌─ Warren Hub ────────────────────────────────────────────────┐
│                                                              │
│  Agent: backend-api (prod-1)                    ● Active    │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Last activity: 2 minutes ago                           │ │
│  │                                                        │ │
│  │ Chat:                                                  │ │
│  │   User: Fix the authentication bug                    │ │
│  │   Agent: I'll check the auth middleware...            │ │
│  │   Agent: Found the issue in auth.py line 45           │ │
│  │                                                        │ │
│  │ Files:                                                 │ │
│  │   📝 src/auth.py (editing)                            │ │
│  │   👁 src/middleware.py (read)                         │ │
│  │   👁 tests/test_auth.py (read)                        │ │
│  │                                                        │ │
│  │ Status: Waiting for permission to edit auth.py        │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Agent: ml-trainer (gpu-box)                    ● Active    │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Last activity: 5 seconds ago                           │ │
│  │                                                        │ │
│  │ Chat:                                                  │ │
│  │   User: Train the model on new dataset                │ │
│  │   Agent: Loading dataset... 45% complete              │ │
│  │                                                        │ │
│  │ Files:                                                 │ │
│  │   👁 data/train.csv (read)                            │ │
│  │   📝 models/checkpoint.pt (writing)                   │ │
│  │                                                        │ │
│  │ Status: Training in progress                          │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Agent: frontend (prod-2)                       ⚠ Waiting   │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Last activity: 10 minutes ago                          │ │
│  │                                                        │ │
│  │ Chat:                                                  │ │
│  │   User: Update the dashboard UI                       │ │
│  │   Agent: I'll modify the Dashboard component          │ │
│  │                                                        │ │
│  │ Files:                                                 │ │
│  │   📝 src/Dashboard.tsx (ready to edit)                │ │
│  │                                                        │ │
│  │ ⚠ PERMISSION PROMPT:                                  │ │
│  │   Allow edit to src/Dashboard.tsx?                    │ │
│  │   [Approve] [Deny] [View Diff]                        │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Technical approach for Phase 1:**

1. **Periodic capture**
   ```python
   def capture_agent_activity(agent_name, server):
       # SSH to server
       # Run: tmux capture-pane -t {session} -p -S -1000
       # Get last 1000 lines of tmux buffer
       raw_output = ssh_exec(server, f"tmux capture-pane -t {session} -p -S -1000")
       
       # Parse the output
       activity = parse_claude_code_output(raw_output)
       return activity
   ```

2. **Parse Claude Code output**
   ```python
   def parse_claude_code_output(raw_text):
       """
       Parse tmux buffer to extract:
       - User messages (look for prompt patterns)
       - Agent responses (look for Claude's output)
       - Tool calls (Read, Write, Edit, Bash, etc.)
       - File paths mentioned
       - Permission prompts
       - Current status
       """
       
       # Look for patterns like:
       # "Reading file: src/auth.py"
       # "Editing src/auth.py"
       # "Running command: npm test"
       # "Allow edit to src/Dashboard.tsx? [y/n]"
       
       return {
           'chat_history': [...],
           'files_accessed': [...],
           'current_status': '...',
           'pending_prompts': [...]
       }
   ```

3. **Aggregate and display**
   ```python
   def refresh_hub():
       for agent in warren.list_agents():
           activity = capture_agent_activity(agent.name, agent.server)
           display_agent_card(agent, activity)
   ```

### Phase 2: Interactive Hub (Read-Write)

**Goal**: Interact with agents without attaching to tmux

**What you can do from Warren hub:**

1. **View files agents are working on**
   ```
   Agent is editing src/auth.py
   [View File] → Opens file viewer in Warren
   ```

2. **Send messages to agents**
   ```
   Type message to backend-api agent:
   > Add error handling for null tokens
   
   [Send] → Message appears in agent's tmux session
   ```

3. **Respond to permission prompts**
   ```
   Agent frontend is waiting:
   Allow edit to src/Dashboard.tsx?
   
   [Approve] [Deny] [View Diff]
   
   → Response sent to agent's tmux session
   ```

4. **View file diffs**
   ```
   Agent wants to edit src/auth.py
   [View Diff] → Shows proposed changes
   [Approve] → Agent proceeds
   ```

5. **Send commands**
   ```
   Quick actions:
   [Run Tests] [Deploy] [Stop] [Restart]
   
   → Sends command to agent's tmux session
   ```

**Technical approach for Phase 2:**

1. **Send input to tmux session**
   ```python
   def send_to_agent(agent_name, message):
       # SSH to server
       # Send keys to tmux session
       ssh_exec(server, f"tmux send-keys -t {session} '{message}' C-m")
   ```

2. **Respond to permission prompts**
   ```python
   def approve_permission(agent_name):
       # Send 'y' or 'yes' to tmux session
       send_to_agent(agent_name, "y")
   ```

3. **Fetch files from remote**
   ```python
   def view_file(agent_name, file_path):
       # SSH to server
       # Cat the file
       content = ssh_exec(server, f"cat {file_path}")
       return content
   ```

## Future: Deep Integration with Claude Code

**Phase 3: Access ~/.claude session data**

Instead of parsing tmux output, directly read Claude Code's session data:

```
~/.claude/
├── sessions/
│   ├── session-abc123/
│   │   ├── conversation.jsonl    # Full conversation history
│   │   ├── state.json             # Current state
│   │   └── files/                 # File snapshots
│   └── session-def456/
│       └── ...
```

**Benefits:**
- Structured data (no parsing needed)
- Full conversation history
- File snapshots at each step
- Tool call details
- Exact state of agent

**What Warren can do with this:**
- Show full conversation thread
- Replay agent's actions
- See exact file changes
- Understand agent's reasoning
- Better context for interaction

## Key Technical Challenges

### Challenge 1: Parsing tmux output
- Claude Code output is not structured
- Need to recognize patterns (file paths, tool calls, prompts)
- Output format may change
- Need robust parsing

**Solution approaches:**
- Pattern matching with regex
- Look for known Claude Code output patterns
- Fallback to showing raw output if parsing fails
- Eventually: use Claude Code API if available

### Challenge 2: Sending input to tmux
- Need to simulate keyboard input
- Handle special characters, escape sequences
- Timing issues (send too fast, agent not ready)
- Permission prompts have different formats

**Solution approaches:**
- Use `tmux send-keys` carefully
- Add delays between inputs
- Verify input was received (capture-pane after send)
- Handle different prompt formats

### Challenge 3: File synchronization
- Files are on remote servers
- Need to fetch files to view/edit
- Keep local cache in sync
- Handle large files

**Solution approaches:**
- Fetch files on demand
- Cache recently viewed files
- Use rsync for efficient sync
- Stream large files

### Challenge 4: Real-time updates
- Need to poll tmux sessions frequently
- Balance between freshness and overhead
- Detect when agent is active vs idle
- Avoid overwhelming SSH connections

**Solution approaches:**
- Adaptive polling (faster when active, slower when idle)
- Use SSH ControlMaster for connection reuse
- Batch captures across multiple agents
- WebSocket for pushing updates to UI

## Data Model for Agent Activity

```python
@dataclass
class AgentActivity:
    agent_name: str
    server: str
    timestamp: datetime
    
    # Parsed from tmux output
    chat_history: List[ChatMessage]
    files_accessed: List[FileAccess]
    current_status: str
    pending_prompts: List[PermissionPrompt]
    
    # Raw data
    raw_tmux_output: str
    
@dataclass
class ChatMessage:
    role: str  # 'user' or 'agent'
    content: str
    timestamp: datetime
    
@dataclass
class FileAccess:
    path: str
    action: str  # 'read', 'write', 'edit'
    timestamp: datetime
    
@dataclass
class PermissionPrompt:
    prompt_text: str
    options: List[str]  # ['y', 'n'] or ['approve', 'deny']
    context: str  # What agent wants to do
```

## Warren Hub Interface Modes

### Mode 1: Overview (default)
- See all agents at once
- Quick status of each
- Highlight agents needing attention
- Recent activity summary

### Mode 2: Agent Focus
- Drill into one agent
- Full chat history
- Detailed file list
- Interactive controls

### Mode 3: File Viewer
- View file content
- See proposed changes
- Approve/deny edits
- Edit file directly

### Mode 4: Multi-Agent Coordination
- See how agents relate
- Agents working on same codebase
- Coordinate actions across agents
- Prevent conflicts

## Success Metrics

Warren succeeds when:
- You can see what all agents are doing without switching contexts
- You can respond to permission prompts from central hub
- You can view files agents are working on
- You spend less time in individual tmux sessions
- You catch issues faster (agent stuck, waiting for input)
- You can coordinate multiple agents effectively

## Open Questions

1. **Parsing reliability**: How robust can we make tmux output parsing?
   - What if Claude Code changes output format?
   - How to handle custom prompts/themes?
   
2. **Input injection**: How to safely send input to tmux?
   - What if agent is in middle of operation?
   - How to queue inputs?
   
3. **File editing**: Should Warren allow editing files directly?
   - Or just view and approve agent's edits?
   - Conflict resolution if both edit same file?
   
4. **Session history**: When to access ~/.claude vs tmux?
   - Phase 1: tmux only (simpler)
   - Phase 2: hybrid approach?
   - Phase 3: ~/.claude as primary source?
   
5. **Performance**: How many agents can Warren monitor?
   - Polling overhead
   - SSH connection limits
   - UI responsiveness

6. **Security**: How to handle sensitive data in agent output?
   - API keys, passwords in logs
   - Filter before displaying?
   - Secure storage of captured data?
