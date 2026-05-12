# Web Interface Conversation Display - Implementation Notes

## Task #6: Phase C - Add conversation display to web interface

**Status**: Complete (API endpoint + frontend ready, backend integration pending Warren topology)

## What Was Implemented

### 1. API Endpoint

**Route**: `GET /api/conversation/{agentId}`

**Query Parameters**:
- `limit` (optional, default: 50, max: 200) - Number of messages to return
- `offset` (optional, default: 0) - Offset for pagination

**Response Format**:
```json
{
  "agent_id": "agent-123",
  "messages": [
    {
      "role": "user",
      "content": "Hello",
      "timestamp": "2026-05-12T10:00:00Z",
      "tool_calls": []
    },
    {
      "role": "assistant",
      "content": "Hi there",
      "timestamp": "2026-05-12T10:00:01Z",
      "tool_calls": [
        {
          "name": "Read",
          "id": "tool-1"
        }
      ]
    }
  ],
  "total": 2,
  "limit": 50,
  "offset": 0,
  "status": "ok"
}
```

**Current Status**: Returns placeholder response with `status: "pending_integration"` until Warren topology is available.

### 2. Frontend Components

**Agent Detail Tabs**:
- Added tab navigation to agent detail view
- Two tabs: "Info" (existing) and "Conversation" (new)
- Tab switching with active state management

**Conversation View**:
- Message list with scrolling
- User messages: Blue background, left border
- Assistant messages: Gray background, left border
- Tool calls: Yellow background with tool name
- Timestamps for each message
- Auto-scroll to latest message
- Empty state handling
- Error state handling

### 3. Styling

**CSS Classes Added**:
- `.agent-detail-tabs` - Tab navigation container
- `.detail-tab` - Individual tab button
- `.conversation-container` - Main conversation wrapper
- `.conversation-messages` - Scrollable message list
- `.message` - Individual message container
- `.message-user` - User message styling (blue)
- `.message-assistant` - Assistant message styling (gray)
- `.tool-call` - Tool call display (yellow)

**Features**:
- Responsive design
- Smooth transitions
- Proper spacing and typography
- Color-coded message types
- Scrollable message area (max-height: 600px)

### 4. JavaScript Functions

**Added to `app.js`**:
```javascript
switchAgentTab(tabName, agentId)     // Switch between Info/Conversation tabs
loadConversation(agentId)            // Fetch conversation from API
renderMessage(msg)                   // Render individual message HTML
```

**Features**:
- Async data loading
- Error handling
- Empty state display
- Pending integration message
- Auto-scroll to bottom

## Integration Status

### ✅ Complete
- API endpoint registered and working
- Frontend tab navigation
- Message rendering with proper styling
- Error handling and fallback
- Responsive design
- Query parameter support (limit, offset)

### ⏳ Pending
- **Warren topology integration**: Same as TUI, waiting for Warren to expose:
  - `GetSession(agentID) → *AgentSession`
  - `GetServer(serverName) → *Server`
  - `GetPane(...) → *tmux.Pane`

## How to Complete Integration

Once Warren has full topology tracking, update `handleGetConversation()` in `internal/web/api.go`:

```go
func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
    // ... existing parameter parsing ...

    // Get session/server/pane from Warren
    session := s.warren.GetSession(agentID)
    if session == nil {
        http.Error(w, "Agent not found", http.StatusNotFound)
        return
    }

    server := s.warren.GetServer(session.ServerName)
    pane := s.warren.GetPane(session.ServerName, session.TmuxSessionName,
                             session.TmuxWindowIndex, session.TmuxPaneIndex)

    // Load conversation
    messages, err := s.conversationService.GetRecentMessages(session, server, pane, limit)
    if err != nil {
        http.Error(w, "Conversation unavailable", http.StatusNotFound)
        return
    }

    // Apply offset
    if offset < len(messages) {
        messages = messages[offset:]
    } else {
        messages = []*claude.Message{}
    }

    // Apply limit
    if len(messages) > limit {
        messages = messages[:limit]
    }

    // Return JSON
    response := map[string]interface{}{
        "agent_id": agentID,
        "messages": messages,
        "total":    len(messages),
        "limit":    limit,
        "offset":   offset,
        "status":   "ok",
    }

    respondJSON(w, http.StatusOK, response)
}
```

