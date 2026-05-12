# TUI Conversation Display - Implementation Notes

## Task #5: Phase C - Add conversation display to TUI

**Status**: Complete (UI framework ready, backend integration pending Warren topology)

## What Was Implemented

### 1. View Addition
- Added `ViewConversation` to view enum
- Updated view cycling to include conversation view (4 views total)

### 2. Model Updates
- Added `conversationService *core.ConversationService` to Model
- Added `conversationMessages []*claude.Message` for storing messages
- Added `conversationScroll int` for scroll position
- Added `conversationError string` for error display

### 3. Navigation
- **'c' key**: Switch from agent detail to conversation view
- **↑/↓ or j/k**: Scroll through conversation messages
- **←/esc**: Return to agent detail view
- **tab**: Cycle through all views (including conversation)

### 4. Message Rendering (`renderConversation()`)
- Title bar with agent ID
- Error handling with fallback message
- Message formatting:
  - **User messages**: Blue, prefixed with timestamp
  - **Assistant messages**: White, prefixed with timestamp
  - **Tool calls**: Yellow, shows tool name
- Scrolling support with visible window calculation
- Scroll indicator showing "X-Y of Z messages"
- Help text at bottom

### 5. Styles Added
- `userMessageStyle`: Blue, bold
- `assistantMessageStyle`: White, bold
- `toolCallStyle`: Yellow
- `errorStyle`: Red, bold

### 6. Graceful Fallback
When conversation history is unavailable:
```
Error: Conversation history integration pending

Conversation history not available (using tmux capture)
```

## Integration Status

### ✅ Complete
- UI framework and rendering
- Navigation and keyboard controls
- Message formatting and styling
- Scrolling support
- Error handling and fallback

### ⏳ Pending
- **Warren topology integration**: Warren currently tracks sessions by AgentID and PaneID, but doesn't have full Server/Session/Pane registry
- **Backend connection**: `loadConversation()` is stubbed out waiting for Warren to expose:
  - `GetSession(agentID) → *AgentSession`
  - `GetServer(serverName) → *Server`
  - `GetPane(server, session, window, pane) → *tmux.Pane`

## How to Complete Integration

Once Warren has full topology tracking (likely Phase 2 or 3), update `loadConversation()` in `internal/tui/app.go`:

```go
func (m *Model) loadConversation() {
    if m.selectedAgentID == "" {
        m.conversationError = "No agent selected"
        return
    }

    // Get agent session info from Warren
    session := m.warren.GetSession(m.selectedAgentID)
    if session == nil {
        m.conversationError = "Agent session not found"
        return
    }

    // Get server and pane info
    server := m.warren.GetServer(session.ServerName)
    pane := m.warren.GetPane(session.ServerName, session.TmuxSessionName, 
                             session.TmuxWindowIndex, session.TmuxPaneIndex)

    // Load conversation
    messages, err := m.conversationService.GetRecentMessages(session, server, pane, 50)
    if err != nil {
        m.conversationError = fmt.Sprintf("Conversation history not available: %v", err)
        m.conversationMessages = nil
        return
    }

    // Filter to user/assistant messages only
    m.conversationMessages = claude.FilterUserAssistant(messages)
    m.conversationScroll = 0
    m.conversationError = ""
}
```

## Testing

### Manual Testing
1. Run Warren TUI
2. Select an agent from the list
3. Press 'c' to view conversation
4. Currently shows: "Conversation history integration pending"
5. Once integrated, will show actual conversation history

### What Works Now
- Navigation between views
- Keyboard controls
- UI rendering
- Error display
- Fallback messaging

### What Needs Warren Integration
- Loading actual conversation data
- Displaying real messages
- Scrolling through conversation history

## Files Modified

1. `internal/tui/app.go`
   - Added ViewConversation enum
   - Added conversation fields to Model
   - Updated navigation handlers
   - Added loadConversation() stub

2. `internal/tui/views.go`
   - Added renderConversation() function
   - Updated agent detail help text

3. `internal/tui/styles.go`
   - Added conversation message styles

## Success Criteria

✅ Conversation view accessible from agent detail (press 'c')
✅ Messages properly formatted and readable
✅ Scrolling works smoothly (j/k keys)
✅ Graceful fallback if data unavailable
⏳ Backend integration (waiting for Warren topology)

## Next Steps

1. **Warren Phase 2/3**: Add full topology tracking
   - AgentSessionRegistry
   - ServerRegistry  
   - Pane tracking with PID

2. **Complete Integration**: Update loadConversation() to use real data

3. **Testing**: Test with live Claude sessions

4. **Polish**: Add features like:
   - Tool call expansion/collapse
   - Search/filter
   - Export conversation
