# Warren - Agent State Detection & Permission Management

## Agent State Detection

Warren needs to understand what state each agent is in by analyzing tmux output.

### Agent States

```python
class AgentState(Enum):
    IDLE = "idle"                    # Waiting for user input
    THINKING = "thinking"            # Processing/reasoning
    EXECUTING = "executing"          # Running tools/commands
    WAITING_PERMISSION = "waiting_permission"  # Blocked on permission prompt
    ASKING_QUESTION = "asking_question"        # Asking user a question
    FINISHED = "finished"            # Completed task
    ERROR = "error"                  # Encountered error
    UNKNOWN = "unknown"              # Can't determine state
```

### State Detection Patterns

**1. IDLE - Waiting for user input**
```
Patterns to detect:
- Cursor at empty prompt
- Last line is just ">" or prompt character
- No recent activity (last 30+ seconds)
- Agent said "What would you like me to do?"

Example tmux output:
  ...previous conversation...
  
  >█
```

**2. THINKING - Agent is processing**
```
Patterns to detect:
- Spinner/progress indicator visible
- "Thinking..." or "Processing..." text
- Recent activity but no tool calls yet
- Cursor not at prompt

Example tmux output:
  Let me analyze the codebase...
  ⠋ Thinking...
```

**3. EXECUTING - Running tools**
```
Patterns to detect:
- Tool call indicators (Read, Write, Edit, Bash, etc.)
- "Running command..." text
- File paths being accessed
- Progress bars

Example tmux output:
  Reading src/auth.py...
  Editing src/middleware.py...
  Running command: npm test
  [=====>    ] 50%
```

**4. WAITING_PERMISSION - Blocked on permission**
```
Patterns to detect:
- Permission prompt visible
- "[y/n]" or "[approve/deny]" options
- "Allow" or "Permit" in prompt text
- Cursor waiting at permission prompt

Example tmux output:
  Allow edit to src/auth.py? [y/n]: █
  
  Or:
  
  ┌─ Permission Required ─────────────┐
  │ Edit src/auth.py                  │
  │ [Approve] [Deny] [View Diff]      │
  └───────────────────────────────────┘
```

**5. ASKING_QUESTION - Needs user input**
```
Patterns to detect:
- Question mark at end of last message
- "Which...", "Should I...", "Do you want..."
- Multiple choice options presented
- AskUserQuestion tool output

Example tmux output:
  I found two approaches:
  1. Refactor the middleware
  2. Add a new auth layer
  
  Which approach would you prefer?
  >█
```

**6. FINISHED - Task completed**
```
Patterns to detect:
- "Done", "Completed", "Finished" in recent output
- Summary of changes made
- "Is there anything else..." question
- No pending operations

Example tmux output:
  ✓ Tests passed
  ✓ Changes committed
  
  Task completed. The authentication bug is fixed.
  
  Is there anything else you'd like me to do?
  >█
```

**7. ERROR - Something went wrong**
```
Patterns to detect:
- "Error:", "Failed:", "Exception:" in output
- Red text (ANSI color codes)
- Stack traces
- "Could not", "Unable to"

Example tmux output:
  Error: File not found: src/missing.py
  
  The operation failed. Would you like me to try a different approach?
  >█
```

### State Detection Implementation

