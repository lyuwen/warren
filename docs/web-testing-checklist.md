# Warren Web Interface - Manual Testing Checklist

## Pre-Test Setup

- [ ] Warren database exists or can be created
- [ ] At least one tmux session with an agent is running
- [ ] Web server binary is built: `go build -o warren-web ./cmd/warren-web`
- [ ] Browser is ready (Chrome, Firefox, Safari, or Edge)

## Test 1: Server Startup

- [ ] Start server: `./warren-web`
- [ ] Server starts without errors
- [ ] Console shows "Warren web interface started on :8080"
- [ ] Console shows discovered agent sessions (if any)
- [ ] Console shows "WebSocket client connected" when browser connects

## Test 2: Initial Page Load

- [ ] Open browser to `http://localhost:8080`
- [ ] Page loads successfully
- [ ] Header displays "Warren" title
- [ ] Connection status shows green dot and "Connected"
- [ ] Three tabs visible: Agents, Notifications, Servers
- [ ] Agents view is active by default

## Test 3: Agents View

- [ ] Agents list displays (or shows "No agent sessions found")
- [ ] Each agent card shows:
  - [ ] Agent ID
  - [ ] State badge (colored, uppercase)
  - [ ] Pane ID
  - [ ] Last poll time
  - [ ] Error count
- [ ] Agent cards are clickable (hover shows visual feedback)
- [ ] Click "Refresh" button reloads agents

## Test 4: Agent Detail View

- [ ] Click on an agent card
- [ ] View switches to agent detail
- [ ] "Back" button is visible
- [ ] Agent ID shown in header
- [ ] Session Info section displays:
  - [ ] Agent ID
  - [ ] Pane ID
  - [ ] Current state (colored badge)
  - [ ] Last poll time
  - [ ] Error count
- [ ] Artifact Profile section displays (or "No artifact profile available")
- [ ] Recent Activities section displays (or "No recent activities")
- [ ] Click "Back" returns to agents list

## Test 5: Notifications View

- [ ] Click "Notifications" tab
- [ ] View switches to notifications
- [ ] Notifications list displays (or "No notifications")
- [ ] Badge on tab shows notification count (if any)
- [ ] Each notification card shows:
  - [ ] Notification type (uppercase)
  - [ ] Timestamp
  - [ ] Message
  - [ ] Agent ID
  - [ ] "Mark as Read" button
- [ ] Click "Mark as Read" removes notification
- [ ] Badge count decreases
- [ ] Click "Refresh" reloads notifications

## Test 6: Servers View

- [ ] Click "Servers" tab
- [ ] View switches to servers
- [ ] At least "localhost" server is shown
- [ ] Server card displays:
  - [ ] Server name
  - [ ] Status badge (green "online")
  - [ ] Host
  - [ ] Agent count
- [ ] Click "Refresh" reloads servers

## Test 7: WebSocket Real-Time Updates

**Setup:** Have a way to trigger agent state changes (e.g., interact with agent in tmux)

- [ ] Connection status shows green dot and "Connected"
- [ ] Trigger an agent state change
- [ ] Agent card updates automatically (no refresh needed)
- [ ] State badge changes color and text
- [ ] If viewing agent detail, detail view updates
- [ ] Trigger a notification-worthy state (e.g., waiting_permission)
- [ ] Notification badge appears/updates automatically
- [ ] "Last updated" timestamp in footer updates

## Test 8: WebSocket Reconnection

- [ ] Stop the web server (Ctrl+C)
- [ ] Connection status changes to gray dot and "Disconnected"
- [ ] Restart the web server
- [ ] Within 3 seconds, connection status returns to green "Connected"
- [ ] No manual page refresh needed

## Test 9: Mobile Responsiveness

- [ ] Resize browser window to mobile width (< 768px)
- [ ] Layout adjusts to single column
- [ ] Header stacks vertically
- [ ] Tabs stack vertically
- [ ] Agent cards stack in single column
- [ ] All text remains readable
- [ ] Buttons remain clickable
- [ ] Navigation still works

## Test 10: Error Handling

- [ ] Navigate to non-existent agent: `http://localhost:8080/api/agents/fake-id`
- [ ] Returns 404 error
- [ ] Stop web server and try to load page
- [ ] Browser shows connection error
- [ ] Restart server and reload page
- [ ] Page loads successfully

## Test 11: Multiple Browser Tabs

- [ ] Open web interface in two browser tabs
- [ ] Trigger a state change
- [ ] Both tabs update simultaneously
- [ ] Mark notification as read in one tab
- [ ] Other tab updates notification count

## Test 12: Performance

- [ ] With multiple agents (3+), page loads quickly (< 2s)
- [ ] Switching between views is instant
- [ ] WebSocket messages don't cause lag
- [ ] Browser console shows no errors
- [ ] Network tab shows reasonable request sizes

## Test 13: Browser Compatibility

Test in multiple browsers:

- [ ] Chrome/Chromium
- [ ] Firefox
- [ ] Safari (macOS)
- [ ] Edge

For each browser:
- [ ] Page loads correctly
- [ ] WebSocket connects
- [ ] All views work
- [ ] Styling is consistent

## Test 14: Long-Running Session

- [ ] Leave web interface open for 5+ minutes
- [ ] WebSocket remains connected (green dot)
- [ ] Periodic updates continue to work
- [ ] No memory leaks (check browser task manager)
- [ ] No console errors accumulate

## Test 15: Edge Cases

- [ ] Start web server with no agents running
  - [ ] Shows "No agent sessions found"
  - [ ] No errors in console
- [ ] Start web server with invalid database path
  - [ ] Server fails gracefully with error message
- [ ] Access API endpoints directly:
  - [ ] `curl http://localhost:8080/api/agents` returns JSON
  - [ ] `curl http://localhost:8080/api/notifications` returns JSON
  - [ ] `curl http://localhost:8080/api/servers` returns JSON

## Post-Test Cleanup

- [ ] Stop web server (Ctrl+C)
- [ ] Server shuts down gracefully
- [ ] Console shows "Shutting down..." and "Shutdown complete"
- [ ] No zombie processes remain

## Issues Found

Document any issues discovered during testing:

| Test # | Issue Description | Severity | Notes |
|--------|------------------|----------|-------|
|        |                  |          |       |

## Test Summary

- **Total Tests:** 15
- **Passed:** ___
- **Failed:** ___
- **Blocked:** ___
- **Date:** ___________
- **Tester:** ___________
- **Environment:** ___________
