# Phase 2 Test Scenarios — Central Read-Only Hub

**Purpose:** Validate Phase 2 implementation against design requirements for the Central Read-Only Hub.

**Test Environment Requirements:**
- 2+ servers with tmux installed (1 local, 1+ remote)
- Claude Code CLI installed on all servers
- Multiple Git repositories for artifact tracking
- Warren daemon running with Phase 2 features enabled

---

## Scenario 1: Basic Multi-Server Monitoring

**Objective:** Verify user can see all agent sessions across multiple tmux servers.

### Setup

**Server Configuration:**
- **Local server** (`localhost`):
  - Tmux session `dev-main` with 2 windows
    - Window 0: Claude Code session working on `~/projects/api-server`
    - Window 1: Claude Code session working on `~/projects/frontend`
- **Remote server** (`dev-box-01`):
  - Tmux session `experiments` with 1 window
    - Window 0: Claude Code session working on `/opt/ml-pipeline`

**Agent States:**
- `api-server` agent: `thinking` (actively processing)
- `frontend` agent: `idle` (waiting for user input)
- `ml-pipeline` agent: `executing` (running bash command)

### Expected Behavior

**TUI Display:**
```
┌─ Warren — Agent Sessions ─────────────────────────────────┐
│ localhost                                                  │
│   ● api-server          [thinking]     ~/projects/api-... │
│   ○ frontend            [idle]         ~/projects/fron... │
│                                                            │
│ dev-box-01                                                 │
│   ⚙ ml-pipeline         [executing]    /opt/ml-pipeline   │
└────────────────────────────────────────────────────────────┘
```

**Success Criteria:**
- [ ] All 3 agent sessions visible in TUI
- [ ] Sessions correctly grouped by server
- [ ] State indicators match actual agent states
- [ ] Artifact paths displayed (truncated if needed)
- [ ] Visual distinction between local and remote servers
- [ ] Navigation works (arrow keys, enter to view details)

### Edge Cases to Test

1. **Server unreachable:** Remote server goes offline
   - Expected: Server marked as disconnected, sessions shown as stale
2. **Empty server:** Server has tmux but no Claude Code sessions
   - Expected: Server listed but shows "No agent sessions"
3. **Tmux not running:** Server has no active tmux sessions
   - Expected: Server listed, shows "No tmux sessions found"

---

## Scenario 2: State Detection and Attention Routing

**Objective:** Verify user can quickly identify which sessions need attention.

### Setup

**5 Agent Sessions in Different States:**

1. **`auth-service`** (localhost): `waiting_permission`
   - Captured content shows: `Allow warren to run 'npm test'? [y/n/always/never]`
   - Last activity: 2 minutes ago

2. **`user-api`** (localhost): `asking_question`
   - Captured content shows: `Should I use JWT or session tokens for auth?`
   - Last activity: 5 minutes ago

3. **`database-migration`** (dev-box-01): `error`
   - Captured content shows: `Error: Connection refused to postgres://...`
   - Last activity: 1 minute ago

4. **`frontend-refactor`** (dev-box-01): `thinking`
   - Captured content shows: `Reading src/components/UserProfile.tsx...`
   - Last activity: 10 seconds ago

5. **`docs-update`** (dev-box-02): `finished`
   - Captured content shows: `Task completed. Updated 5 files.`
   - Last activity: 30 seconds ago

### Expected Behavior

**TUI Display (Sorted by Priority):**
```
┌─ Warren — Agent Sessions (5 active, 3 need attention) ────┐
│ ⚠ NEEDS ATTENTION                                          │
│   🔒 auth-service       [waiting_permission]  2m ago      │
│   ❓ user-api           [asking_question]     5m ago      │
│   ❌ database-migration [error]               1m ago      │
│                                                            │
│ ✓ ACTIVE                                                   │
│   ● frontend-refactor   [thinking]            10s ago     │
│   ✓ docs-update         [finished]            30s ago     │
└────────────────────────────────────────────────────────────┘
```

**Notification Inbox:**
```
┌─ Notifications (3 unread) ─────────────────────────────────┐
│ [1m ago]  ❌ database-migration entered error state        │
│ [2m ago]  🔒 auth-service waiting for permission          │
│ [5m ago]  ❓ user-api asking question                     │
└────────────────────────────────────────────────────────────┘
```

