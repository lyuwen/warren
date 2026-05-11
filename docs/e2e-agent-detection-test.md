# E2E Test: Agent Detection and Monitoring

**Test Objective:** Validate that Warren can automatically detect Claude Code agent instances in tmux sessions and monitor them in real-time without requiring direct tmux attachment.

**Core Promise Being Validated:** "See all agent sessions without SSHing to tmux"

---

## Prerequisites

**Required:**
- Warren binaries built (`warren-web`, `warren-tui`)
- Claude Code CLI installed (`claude` command available)
- tmux installed
- `curl` or similar HTTP client for API testing
- `jq` for JSON parsing (optional but recommended)

**Optional:**
- Remote server with SSH access (for multi-server validation)
- WebSocket client for real-time update testing

**Estimated Time:** 30-40 minutes

---

## Test Scenario: Agent Detection and State Monitoring

### Phase 1: Environment Setup (5 minutes)

#### Step 1.1: Build Warren Binaries

```bash
# Navigate to Warren project
cd /path/to/warren

# Build binaries
go build -o warren-web ./cmd/warren-web
go build -o warren-tui ./cmd/warren-tui

# Verify binaries created
ls -lh warren-web warren-tui

# Expected output:
# -rwxr-xr-x ... warren-web
# -rwxr-xr-x ... warren-tui
```

**Success Criteria:**
- [ ] `warren-web` binary exists and is executable
- [ ] `warren-tui` binary exists and is executable

#### Step 1.2: Start Warren Web Server

```bash
# Start web server on port 8080
./warren-web -addr :8080 &

# Save PID for cleanup
WARREN_PID=$!
echo $WARREN_PID > /tmp/warren-web.pid

# Wait for server to start
sleep 2

# Verify server is running
curl -s http://localhost:8080/health

# Expected output:
# {"status":"ok"}
```

**Success Criteria:**
- [ ] Web server starts without errors
- [ ] Health endpoint responds with 200 OK
- [ ] Server process running in background

#### Step 1.3: Create Test Tmux Session

```bash
# Create a new tmux session for testing
tmux new-session -d -s warren-test

# Verify session created
tmux list-sessions | grep warren-test

# Expected output:
# warren-test: 1 windows (created ...)
```

**Success Criteria:**
- [ ] Tmux session `warren-test` created
- [ ] Session visible in `tmux list-sessions`

---

### Phase 2: Agent Creation (5 minutes)

#### Step 2.1: Start First Agent (Idle State)

```bash
# Start Claude Code in first pane
tmux send-keys -t warren-test:0 'cd /tmp && claude' Enter

# Wait 3 seconds for Claude to initialize
sleep 3
```

**Expected State:** Agent idle, waiting for user input

**Verification:**
```bash
# Capture pane content to verify Claude Code is running
tmux capture-pane -t warren-test:0 -p | head -20

# Expected output should contain:
# - Claude Code prompt or welcome message
# - Cursor waiting for input
```

**Success Criteria:**
- [ ] Claude Code started in pane
- [ ] Agent in idle state (no active task)

#### Step 2.2: Start Second Agent (Thinking State)

```bash
# Create second window in same session
tmux new-window -t warren-test -n thinking-agent

# Start Claude Code with a task
tmux send-keys -t warren-test:thinking-agent 'cd /tmp && claude' Enter
sleep 3

# Send a task that will take time
tmux send-keys -t warren-test:thinking-agent 'List all files in /usr/bin and explain what each does' Enter
```

**Expected State:** Agent thinking/processing the request

**Verification:**
```bash
# Check agent is processing
tmux capture-pane -t warren-test:thinking-agent -p | tail -10

# Expected output should show:
# - Agent response in progress
# - Tool calls or thinking indicators
```

**Success Criteria:**
- [ ] Second Claude Code instance started
- [ ] Agent actively processing task
- [ ] Agent in thinking/executing state

#### Step 2.3: Start Third Agent (Permission Waiting State)

```bash
# Create third window
tmux new-window -t warren-test -n permission-agent

# Start Claude Code
tmux send-keys -t warren-test:permission-agent 'cd /tmp && claude' Enter
sleep 3

# Send a task that requires permission
tmux send-keys -t warren-test:permission-agent 'Create a test file at /tmp/test.txt with some content' Enter

# Wait for permission prompt to appear
sleep 2
```

**Expected State:** Agent waiting for permission approval

**Verification:**
```bash
# Check for permission prompt
tmux capture-pane -t warren-test:permission-agent -p | grep -i "allow\|permission\|approve"

# Expected output should contain permission prompt
```

