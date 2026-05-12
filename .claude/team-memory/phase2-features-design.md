---
name: phase2-features-design
description: Technical design for detailed event stream display and state detection fixes
metadata:
  type: project
---

# Phase 2 Critical Features - Technical Design

## Problem Statement

Two critical issues blocking Phase 2 completion:

1. **Insufficient event detail**: TUI/web show only high-level summaries. The `./warren capture %1` command provides rich detail, but UIs don't expose full conversation history.

2. **Inaccurate state detection**: Agents showing "unknown" or "asking_question" when they should be "idle". State detection heuristics are too aggressive and lack proper idle detection.

## Current Architecture Analysis

### Event Flow
```
tmux pane → capture (500 lines) → parser → activities → event store → state detector → UI
```

### Key Components
- **Parser** (`internal/parser/activity.go`): Regex-based extraction of chat/file/tool/prompt activities
- **State Detector** (`internal/state/detector.go`): Signal-based state inference with priority system
- **Event Store** (`internal/events/store.go`): SQLite-backed append-only event log
- **Warren Core** (`internal/core/warren.go`): 500ms polling loop per session

### Current Limitations

1. **Parser only sees tmux capture window** (500 lines)
   - Misses full conversation context
   - Can't distinguish between active work and old output
   - No access to structured Claude Code session data

2. **State detection issues**:
   - No proper idle detection (only checks 5min timeout)
   - Question patterns too broad (matches rhetorical questions in explanations)
   - No "waiting for user input" vs "actively working" distinction
   - Doesn't detect when agent is at prompt waiting

## Proposed Solution

### Feature 1: Claude Code Session Integration

**Goal**: Access full conversation history from `~/.claude` directory structure.

#### Discovery Strategy

Claude Code stores session data in:
```
~/.claude/
├── sessions/           # Session metadata (PID → session ID mapping)
│   └── {pid}.json     # {"pid":123,"sessionId":"uuid","cwd":"...","status":"idle|busy"}
├── session-env/        # Per-session conversation history
│   └── {sessionId}/
│       └── conversation.jsonl  # Full message history (NOT CONFIRMED - needs investigation)
└── history.jsonl       # Global command history
```

**Implementation Plan**:

1. **Session Mapping** (`internal/claude/session_mapper.go`)
   - Map tmux pane PID to Claude session ID
   - Read `~/.claude/sessions/{pid}.json` to get session UUID
   - Cache mappings (PID → sessionId)

2. **Conversation Reader** (`internal/claude/conversation_reader.go`)
   - Locate conversation file: `~/.claude/session-env/{sessionId}/`
   - **INVESTIGATION NEEDED**: Determine actual conversation file format
   - Parse JSONL conversation history
   - Extract structured messages (role, content, tool calls, timestamps)

3. **Event Enrichment** (`internal/core/warren.go`)
   - When session discovered, attempt Claude session mapping
   - If mapping succeeds, read full conversation periodically (every 5s, not 500ms)
   - Store enriched events with `source: "claude"` metadata
   - Fall back to tmux capture if Claude data unavailable

4. **UI Display** (`internal/tui/views.go`, web interface)
   - Add "Conversation" tab in agent detail view
   - Show full message history with proper formatting
   - Distinguish user messages, assistant responses, tool calls
   - Add timestamps and message IDs
   - Support scrolling through full history

**Why This Approach**:
- ✅ Provides complete conversation context
- ✅ Structured data (no regex parsing)
- ✅ Includes tool calls, thinking, all metadata
- ✅ Falls back gracefully if unavailable
- ⚠️ Requires investigation of actual file format
- ⚠️ May need to handle multiple conversation file formats

**Open Questions**:
1. What is the actual conversation file name/format in `session-env/{sessionId}/`?
2. Does Claude Code use JSONL, JSON, or another format?
3. How are tool calls represented in the conversation log?
4. What happens when session is remote (SSH)? Can we access remote `~/.claude`?

### Feature 2: State Detection Fixes

**Goal**: Accurately detect idle state and reduce false positives for questions.

#### Root Cause Analysis

Current issues:
1. **Idle detection too weak**: Only triggers after 5min of no activity
2. **Question detection too broad**: Matches any "?" in recent output
3. **No prompt detection**: Can't tell when agent is waiting at input prompt
4. **Priority system flawed**: "asking_question" overrides "idle" even when question is old

#### Proposed Fixes

**1. Enhanced Idle Detection** (`internal/state/detector.go`)

Add multiple idle signals:
```go
// Signal 1: Prompt waiting (strongest)
if strings.HasSuffix(trimmedContent, "> ") || 
   strings.HasSuffix(trimmedContent, "$ ") {
    signals = append(signals, &Signal{
        State: StateIdle,
        Strength: 0.95,
        Evidence: "waiting at prompt",
    })
}

// Signal 2: No recent activity (medium)
if timeSinceLastActivity > 30*time.Second {
    signals = append(signals, &Signal{
        State: StateIdle,
        Strength: 0.7 + min(0.2, timeSinceLastActivity.Minutes()/10),
        Evidence: fmt.Sprintf("no activity for %v", timeSinceLastActivity),
    })
}

// Signal 3: Status indicators (medium)
if strings.Contains(contentLower, "waiting for input") ||
   strings.Contains(contentLower, "ready for next") {
    signals = append(signals, &Signal{
        State: StateIdle,
        Strength: 0.8,
        Evidence: "idle status indicator",
    })
}
```

**2. Stricter Question Detection**

