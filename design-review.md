# Warren — Revised Design Review

## 1. Overview

Warren is a central hub for supervising and interacting with distributed coding-agent sessions that run inside tmux across multiple local and remote servers.

The core problem is not generic orchestration. The problem is that one working environment now spans many machines, many tmux sessions, many agent panes, many plugin states, and many permission prompts. Today that workspace is fragmented across SSH logins and tmux windows. Warren pulls it back into one coherent control surface.

At its core, Warren does four things:

1. tracks where sessions and agents live,
2. captures what those agent panes are doing,
3. classifies when they need user attention,
4. lets the user act on them from a central hub.

The foundation is a local core that knows about servers, tmux topology, pane capture, and control. On top of that sit two user interfaces: a desktop TUI and a web interface. The web interface is the remote/mobile-friendly surface; the TUI is the operator console.

---

## 2. Product Vision

Warren should feel like mission control for a distributed coding workspace.

Instead of remembering which server hosts which tmux session, which window contains which agent, and which pane is waiting for permission, the user should be able to open Warren and immediately answer:

- What agent sessions are currently active?
- Which tmux session, window, and pane hosts each one?
- What is each agent working on right now?
- Which files or repositories has each agent touched?
- Which agent is blocked, idle, finished, or asking for help?
- Which session needs intervention right now?

Warren is not primarily a replacement for tmux or SSH. It is a control layer over them.

It is also not a generic cluster manager. Its center of gravity is Claude Code-style agent sessions: chat-oriented, tool-using, permission-gated coding sessions that usually live in a single tmux pane.

---

## 3. Scope and Non-Goals

### In scope

- Central registry of local and remote servers
- Tmux-backed session discovery and management
- Mapping from server → tmux session → window → pane → agent session
- Capture of visible and recent pane content from tmux
- Parsing captured content into chat/activity/file/state summaries
- Detection of actionable states such as waiting for permission, asking a question, or finished
- Central interaction with agent panes by sending text or keystrokes back into tmux
- A lightweight permission-state tracker and permission-mode rotator
- A plugin inventory model and plugin activation/configuration control surface
- Dedicated desktop TUI and web interface surfaces
- A notification system driven by agent-state transitions

### Out of scope for the current design

- Full autonomous orchestration of multi-agent workflows
- General-purpose deployment pipeline management
- Replacing existing monitoring stacks like Prometheus or Grafana
- Multi-user collaboration and RBAC-heavy enterprise sharing
- Building a heavy rule engine for permissions in the first version
- Deep structured extraction from `~/.claude` as a first implementation step

### Deferred but expected later

- Reading and indexing session history from `~/.claude`
- Rich file diff review tied to actual pending edits
- Better replay and timeline views
- More explicit coordination primitives across agents
- Richer plugin management beyond inventory and activation state

---

## 4. Core Design Principles

1. **Centralize attention, not execution.** Agents still run remotely, inside tmux, on the machines where they belong. Warren centralizes visibility and control.
2. **Model tmux topology explicitly.** Tmux session, window, pane, and agent session are not the same thing.
3. **Use tmux as the first source of truth.** The first usable version should work by targeting real tmux panes.
4. **Separate core from interfaces.** Session capture/control logic must not be tangled with TUI or web UI code.
5. **Design for intervention.** The most important states are the ones where the user must act: permission prompts, questions, completion, and errors.
6. **Keep permission management simple.** Track state, expose mode, and allow rotation through modes or prompt responses. Do not overbuild rules too early.
7. **Treat plugins as operational inventory, not bundled payload.** Warren tracks and manages installation/activation state, but plugin files remain on the target systems.
8. **Make event streams first-class.** Activities and notifications should be modeled as persisted event streams.
9. **Stay reviewable by humans.** The product must make it easy to inspect what the agent saw, what it did, where it lived, and why Warren thinks it is in a given state.

---

## 5. System Context

### High-level picture

