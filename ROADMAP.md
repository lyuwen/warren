# Warren — Implementation Roadmap

This roadmap breaks the design into concrete implementation tasks with clear success criteria.

---

## Phase 1: Topology + Capture Validation

**Goal:** Prove the tmux interface model works cleanly enough to build on.

**Success criteria:** Can reliably target a specific tmux pane, capture its content, send text/keystrokes into it, and verify the result.

### Tasks

#### 1.1 Server Model
- [x] Define `Server` entity (name, host, user, port, ssh_options, kind: local/remote)
- [x] Implement local server detection (localhost as special case)
- [x] Implement SSH connection pooling for remote servers
- [x] Add server registry storage (`~/.warren/servers.yaml`)
- [x] Test: connect to local and remote servers

#### 1.2 Tmux Topology Discovery
- [x] Implement `tmux list-sessions` wrapper
- [x] Implement `tmux list-windows` wrapper for a given session
- [x] Implement `tmux list-panes` wrapper for a given window
- [x] Build topology model: Server → TmuxSession → Window → Pane
- [x] Store pane metadata (pane_id, title, dimensions)
- [x] Test: discover full tmux topology on local machine
- [x] Test: discover full tmux topology on remote server

#### 1.3 Pane Capture
- [x] Implement `tmux capture-pane` wrapper
- [x] Add configurable capture window (visible + scrollback lines)
- [x] Normalize captured content (strip ANSI if needed, handle encoding)
- [x] Test: capture content from a known pane
- [x] Test: capture content from a pane running Claude Code
- [x] Measure: capture latency and overhead

#### 1.4 Pane Control
- [x] Implement `tmux send-keys` wrapper for text input
- [x] Implement `tmux send-keys` wrapper for keystroke sequences
- [x] Add target pane validation before sending input
- [x] Test: send text into a pane and verify it appears
- [x] Test: send keystroke (e.g., Enter, Shift+Tab) and verify effect
- [x] Test: send input to wrong pane and handle error gracefully

#### 1.5 Control Loop Validation
- [x] Implement: capture → validate state → send input → recapture → verify transition
- [x] Test: approve a permission prompt via keystroke
- [x] Test: send a reply to a question via text
- [x] Test: detect when target pane state changed between capture and action
- [x] Document: what works, what doesn't, what needs a better interface

**Phase 1 Gate:** If tmux capture/control is too fragile or unreliable, stop and design a better tmux integration layer before proceeding.

---

## Phase 2: Central Read-Only Hub

**Goal:** Make the distributed workspace visible from one place.

**Success criteria:** User can open Warren and see all agent sessions, their current state, recent activity, and which ones need attention.

### Tasks

#### 2.1 Agent Session Registry
- [x] Define `AgentSession` entity (logical name, server ref, tmux topology ref, created_at, last_seen_at)
- [x] Implement agent session discovery (heuristic: detect Claude Code in pane title or content)
- [ ] Store agent session mappings in registry (`~/.warren/registry.json`) - currently in-memory only
- [x] Add manual agent session registration (user specifies server/session/window/pane)
- [ ] Test: discover agent sessions across multiple servers

#### 2.2 Event Store
- [x] Choose DB (SQLite recommended for local-first design)
- [x] Design event schema for `AgentActivityEvent` and `NotificationEvent`
- [x] Implement event append (write-only, immutable events)
- [x] Implement event query (by agent, by time range, by type)
- [x] Add event retention policy (configurable, default 30 days)
- [x] Test: write and query activity events

#### 2.3 Activity Parser
- [x] Implement normalization stage (clean captured content)
- [x] Implement chat extraction (detect user/agent messages)
- [x] Implement file interaction extraction (detect Read/Edit/Write tool calls)
- [x] Implement tool activity extraction (detect Bash, LSP, other tools)
- [x] Implement prompt detection (permission prompts, questions)
- [x] Emit parsed results as `AgentActivityEvent` records
- [x] Attach confidence scores to parsed results
- [x] Test: parse known Claude Code session captures

#### 2.4 State Detection
- [x] Implement state inference from recent activity events
- [x] Map detected signals to canonical states (idle, thinking, executing, waiting_permission, asking_question, finished, error, unknown)
- [x] Apply state priority rules when multiple signals present
- [x] Emit state transition events
- [x] Test: detect state from real session captures

