# Warren Testing Checklist

This document provides a comprehensive testing checklist for Warren across all phases and components.

## Automated Tests

### State Detection Tests

#### Unit Tests (internal/state/detector_test.go)
- [x] Empty activities detection
- [x] Permission prompt detection
- [x] Question detection
- [x] Executing state detection
- [x] Error state detection
- [x] Finished state detection
- [x] Idle state detection
- [x] Content-based detection
- [x] State priority handling
- [x] State transition logic
- [x] Multiple signals handling

#### Integration Tests (internal/state/detector_integration_test.go)
- [x] Real permission prompt capture
- [x] Real question capture
- [x] Real command execution capture
- [x] Real error state capture
- [x] Real task completion capture
- [x] Multiple signals in real content
- [x] Conversation flow state detection
- [x] Idle detection from old activities
- [x] Ambiguous content handling
- [x] Empty content handling
- [x] Permission approval sequence

#### Performance Tests (internal/state/detector_performance_test.go)
- [x] Content detection < 10ms (1000 lines)
- [x] Activity detection < 10ms (100 activities)
- [x] Repeated detection performance
- [x] Large content handling (10,000 lines < 50ms)
- [x] Many activities handling (1000 activities < 50ms)
- [x] Concurrent detection safety
- [x] Benchmarks for all detection methods

**Performance Results:**
- Content detection: ~60-80µs for 1000 lines ✅
- Activity detection: ~60-80µs for 100 activities ✅
- Large content: ~450-600µs for 10,000 lines ✅
- Concurrent: ~6µs average per operation ✅

### Activity Parser Tests

#### Unit Tests (internal/parser/activity_test.go)
- [x] Chat message parsing
- [x] File interaction parsing
- [x] Tool usage parsing
- [x] Permission prompt parsing
- [x] Question parsing
- [x] Confidence scoring
- [x] Recent chat extraction
- [x] Metadata population
- [x] Empty content handling

### Tmux Integration Tests

#### Topology Tests (internal/tmux/topology_test.go)
- [x] Session discovery
- [x] Window discovery
- [x] Pane discovery
- [x] Topology building

#### Capture Tests (internal/tmux/capture_test.go)
- [x] Pane content capture
- [x] ANSI stripping
- [x] Scrollback handling

#### Control Tests (internal/tmux/control_test.go)
- [x] Send keys functionality
- [x] Send text functionality
- [x] Target validation

#### Control Loop Tests (internal/tmux/control_loop_test.go)
- [x] Capture-validate-send cycle
- [x] State change detection

### Core Component Tests

#### Warren Core Tests (internal/core/warren_test.go)
- [x] Warren initialization
- [x] Server management
- [x] Agent session tracking

#### Agent Session Tests (internal/core/agent_session_test.go)
- [x] Session creation
- [x] Session state management
- [x] Session metadata

#### Discovery Tests (internal/core/discovery_test.go)
- [x] Agent session discovery
- [x] Heuristic detection

#### Registry Tests (internal/core/registry_test.go)
- [x] Session registration
- [x] Session lookup

#### Artifact Profile Tests (internal/core/artifact_profile_test.go)
- [x] File tracking
- [x] Repository tracking
- [x] Profile extraction

### Event Store Tests

#### Store Tests (internal/events/store_test.go)
- [x] Event append
- [x] Event query
- [x] Time range queries
- [x] Event retention

### Notification Tests

#### Engine Tests (internal/notifications/engine_test.go)
- [x] Notification generation
- [x] Priority handling
- [x] Notification filtering

### TUI Tests

#### App Tests (internal/tui/app_test.go)
- [x] TUI initialization
- [x] View rendering
- [x] Keyboard handling

## Manual Testing Checklist

### Phase 1: Topology + Capture Validation

#### Local Tmux Environment
- [ ] Discover all local tmux sessions
- [ ] Discover all windows in each session
- [ ] Discover all panes in each window
- [ ] Capture content from a specific pane
- [ ] Capture content from a pane running Claude Code
- [ ] Send text to a pane
- [ ] Send keystroke (Enter) to a pane
- [ ] Send keystroke (y) to approve permission
- [ ] Verify capture latency is acceptable
- [ ] Handle non-existent pane gracefully