```python
class AgentStateDetector:
    def detect_state(self, tmux_output: str, last_activity_time: datetime) -> AgentState:
        """
        Analyze tmux output to determine agent state
        """
        lines = tmux_output.strip().split('\n')
        last_line = lines[-1] if lines else ""
        recent_lines = lines[-20:]  # Last 20 lines
        
        # Check for permission prompts (highest priority)
        if self._is_permission_prompt(last_line, recent_lines):
            return AgentState.WAITING_PERMISSION
        
        # Check for questions
        if self._is_asking_question(last_line, recent_lines):
            return AgentState.ASKING_QUESTION
        
        # Check for errors
        if self._has_error(recent_lines):
            return AgentState.ERROR
        
        # Check for completion
        if self._is_finished(recent_lines):
            return AgentState.FINISHED
        
        # Check for active execution
        if self._is_executing(recent_lines):
            return AgentState.EXECUTING
        
        # Check for thinking
        if self._is_thinking(recent_lines):
            return AgentState.THINKING
        
        # Check for idle
        time_since_activity = datetime.now() - last_activity_time
        if time_since_activity.seconds > 30 and self._is_at_prompt(last_line):
            return AgentState.IDLE
        
        return AgentState.UNKNOWN
    
    def _is_permission_prompt(self, last_line: str, recent_lines: List[str]) -> bool:
        """Detect permission prompts"""
        permission_patterns = [
            r'\[y/n\]',
            r'\[approve/deny\]',
            r'Allow.*\?',
            r'Permit.*\?',
            r'Permission Required',
            r'\[Approve\].*\[Deny\]',
        ]
        
        text = '\n'.join(recent_lines)
        return any(re.search(pattern, text, re.IGNORECASE) for pattern in permission_patterns)
    
    def _is_asking_question(self, last_line: str, recent_lines: List[str]) -> bool:
        """Detect when agent is asking a question"""
        question_patterns = [
            r'\?$',  # Ends with question mark
            r'^(Which|Should I|Do you want|Would you like|How should)',
            r'^\d+\.',  # Numbered options
            r'\[.*\].*\[.*\]',  # Multiple choice brackets
        ]
        
        text = '\n'.join(recent_lines[-5:])
        return any(re.search(pattern, text, re.IGNORECASE) for pattern in question_patterns)
    
    def _has_error(self, recent_lines: List[str]) -> bool:
        """Detect errors"""
        error_patterns = [
            r'Error:',
            r'Failed:',
            r'Exception:',
            r'Could not',
            r'Unable to',
            r'Traceback',
            r'\x1b\[31m',  # Red ANSI color
        ]
        
        text = '\n'.join(recent_lines)
        return any(re.search(pattern, text, re.IGNORECASE) for pattern in error_patterns)
    
    def _is_finished(self, recent_lines: List[str]) -> bool:
        """Detect task completion"""
        completion_patterns = [
            r'✓.*completed',
            r'Done',
            r'Finished',
            r'Task completed',
            r'Is there anything else',
            r'All set',
        ]
        
        text = '\n'.join(recent_lines)
        return any(re.search(pattern, text, re.IGNORECASE) for pattern in completion_patterns)
    
    def _is_executing(self, recent_lines: List[str]) -> bool:
        """Detect active tool execution"""
        execution_patterns = [
            r'Reading.*\.py',
            r'Writing.*\.py',
            r'Editing.*\.py',
            r'Running command:',
            r'Executing:',
            r'\[=+>?\s*\]',  # Progress bar
        ]
        
        text = '\n'.join(recent_lines)
        return any(re.search(pattern, text) for pattern in execution_patterns)
    
    def _is_thinking(self, recent_lines: List[str]) -> bool:
        """Detect thinking/processing"""
        thinking_patterns = [
            r'Thinking\.\.\.',
            r'Processing\.\.\.',
            r'Analyzing\.\.\.',
            r'⠋|⠙|⠹|⠸|⠼|⠴|⠦|⠧|⠇|⠏',  # Spinner characters
        ]
        
        text = '\n'.join(recent_lines)
        return any(re.search(pattern, text) for pattern in thinking_patterns)
    
    def _is_at_prompt(self, last_line: str) -> bool:
        """Check if cursor is at empty prompt"""
        return last_line.strip() in ['>', '█', '>█', '> █']
```

## Permission Mode Management

Each agent session runs in a specific permission mode that controls how it handles tool calls.

### Claude Code Permission Modes

```python
class PermissionMode(Enum):
    AUTO = "auto"                    # Auto-approve all tools
    DEFAULT = "default"              # Prompt for risky operations
    PLAN = "plan"                    # Require plan approval
    ACCEPT_EDITS = "acceptEdits"     # Auto-approve edits only
    DONT_ASK = "dontAsk"             # Never prompt (dangerous)
    BYPASS_PERMISSIONS = "bypassPermissions"  # Skip all permission checks
```

### Permission Mode Detection

Warren needs to detect the current permission mode of each agent session.

**Method 1: Parse from tmux output**
```
Look for indicators in Claude Code output:
- "Permission mode: auto"
- Settings displayed in status bar
- Behavior patterns (auto-approving vs prompting)
```