**Success Criteria:**
- [ ] Third Claude Code instance started
- [ ] Permission prompt visible
- [ ] Agent in waiting_permission state

---

### Phase 3: Warren Detection via Web API (10 minutes)

#### Step 3.1: Trigger Discovery and List Agents

```bash
# Wait for Warren's automatic discovery cycle (5-10 seconds)
sleep 10

# List all discovered agents via API
curl -s http://localhost:8080/api/agents | jq .

# Expected output (JSON array):
# [
#   {
#     "id": "localhost:warren-test:0:0",
#     "name": "warren-test:0:0",
#     "server": "localhost",
#     "state": "idle",
#     "last_seen": "2026-05-10T14:30:00Z",
#     "working_dir": "/tmp"
#   },
#   {
#     "id": "localhost:warren-test:1:0",
#     "name": "warren-test:thinking-agent:0",
#     "server": "localhost",
#     "state": "thinking",
#     "last_seen": "2026-05-10T14:30:05Z",
#     "working_dir": "/tmp"
#   },
#   {
#     "id": "localhost:warren-test:2:0",
#     "name": "warren-test:permission-agent:0",
#     "server": "localhost",
#     "state": "waiting_permission",
#     "last_seen": "2026-05-10T14:30:10Z",
#     "working_dir": "/tmp"
#   }
# ]
```

**Success Criteria:**
- [ ] API returns 200 OK
- [ ] All 3 agents present in response
- [ ] Agents correctly mapped to tmux panes
- [ ] States detected accurately
- [ ] Last seen timestamps recent (<30s)

#### Step 3.2: Get Agent Details via API

```bash
# Get details for idle agent
curl -s http://localhost:8080/api/agents/localhost:warren-test:0:0 | jq .

# Expected output:
# {
#   "id": "localhost:warren-test:0:0",
#   "name": "warren-test:0:0",
#   "server": "localhost",
#   "state": "idle",
#   "last_seen": "2026-05-10T14:30:00Z",
#   "working_dir": "/tmp",
#   "tmux": {
#     "session": "warren-test",
#     "window": "0",
#     "pane": "0"
#   },
#   "recent_activity": [
#     {
#       "timestamp": "2026-05-10T14:28:00Z",
#       "type": "agent_started",
#       "content": "Claude Code initialized"
#     }
#   ]
# }
```

**Success Criteria:**
- [ ] API returns agent details
- [ ] State matches expected (idle)
- [ ] Tmux topology correct
- [ ] Recent activity present

#### Step 3.3: Get Thinking Agent Details

```bash
# Get details for thinking agent
curl -s http://localhost:8080/api/agents/localhost:warren-test:1:0 | jq .

# Expected output should include:
# - state: "thinking" or "executing"
# - recent_activity with user message and agent response
# - tool calls (Bash, Read, etc.)
```

**Success Criteria:**
- [ ] State is `thinking` or `executing`
- [ ] Recent activity shows user request
- [ ] Tool calls captured

#### Step 3.4: Get Permission Agent Details

```bash
# Get details for permission agent
curl -s http://localhost:8080/api/agents/localhost:warren-test:2:0 | jq .

# Expected output should include:
# - state: "waiting_permission"
# - recent_activity with permission prompt
# - prompt details (what permission is being requested)
```

**Success Criteria:**
- [ ] State is `waiting_permission`
- [ ] Permission prompt visible in activity
- [ ] Prompt details captured

#### Step 3.5: Check Notifications via API

```bash
# Get all notifications
curl -s http://localhost:8080/api/notifications | jq .

# Expected output (JSON array):
# [
#   {
#     "id": "notif-1",
#     "agent_id": "localhost:warren-test:2:0",
#     "type": "permission_required",
#     "message": "Agent waiting for permission",
#     "timestamp": "2026-05-10T14:30:10Z",
#     "read": false
#   }
# ]
```

**Success Criteria:**
- [ ] API returns notifications
- [ ] Notification for permission agent present
- [ ] Notification type correct (`permission_required`)
- [ ] Unread status tracked

---

### Phase 4: Manual TUI Testing (10 minutes)

#### Step 4.1: Launch Warren TUI

```bash
# Launch TUI in a new terminal or tmux pane
./warren-tui
```

**Manual Testing Checklist:**

**Initial Display:**
- [ ] TUI launches without errors
- [ ] Agent list view displayed
- [ ] All 3 agents visible
- [ ] Agents grouped by server (localhost)
- [ ] State indicators visible (colors/icons)

