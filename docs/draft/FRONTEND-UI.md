# Warren - Frontend UI Structure

## UI Architecture Overview

Warren's frontend needs multiple specialized interfaces to handle different aspects of agent management.

```
┌─────────────────────────────────────────────────────────────┐
│                     Warren Frontend                         │
│                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │   Desktop    │  │    Mobile    │  │   Web App    │     │
│  │     TUI      │  │     App      │  │   (Future)   │     │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘     │
│         │                 │                  │             │
│         └─────────────────┴──────────────────┘             │
│                           │                                │
│              ┌────────────▼────────────┐                   │
│              │   UI Component Layer    │                   │
│              └────────────┬────────────┘                   │
└───────────────────────────┼─────────────────────────────────┘
                            │
                ┌───────────┴───────────┐
                │                       │
        ┌───────▼────────┐      ┌──────▼──────┐
        │  Warren Core   │      │   Warren    │
        │   (Library)    │      │   Server    │
        └────────────────┘      └─────────────┘
```

## Core UI Areas

### 1. Session Management (Primary View)

**Purpose**: Monitor and interact with agent sessions

**Components:**
- Agent list/grid
- Agent detail view
- Session activity feed
- Quick actions

**Desktop TUI Layout:**
```
┌─ Warren - Sessions ─────────────────────────────────────────┐
│                                                              │
│  ┌─ Agent List ──────────┐  ┌─ Agent Detail ─────────────┐ │
│  │                        │  │                            │ │
│  │ ● backend-api    2h    │  │ Agent: backend-api         │ │
│  │   prod-1               │  │ Server: prod-1             │ │
│  │   ⚠ Waiting permission │  │ Status: ⚠ Waiting         │ │
│  │                        │  │ Uptime: 2h 34m             │ │
│  │ ● ml-trainer     5h    │  │                            │ │
│  │   gpu-box              │  │ Chat History:              │ │
│  │   ✓ Executing          │  │ ┌────────────────────────┐ │ │
│  │                        │  │ │ User: Fix auth bug     │ │ │
│  │ ? frontend       1h    │  │ │ Agent: Checking...     │ │ │
│  │   prod-2               │  │ │ Agent: Found issue     │ │ │
│  │   ? Asking question    │  │ └────────────────────────┘ │ │
│  │                        │  │                            │ │
│  │ ✓ worker-3       3h    │  │ Files:                     │ │
│  │   staging              │  │ 📝 src/auth.py            │ │
│  │   ✓ Finished           │  │ 👁 src/middleware.py      │ │
│  │                        │  │                            │ │
│  │ ✗ scraper        -     │  │ ⚠ PERMISSION REQUIRED:    │ │
│  │   prod-2               │  │   Edit src/auth.py?       │ │
│  │   ✗ Dead               │  │   [Approve] [Deny] [Diff] │ │
│  │                        │  │                            │ │
│  └────────────────────────┘  └────────────────────────────┘ │
│                                                              │
│  [1] Sessions  [2] Plugins  [3] Permissions  [4] Settings   │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Mobile Layout:**
```
┌─────────────────────────┐
│ Warren - Sessions   [≡] │
├─────────────────────────┤
│                         │
│ ● backend-api           │
│   prod-1 • 2h           │
│   ⚠ Needs permission    │
│   [View]                │
│                         │
│ ● ml-trainer            │
│   gpu-box • 5h          │
│   ✓ Executing           │
│   [View]                │
│                         │
│ ? frontend              │
│   prod-2 • 1h           │
│   ? Asking question     │
│   [View]                │
│                         │
│ ✓ worker-3              │
│   staging • 3h          │
│   ✓ Finished            │
│   [View]                │
│                         │
│         [+ New]         │
│                         │
└─────────────────────────┘
```

### 2. Notification System

**Purpose**: Alert user to important agent state changes

**Notification Types:**

```python
class NotificationType(Enum):
    PERMISSION_REQUIRED = "permission_required"    # High priority
    QUESTION_ASKED = "question_asked"              # High priority
    TASK_FINISHED = "task_finished"                # Medium priority
    ERROR_OCCURRED = "error_occurred"              # High priority
    AGENT_IDLE = "agent_idle"                      # Low priority
    AGENT_STARTED = "agent_started"                # Low priority
    AGENT_STOPPED = "agent_stopped"                # Medium priority
    PLUGIN_UPDATE = "plugin_update"                # Low priority