### Success Criteria

- [ ] Sessions sorted by attention priority (error > waiting_permission > asking_question > finished > active)
- [ ] Visual indicators clearly distinguish state types
- [ ] Time since last activity displayed accurately
- [ ] Notification count matches sessions needing attention
- [ ] Pressing 'n' key opens notification inbox
- [ ] Selecting a notification navigates to agent detail view

### Edge Cases to Test

1. **Ambiguous state:** Captured content has mixed signals
   - Expected: State marked as `unknown`, confidence score shown
2. **Rapid state transitions:** Agent moves from `thinking` → `waiting_permission` → `executing` in 10 seconds
   - Expected: All transitions captured as events, latest state shown
3. **Stale session:** Agent hasn't produced output in 30 minutes
   - Expected: State shown with staleness indicator (e.g., "idle (stale)")

---

## Scenario 3: Activity Tracking and Chat History

**Objective:** Verify user can see what each agent is doing and has done.

### Setup

**Agent Session:** `api-refactor` on localhost

**Simulated Activity Sequence (last 10 minutes):**
```
[10m ago] User: "Refactor the authentication middleware to use async/await"
[10m ago] Agent: "I'll refactor the auth middleware. Let me start by reading the current implementation."
[9m ago]  Tool: Read(src/middleware/auth.js)
[8m ago]  Agent: "I see the middleware uses callbacks. I'll convert it to async/await and add error handling."
[7m ago]  Tool: Edit(src/middleware/auth.js) — 45 lines changed
[6m ago]  Tool: Bash("npm test -- auth.test.js")
[5m ago]  Agent: "Tests are passing. I'll also update the related route handlers."
[4m ago]  Tool: Read(src/routes/users.js)
[3m ago]  Tool: Edit(src/routes/users.js) — 12 lines changed
[2m ago]  Tool: Bash("npm test")
[1m ago]  Agent: "All tests passing. Refactoring complete."
[30s ago] Agent: "Would you like me to add JSDoc comments to the refactored functions?"
```

### Expected Behavior

**Agent Detail View:**
```
┌─ api-refactor (localhost:dev-main:0:1) ────────────────────┐
│ State: asking_question                                     │
│ Working on: ~/projects/api-server                          │
│ Last activity: 30 seconds ago                              │
│                                                            │
│ ─── Recent Activity ───────────────────────────────────── │
│ [30s] Agent: Would you like me to add JSDoc comments...   │
│ [1m]  Agent: All tests passing. Refactoring complete.     │
│ [2m]  Tool: Bash("npm test") → exit 0                     │
│ [3m]  Tool: Edit(src/routes/users.js) — 12 lines         │
│ [4m]  Tool: Read(src/routes/users.js)                     │
│ [5m]  Agent: Tests are passing. I'll also update...       │
│ [6m]  Tool: Bash("npm test -- auth.test.js") → exit 0    │
│ [7m]  Tool: Edit(src/middleware/auth.js) — 45 lines      │
│                                                            │
│ [Press 'h' for full history, 'f' for files touched]       │
└────────────────────────────────────────────────────────────┘
```

### Success Criteria

- [ ] Chat messages (user and agent) displayed in chronological order
- [ ] Tool calls shown with tool name and key parameters
- [ ] Bash commands show exit codes
- [ ] File edits show line counts
- [ ] Timestamps relative and human-readable
- [ ] Activity scrollable (up/down arrows)
- [ ] Full history accessible via keyboard shortcut

### Edge Cases to Test

1. **Long tool output:** Bash command produces 1000 lines of output
   - Expected: Output truncated in activity view, full output accessible via detail view
2. **Rapid tool calls:** Agent makes 10 Read calls in 5 seconds
   - Expected: All calls logged, display condensed (e.g., "Read 10 files")
3. **Parse failure:** Captured content doesn't match expected patterns
   - Expected: Raw content shown with confidence score, marked as unparsed

---

## Scenario 4: Artifact Profile and File Tracking

**Objective:** Verify user can see which files and repos each agent is working on.

### Setup

**Agent Session:** `full-stack-feature` on localhost