```text
┌─────────────────────────────────────────────────────────────┐
│                        Warren Frontends                     │
│                                                             │
│   Desktop TUI                     Web Interface             │
│   - operator console              - remote/mobile-friendly  │
│   - keyboard-first                - compact intervention    │
│   - deep workspace view           - easy access anywhere    │
└───────────────────────────────┬─────────────────────────────┘
                                │
                    commands / queries / events
                                │
┌───────────────────────────────▼─────────────────────────────┐
│                         Warren Core                         │
│                                                             │
│  - server registry                                          │
│  - tmux topology model                                      │
│  - agent session registry                                   │
│  - SSH transport                                            │
│  - tmux capture / keystroke control                         │
│  - activity parser                                          │
│  - state detector                                           │
│  - event store                                              │
│  - permission tracker                                       │
│  - plugin inventory                                         │
│  - notification engine                                      │
└───────────────────────────────┬─────────────────────────────┘
                                │
                                │ SSH / local shell
                                │
        ┌───────────────────────┼────────────────────────┐
        │                       │                        │
┌───────▼───────┐      ┌────────▼────────┐      ┌───────▼───────┐
│ local machine │      │    server-b     │      │   server-c    │
│ tmux sessions │      │ tmux sessions    │      │ tmux sessions │
│ Claude Code   │      │ Claude Code      │      │ Claude Code   │
│ plugins       │      │ plugins          │      │ plugins       │
│ ~/.claude     │      │ ~/.claude        │      │ ~/.claude     │
└───────────────┘      └──────────────────┘      └───────────────┘
```

### Conceptual model

There are two kinds of sessions and they must not be collapsed into one term.

- A **tmux session** is a tmux container on a given server. A single server may host many tmux sessions. Each tmux session contains windows and panes.
- An **agent session** usually lives in one specific tmux pane within one specific window of one specific tmux session. In practice this is typically one Claude Code instance.

There is also an artifact model:

- An **artifact profile** is the git repository and/or the files an agent session has visited, read, edited, or otherwise worked on.

Additional key concepts:

- A **server** is a local or remote machine Warren can reach. A local tmux environment is just a special case of a server.
- A **capture** is a recent snapshot of one agent pane.
- An **activity summary** is a parsed view over one or more captures.
- A **permission state** is the currently observed permission posture or pending permission prompt for an agent session.
- A **plugin inventory** is Warren’s abstract central view of plugin installation/activation/configuration state across environments.

---

## 6. Core Architecture

### 6.1 Warren Core

Warren Core is the stable center of the system. It is not a UI. It is a library plus local command surface responsible for remote session supervision.

Its responsibilities are:

- store server definitions, including local servers as a special case,
- store known mappings down to tmux session → window → pane → agent session,
- connect to local or remote servers,
- discover or target tmux sessions, windows, and panes,
- capture pane content,
- parse captured content,
- detect session state,
- send input back into sessions as either string text or keystrokes,
- manage permission state and permission-mode rotation,
- manage plugin inventory and configuration state,
- emit events for notifications and UI updates.

Permission interaction is intentionally simple in the first design. In practice, many permission interactions are just keystrokes, including mode rotation behaviors like `Shift+Tab`.

Plugin inventory is also intentionally abstract. It does not contain actual plugin files. The actual installation and activation at user scope or project scope is managed by the Core on the target environment.

### 6.2 Remote runtime assumptions

The first design assumes:

- the coding agent already runs inside tmux on a local or remote server,
- Warren can reach that environment via SSH or local shell,
- Warren can inspect tmux sessions, windows, and panes there,
- Warren can send either text or keystrokes into the target pane,
- Warren can read relevant Claude Code config/plugin directories,
- Warren may later read `~/.claude` session artifacts directly.

### 6.3 Interface surfaces

Warren Core supports two frontends:

- **Desktop TUI** as the full operator interface.
- **Web interface** as the remote/mobile-friendly surface.

Both frontends should consume the same model: server list, tmux topology, agent sessions, activity stream, artifact profile, permission state, plugin inventory, alerts, and actions.

---

## 7. Data and Domain Model

### 7.1 Core entities

```text
Server
- name
- host
- user
- port
- ssh options (use $HOME/.ssh)
- tags
- kind (local or remote)

TmuxSession
- server reference
- tmux session name
- windows
- last_seen_at

TmuxPane
- tmux session reference
- window index
- pane index
- pane id
- title
- last_seen_at

AgentSession
- logical name
- server reference
- tmux session reference
- window reference
- pane reference
- created_at
- last_seen_at
- capture policy
- permission mode

ArtifactProfile
- repo roots touched
- files visited
- files edited
- last_artifact_event_at

AgentActivityEvent
- timestamp
- agent session reference
- event type
- raw tmux snapshot or excerpt
- parsed chat excerpt
- parsed file interaction
- parsed tool activity
- pending prompt state
- inferred state transition

PermissionState
- current mode
- pending prompt
- last prompt type
- last observed rotation state

PluginInventoryEntry
- scope (user/project)
- target server
- plugin name
- installed
- enabled
- version
- components exposed
- config overrides

NotificationEvent
- type
- source agent
- timestamp
- severity
- action affordances
- consumed_at
```