```

**Notification UI - Desktop TUI:**
```
┌─ Notifications ─────────────────────────────────────────────┐
│                                                              │
│  ⚠ backend-api needs permission (2 min ago)                 │
│     Allow edit to src/auth.py?                              │
│     [View] [Approve] [Deny] [Dismiss]                       │
│                                                              │
│  ? frontend is asking a question (5 min ago)                │
│     Which approach should I use?                            │
│     [View] [Reply] [Dismiss]                                │
│                                                              │
│  ✓ worker-3 finished task (10 min ago)                      │
│     All tests passed                                        │
│     [View] [Dismiss]                                        │
│                                                              │
│  ✗ scraper encountered error (15 min ago)                   │
│     Connection timeout                                      │
│     [View] [Restart] [Dismiss]                              │
│                                                              │
│  [Clear All] [Settings]                                     │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Notification UI - Mobile:**
```
┌─────────────────────────┐
│ Notifications       [×] │
├─────────────────────────┤
│                         │
│ ⚠ backend-api           │
│   Needs permission      │
│   2 min ago             │
│   [View] [Approve]      │
│                         │
│ ? frontend              │
│   Asking question       │
│   5 min ago             │
│   [View] [Reply]        │
│                         │
│ ✓ worker-3              │
│   Task finished         │
│   10 min ago            │
│   [View]                │
│                         │
│ ✗ scraper               │
│   Error occurred        │
│   15 min ago            │
│   [View] [Restart]      │
│                         │
│     [Clear All]         │
│                         │
└─────────────────────────┘
```

**Notification Badge/Indicator:**
```
Desktop TUI:
┌─ Warren ─────────────────────────────────────────────────────┐
│  Sessions  Plugins  Permissions  Notifications (4) ⚠         │
└──────────────────────────────────────────────────────────────┘

Mobile:
┌─────────────────────────┐
│ Warren          [🔔 4]  │  ← Badge shows unread count
└─────────────────────────┘
```

**Push Notifications (Mobile):**
```
┌─────────────────────────────────┐
│ Warren                          │
│ backend-api needs permission    │
│ Allow edit to src/auth.py?      │
│                                 │
│ [Approve] [View]                │
└─────────────────────────────────┘
```

**Notification Settings:**
```
┌─ Notification Settings ─────────────────────────────────────┐
│                                                              │
│  Enable notifications:           [✓]                        │
│                                                              │
│  Notification types:                                         │
│    [✓] Permission required       (High priority)            │
│    [✓] Question asked            (High priority)            │
│    [✓] Task finished             (Medium priority)          │
│    [✓] Error occurred            (High priority)            │
│    [ ] Agent idle                (Low priority)             │
│    [ ] Agent started             (Low priority)             │
│    [✓] Agent stopped             (Medium priority)          │
│                                                              │
│  Delivery:                                                   │
│    Desktop: [✓] In-app  [✓] System notification            │
│    Mobile:  [✓] In-app  [✓] Push notification              │
│                                                              │
│  Quiet hours:                                                │
│    [✓] Enable quiet hours                                   │
│    From: [22:00] To: [08:00]                                │
│    During quiet hours: [ ] Disable all                      │
│                        [✓] High priority only               │
│                                                              │
│  Per-agent settings:                                         │
│    backend-api:  [All notifications]                        │
│    ml-trainer:   [Errors only]                              │
│    frontend:     [All notifications]                        │
│    worker-3:     [Muted]                                    │
│                                                              │
│  [Save] [Cancel]                                            │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 3. Plugin Management Interface

**Purpose**: Manage plugins across all servers and agents

**Desktop TUI Layout:**
```
┌─ Warren - Plugins ──────────────────────────────────────────┐
│                                                              │
│  ┌─ Servers ─────────┐  ┌─ Plugin Details ───────────────┐ │
│  │                    │  │                                │ │
│  │ ▼ prod-1 (5)       │  │ Plugin: code-review           │ │
│  │   ✓ code-review    │  │ Version: 1.2.0                │ │
│  │   ✓ database-tools │  │ Author: warren-plugins        │ │
│  │   ✓ deployment     │  │                                │ │
│  │   ✗ testing-utils  │  │ Description:                  │ │
│  │   ⚠ security-scan  │  │ Automated code review tool    │ │
│  │                    │  │ with security checks          │ │
│  │ ▼ prod-2 (3)       │  │                                │ │
│  │   ✓ code-review    │  │ Components:                   │ │
│  │   ✓ database-tools │  │ • Commands: /review           │ │
│  │   ✗ deployment     │  │ • Skills: code-review         │ │
│  │                    │  │ • Hooks: PreToolUse           │ │
│  │ ▼ gpu-box (2)      │  │                                │ │
│  │   ✓ ml-tools       │  │ Installed on:                 │ │
│  │   ✓ data-proc      │  │ • prod-1 (v1.2.0) ✓           │ │
│  │                    │  │ • prod-2 (v1.1.0) ⚠           │ │
│  │                    │  │ • staging (v1.2.0) ✓          │ │
│  │                    │  │                                │ │
│  │ [Install Plugin]   │  │ [Configure] [Update]          │ │
│  │ [Sync Versions]    │  │ [Enable All] [Disable All]    │ │
│  │                    │  │                                │ │
│  └────────────────────┘  └────────────────────────────────┘ │
│                                                              │
│  [1] Sessions  [2] Plugins  [3] Permissions  [4] Settings   │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Mobile Layout:**
```
┌─────────────────────────┐
│ Warren - Plugins    [≡] │
├─────────────────────────┤
│                         │
│ ▼ prod-1 (5 plugins)    │
│   ✓ code-review  v1.2.0 │
│   ✓ database-tools      │
│   ✓ deployment          │
│   ✗ testing-utils       │
│   ⚠ security-scan       │
│                         │
│ ▼ prod-2 (3 plugins)    │
│   ✓ code-review  v1.1.0 │
│     ⚠ Update available  │
│   ✓ database-tools      │
│   ✗ deployment          │
│                         │
│ ▼ gpu-box (2 plugins)   │
│   ✓ ml-tools            │
│   ✓ data-processing     │
│                         │
│   [Install Plugin]      │
│   [Sync All]            │
│                         │
└─────────────────────────┘
```