**Simulated File Interactions:**
```
Read:   backend/src/api/users.go (3 times)
Edit:   backend/src/api/users.go (2 times, 87 lines changed)
Write:  backend/src/api/users_test.go (new file, 145 lines)
Read:   frontend/src/components/UserList.tsx (1 time)
Edit:   frontend/src/components/UserList.tsx (1 time, 23 lines changed)
Read:   frontend/src/api/client.ts (2 times)
Edit:   frontend/src/api/client.ts (1 time, 12 lines changed)
Read:   docs/API.md (1 time)
Edit:   docs/API.md (1 time, 8 lines changed)
```

**Detected Repositories:**
- `~/projects/monorepo` (root)
  - `backend/` (Go project)
  - `frontend/` (TypeScript/React project)

### Expected Behavior

**Artifact Profile View:**
```
┌─ full-stack-feature — Artifact Profile ────────────────────┐
│ Repository: ~/projects/monorepo                            │
│                                                            │
│ ─── Files Touched (8 files) ──────────────────────────── │
│ backend/src/api/users.go              [R:3 E:2] (87Δ)    │
│ backend/src/api/users_test.go         [W:1]    (145+)    │
│ frontend/src/components/UserList.tsx  [R:1 E:1] (23Δ)    │
│ frontend/src/api/client.ts            [R:2 E:1] (12Δ)    │
│ docs/API.md                            [R:1 E:1] (8Δ)     │
│                                                            │
│ ─── Summary ──────────────────────────────────────────── │
│ Total reads:  10                                           │
│ Total edits:  5 (130 lines changed)                       │
│ Total writes: 1 (145 lines added)                         │
│ Repositories: 1                                            │
│                                                            │
│ [Press 'd' to view file diffs, 'g' to open in editor]     │
└────────────────────────────────────────────────────────────┘
```

### Success Criteria

- [ ] All file interactions tracked accurately
- [ ] Read/Edit/Write counts correct per file
- [ ] Line change statistics accurate
- [ ] Git repository detected automatically
- [ ] Files grouped by directory or repository
- [ ] Summary statistics match detailed counts
- [ ] File paths relative to repository root

### Edge Cases to Test

1. **Multiple repositories:** Agent works across 3 different Git repos
   - Expected: Each repo listed separately with its files
2. **Non-Git files:** Agent edits files outside any Git repository
   - Expected: Files listed under "Other files" section
3. **Large file set:** Agent touches 100+ files
   - Expected: Display paginated or grouped by directory, summary accurate

---

## Scenario 5: Notification Flow and Real-Time Updates

**Objective:** Verify user receives notifications when agents need attention and sees real-time updates.

### Setup

**Initial State:**
- Warren TUI open, showing 3 agent sessions
- All agents in `thinking` or `idle` state
- No unread notifications

**Event Sequence:**
1. **T+0s:** Agent `backend-api` transitions to `waiting_permission`
2. **T+10s:** Agent `frontend-ui` transitions to `asking_question`
3. **T+20s:** Agent `database-setup` transitions to `error`
4. **T+30s:** User navigates to notification inbox
5. **T+40s:** User selects notification for `backend-api`
6. **T+50s:** Agent `backend-api` transitions to `executing` (user approved via different interface)

### Expected Behavior

**T+0s — Permission Notification:**
```
┌─ Warren ───────────────────────────────────────────────────┐
│ [Notification] backend-api waiting for permission          │
│ Sessions: 3 active, 1 needs attention                      │
└────────────────────────────────────────────────────────────┘
```

**T+10s — Question Notification:**
```
┌─ Warren ───────────────────────────────────────────────────┐
│ [Notification] frontend-ui asking question                 │
│ Sessions: 3 active, 2 need attention                       │
└────────────────────────────────────────────────────────────┘
```

**T+20s — Error Notification:**
```
┌─ Warren ───────────────────────────────────────────────────┐
│ [Notification] database-setup entered error state          │
│ Sessions: 3 active, 3 need attention                       │
└────────────────────────────────────────────────────────────┘
```