**Method 2: Check Claude Code settings**
```bash
# SSH to server and check settings
cat ~/.claude/settings.json | jq '.permissions.mode'
```

**Method 3: Query via Claude Code API (if available)**
```python
# Future: if Claude Code exposes API
agent_info = claude_code_api.get_session_info(session_id)
permission_mode = agent_info.permission_mode
```

### Permission Mode Configuration

Warren should allow configuring permission mode for each agent session.

**Data model:**
```python
@dataclass
class AgentConfig:
    name: str
    server: str
    tmux_session: str
    permission_mode: PermissionMode
    auto_approve_tools: List[str]  # Specific tools to auto-approve
    deny_tools: List[str]          # Tools to always deny
    
    # Permission rules
    allow_read: bool = True
    allow_write: bool = False  # Require approval
    allow_bash: bool = False   # Require approval
    allow_edit: bool = False   # Require approval
```

**Configuration file** (`~/.warren/agents/backend-api.yaml`):
```yaml
name: backend-api
server: prod-1
tmux_session: warren-backend-api

# Permission configuration
permission_mode: default

# Auto-approve specific tools
auto_approve:
  - Read
  - LSP
  - WebSearch

# Always deny these tools
deny:
  - Bash  # Never allow bash in production

# Fine-grained rules
permissions:
  read: allow
  write: prompt
  edit: prompt
  bash: deny
  
# Path-based rules
path_rules:
  - path: "src/**/*.py"
    allow: [read, edit]
  - path: "config/**"
    allow: [read]
    deny: [write, edit]
  - path: ".env*"
    deny: [read, write, edit]  # Never touch env files
```

### Changing Permission Mode

**From Warren CLI:**
```bash
# Set permission mode for an agent
warren config backend-api --mode auto
warren config backend-api --mode default
warren config backend-api --mode plan

# Configure specific permissions
warren config backend-api --allow-bash
warren config backend-api --deny-write

# Path-based rules
warren config backend-api --allow-edit "src/**/*.py"
warren config backend-api --deny-read ".env*"
```

**From Warren TUI:**
```
┌─ Agent: backend-api ───────────────────────────────────────┐
│                                                             │
│  Permission Mode: [Default ▼]                              │
│    ○ Auto (approve all)                                    │
│    ● Default (prompt for risky)                            │
│    ○ Plan (require plan approval)                          │
│    ○ Accept Edits (auto-approve edits)                     │
│                                                             │
│  Tool Permissions:                                          │
│    ✓ Read         (always allow)                           │
│    ? Write        (prompt)                                 │
│    ? Edit         (prompt)                                 │
│    ✗ Bash         (always deny)                            │
│                                                             │
│  Path Rules:                                                │
│    src/**/*.py    → allow read, edit                       │
│    config/**      → allow read only                        │
│    .env*          → deny all                               │
│                                                             │
│  [Save] [Cancel]                                            │
└─────────────────────────────────────────────────────────────┘
```

**Applying configuration to running agent:**

Warren needs to inject the permission settings into the agent session.

**Option 1: Restart agent with new settings**
```bash
# Stop current session
warren stop backend-api

# Start with new permission mode
warren dig backend-api --server prod-1 --mode auto
```

**Option 2: Modify Claude Code settings remotely**
```bash
# SSH to server and modify settings
ssh prod-1 "echo '{\"permissions\": {\"mode\": \"auto\"}}' > ~/.claude/settings.local.json"

# Send signal to Claude Code to reload settings (if supported)
tmux send-keys -t warren-backend-api C-c "reload settings" C-m
```

**Option 3: Send commands to agent session**
```bash
# If Claude Code supports runtime permission changes
tmux send-keys -t warren-backend-api "/config permissions.mode auto" C-m
```

## Warren Hub Display with State & Permissions