**Plugin Detail View (Mobile):**
```
┌─────────────────────────┐
│ ← code-review           │
├─────────────────────────┤
│ Version: 1.2.0          │
│ Author: warren-plugins  │
│                         │
│ Description:            │
│ Automated code review   │
│ tool with security      │
│ checks                  │
│                         │
│ Components:             │
│ • Commands: /review     │
│ • Skills: code-review   │
│ • Hooks: PreToolUse     │
│                         │
│ Installed on:           │
│ • prod-1 (v1.2.0) ✓     │
│ • prod-2 (v1.1.0) ⚠     │
│ • staging (v1.2.0) ✓    │
│                         │
│ [Configure]             │
│ [Update All]            │
│ [Enable All]            │
│ [Disable All]           │
│                         │
└─────────────────────────┘
```

**Plugin Configuration View:**
```
┌─ Configure: code-review ────────────────────────────────────┐
│                                                              │
│  Scope: ○ Global  ● Server (prod-1)  ○ Agent (backend-api) │
│                                                              │
│  ┌─ Settings ──────────────────────────────────────────────┐│
│  │                                                          ││
│  │ auto_review:        [✓] Enable                          ││
│  │                                                          ││
│  │ review_depth:       ○ Quick                             ││
│  │                     ● Standard                           ││
│  │                     ○ Thorough                           ││
│  │                                                          ││
│  │ review_checklist:   [✓] Security                        ││
│  │                     [✓] Performance                      ││
│  │                     [✓] Tests                            ││
│  │                     [ ] Documentation                    ││
│  │                     [ ] Style                            ││
│  │                                                          ││
│  │ max_file_size:      [1000] KB                           ││
│  │                                                          ││
│  │ exclude_paths:      node_modules/                       ││
│  │                     dist/                                ││
│  │                     [+ Add]                              ││
│  │                                                          ││
│  └──────────────────────────────────────────────────────────┘│
│                                                              │
│  [Save] [Cancel] [Reset to Defaults]                        │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 4. Permission Management Interface

**Purpose**: Configure and manage agent permission modes

**Desktop TUI Layout:**
```
┌─ Warren - Permissions ──────────────────────────────────────┐
│                                                              │
│  ┌─ Agents ──────────┐  ┌─ Permission Settings ──────────┐ │
│  │                    │  │                                │ │
│  │ backend-api        │  │ Agent: backend-api            │ │
│  │   Mode: Default    │  │ Server: prod-1                │ │
│  │                    │  │                                │ │
│  │ ml-trainer         │  │ Permission Mode:              │ │
│  │   Mode: Auto       │  │   ○ Auto (approve all)        │ │
│  │                    │  │   ● Default (prompt risky)    │ │
│  │ frontend           │  │   ○ Plan (require plan)       │ │
│  │   Mode: Plan       │  │   ○ Accept Edits              │ │
│  │                    │  │   ○ Don't Ask                 │ │
│  │ worker-3           │  │                                │ │
│  │   Mode: Default    │  │ Tool Permissions:             │ │
│  │                    │  │   ✓ Read      (allow)         │ │
│  │ scraper            │  │   ? Write     (prompt)        │ │
│  │   Mode: Auto       │  │   ? Edit      (prompt)        │ │
│  │                    │  │   ✗ Bash      (deny)          │ │
│  │                    │  │   ✓ LSP       (allow)         │ │
│  │                    │  │   ? WebSearch (prompt)        │ │
│  │                    │  │                                │ │
│  │ [Set Defaults]     │  │ Path Rules:                   │ │
│  │ [Bulk Edit]        │  │   src/**/*.py → allow edit    │ │
│  │                    │  │   config/**   → read only     │ │
│  │                    │  │   .env*       → deny all      │ │
│  │                    │  │   [+ Add Rule]                │ │
│  │                    │  │                                │ │
│  │                    │  │ [Save] [Apply to All]         │ │
│  │                    │  │                                │ │
│  └────────────────────┘  └────────────────────────────────┘ │
│                                                              │
│  [1] Sessions  [2] Plugins  [3] Permissions  [4] Settings   │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Mobile Layout:**
```
┌─────────────────────────┐
│ Warren - Permissions[≡] │
├─────────────────────────┤
│                         │
│ backend-api             │
│   Mode: Default         │
│   [Configure]           │
│                         │
│ ml-trainer              │
│   Mode: Auto            │
│   [Configure]           │
│                         │
│ frontend                │
│   Mode: Plan            │
│   [Configure]           │
│                         │
│ worker-3                │
│   Mode: Default         │
│   [Configure]           │
│                         │
│ scraper                 │
│   Mode: Auto            │
│   [Configure]           │
│                         │
│   [Set Defaults]        │
│   [Bulk Edit]           │
│                         │
└─────────────────────────┘
```

**Permission Detail View (Mobile):**
```
┌─────────────────────────┐
│ ← backend-api           │
├─────────────────────────┤
│ Permission Mode:        │
│ ○ Auto                  │
│ ● Default               │
│ ○ Plan                  │
│ ○ Accept Edits          │
│                         │
│ Tool Permissions:       │
│ ✓ Read      (allow)     │
│ ? Write     (prompt)    │
│ ? Edit      (prompt)    │
│ ✗ Bash      (deny)      │
│ ✓ LSP       (allow)     │
│                         │
│ Path Rules:             │
│ src/**/*.py             │
│   → allow edit          │
│ config/**               │
│   → read only           │
│ .env*                   │
│   → deny all            │
│ [+ Add Rule]            │
│                         │
│ [Save]                  │
│                         │
└─────────────────────────┘
```

**Bulk Permission Edit:**
```
┌─ Bulk Edit Permissions ─────────────────────────────────────┐
│                                                              │
│  Apply to:                                                   │
│    [✓] backend-api                                          │
│    [✓] ml-trainer                                           │
│    [ ] frontend                                             │
│    [✓] worker-3                                             │
│    [ ] scraper                                              │
│                                                              │
│  Set permission mode:                                        │
│    ○ Auto                                                   │
│    ● Default                                                │
│    ○ Plan                                                   │
│    ○ Accept Edits                                           │
│    ○ Don't change                                           │
│                                                              │
│  Tool permissions:                                           │
│    Read:      [Allow ▼]                                     │
│    Write:     [Prompt ▼]                                    │
│    Edit:      [Prompt ▼]                                    │
│    Bash:      [Deny ▼]                                      │
│                                                              │
│  [Apply] [Cancel]                                           │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## Navigation & Layout

### Desktop TUI - Tab Navigation

```
┌─ Warren ─────────────────────────────────────────────────────┐
│                                                              │
│  [1] Sessions  [2] Plugins  [3] Permissions  [4] Settings   │
│  ─────────────                                               │
│                                                              │
│  ... Session Management UI ...                               │
│                                                              │
│  Notifications (4) ⚠  [View All]                            │
│                                                              │
└──────────────────────────────────────────────────────────────┘

Keyboard shortcuts:
- 1, 2, 3, 4: Switch tabs
- n: Open notifications
- /: Search/filter
- ?: Help
```

### Mobile - Bottom Navigation

```
┌─────────────────────────┐
│                         │
│   ... Current View ...  │
│                         │
│                         │
│                         │
├─────────────────────────┤
│ [🏠] [🔌] [🔒] [⚙️] [🔔]│
│ Home Plug Perm Set Notif│
└─────────────────────────┘
```

### Mobile - Hamburger Menu

```
┌─────────────────────────┐
│ Warren          [≡] [🔔]│
├─────────────────────────┤
│ ≡ Menu                  │
│                         │
│ 🏠 Sessions             │
│ 🔌 Plugins              │
│ 🔒 Permissions          │
│ 📊 Dashboard            │
│ ⚙️  Settings            │
│ ❓ Help                 │
│ 🚪 Logout               │
│                         │
└─────────────────────────┘
```

## Responsive Design Considerations

### Desktop TUI (Terminal)
- Multi-pane layout (list + detail)
- Keyboard-driven navigation
- Rich text formatting
- Real-time updates
- Minimum width: 80 columns

### Mobile App
- Single-pane navigation
- Touch-friendly controls
- Swipe gestures
- Pull-to-refresh
- Bottom navigation
- Minimum width: 320px

### Tablet (Future)
- Hybrid layout (can show 2 panes)
- Touch + keyboard support
- Landscape optimization

## State Synchronization

All UI layers need to stay synchronized:

```python
class UIStateManager:
    """Synchronize state across UI components"""
    
    def __init__(self):
        self.observers = []
        
    def subscribe(self, observer):
        """Subscribe to state changes"""
        self.observers.append(observer)
        
    def notify(self, event_type, data):
        """Notify all observers of state change"""
        for observer in self.observers:
            observer.on_state_change(event_type, data)
            
    # State change events
    def agent_state_changed(self, agent_name, new_state):
        self.notify('agent_state', {
            'agent': agent_name,
            'state': new_state
        })
        
    def plugin_enabled(self, agent_name, plugin_name):
        self.notify('plugin_state', {
            'agent': agent_name,
            'plugin': plugin_name,
            'enabled': True
        })
        
    def permission_changed(self, agent_name, permission_mode):
        self.notify('permission_mode', {
            'agent': agent_name,
            'mode': permission_mode
        })
```

## UI Component Hierarchy

```
Warren UI
├── Session Management
│   ├── Agent List
│   │   ├── Agent Card
│   │   └── Agent Status Badge
│   ├── Agent Detail
│   │   ├── Chat History
│   │   ├── File List
│   │   ├── Permission Prompt
│   │   └── Quick Actions
│   └── Session Activity Feed
│
├── Notification System
│   ├── Notification Center
│   │   ├── Notification List
│   │   └── Notification Item
│   ├── Notification Badge
│   ├── Push Notification Handler
│   └── Notification Settings
│
├── Plugin Management
│   ├── Server Plugin List
│   ├── Plugin Detail
│   │   ├── Plugin Info
│   │   ├── Installation Status
│   │   └── Version Comparison
│   ├── Plugin Configuration
│   └── Plugin Actions
│
├── Permission Management
│   ├── Agent Permission List
│   ├── Permission Detail
│   │   ├── Mode Selector
│   │   ├── Tool Permissions
│   │   └── Path Rules
│   ├── Bulk Edit
│   └── Permission Templates
│
└── Settings
    ├── General Settings
    ├── Notification Settings
    ├── Server Configuration
    └── User Preferences
```

## Implementation Priority

### Phase 1: Core Session Management
1. Agent list view
2. Agent detail view
3. Basic notifications (in-app only)
4. Session activity monitoring

### Phase 2: Notification System
1. Notification center
2. State-based notifications
3. Notification settings
4. Push notifications (mobile)

### Phase 3: Plugin Management
1. Plugin discovery and listing
2. Enable/disable plugins
3. Plugin configuration UI
4. Version management

### Phase 4: Permission Management
1. Permission mode configuration
2. Tool permission settings
3. Path-based rules
4. Bulk editing

### Phase 5: Polish & Integration
1. Cross-component navigation
2. Search and filtering
3. Keyboard shortcuts (TUI)
4. Gesture support (mobile)
5. Real-time synchronization

## Open Questions

1. **UI framework for TUI**: Textual vs rich+prompt_toolkit?
   - Textual: More modern, better widgets
   - rich+prompt_toolkit: More control, lighter

2. **Mobile app approach**: PWA vs React Native vs Flutter?
   - PWA: Fastest to market, no app store
   - React Native: Better native feel
   - Flutter: Best performance

3. **Real-time updates**: WebSocket vs polling?
   - WebSocket: True real-time, more complex
   - Polling: Simpler, good enough for most cases

4. **State management**: Redux vs Context vs Zustand?
   - Need to keep UI in sync across components
   - Handle optimistic updates

5. **Offline support**: How much should work offline?
   - View cached state?
   - Queue actions?
   - Sync when reconnected?