**T+30s — Notification Inbox:**
```
┌─ Notifications (3 unread) ─────────────────────────────────┐
│ [20s ago] ❌ database-setup entered error state           │
│ [10s ago] ❓ frontend-ui asking question                  │
│ [30s ago] 🔒 backend-api waiting for permission           │
│                                                            │
│ [Press Enter to view details, 'd' to dismiss]             │
└────────────────────────────────────────────────────────────┘
```

**T+40s — Agent Detail View (from notification):**
```
┌─ backend-api (localhost:dev:0:0) ──────────────────────────┐
│ State: waiting_permission                                  │
│ Notification: Permission required                          │
│                                                            │
│ ─── Permission Prompt ────────────────────────────────── │
│ Allow warren to run 'npm install express'?                 │
│ [y/n/always/never]                                         │
│                                                            │
│ [Note: This is read-only view. Approve in agent session]  │
└────────────────────────────────────────────────────────────┘
```

**T+50s — Real-Time State Update:**
```
┌─ backend-api (localhost:dev:0:0) ──────────────────────────┐
│ State: executing                                           │
│ Last activity: just now                                    │
│                                                            │
│ ─── Recent Activity ───────────────────────────────────── │
│ [now] Tool: Bash("npm install express")                   │
│ [20s] Agent: I'll install express for the API server.     │
└────────────────────────────────────────────────────────────┘
```

### Success Criteria

- [ ] Notifications appear immediately when state changes (< 2 second delay)
- [ ] Notification count updates in real-time
- [ ] Notification inbox shows all unread notifications
- [ ] Selecting notification navigates to agent detail view
- [ ] Agent detail view shows relevant context (permission prompt, question, error)
- [ ] State updates reflected in TUI without manual refresh
- [ ] Dismissed notifications removed from inbox
- [ ] Notification marked as read when user views agent detail

### Edge Cases to Test

1. **Notification flood:** 5 agents transition to `error` state simultaneously
   - Expected: All notifications queued, displayed in order, TUI remains responsive
2. **Stale notification:** User views notification for agent that has already transitioned to different state
   - Expected: Detail view shows current state, notification marked as stale
3. **Missed notifications:** Warren TUI closed when notification fires
   - Expected: Notification persisted in DB, shown when TUI reopens

---

## Scenario 6: Real-World Development Workflow

**Objective:** Validate Warren in realistic multi-agent development scenario.

### Setup

**Development Task:** Implement user authentication feature across microservices

**Agent Sessions (6 total):**

1. **`auth-service`** (localhost): Implementing JWT authentication service
   - State: `thinking`
   - Files: `services/auth/src/jwt.go`, `services/auth/src/handlers.go`

2. **`user-service`** (localhost): Adding user model and database schema
   - State: `executing` (running database migration)
   - Files: `services/users/migrations/003_add_auth.sql`

3. **`api-gateway`** (dev-box-01): Integrating auth middleware
   - State: `waiting_permission` (wants to install dependency)
   - Files: `gateway/src/middleware/auth.ts`

4. **`frontend-login`** (dev-box-01): Building login UI component
   - State: `asking_question` ("Should I use form validation library or custom validation?")
   - Files: `frontend/src/components/Login.tsx`, `frontend/src/hooks/useAuth.ts`

5. **`integration-tests`** (dev-box-02): Writing end-to-end tests
   - State: `error` (test environment setup failed)
   - Files: `tests/e2e/auth.spec.ts`

6. **`docs`** (dev-box-02): Documenting authentication flow
   - State: `idle` (waiting for implementation details)
   - Files: `docs/authentication.md`

### Expected Behavior

**Warren TUI Overview:**
```
┌─ Warren — Development Workspace ───────────────────────────┐
│ 6 sessions, 3 need attention                               │
│                                                            │
│ ⚠ NEEDS ATTENTION                                          │
│   ❌ integration-tests  [error]               dev-box-02  │
│   🔒 api-gateway        [waiting_permission]  dev-box-01  │
│   ❓ frontend-login     [asking_question]     dev-box-01  │
│                                                            │
│ ✓ ACTIVE                                                   │
│   ● auth-service        [thinking]            localhost   │
│   ⚙ user-service        [executing]           localhost   │
│   ○ docs                [idle]                dev-box-02  │
│                                                            │
│ ─── Workspace Summary ────────────────────────────────── │
│ Repositories: 4 (auth, users, gateway, frontend)          │
│ Files touched: 23                                          │
│ Active servers: 3                                          │
│                                                            │
│ [Press 'w' for workspace view, 'n' for notifications]     │
└────────────────────────────────────────────────────────────┘
```