```
┌─ Warren Hub ────────────────────────────────────────────────┐
│                                                              │
│  Agent: backend-api (prod-1)          ⚠ WAITING PERMISSION  │
│  Mode: Default | Uptime: 2h 34m                             │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Chat:                                                  │ │
│  │   User: Fix the authentication bug                    │ │
│  │   Agent: I'll modify the auth middleware...           │ │
│  │                                                        │ │
│  │ ⚠ PERMISSION REQUIRED:                                │ │
│  │   Allow edit to src/auth.py?                          │ │
│  │   [Approve] [Deny] [View Diff] [Always Allow]         │ │
│  │                                                        │ │
│  │ Files: src/auth.py (pending edit)                     │ │
│  └────────────────────────────────────────────────────────┘ │
│  [Change Mode: Auto] [Configure Permissions]                │
│                                                              │
│  Agent: ml-trainer (gpu-box)                  ● EXECUTING   │
│  Mode: Auto | Uptime: 5h 12m                                │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Chat:                                                  │ │
│  │   User: Train the model                               │ │
│  │   Agent: Training... epoch 45/100                     │ │
│  │                                                        │ │
│  │ Status: Running training loop                         │ │
│  │ Progress: [=========>    ] 45%                        │ │
│  │                                                        │ │
│  │ Files: models/checkpoint.pt (writing)                 │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Agent: frontend (prod-2)                     ? ASKING      │
│  Mode: Plan | Uptime: 1h 05m                                │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Chat:                                                  │ │
│  │   User: Update the dashboard                          │ │
│  │   Agent: I can either:                                │ │
│  │          1. Refactor the component                    │ │
│  │          2. Add new features                          │ │
│  │          Which approach do you prefer?                │ │
│  │                                                        │ │
│  │ ? QUESTION - Waiting for response                     │ │
│  │   [Reply] [Option 1] [Option 2]                       │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  Agent: worker-3 (staging)                    ✓ FINISHED    │
│  Mode: Default | Uptime: 3h 22m                             │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Chat:                                                  │ │
│  │   User: Run the tests                                 │ │
│  │   Agent: ✓ All tests passed                           │ │
│  │                                                        │ │
│  │ Status: Task completed                                │ │
│  │                                                        │ │
│  │ Files: tests/test_*.py (read)                         │ │
│  └────────────────────────────────────────────────────────┘ │
│  [New Task]                                                  │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## State-Based Actions

Warren can take different actions based on agent state:

```python
class WarrenHub:
    def handle_agent_state(self, agent: Agent, state: AgentState):
        if state == AgentState.WAITING_PERMISSION:
            # Highlight in UI, enable approve/deny buttons
            self.show_permission_prompt(agent)
            
        elif state == AgentState.ASKING_QUESTION:
            # Enable reply input, show quick response options
            self.show_question_prompt(agent)
            
        elif state == AgentState.FINISHED:
            # Show completion notification
            # Offer to assign new task
            self.notify_completion(agent)
            
        elif state == AgentState.ERROR:
            # Alert user, show error details
            # Offer to retry or debug
            self.show_error_alert(agent)
            
        elif state == AgentState.IDLE:
            # Show as available for new tasks
            self.mark_available(agent)
```

## Notifications Based on State

```python
# Send notifications when state changes
def on_state_change(agent: Agent, old_state: AgentState, new_state: AgentState):
    if new_state == AgentState.WAITING_PERMISSION:
        notify(f"Agent {agent.name} needs permission approval")
        
    elif new_state == AgentState.ASKING_QUESTION:
        notify(f"Agent {agent.name} is asking a question")
        
    elif new_state == AgentState.FINISHED:
        notify(f"Agent {agent.name} completed the task")
        
    elif new_state == AgentState.ERROR:
        notify(f"Agent {agent.name} encountered an error", priority="high")
```

## Open Questions

1. **State detection accuracy**: How reliable is pattern matching?
   - Need extensive testing with real Claude Code output
   - Handle different themes/configurations
   - Fallback when uncertain?

2. **Permission mode changes**: How to apply changes to running agents?
   - Restart required?
   - Runtime configuration possible?
   - Need Claude Code API support?

3. **State transitions**: How to track state history?
   - Log all state changes?
   - Useful for debugging agent behavior?
   - Performance implications?

4. **Multiple permission prompts**: What if agent has multiple pending prompts?
   - Queue them?
   - Show all at once?
   - Prioritize?

5. **Permission inheritance**: Should new agents inherit permission settings?
   - From template?
   - From previous agent on same server?
   - Global defaults?