The important modeling change is that activity and notification records are events, not just current-state snapshots.

### 7.2 Storage direction

Configuration can stay file-based, but activities and notifications should be persisted as event streams in a database.

Likely local storage areas:

- `~/.warren/config.yaml`
- `~/.warren/servers.yaml`
- `~/.warren/registry.json`
- `~/.warren/plugins.json` or equivalent config projection
- `~/.warren/state.db` for persisted event streams

Recommended split:

- **Files** for durable configuration and desired state
- **DB** for activity events and notification events

Notifications are consumable events, not just static records.

The design should preserve a clear distinction between:

- **observed state** from local/remote environments,
- **desired state** configured in Warren,
- **derived state** such as inferred agent status or attention priority,
- **event streams** that record how the observed state changed over time.

---

## 8. Capture and Event Pipeline

### 8.1 Phase-1 capture source

The first operational source is tmux itself.

Warren targets a specific pane and captures recent pane content. The capture window should include:

- currently visible pane content,
- recent scrollback,
- enough lines to reconstruct recent chat and tool activity,
- optionally terminal metadata if available.

### 8.2 Capture pipeline

This needs more validation before implementation. A key open question is whether plain pane capture is enough, or whether Warren needs a cleaner interface to tmux itself.

So the first design question here is not “how do we parse everything,” but:

- Can tmux capture plus targeted keystroke injection form a clean enough interface?
- Or do we need a more explicit tmux integration layer first?

Provisional pipeline:

```text
server → tmux session → window → pane
                          ↓
                    pane capture / pane control
                          ↓
                    normalization
                          ↓
                    parser stages
                          ↓
              tmux / Claude Code event stream
                          ↓
                 activity + notification events
                          ↓
                    frontend views
```

This part should be treated as an investigation gate, not a solved detail.

### 8.3 Parser outputs

The parser should attempt to extract at least:

- recent user messages,
- recent agent responses,
- files read, written, edited, or referenced,
- commands executed,
- pending permission prompts,
- pending user-choice questions,
- finish/error markers,
- artifact-profile updates,
- recency of visible activity.

### 8.4 Parser strategy

The parser will initially be heuristic. It should be a pipeline that emits events and confidence, rather than a monolithic parser that claims full certainty.

### 8.5 Future structured source

Later, Warren should support structured session/history ingestion from `~/.claude`.

The long-term output model should still converge on a unified tmux/Claude Code event stream, regardless of whether the upstream source was pane capture or Claude-local structured data.

---

## 9. Agent State Model

Warren needs a first-class model of session state because the UI and notification system revolve around it.

### Canonical states

```text
idle
thinking
executing
waiting_permission
asking_question
finished
error
unknown
```

### State priority

When multiple signals are present, Warren should classify according to user-action priority.

Recommended precedence:

1. `waiting_permission`
2. `asking_question`
3. `error`
4. `finished`
5. `executing`
6. `thinking`
7. `idle`
8. `unknown`

### State transitions

State changes should be recorded as events, not just current flags. That supports notifications, timelines, debugging, and later replay.

---

## 10. Interaction Model

The central promise of Warren is not just seeing sessions but acting on them without attaching directly to tmux.

### Passive operations

- see current status,
- inspect recent chat,
- inspect recent file interactions,
- inspect artifact profile,
- inspect plugin state,
- inspect permission state,
- inspect drift across servers.

### Active operations

- send a text message into a target pane,
- send a keystroke or shortcut into a target pane,
- answer a question from a session,
- approve or deny a permission prompt,
- request a file view from the remote workspace,
- open a raw session directly when needed,
- rotate permission mode,
- activate/deactivate/configure plugins.

### Safety principle

Warren should not pretend to know more than it does. A safe control loop is:

capture target pane → detect current prompt/state → verify target still matches → send text or keystroke → recapture → confirm state transition.

---

## 11. Permission Management Design

Permission management is intentionally lightweight in this design.

It is primarily a state tracker and action surface, not a large policy engine.

Warren should know, per agent session:

- current permission mode,
- whether the session is waiting on a permission prompt,
- what the next rotation action would do,
- what prompt response is currently available.

The first implementation should focus on:

- showing the current mode,
- tracking prompt state,
- sending prompt responses,
- rotating modes when needed.

Do not overbuild tool-rule or path-rule systems in the first version unless the real runtime proves they are essential.