**User Workflow:**
1. User opens Warren TUI
2. Sees 3 agents need attention
3. Presses 'n' to open notification inbox
4. Selects `integration-tests` error notification
5. Views error details: "Docker daemon not running"
6. Notes to fix Docker on dev-box-02
7. Returns to overview, selects `api-gateway` permission prompt
8. Notes to approve dependency installation
9. Selects `frontend-login` question
10. Reads question context, decides on answer
11. Switches to workspace view to see all files being modified
12. Monitors progress as agents complete tasks

### Success Criteria

- [ ] All 6 agent sessions visible and correctly categorized
- [ ] Attention routing works (user can quickly find agents needing action)
- [ ] Navigation between views is smooth (overview → notifications → detail → workspace)
- [ ] Workspace summary accurate (repo count, file count, server count)
- [ ] User can understand full development context from Warren alone
- [ ] Real-time updates show progress (e.g., `user-service` migration completes)
- [ ] No performance degradation with 6 active sessions

### Edge Cases to Test

1. **Agent completes while viewing:** User viewing `auth-service` detail when it transitions to `finished`
   - Expected: Detail view updates in real-time, notification fires
2. **Server disconnect:** dev-box-02 loses network connection
   - Expected: Sessions on dev-box-02 marked as stale, user notified
3. **New agent discovered:** User starts 7th Claude Code session in tmux
   - Expected: New session appears in Warren within polling interval (< 5 seconds)

---

## Test Execution Checklist

### Pre-Test Setup
- [ ] Warren daemon running with Phase 2 features enabled
- [ ] Test servers configured and accessible
- [ ] Claude Code installed on all test servers
- [ ] Test Git repositories prepared
- [ ] Tmux sessions created with known topology

### Test Execution
- [ ] Run Scenario 1: Basic Monitoring
- [ ] Run Scenario 2: State Detection
- [ ] Run Scenario 3: Activity Tracking
- [ ] Run Scenario 4: Artifact Profile
- [ ] Run Scenario 5: Notification Flow
- [ ] Run Scenario 6: Real-World Workflow

### Post-Test Validation
- [ ] Review event database for completeness
- [ ] Check for parsing errors or low-confidence states
- [ ] Measure capture latency and overhead
- [ ] Verify no memory leaks or resource exhaustion
- [ ] Collect user feedback on TUI usability

---

## Success Metrics

**Phase 2 is validated when:**

1. **Visibility:** User can see all agent sessions across all servers without SSH
2. **Attention Routing:** User can identify which agents need action within 5 seconds
3. **Activity Understanding:** User can understand what each agent is doing from activity log
4. **Artifact Awareness:** User can see which files/repos each agent has touched
5. **Notification Reliability:** User receives notifications for all state transitions requiring attention
6. **Real-Time Updates:** TUI reflects state changes within 2 seconds
7. **Performance:** Warren handles 10+ agent sessions without noticeable lag
8. **Accuracy:** State detection and activity parsing achieve >90% accuracy on real Claude Code sessions

---

## Known Limitations (Phase 2)

These are expected limitations that will be addressed in Phase 3:

- **Read-only:** User cannot approve permissions or answer questions from Warren
- **No direct control:** User cannot send messages or commands to agents
- **Heuristic parsing:** Activity parsing may have false positives/negatives
- **Polling-based:** Updates not instant, depends on capture interval
- **No structured data:** Cannot access Claude Code's internal state directly

---

## Next Steps After Validation

If Phase 2 validation succeeds:
- Proceed to Phase 3: Interactive Hub (add control capabilities)
- Address any parsing accuracy issues discovered during testing
- Optimize capture frequency based on performance measurements
- Gather user feedback on TUI design and navigation

If Phase 2 validation fails:
- Identify root cause (tmux interface, parsing accuracy, state detection, performance)
- Revise design or implementation as needed
- Re-test before proceeding to Phase 3