Only detect questions that are:
- At the end of output (last 3 lines)
- Standalone (not in code blocks or comments)
- Followed by options or explicit user prompt

```go
// Only check last 3 non-empty lines
lastLines := getLastNonEmptyLines(content, 3)

// Must be at end AND have question marker
hasQuestionAtEnd := false
for _, line := range lastLines {
    if strings.HasSuffix(strings.TrimSpace(line), "?") {
        hasQuestionAtEnd = true
        break
    }
}

// Must also have AskUserQuestion tool OR multiple choice options
hasQuestionTool := strings.Contains(content, "AskUserQuestion")
hasOptions := regexp.MustCompile(`(?m)^\d+\.\s+`).MatchString(content)

if hasQuestionAtEnd && (hasQuestionTool || hasOptions) {
    // This is a real question
    signals = append(signals, &Signal{
        State: StateAskingQuestion,
        Strength: 0.9,
        Evidence: "question with options detected",
    })
}
```

**3. Time-Decay for Old Signals**

Reduce strength of old signals:
```go
func (d *StateDetector) collectSignals(activities []*events.AgentActivityEvent) []*Signal {
    signals := []*Signal{}
    now := time.Now()
    
    for _, activity := range activities {
        age := now.Sub(activity.Timestamp)
        
        // Decay strength for old activities
        decayFactor := 1.0
        if age > 30*time.Second {
            decayFactor = 0.5 // 50% strength after 30s
        }
        if age > 2*time.Minute {
            decayFactor = 0.2 // 20% strength after 2min
        }
        
        // Apply decay to signal strength
        signal := extractSignalFromActivity(activity)
        signal.Strength *= decayFactor
        signals = append(signals, signal)
    }
    
    return signals
}
```

**4. State Priority Adjustment**

Update priority to favor idle when no recent high-priority signals:
```go
statePriority: map[types.AgentState]int{
    types.StateError:             100, // Highest
    types.StateWaitingPermission: 90,
    types.StateAskingQuestion:    80,
    types.StateFinished:          70,
    types.StateStopped:           60,
    types.StateExecuting:         50,
    types.StateThinking:          40,
    types.StateIdle:              35,  // Raised from 30
    types.StateUnknown:           10,  // Lowest
}
```

**Why This Approach**:
- ✅ Multiple independent idle signals (more robust)
- ✅ Time-decay prevents stale signals from dominating
- ✅ Stricter question detection reduces false positives
- ✅ Prompt detection is reliable indicator of idle state
- ✅ Backward compatible (doesn't break existing detection)

## Implementation Phases

### Phase A: State Detection Fixes (High Priority)
**Estimated effort**: 1-2 days

1. Implement enhanced idle detection
2. Add time-decay to signal strength
3. Tighten question detection patterns
4. Add comprehensive tests
5. Validate against real sessions

**Success Criteria**:
- All idle agents show "idle" state
- No false positive "asking_question" states
- State transitions happen within 5 seconds of actual change

### Phase B: Claude Session Investigation (Medium Priority)
**Estimated effort**: 1 day

1. Investigate `~/.claude/session-env/` structure
2. Document conversation file format
3. Create proof-of-concept reader
4. Test with multiple session types

**Success Criteria**:
- Can reliably map PID → session ID
- Can read conversation history
- Understand message format

### Phase C: Conversation Display (Medium Priority)
**Estimated effort**: 2-3 days

1. Implement session mapper
2. Implement conversation reader
3. Add conversation tab to TUI
4. Add conversation view to web interface
5. Handle edge cases (remote sessions, missing files)

**Success Criteria**:
- Full conversation visible in UI
- Proper formatting of messages and tool calls
- Graceful fallback when Claude data unavailable

## Testing Strategy

### State Detection Tests
- Unit tests for each signal type
- Integration tests with captured real session content
- Regression tests for known false positives
- Performance tests (state detection should be <10ms)

### Claude Integration Tests
- Test PID → session ID mapping
- Test conversation parsing
- Test with active vs idle sessions
- Test with missing/corrupted files
- Test with remote sessions (if applicable)

### UI Tests
- Manual testing of conversation display
- Test with long conversations (1000+ messages)
- Test scrolling and navigation
- Test fallback to tmux capture

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Claude conversation format undocumented | High | Investigate first, document findings, build flexible parser |
| Remote session access to ~/.claude | Medium | Detect remote sessions, fall back to tmux capture |
| Performance impact of reading large conversations | Medium | Cache conversations, only re-read on change, limit history depth |
| State detection still has false positives | Medium | Extensive testing, tunable thresholds, user feedback loop |

## Open Questions for User

1. **Priority**: Should we fix state detection first (Phase A) or investigate Claude integration first (Phase B)?
   - Recommendation: **Phase A first** - it's blocking and well-defined

2. **Remote sessions**: Do we need to support reading `~/.claude` from remote servers via SSH?
   - If yes, adds complexity (need to SSH and read remote files)
   - If no, can skip for now and add later

3. **Conversation depth**: How much history should we display?
   - Full conversation (could be 1000+ messages)
   - Last N messages (e.g., 100)
   - Configurable limit

4. **UI preference**: Which interface is higher priority?
   - TUI (keyboard-driven, operator console)
   - Web (remote access, mobile-friendly)
   - Both equally

## Next Steps

**Awaiting user approval on**:
1. Overall technical approach
2. Phase priority (A → B → C vs different order)
3. Answers to open questions
4. Any concerns or alternative approaches

**Once approved**:
1. Create detailed task breakdown
2. Assign tasks to team members
3. Begin implementation with Phase A