#### Remote Tmux Environment
- [ ] Connect to remote server via SSH
- [ ] Discover remote tmux sessions
- [ ] Capture content from remote pane
- [ ] Send input to remote pane
- [ ] Handle SSH connection failure
- [ ] Handle SSH timeout
- [ ] Verify remote latency is acceptable

#### Control Loop
- [ ] Capture → detect permission prompt → send 'y' → verify approval
- [ ] Capture → detect question → send answer → verify response
- [ ] Detect state change between capture and action
- [ ] Handle pane disappearing during operation
- [ ] Handle tmux server restart

### Phase 2: Central Read-Only Hub

#### Agent Session Discovery
- [ ] Discover Claude Code sessions by pane title
- [ ] Discover Claude Code sessions by content
- [ ] Manually register an agent session
- [ ] Handle false positives in discovery
- [ ] Track multiple agents on same server
- [ ] Track agents across multiple servers

#### Activity Parsing
- [ ] Parse chat messages correctly
- [ ] Parse file operations correctly
- [ ] Parse tool usage correctly
- [ ] Parse permission prompts correctly
- [ ] Parse questions correctly
- [ ] Handle mixed content types
- [ ] Handle malformed content
- [ ] Verify confidence scores are reasonable

#### State Detection
- [ ] Detect idle state correctly
- [ ] Detect thinking state correctly
- [ ] Detect executing state correctly
- [ ] Detect waiting_permission state correctly
- [ ] Detect asking_question state correctly
- [ ] Detect finished state correctly
- [ ] Detect error state correctly
- [ ] Handle state transitions correctly
- [ ] Prioritize states correctly (error > permission > question)

#### Artifact Tracking
- [ ] Track files read by agent
- [ ] Track files edited by agent
- [ ] Track files written by agent
- [ ] Identify repository from file paths
- [ ] Handle files outside repositories
- [ ] Track multiple repositories

#### Event Storage
- [ ] Store activity events
- [ ] Store notification events
- [ ] Query events by agent
- [ ] Query events by time range
- [ ] Query events by type
- [ ] Verify event retention policy works

### Phase 3: Interactive Hub (TUI)

#### TUI Display
- [ ] Display list of all agent sessions
- [ ] Display current state for each agent
- [ ] Display recent activity for each agent
- [ ] Display notifications for each agent
- [ ] Highlight agents needing attention
- [ ] Show file/repo artifacts
- [ ] Update display in real-time

#### TUI Navigation
- [ ] Navigate between agent sessions with arrow keys
- [ ] Select an agent session
- [ ] View detailed agent information
- [ ] View activity history
- [ ] View notification history
- [ ] Return to main view

#### TUI Actions
- [ ] Approve permission prompt via TUI
- [ ] Answer question via TUI
- [ ] Send custom message to agent
- [ ] Refresh agent state manually
- [ ] Mark notification as read
- [ ] Filter agents by state
- [ ] Search agents by name

#### TUI Performance
- [ ] TUI remains responsive with 10+ agents
- [ ] TUI remains responsive with 100+ events
- [ ] TUI updates smoothly (no flicker)
- [ ] TUI handles terminal resize

### Phase 3: Interactive Hub (Web)

#### Web Display
- [ ] Display agent list in web UI
- [ ] Display agent state in web UI
- [ ] Display recent activity in web UI
- [ ] Display notifications in web UI
- [ ] Highlight agents needing attention
- [ ] Show file/repo artifacts
- [ ] Auto-refresh display

#### Web Navigation
- [ ] Click to select agent
- [ ] View agent details page
- [ ] View activity timeline
- [ ] View notification list
- [ ] Navigate back to agent list

#### Web Actions
- [ ] Approve permission via web UI
- [ ] Answer question via web UI
- [ ] Send custom message via web UI
- [ ] Refresh agent state
- [ ] Mark notification as read
- [ ] Filter agents by state
- [ ] Search agents

#### Web Performance
- [ ] Web UI loads quickly
- [ ] WebSocket updates work correctly
- [ ] Web UI handles 10+ agents
- [ ] Web UI handles 100+ events
- [ ] Web UI works on mobile browser