### Open implementation caution

Prefer a simple rotation model for permissions first. If permission changes are usually mediated by keystrokes, Warren should treat that as the primary control path until a better runtime interface is proven.

---

## 12. Plugin Management Design

Plugin management is relatively low priority in the first iteration, but the design should leave space for it.

The first version should focus on:

- central inventory,
- installation state,
- enable/disable state,
- version visibility,
- scope visibility (user/project),
- minimal activation/configuration controls.

This should be treated as an extensible subsystem, not a phase-1 centerpiece.

---

## 13. Notification System Design

Notifications are events emitted from agent-state transitions.

At minimum:

- permission required,
- agent asked a question,
- agent finished,
- agent hit an error,
- agent stopped unexpectedly.

Notifications should be persisted as consumable events in the DB.

That means:

- they can be unread or consumed,
- they can drive web/TUI attention surfaces,
- they can be replayed or filtered later.

---

## 14. Frontend Design Overview

The frontend is not one screen. It is a set of specialized views around four domains:

1. session management,
2. notifications,
3. permission tracking,
4. plugin inventory.

The desktop TUI is the heavy-duty operator console. The web interface is the accessible remote surface.

---

## 15. Desktop TUI Design

The desktop TUI is the main operator console. It should support:

- scanning the whole workspace,
- navigating tmux topology,
- drilling into one agent pane,
- resolving prompts quickly,
- comparing session state,
- doing bulk administrative actions.

Recommended shape:

- left pane: servers / tmux sessions / windows / panes / agents,
- main pane: selected agent detail,
- lower or side surfaces: notifications, activity stream, artifact profile,
- top-level navigation tabs: Sessions / Notifications / Permissions / Plugins / Settings.

---

## 16. Web Interface Design

The web interface is the remote/mobile-friendly surface.

Its primary jobs are:

- tell the user what needs attention,
- let the user inspect recent context,
- let the user approve/deny/reply quickly,
- let the user view where an agent lives in tmux topology,
- let the user monitor progress without needing a terminal.

The web interface should stay lighter than the TUI, but it should still expose the same underlying model.

---

## 17. Visualized Core Design

```text
server (local or remote)
    ↓
 tmux session
    ↓
   window
    ↓
    pane
    ↓
 agent session
    ↓
 capture / parse / control
    ↓
 activity + notification event streams
    ↓
 TUI / web interface
```

---

## 18. Phased Delivery

### Phase 1 — Topology + Capture Validation

Goal: prove the tmux interface model.

Includes:

- server model,
- tmux topology model,
- pane targeting,
- pane capture,
- basic keystroke injection,
- proof that capture/control can be done cleanly enough.

### Phase 2 — Central Read-Only Hub

Goal: make the distributed workspace visible from one place.

Includes:

- agent session registry,
- activity parsing,
- state detection,
- artifact profile extraction,
- notification event stream,
- basic TUI/web read-only views.

### Phase 3 — Interactive Hub

Goal: act on sessions from the hub.

Includes:

- send text into sessions,
- send keystrokes into sessions,
- answer questions,
- approve/deny permission prompts,
- rotate permission mode,
- verify state transitions after actions.

### Phase 4 — Extended Operational Surfaces

Goal: add secondary control planes without overcomplicating the core.

Includes:

- plugin inventory and activation controls,
- richer notification filtering,
- deeper artifact views,
- later `~/.claude` integration.

---

## 19. Open Questions

1. Is plain tmux pane capture and keystroke control clean enough, or do we need a more explicit tmux integration layer?
2. What is the safest exact control loop for sending text or keystrokes into a live pane?
3. What is the minimum topology model needed in the UI: server → tmux session → pane, or server → tmux session → window → pane?
4. What should the persisted event schema for activities and notifications look like?
5. How should artifact profiles be projected from the raw event stream?
6. How much plugin state is truly useful in the first version?
7. When Warren later reads `~/.claude`, how do tmux-derived and Claude-derived events merge into one stream?

---

## 20. Recommended Design Direction

The strongest path forward is:

- model tmux topology explicitly,
- treat local and remote environments uniformly,
- validate the tmux capture/control interface before building too much on top of it,
- store activity and notification history as event streams in a DB,
- keep permission handling simple and action-oriented,
- leave plugin management extensible but secondary,
- build a TUI and a web interface over the same core model.

In short: Warren should first become the place where the user understands and supervises the whole distributed workspace, with the tmux pane as the primary unit of observation and control.
