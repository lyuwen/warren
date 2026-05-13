# Phase 2 Technical Debt

**Date:** May 13, 2026  
**Phase:** Phase 2 - Central Read-Only Hub  
**Status:** Production Ready (with known limitations)

## Overview

This document tracks known issues, limitations, and technical debt from Phase 2. Items are prioritized by impact and effort required to address.

---

## High Priority

### 1. Multi-Server Discovery Not Tested at Scale

**Issue:** Agent discovery has only been tested with localhost. Multi-server scenarios (5+ remote servers, 20+ agents) have not been validated.

**Impact:** Unknown behavior at scale. Potential performance issues, SSH connection exhaustion, or discovery failures.

**Effort:** Medium (1-2 days)

**Recommendation:** Test with 5-10 remote servers and 20-30 agents before Phase 3.

**Workaround:** None. Users with multi-server setups may experience issues.

**Tracking:** ROADMAP.md Phase 2.1, Task 2.1

---

### 2. No File Locking for Registry

**Issue:** Registry JSON file has no file locking. Concurrent Warren instances could corrupt the registry.

**Impact:** Data corruption if multiple Warren instances run in same directory.

**Effort:** Low (4-8 hours)

**Recommendation:** Add file locking (flock on Linux, LockFileEx on Windows) before Phase 3.

**Workaround:** Don't run multiple Warren instances in same directory.

**Code Location:** `internal/core/agent_session.go` (Save/Load methods)

---

### 3. TUI Conversation View Has No Search/Filter

**Issue:** Conversation view shows all messages with no search or filter capability. Large conversations (500+ messages) are hard to navigate.

**Impact:** Poor UX for large conversations. Users must scroll through entire history.

**Effort:** Medium (1-2 days)

**Recommendation:** Add search (Ctrl+F) and filter (by type, by role) in Phase 3.

**Workaround:** Use web interface with browser search (Ctrl+F).

**Code Location:** `internal/tui/conversation.go`

---

### 4. No E2E Test Infrastructure

**Issue:** TUI and web interfaces have minimal automated testing. Most testing is manual.

**Impact:** Regressions in UI/UX may go undetected. Slows down development.

**Effort:** High (3-5 days)

**Recommendation:** Set up E2E test framework (Playwright for web, expect for TUI) in Phase 3.

**Workaround:** Manual testing before releases.

**Code Location:** N/A (infrastructure needed)

---

## Medium Priority

### 5. Event Store Has No Compaction/Archival

**Issue:** SQLite database grows indefinitely. 30-day retention policy is configured but not enforced automatically.

**Impact:** Database size grows over time. May impact performance after months of use.

**Effort:** Low (4-8 hours)

**Recommendation:** Add background job to prune old events. Run daily or weekly.

**Workaround:** Manual cleanup: `DELETE FROM events WHERE timestamp < datetime('now', '-30 days')`

**Code Location:** `internal/events/store.go`

---

### 6. No Metrics/Observability for Warren

**Issue:** Warren has no internal metrics or observability. Can't track performance, errors, or usage patterns.

**Impact:** Hard to diagnose performance issues or understand usage patterns.

**Effort:** Medium (2-3 days)

**Recommendation:** Add Prometheus metrics or structured logging in Phase 3.

**Workaround:** Use system monitoring tools (top, htop, strace).

**Code Location:** N/A (infrastructure needed)

---

### 7. Web Interface Has No Authentication

**Issue:** Web interface is localhost-only with no authentication. Cannot be safely exposed to network.

**Impact:** Cannot access Warren from remote machines or mobile devices.

**Effort:** High (5-7 days for proper auth)

**Recommendation:** Add authentication in Phase 3+ if network deployment is needed.

**Workaround:** SSH tunnel: `ssh -L 8080:localhost:8080 user@server`

**Code Location:** `internal/web/api.go`

---

### 8. Parser Accuracy Not Measured Quantitatively

**Issue:** Activity parser accuracy is not measured. No metrics for precision/recall.

**Impact:** Unknown false positive/negative rates. Hard to improve parser systematically.

**Effort:** Medium (2-3 days)

**Recommendation:** Create labeled test dataset and measure accuracy in Phase 3.

**Workaround:** Manual validation during testing.

**Code Location:** `internal/parser/`

---

### 9. State Detection Priority System is Complex

**Issue:** State detection has multiple overlapping rules (priority, confidence, time-decay, 2x override). Hard to reason about and debug.

**Impact:** Difficult to predict state detection behavior. Hard to add new states.

**Effort:** High (refactor required, 3-5 days)

**Recommendation:** Simplify or document decision tree more clearly in Phase 3.

**Workaround:** Extensive testing and documentation.

**Code Location:** `internal/state/detector.go`

---

## Low Priority

### 10. Conversation Cache Has Fixed 5s TTL

**Issue:** Conversation cache TTL is hardcoded to 5 seconds. Not configurable.

**Impact:** Cannot tune cache behavior for different workloads.

**Effort:** Low (1-2 hours)

**Recommendation:** Make TTL configurable via config file.

**Workaround:** Edit code and rebuild.

**Code Location:** `internal/claude/conversation_service.go`

---

### 11. No Support for Non-Claude Agents