#### 2.5 Artifact Profile Extraction
- [ ] Define `ArtifactProfile` entity (repo roots, files visited, files edited)
- [ ] Extract artifact interactions from activity events
- [ ] Build cumulative artifact profile per agent session
- [ ] Test: track files touched by an agent over time

#### 2.6 Notification Engine
- [x] Define notification triggers (permission required, question asked, finished, error, stopped)
- [x] Emit `NotificationEvent` on state transitions - schema exists
- [x] Store notifications in event DB
- [ ] Implement notification service that watches state changes
- [ ] Mark notifications as consumed when user acts on them
- [ ] Test: generate notifications from state changes

#### 2.7 Basic TUI (Read-Only)
- [ ] Choose TUI framework (Bubble Tea, tview, or similar)
- [ ] Implement server list view
- [ ] Implement agent session list view (grouped by server)
- [ ] Implement agent detail view (state, recent chat, files touched, artifact profile)
- [ ] Implement notification inbox view
- [ ] Add keyboard navigation
- [ ] Test: browse sessions and notifications in TUI

#### 2.8 Basic Web Interface (Read-Only)
- [ ] Choose web framework (Go stdlib http, Gin, or similar)
- [ ] Implement REST API or WebSocket for core queries
- [ ] Implement server list page
- [ ] Implement agent session list page
- [ ] Implement agent detail page
- [ ] Implement notification inbox page
- [ ] Add responsive layout for mobile
- [ ] Test: browse sessions and notifications in browser

**Phase 2 Success:** User can see all agent sessions, understand what each is doing, and identify which ones need attention — all without SSHing or attaching to tmux.

---

## Phase 3: Interactive Hub

**Goal:** Act on sessions from the hub without attaching to tmux directly.

**Success criteria:** User can approve permissions, answer questions, and send messages into agent sessions from Warren, and verify the agent responded correctly.

### Tasks

#### 3.1 Action Framework
- [ ] Define action types (send_text, send_keystroke, approve_permission, deny_permission, answer_question)
- [ ] Implement action validation (check target pane still matches expected state)
- [ ] Implement action execution (send input via tmux)
- [ ] Implement action verification (recapture and confirm state transition)
- [ ] Log action results as events
- [ ] Test: execute actions and verify outcomes

#### 3.2 Permission Response
- [ ] Detect permission prompt type from captured content
- [ ] Expose approve/deny actions in TUI
- [ ] Expose approve/deny actions in web interface
- [ ] Send appropriate keystroke (e.g., `y`, `n`, or `Shift+Tab` for mode rotation)
- [ ] Verify permission prompt resolved
- [ ] Test: approve and deny permissions from Warren

#### 3.3 Question Response
- [ ] Detect question prompts from captured content
- [ ] Expose reply action in TUI (text input)
- [ ] Expose reply action in web interface (text input)
- [ ] Send reply text into target pane
- [ ] Verify agent resumed work
- [ ] Test: answer questions from Warren

#### 3.4 Message Sending
- [ ] Add "send message" action to agent detail view
- [ ] Implement text input UI in TUI
- [ ] Implement text input UI in web interface
- [ ] Send message into target pane
- [ ] Verify message appeared in pane
- [ ] Test: send messages and observe agent responses

#### 3.5 Permission Mode Rotation
- [ ] Track current permission mode per agent session
- [ ] Expose mode rotation action (send `Shift+Tab` or equivalent)
- [ ] Verify mode changed after rotation
- [ ] Display current mode in agent detail view
- [ ] Test: rotate through permission modes

#### 3.6 Notification-to-Action Flow
- [ ] Link notifications to relevant agent session
- [ ] Add "jump to agent" action from notification
- [ ] Add quick actions from notification (approve, deny, reply)
- [ ] Mark notification as consumed after action
- [ ] Test: act on notification and verify it's resolved

#### 3.7 Safety and Error Handling
- [ ] Detect when target pane state changed before action
- [ ] Show clear error when action cannot be safely executed
- [ ] Add confirmation for destructive actions
- [ ] Log failed actions with reason
- [ ] Test: handle stale state gracefully

**Phase 3 Success:** User rarely needs to attach directly to tmux for routine intervention. Most permission prompts, questions, and messages can be handled from Warren.

---

## Phase 4: Extended Operational Surfaces

**Goal:** Add secondary control planes without overcomplicating the core.

**Success criteria:** User can manage plugin state, view richer artifact history, and optionally integrate structured Claude data.