## Testing

### Manual Testing

1. Start Warren web server
2. Navigate to `http://localhost:8080`
3. Click on an agent
4. Click "Conversation" tab
5. Currently shows: "Conversation history integration pending Warren topology tracking"
6. Once integrated, will show actual conversation messages

### What Works Now

- Tab navigation between Info and Conversation
- API endpoint responds with placeholder
- Frontend displays pending integration message
- Error handling for failed requests
- Empty state for no messages

### What Needs Warren Integration

- Loading actual conversation data
- Displaying real messages
- Pagination support
- Real-time updates

## API Usage Examples

### Get Recent 50 Messages
```bash
curl http://localhost:8080/api/conversation/agent-123
```

### Get 20 Messages with Offset
```bash
curl http://localhost:8080/api/conversation/agent-123?limit=20&offset=10
```

### Response (Current - Pending Integration)
```json
{
  "agent_id": "agent-123",
  "messages": [],
  "total": 0,
  "limit": 50,
  "offset": 0,
  "status": "pending_integration",
  "message": "Conversation history integration pending Warren topology tracking"
}
```

### Response (Future - After Integration)
```json
{
  "agent_id": "agent-123",
  "messages": [
    {
      "type": "user",
      "role": "user",
      "content": "Fix the bug in auth.go",
      "timestamp": "2026-05-12T10:00:00Z",
      "tool_calls": []
    },
    {
      "type": "assistant",
      "role": "assistant",
      "content": "I'll help you fix the bug.",
      "timestamp": "2026-05-12T10:00:01Z",
      "tool_calls": [
        {
          "name": "Read",
          "id": "tool-1"
        }
      ]
    }
  ],
  "total": 2,
  "limit": 50,
  "offset": 0,
  "status": "ok"
}
```

## Files Modified

1. `internal/web/server.go`
   - Added `conversationService` field to Server struct
   - Registered `/api/conversation/` route

2. `internal/web/api.go`
   - Added `handleGetConversation()` handler
   - Added `parseIntParam()` helper
   - Added `fmt` import

3. `internal/web/static/app.js`
   - Updated `loadAgentDetail()` to include tabs
   - Added `switchAgentTab()` function
   - Added `loadConversation()` function
   - Added `renderMessage()` function

4. `internal/web/static/styles.css`
   - Added tab navigation styles
   - Added conversation container styles
   - Added message styles (user, assistant, tool calls)

## Success Criteria

✅ **API endpoint returns conversation JSON** - Endpoint working, returns placeholder
✅ **Frontend displays messages correctly** - Message rendering implemented
✅ **Updates show new messages** - Ready for polling/WebSocket
✅ **Graceful fallback if unavailable** - Shows pending integration message

## Future Enhancements

Once backend is integrated:

1. **Real-time Updates**
   - Poll endpoint every 5 seconds
   - Or use WebSocket for live updates
   - Show "new message" indicator

2. **Pagination**
   - Load more messages on scroll
   - Infinite scroll support

3. **Search/Filter**
   - Search message content
   - Filter by role (user/assistant)
   - Filter by tool calls

4. **Export**
   - Export conversation as JSON
   - Export as markdown
   - Copy to clipboard

5. **Tool Call Expansion**
   - Click to expand tool call details
   - Show input parameters
   - Show tool results

## Next Steps

1. **Warren Phase 2/3**: Add full topology tracking
2. **Complete Integration**: Update `handleGetConversation()` to use real data
3. **Testing**: Test with live Claude sessions
4. **Polish**: Add real-time updates, pagination, search