**Expected TUI Layout:**
```
┌─ Warren — Agent Sessions ─────────────────────────────────┐
│ localhost (3 agents, 1 needs attention)                    │
│                                                            │
│ ⚠ NEEDS ATTENTION                                          │
│   🔒 warren-test:2:0    [waiting_permission]  10s ago    │
│                                                            │
│ ✓ ACTIVE                                                   │
│   ● warren-test:1:0     [thinking]            5s ago     │
│   ○ warren-test:0:0     [idle]                2m ago     │
│                                                            │
│ [↑/↓] Navigate  [Enter] Details  [n] Notifications  [q] Quit │
└────────────────────────────────────────────────────────────┘
```

#### Step 4.2: Test Navigation

**Actions to Perform:**

1. **Arrow Key Navigation:**
   - [ ] Press ↓ to move selection down
   - [ ] Press ↑ to move selection up
   - [ ] Selection highlight moves correctly

2. **View Agent Details:**
   - [ ] Select permission agent (warren-test:2:0)
   - [ ] Press Enter
   - [ ] Detail view opens

**Expected Detail View:**
```
┌─ warren-test:2:0 (localhost:warren-test:permission-agent:0) ┐
│ State: waiting_permission                                  │
│ Working on: /tmp                                           │
│ Last activity: 10 seconds ago                              │
│                                                            │
│ ─── Recent Activity ───────────────────────────────────── │
│ [10s] Permission: Allow warren to write /tmp/test.txt?    │
│       [y/n/always/never]                                   │
│ [30s] Agent: I'll create the file for you.                │
│ [1m]  User: Create a test file at /tmp/test.txt...        │
│                                                            │
│ [Esc] Back  [h] Full history  [f] Files touched           │
└────────────────────────────────────────────────────────────┘
```

**Detail View Checklist:**
- [ ] Agent state displayed
- [ ] Working directory shown
- [ ] Recent activity visible
- [ ] Permission prompt clearly shown
- [ ] Timestamps relative and readable
- [ ] Navigation hints displayed

3. **Return to Overview:**
   - [ ] Press Esc
   - [ ] Returns to agent list view

#### Step 4.3: Test Notification View

**Actions:**
1. [ ] Press 'n' to open notifications
2. [ ] Notification inbox displayed
3. [ ] Permission notification visible
4. [ ] Press Esc to close

**Expected Notification View:**
```
┌─ Notifications (1 unread) ─────────────────────────────────┐
│ [10s ago] 🔒 warren-test:2:0 waiting for permission       │
│                                                            │
│ [Enter] View  [d] Dismiss  [Esc] Close                    │
└────────────────────────────────────────────────────────────┘
```

**Notification View Checklist:**
- [ ] Notification count accurate
- [ ] Permission notification listed
- [ ] Timestamp displayed
- [ ] Navigation instructions shown

#### Step 4.4: Test State Color Coding

**Visual Verification:**
- [ ] `waiting_permission` agent highlighted/colored differently (red/yellow)
- [ ] `thinking` agent has distinct indicator (blue/green)
- [ ] `idle` agent has neutral indicator (gray/white)
- [ ] "Needs attention" section visually distinct

---

### Phase 5: Real-Time Updates (10 minutes)

#### Step 5.1: Approve Permission (External Action)

**Keep TUI open in one terminal, perform action in another:**

```bash
# In a separate terminal, attach to tmux and approve permission
tmux attach -t warren-test:permission-agent

# Press 'y' to approve
# Then detach: Ctrl+b, d
```

**Expected Behavior:**
- Agent transitions from `waiting_permission` to `executing`
- Warren detects state change within 5-10 seconds
- TUI updates automatically (no manual refresh needed)

#### Step 5.2: Verify Real-Time Update in TUI

**Watch TUI (should update automatically within 10 seconds):**

**Before approval:**
```
│ ⚠ NEEDS ATTENTION                                          │
│   🔒 warren-test:2:0    [waiting_permission]  30s ago    │
```

**After approval (within 10 seconds):**
```
│ ✓ ACTIVE                                                   │
│   ⚙ warren-test:2:0     [executing]           just now   │
```

**TUI Update Checklist:**
- [ ] State changes from `waiting_permission` to `executing`
- [ ] Agent moves from "needs attention" to "active" section
- [ ] Update happens automatically (no key press needed)
- [ ] Timestamp updates to "just now"
- [ ] Update latency < 10 seconds

#### Step 5.3: Verify Update via API

```bash
# Query agent state via API
curl -s http://localhost:8080/api/agents/localhost:warren-test:2:0 | jq '.state'

# Expected output:
# "executing" or "finished"
```

**Success Criteria:**
- [ ] API reflects state change
- [ ] State is `executing` or `finished`
- [ ] Last seen timestamp updated

#### Step 5.4: Test WebSocket Real-Time Updates (Optional)