### Phase 4: Extended Features

#### Plugin Management
- [ ] List plugins for an agent
- [ ] Enable plugin for an agent
- [ ] Disable plugin for an agent
- [ ] Track plugin state changes
- [ ] Sync plugin state across sessions

#### Permission Management
- [ ] View current permission mode
- [ ] Change permission mode (auto/ask/deny)
- [ ] Track permission history
- [ ] Apply permission rules

#### Conversation History
- [ ] View full conversation for an agent
- [ ] Search conversation history
- [ ] Export conversation
- [ ] Navigate conversation timeline

## Performance Benchmarks

### Target Performance
- State detection: < 10ms per detection
- Activity parsing: < 50ms per capture
- Pane capture: < 100ms per pane
- Event query: < 50ms per query
- TUI refresh: < 100ms per cycle
- Web API response: < 200ms per request

### Load Testing
- [ ] 10 concurrent agent sessions
- [ ] 50 concurrent agent sessions
- [ ] 100 concurrent agent sessions
- [ ] 1000 events in database
- [ ] 10,000 events in database
- [ ] 100,000 events in database

## Error Handling

### Network Errors
- [ ] SSH connection timeout
- [ ] SSH connection refused
- [ ] SSH authentication failure
- [ ] Network interruption during operation
- [ ] Reconnection after network recovery

### Tmux Errors
- [ ] Tmux server not running
- [ ] Tmux session not found
- [ ] Tmux pane not found
- [ ] Tmux command timeout
- [ ] Invalid tmux command

### Application Errors
- [ ] Database connection failure
- [ ] Database write failure
- [ ] Database query failure
- [ ] Invalid configuration
- [ ] Missing dependencies

## Security Testing

### SSH Security
- [ ] SSH key authentication works
- [ ] SSH password authentication works (if enabled)
- [ ] SSH agent forwarding works
- [ ] SSH connection is encrypted
- [ ] SSH host key verification works

### Data Security
- [ ] Event data is stored securely
- [ ] Sensitive data is not logged
- [ ] Database file permissions are correct
- [ ] Configuration file permissions are correct

## Compatibility Testing

### Tmux Versions
- [ ] Tmux 2.x
- [ ] Tmux 3.0
- [ ] Tmux 3.1
- [ ] Tmux 3.2
- [ ] Tmux 3.3+

### Operating Systems
- [ ] Linux (Ubuntu 20.04+)
- [ ] Linux (Debian 11+)
- [ ] Linux (Fedora 35+)
- [ ] macOS 12+
- [ ] macOS 13+

### Terminal Emulators
- [ ] iTerm2
- [ ] Terminal.app
- [ ] Alacritty
- [ ] Kitty
- [ ] GNOME Terminal
- [ ] Konsole

### Claude Code Versions
- [ ] Claude Code CLI (latest)
- [ ] Claude Code CLI (previous version)
- [ ] Different agent types (if applicable)

## Regression Testing

After each change, verify:
- [ ] All automated tests pass
- [ ] Core functionality still works
- [ ] Performance has not degraded
- [ ] No new errors in logs
- [ ] Documentation is up to date

## Test Data

### Sample Captures
Create sample tmux captures for:
- [ ] Permission prompt
- [ ] Question prompt
- [ ] Command execution
- [ ] Error state
- [ ] Finished state
- [ ] Mixed content
- [ ] Long conversation
- [ ] Multiple file operations

### Test Environments
Set up test environments with:
- [ ] Local tmux with 1 agent
- [ ] Local tmux with 5 agents
- [ ] Remote tmux with 1 agent
- [ ] Remote tmux with 5 agents
- [ ] Mixed local and remote agents

## Documentation Testing

- [ ] README instructions work
- [ ] Installation guide works
- [ ] Configuration examples work
- [ ] API documentation is accurate
- [ ] Troubleshooting guide is helpful

## Notes

- All automated tests should pass before manual testing
- Performance tests should be run on representative hardware
- Manual tests should be performed on clean test environments
- Document any issues found during testing
- Update this checklist as new features are added