### Tasks

#### 4.1 Plugin Inventory
- [ ] Define `PluginInventoryEntry` entity (scope, server, plugin name, installed, enabled, version)
- [ ] Discover installed plugins on each server (read Claude Code plugin directories)
- [ ] Discover enabled plugins per agent session (read session config)
- [ ] Store plugin inventory in config or DB
- [ ] Test: discover plugins across servers

#### 4.2 Plugin Management UI
- [ ] Add plugin inventory view in TUI
- [ ] Add plugin inventory view in web interface
- [ ] Show plugin state per server and per agent
- [ ] Highlight version drift across servers
- [ ] Test: view plugin inventory

#### 4.3 Plugin Activation Control
- [ ] Implement enable/disable plugin actions
- [ ] Implement plugin configuration editing
- [ ] Apply changes to target server/session
- [ ] Verify plugin state changed
- [ ] Test: enable and disable plugins from Warren

#### 4.4 Richer Artifact Views
- [ ] Add file diff preview (if available from captures)
- [ ] Add git repo context (branch, commit, status)
- [ ] Add file timeline (when each file was touched)
- [ ] Test: view artifact history for an agent

#### 4.5 Notification Filtering and History
- [ ] Add notification filters (by type, by severity, by agent)
- [ ] Add notification history view (consumed notifications)
- [ ] Add notification search
- [ ] Test: filter and search notifications

#### 4.6 Structured Claude Data Integration (Optional)
- [ ] Read `~/.claude` session artifacts on target servers
- [ ] Parse structured session history
- [ ] Merge structured events with tmux-derived events
- [ ] Use structured data as authoritative source when available
- [ ] Fall back to tmux capture when structured data unavailable
- [ ] Test: compare tmux-derived vs Claude-derived activity

#### 4.7 Session Replay and Timeline
- [ ] Build timeline view from activity events
- [ ] Add replay controls (step through events)
- [ ] Show state transitions over time
- [ ] Test: replay a session's history

**Phase 4 Success:** Warren provides deep operational visibility and control, with plugin management, richer artifact views, and optional structured data integration.

---

## Cross-Cutting Concerns

These apply across all phases:

### Configuration Management
- [ ] Define config schema (`~/.warren/config.yaml`)
- [ ] Add config validation
- [ ] Add config migration for schema changes
- [ ] Document all config options

### Testing Strategy
- [ ] Unit tests for core logic (parser, state detector, event store)
- [ ] Integration tests for tmux interface (requires tmux environment)
- [ ] End-to-end tests for TUI and web interface
- [ ] Test with real Claude Code sessions
- [ ] Test with multiple servers and sessions

### Documentation
- [ ] User guide (how to set up Warren)
- [ ] Operator guide (how to use TUI and web interface)
- [ ] Developer guide (how to extend Warren)
- [ ] Architecture doc (how Warren works internally)
- [ ] Troubleshooting guide (common issues and solutions)

### Performance and Scalability
- [ ] Measure capture overhead per agent
- [ ] Optimize capture frequency (balance responsiveness vs overhead)
- [ ] Add rate limiting for SSH connections
- [ ] Add connection pooling for remote servers
- [ ] Test with 10+ servers and 50+ agent sessions

### Security
- [ ] Use SSH agent forwarding where appropriate
- [ ] Never log or store SSH credentials
- [ ] Validate all user input before sending to tmux
- [ ] Sanitize captured content before displaying
- [ ] Add audit log for all actions

---

## Open Implementation Questions

These need answers during implementation:

1. **Tmux interface reliability:** Is plain capture/send-keys clean enough, or do we need a tmux plugin or socket protocol?
2. **Parser accuracy:** What is the real-world accuracy of heuristic parsing on Claude Code sessions?
3. **Capture frequency:** What polling interval balances responsiveness and overhead?
4. **Event schema:** What is the right granularity for activity events?
5. **State detection confidence:** How should Warren display uncertainty when state is ambiguous?
6. **Plugin scope:** How much plugin management is actually useful in practice?
7. **Structured data merge:** When both tmux-derived and Claude-derived events exist, which takes precedence?

---

## Recommended Start

Begin with **Phase 1, Task 1.1-1.4** to validate the core tmux interface. If that works cleanly, proceed to Phase 2. If it's too fragile, pause and design a better integration layer before building more on top of it.