**If WebSocket support implemented:**

```bash
# Connect to WebSocket endpoint
wscat -c ws://localhost:8080/ws/agents

# Expected: Receive real-time state change events
# {
#   "type": "state_change",
#   "agent_id": "localhost:warren-test:2:0",
#   "old_state": "waiting_permission",
#   "new_state": "executing",
#   "timestamp": "2026-05-10T14:35:00Z"
# }
```

**WebSocket Checklist:**
- [ ] WebSocket connection established
- [ ] State change events received
- [ ] Event data accurate
- [ ] Events arrive within 5 seconds of state change

---

### Phase 6: Activity and Artifact Tracking (5 minutes)

#### Step 6.1: Query Activity Events via API

```bash
# Get recent activity for permission agent
curl -s "http://localhost:8080/api/agents/localhost:warren-test:2:0/activity?limit=10" | jq .

# Expected output (JSON array):
# [
#   {
#     "timestamp": "2026-05-10T14:35:00Z",
#     "type": "state_transition",
#     "details": "waiting_permission → executing"
#   },
#   {
#     "timestamp": "2026-05-10T14:35:01Z",
#     "type": "tool_call",
#     "tool": "Write",
#     "args": "/tmp/test.txt"
#   },
#   {
#     "timestamp": "2026-05-10T14:35:02Z",
#     "type": "state_transition",
#     "details": "executing → finished"
#   }
# ]
```

**Success Criteria:**
- [ ] Activity events returned
- [ ] State transitions captured
- [ ] Tool calls logged
- [ ] Timestamps accurate

#### Step 6.2: Query Artifact Profile via API

```bash
# Get files touched by agent
curl -s "http://localhost:8080/api/agents/localhost:warren-test:2:0/artifacts" | jq .

# Expected output:
# {
#   "files": [
#     {
#       "path": "/tmp/test.txt",
#       "operations": {
#         "write": 1
#       },
#       "lines_added": 10
#     }
#   ],
#   "summary": {
#     "total_reads": 0,
#     "total_writes": 1,
#     "total_edits": 0,
#     "files_created": 1
#   }
# }
```

**Success Criteria:**
- [ ] File write tracked
- [ ] Artifact profile accurate
- [ ] Statistics correct

#### Step 6.3: Verify in TUI

**In TUI:**
1. [ ] Navigate to permission agent detail view
2. [ ] Press 'f' to view files touched
3. [ ] Verify `/tmp/test.txt` listed
4. [ ] Verify operation count correct

---

## Test Validation Checklist

### Core Functionality
- [ ] **Agent Discovery:** Warren detects all Claude Code agents in tmux
- [ ] **State Detection:** Agent states (idle, thinking, waiting_permission, executing, finished) detected correctly
- [ ] **Real-Time Monitoring:** State changes reflected in TUI/API within 10 seconds
- [ ] **Activity Tracking:** Chat messages, tool calls, and file operations captured
- [ ] **Notifications:** Alerts generated for actionable states (waiting_permission)

### Web API
- [ ] **Health Endpoint:** `/health` returns 200 OK
- [ ] **List Agents:** `/api/agents` returns all agents
- [ ] **Agent Details:** `/api/agents/:id` returns agent details
- [ ] **Activity:** `/api/agents/:id/activity` returns activity events
- [ ] **Artifacts:** `/api/agents/:id/artifacts` returns file tracking
- [ ] **Notifications:** `/api/notifications` returns notifications

### TUI
- [ ] **Launch:** TUI starts without errors
- [ ] **Display:** All agents visible with correct states
- [ ] **Navigation:** Arrow keys, Enter, Esc work correctly
- [ ] **Detail View:** Agent details displayed accurately
- [ ] **Notifications:** Notification inbox accessible and accurate
- [ ] **Real-Time Updates:** TUI updates automatically without refresh
- [ ] **Color Coding:** States visually distinct

### Data Accuracy
- [ ] **Topology Mapping:** Agents correctly mapped to tmux server:session:window:pane
- [ ] **Activity Parsing:** Chat and tool calls parsed with >90% accuracy
- [ ] **Artifact Tracking:** Files touched tracked correctly
- [ ] **Event Storage:** All events accessible via API

### Performance
- [ ] **Discovery Latency:** Agents detected within 10 seconds of creation
- [ ] **Update Latency:** State changes reflected within 10 seconds
- [ ] **TUI Responsiveness:** No lag or freezing with 3 agents
- [ ] **API Response Time:** API requests complete in <500ms

---

## Success Criteria Summary