**Issue:** Agent detection is Claude Code-specific. Won't detect Copilot, Cursor, or custom agents.

**Impact:** Limited to Claude Code users only.

**Effort:** Medium (2-3 days per agent type)

**Recommendation:** Add pluggable agent detection in Phase 4.

**Workaround:** Manual registration of non-Claude agents.

**Code Location:** `internal/core/discovery.go`

---

### 12. Registry Prune Threshold is Hardcoded

**Issue:** Registry prunes sessions older than 24 hours. Not configurable.

**Impact:** Cannot adjust for different usage patterns (e.g., long-running agents).

**Effort:** Low (1-2 hours)

**Recommendation:** Make threshold configurable via config file.

**Workaround:** Edit code and rebuild.

**Code Location:** `internal/core/agent_session.go` (Prune method)

---

### 13. No Session Replay Feature

**Issue:** Event store captures all activity but no UI to replay sessions.

**Impact:** Cannot review historical sessions or debug past issues.

**Effort:** High (5-7 days)

**Recommendation:** Add session replay in Phase 4.

**Workaround:** Query event store directly with SQL.

**Code Location:** N/A (feature not implemented)

---

### 14. TUI Has No Mouse Support

**Issue:** TUI is keyboard-only. No mouse support for clicking or scrolling.

**Impact:** Less accessible for users who prefer mouse interaction.

**Effort:** Medium (2-3 days)

**Recommendation:** Add mouse support in Phase 3 if user demand exists.

**Workaround:** Use web interface for mouse interaction.

**Code Location:** `internal/tui/`

---

### 15. Web Interface Has No Dark Mode

**Issue:** Web interface has fixed light theme. No dark mode option.

**Impact:** Poor UX for users who prefer dark themes.

**Effort:** Low (4-8 hours)

**Recommendation:** Add dark mode toggle in Phase 3.

**Workaround:** Use browser extensions for dark mode.

**Code Location:** `internal/web/static/app.js` (CSS)

---

### 16. No Notification Sound/Desktop Alerts

**Issue:** Notifications are visual only. No sound or desktop notifications.

**Impact:** Users may miss important notifications if not actively watching Warren.

**Effort:** Medium (1-2 days)

**Recommendation:** Add desktop notifications (libnotify on Linux, NSUserNotification on macOS) in Phase 3.

**Workaround:** Check notifications view periodically.

**Code Location:** `internal/notifications/`

---

### 17. Conversation Display Has No Syntax Highlighting

**Issue:** Code blocks in conversation are plain text. No syntax highlighting.

**Impact:** Harder to read code in conversation history.

**Effort:** Medium (2-3 days)

**Recommendation:** Add syntax highlighting (highlight.js) in Phase 3.

**Workaround:** Copy code to editor for syntax highlighting.

**Code Location:** `internal/web/static/app.js`, `internal/tui/conversation.go`

---

### 18. No Configuration File Validation

**Issue:** Config file parsing has minimal validation. Invalid config may cause crashes.

**Impact:** Poor error messages for config issues.

**Effort:** Low (4-8 hours)

**Recommendation:** Add config validation with clear error messages.

**Workaround:** Validate config manually before starting Warren.

**Code Location:** `internal/core/config.go`

---

## Deferred to Phase 4

### 19. No Structured Claude Data Integration

**Issue:** Warren uses heuristic parsing only. Doesn't use structured Claude session data as authoritative source.

**Impact:** Parser may miss or misinterpret activity.

**Effort:** High (5-7 days)

**Recommendation:** Integrate structured Claude data in Phase 4 as planned.

**Workaround:** Heuristic parsing is good enough for Phase 2.

**Code Location:** `internal/parser/`

---

### 20. No Plugin Management

**Issue:** Warren doesn't track or manage Claude Code plugins.

**Impact:** Cannot see which plugins are installed or enabled per agent.

**Effort:** High (7-10 days)

**Recommendation:** Implement in Phase 4 as planned.

**Workaround:** Check plugins manually via SSH.

**Code Location:** N/A (feature not implemented)

---

## Won't Fix (By Design)

### 21. No Write Operations in Phase 2

**Status:** By design. Phase 3 will add interactive capabilities.

**Rationale:** Validate monitoring before adding control.

---

### 22. Localhost-Only Web Interface

**Status:** By design for Phase 2. Network deployment deferred to Phase 3+.

**Rationale:** Simplifies security model, reduces scope.

---

### 23. Polling Instead of Push

**Status:** By design for Phase 2. Push-based updates deferred to Phase 4.

**Rationale:** Simpler implementation, good enough for human-scale responsiveness.

---

## Summary

**Total Items:** 23  
**High Priority:** 4  
**Medium Priority:** 5  
**Low Priority:** 10  
**Deferred:** 2  
**Won't Fix:** 2  

**Recommended for Phase 3:**
- Multi-server testing (#1)
- File locking (#2)
- E2E test infrastructure (#4)
- Metrics/observability (#6)
- Authentication (if network deployment needed) (#7)

**Can Wait:**
- Most low-priority items can be addressed as user demand dictates
- Phase 4 items are already planned in ROADMAP

---

*Document created: May 13, 2026*  
*Phase 2 status: Production ready with known limitations*