**Test PASSES if:**
1. ✅ All 3 agents detected automatically
2. ✅ States detected correctly (idle, thinking, waiting_permission)
3. ✅ TUI displays agents without requiring tmux attachment
4. ✅ Real-time updates work (state changes reflected within 10s)
5. ✅ Notifications generated for actionable states
6. ✅ Activity tracking captures chat and tool calls
7. ✅ Artifact profile tracks file operations
8. ✅ Web API returns accurate data
9. ✅ TUI navigation works smoothly

**Test FAILS if:**
- ❌ Any agent not detected after 30 seconds
- ❌ State detection incorrect (>10% error rate)
- ❌ TUI requires manual refresh to see updates
- ❌ Notifications not generated for permission prompts
- ❌ Activity parsing fails (cannot extract chat or tool calls)
- ❌ Web server crashes or becomes unresponsive
- ❌ TUI crashes or freezes

---

## Cleanup

```bash
# Stop Warren web server
kill $(cat /tmp/warren-web.pid)
rm /tmp/warren-web.pid

# Kill tmux session
tmux kill-session -t warren-test

# Remove test files
rm -f /tmp/test.txt

# Optional: Clear Warren database
rm -f ~/.warren/warren.db
```

---

## Troubleshooting

### Agent Not Detected

**Symptom:** API returns empty array or missing agents

**Diagnosis:**
```bash
# Check web server logs
# (Look for discovery cycle logs)

# Check tmux sessions exist
tmux list-sessions

# Check pane content manually
tmux capture-pane -t warren-test:0 -p

# Check API health
curl http://localhost:8080/health
```

**Common Causes:**
- Web server not running
- Tmux session name doesn't match expected pattern
- Claude Code not fully initialized
- Parsing heuristics failed to detect Claude Code
- Discovery cycle hasn't run yet (wait 10 seconds)

### State Detection Incorrect

**Symptom:** Agent shown as `unknown` or wrong state

**Diagnosis:**
```bash
# Capture pane content and check manually
tmux capture-pane -t warren-test:0 -p | less

# Check agent details via API
curl -s http://localhost:8080/api/agents/localhost:warren-test:0:0 | jq .

# Look for confidence scores or parsing errors in response
```

**Common Causes:**
- Pane content ambiguous (mixed signals)
- Parsing patterns outdated (Claude Code UI changed)
- Capture timing issue (caught mid-transition)

### Real-Time Updates Not Working

**Symptom:** TUI doesn't update when agent state changes

**Diagnosis:**
```bash
# Check if state change reflected in API
curl -s http://localhost:8080/api/agents/localhost:warren-test:2:0 | jq '.state'

# If API shows new state but TUI doesn't:
# - TUI update mechanism broken
# - Polling interval too long

# If API also shows old state:
# - Discovery cycle not running
# - Capture not detecting state change
```

**Common Causes:**
- Discovery/capture interval too long
- TUI not polling for updates
- WebSocket connection broken (if using WebSocket)
- State transition not detected by parser

### TUI Crashes or Freezes

**Symptom:** TUI becomes unresponsive or exits unexpectedly

**Diagnosis:**
```bash
# Run TUI with debug output (if available)
./warren-tui -debug

# Check for panic or error messages
```

**Common Causes:**
- API endpoint unreachable
- Malformed JSON response
- Terminal size too small
- Rendering bug with specific content

---

## Expected Test Duration

- **Setup:** 5 minutes
- **Agent Creation:** 5 minutes
- **API Testing:** 10 minutes
- **TUI Testing:** 10 minutes
- **Real-Time Updates:** 10 minutes
- **Activity Validation:** 5 minutes
- **Cleanup:** 2 minutes

**Total:** ~45 minutes

---

## Next Steps After Test

**If test passes:**
- Mark Phase 2 as validated
- Document any parsing edge cases discovered
- Measure and record performance metrics
- Proceed to Phase 3 (Interactive Hub)

**If test fails:**
- Identify failure mode (detection, parsing, state, performance, API, TUI)
- File issues with reproduction steps
- Fix root cause before proceeding
- Re-run test to verify fix

---

## Notes for Test Executor

- **Run in clean environment:** Fresh tmux sessions, no existing Warren state
- **Document observations:** Note any unexpected behavior, even if test passes
- **Measure performance:** Record discovery latency, update latency, API response times
- **Test edge cases:** Try unusual agent states, rapid transitions, long-running tasks
- **Capture screenshots:** TUI views for documentation
- **Save logs:** Web server logs for debugging
- **API Testing:** Use `jq` for readable JSON output, or any HTTP client you prefer
- **TUI Testing:** Manual testing required - no automation for interactive TUI

This test validates the core Phase 2 promise: **centralized visibility without tmux attachment**.
