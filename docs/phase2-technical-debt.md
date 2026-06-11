# Phase 2 Technical Debt

**Date:** May 13, 2026  
**Phase:** Phase 2 - Central Read-Only Hub  
**Status:** Production Ready (with known limitations)

## Overview

This document tracks known issues, limitations, and technical debt from Phase 2. Items are prioritized by impact and effort required to address.

---

## High Priority

### 0a. ~~SSH Host Key Verification Fails Open~~ ✅ **RESOLVED** (Phase 3 entry criterion)

**Issue:** `defaultHostKeyCallback` in `internal/core/server.go` silently fell back to `ssh.InsecureIgnoreHostKey()` when `~/.ssh/known_hosts` was missing or unreadable. This was MITM-prone — any user-facing surface that invoked `ConnectionPool.Get` against an unknown host would accept any server key without warning.

**Impact:** SECURITY. Phase 3 multi-server fanout would have exposed this as a CVE-class fail-open default.

**Resolution:** Replaced `defaultHostKeyCallback` with `hostKeyCallbackForServer(*Server)` in Batch 1 (Phase 2 audit remediation, June 2026). New behaviour:

- **Default is strict.** If `~/.ssh/known_hosts` is missing, unreadable, or malformed the function returns an actionable error (with three distinct messages — `missing`, `unreadable`, `malformed`) and refuses to produce a callback.
- **Insecure fallback is explicit opt-in.** Operator must set `WARREN_SSH_INSECURE_HOSTKEY=1`. When that env var is set we still emit a WARN log once per (host:port) per process lifetime so operators grepping logs can spot endpoints that were accepted insecurely.
- **Function signature accepts `*Server`** so a future per-server `InsecureHostKey bool` field on `Server` (see new P1 entry below) can layer in without breaking callers.

**Resolved:** June 1, 2026 — commit on `feat/phase2-audit-batch1`.

**Critique pre-validation:** `docs/critiques/phase2-audit-batch1-planval.md` (Item 1 verdict + required changes).

**Code Location:** `internal/core/server.go` (`hostKeyCallbackForServer`, `loadKnownHostsCallback`, `newHostKeyUnavailableError`).

**Tests:** `internal/core/hostkey_test.go` covers all five Critique-mandated cases — present-and-valid, missing-no-env, missing-with-env, unreadable-no-env, malformed-no-env — plus the once-per-host WARN-log invariant. `internal/core/server_test.go` `TestConnectionPool_ConcurrentGet` and `TestConnectionPool_RemoteServerDialError` set `t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")` so they keep exercising the dial path on CI runners without a populated known_hosts.

**Tracking:** Flagged by Phase 2 Critique (docs/reviews/phase2-fixes-critique.md). P0 entry criterion for Phase 3 — now met.

---

### 0a-followup. Per-server `insecure_host_key` config field (P1 — Phase 3 entry-criterion follow-up)

**Issue:** The Batch 1 fix for 0a routes the insecure-fallback opt-in through a binary-global env var (`WARREN_SSH_INSECURE_HOSTKEY=1`). That conflates "every SSH connection in this process is insecure" with "this one dev VM has no known_hosts entry" — an operator who needs the latter is forced to take the former and inherit MITM exposure on every other host in the same Warren process.

**Impact:** Operational. The env var is fit-for-purpose as a minimum-viable fallback (and was the only realistic scope for Batch 1), but it does not let operators express the natural axis "production hosts strict / dev VM lax."

**Effort:** Small (2-4 hours)

**Recommendation:** Add a per-server `insecure_host_key: bool` field on `Server` in `internal/core/server.go` (and the corresponding `servers.yaml` schema). `hostKeyCallbackForServer(*Server)` already accepts `*Server` exactly so this can layer in without breaking callers; a TODO comment in `hostKeyCallbackForServer` references this entry. The env var should remain as a process-wide override for CI / scratch environments, with the per-server field taking precedence.

**Tracking:** New entry filed by Batch 1 remediation, June 2026. Marked P1 — should land before Phase 3 multi-server UX exposes the env-var-only design to real operators.

---

### 0b. ~~ConnectionPool.Get Serializes the Entire Pool~~ ✅ **RESOLVED** (Phase 3 entry criterion)

**Issue:** `ConnectionPool.Get` held `p.mu` across the full SSH `Dial` + handshake (`internal/core/server.go`). Under any slow target, every other Get call to any host blocked behind it. Single-supervisor / single-target usage hid this defect.

**Impact:** Concurrency. The moment Phase 3 fans out to multiple SSH targets in parallel, slow hosts would have serialized all reconnection attempts and effectively starved the pool.

**Resolution:** Refactored `ConnectionPool.Get` in Batch 2 (Phase 2 audit remediation, June 2026) to a double-checked-locking + per-key in-flight promise (`connDial`) pattern. The pool's main mutex is now acquired only to read/install the in-flight entry; the SSH dial + handshake run outside the lock so calls to different servers proceed in parallel. Concurrent Get calls for the *same* server share a single `chan struct{}` ready channel and observe the same result, preserving dedup. Tests cover both invariants in `internal/core/server_test.go`.

**Resolved:** June 11, 2026 — commit on `feat/phase2-audit-batch2`.

**Code Location:** `internal/core/server.go` (`ConnectionPool.Get`, `connDial`, `ConnectionPool.dial`).

**Tracking:** Closed Phase 2 audit Item #3.

---

### 0c. ~~REST API has no CORS protection and binds to 0.0.0.0 by default~~ ✅ **RESOLVED** (Phase 3 entry criterion)

**Issue:** `warren-web` defaulted to `--addr :8080`, i.e. `0.0.0.0:8080`, exposing the REST API on every network interface, and the HTTP handlers emitted no CORS headers at all. Any browser tab on any site could hit the API; any reachable network host could query agent state.

**Impact:** SECURITY. Open API + open bind on a developer workstation that joins coffee-shop / hotel / corporate networks.

**Resolution:** Batch 2 (Phase 2 audit remediation, June 2026):

- Default bind is now `127.0.0.1:8080` (`web.DefaultBindAddr`), loopback only.
- `--bind host:port` flag replaces `--addr` (which is retained with a deprecation WARN for one release).
- `WARREN_WEB_BIND` env var supplies the default for `--bind`, mirroring `WARREN_SSH_INSECURE_HOSTKEY` from Batch 1.
- `corsMiddleware` enforces a strict allow-list. Default origins: `http://localhost:8080`, `http://127.0.0.1:8080`. `WARREN_WEB_CORS_ORIGINS=https://app.example.com,https://...` adds extra entries.
- Binding to anything other than a loopback address emits a one-shot WARN log mirroring the insecure-host-key WARN style, so operators grepping logs can spot exposed surfaces.

**Resolved:** June 11, 2026 — commit on `feat/phase2-audit-batch2`.

**Code Location:** `internal/web/server.go` (`DefaultBindAddr`, `EnvBindAddr`, `EnvCORSOrigins`, `resolveCORSOrigins`, `corsMiddleware`, `isLoopbackBind`, `originHostsForBind`); `cmd/warren-web/main.go` (`--bind` flag wiring).

**Tracking:** Closed Phase 2 audit Item #4.

---

### 1. Multi-Server Discovery Not Tested at Scale

**Issue:** Agent discovery has only been tested with localhost. Multi-server scenarios (5+ remote servers, 20+ agents) have not been validated.

**Impact:** Unknown behavior at scale. Potential performance issues, SSH connection exhaustion, or discovery failures.

**Effort:** Medium (1-2 days)

**Recommendation:** Test with 5-10 remote servers and 20-30 agents before Phase 3.

**Workaround:** None. Users with multi-server setups may experience issues.

**Tracking:** ROADMAP.md Phase 2.1, Task 2.1

---

### 2. ~~No File Locking for Registry~~ ✅ **RESOLVED**

**Issue:** Registry JSON file has no file locking. Concurrent Warren instances could corrupt the registry.

**Impact:** Data corruption if multiple Warren instances run in same directory.

**Effort:** Low (4-8 hours)

**Resolution:** Implemented file locking using `github.com/gofrs/flock` with 5-second timeout and 100ms retry interval. Comprehensive concurrent access tests added.

**Resolved:** May 13, 2026

**Documentation:** `docs/file-locking.md`

**Code Location:** `internal/core/agent_session.go` (Save/Load methods)

**Tests:** `internal/core/agent_session_persistence_test.go` (6 concurrent access tests)

**Demo:** `examples/concurrent_registry_demo.go`

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

### 5. ~~Event Store Has No Compaction/Archival~~ ✅ **RESOLVED**

**Issue:** SQLite database grows indefinitely. 30-day retention policy is configured but not enforced automatically.

**Impact:** Database size grows over time. May impact performance after months of use.

**Effort:** Low (4-8 hours)

**Resolution:** Implemented automatic event pruning with configurable retention period and pruning interval. Background job runs periodically to delete old events. Defaults to 30-day retention with daily pruning.

**Resolved:** May 13, 2026

**Documentation:** `docs/event-store-compaction.md`

**Code Location:** `internal/events/store.go` (PruneOldEvents, StartPruningJob, runPruning methods)

**Configuration:** `internal/core/warren.go` (EventRetentionPeriod, EventPruningInterval in Config)

**Tests:** `internal/events/store_pruning_test.go` (8 comprehensive tests)

**Features:**
- Automatic background pruning job
- Configurable retention period (default: 30 days)
- Configurable pruning interval (default: 24 hours)
- Manual pruning API available
- Validation of configuration values
- Logging of pruning operations
- Graceful shutdown of pruning goroutine

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

### P3. SubscribeToFile Has No Production Callers

**Issue:** `ConversationService.SubscribeToFile(path, interval)` is exported but only used by tests. Real consumers go through `SubscribeToUpdates`. This is a backwards test seam — tests should use the internal `subscribeWithFetcher` hook instead.

**Impact:** Public API surface area unjustified by use.

**Effort:** Small (1 hour)

**Recommendation:** Unexport `SubscribeToFile` (rename to `subscribeToFile`) or delete it. Update tests to use `subscribeWithFetcher`.

**Tracking:** Flagged by Phase 2 Critique. P3 cleanup.

---

### P3. SubscribeToUpdates / SSH Minor Hygiene Items

**Issue:** Bundle of MINOR items from Phase 2 review:
- `findRepoRoot` worktree branch doesn't verify the `.git` file starts with `gitdir:` prefix
- SSH agent socket leak on failed handshake
- Encrypted private-key files silently skipped (no log)
- `defaultClientConfig` uses raw int port instead of `strconv.Itoa`
- `TestRegisterAgentSession_EmptyPathNoSave` doesn't restore log writer
- `SubscribeToFile` relay has a redundant `done` channel
- `SubscribeToUpdates` doesn't validate non-nil session (will NPE in goroutine)
- Prime-vs-first-tick edge in poller can spuriously fire if data changes between them
- Stale design comment in tests

**Impact:** Low — none affect correctness under normal use.

**Effort:** Small (half day total)

**Recommendation:** Batch into a single cleanup PR alongside the P0/P1 SSH items.

**Tracking:** Phase 2 Reviewer findings (docs/reviews/phase2-fixes-review.md).

---

### Phase 3 Design Question: ConnectionPool Key

**Issue:** `ConnectionPool` keys connections by `server.Name`. Two configs with same name + different host would alias; same host + different names would duplicate.

**Impact:** Design ambiguity, not a current bug.

**Recommendation:** Make a design call before Phase 3 SSH UX — either document Name as authoritative, or switch key to `host:port`.

**Tracking:** Flagged by Phase 2 Critique.

---

## Low Priority

### 10. ~~Conversation Cache Has Fixed 5s TTL~~ ✅ **RESOLVED**

**Issue:** Conversation cache TTL is hardcoded to 5 seconds. Not configurable.

**Impact:** Cannot tune cache behavior for different workloads.

**Effort:** Low (1-2 hours)

**Resolution:** Added `CacheTTL` field to Warren Config with default of 5 seconds. Created `NewConversationServiceWithTTL()` constructor to accept custom TTL. Validation ensures TTL is positive and at most 1 hour.

**Resolved:** May 13, 2026

**Code Location:** `internal/core/conversation_service.go` (NewConversationServiceWithTTL)

**Configuration:** `internal/core/warren.go` (CacheTTL in Config)

**Tests:** `internal/core/config_test.go` (TestConfigValidate_CacheTTL with 6 test cases)

---

### 11. No Support for Non-Claude Agents

**Issue:** Agent detection is Claude Code-specific. Won't detect Copilot, Cursor, or custom agents.

**Impact:** Limited to Claude Code users only.

**Effort:** Medium (2-3 days per agent type)

**Recommendation:** Add pluggable agent detection in Phase 4.

**Workaround:** Manual registration of non-Claude agents.

**Code Location:** `internal/core/discovery.go`

---

### 12. ~~Registry Prune Threshold is Hardcoded~~ ✅ **RESOLVED**

**Issue:** Registry prunes sessions older than 24 hours. Not configurable.

**Impact:** Cannot adjust for different usage patterns (e.g., long-running agents).

**Effort:** Low (1-2 hours)

**Resolution:** Added `RegistryPruneThreshold` field to Warren Config with default of 24 hours. Created `PruneWithThreshold()` method to accept custom threshold. Validation ensures threshold is positive and at least 1 hour.

**Resolved:** May 13, 2026

**Code Location:** `internal/core/agent_session.go` (PruneWithThreshold method)

**Configuration:** `internal/core/warren.go` (RegistryPruneThreshold in Config)

**Tests:** `internal/core/config_test.go` (TestConfigValidate_RegistryPruneThreshold with 6 test cases)

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

### 18. ~~No Configuration File Validation~~ ✅ **RESOLVED**

**Issue:** Config file parsing has minimal validation. Invalid config may cause crashes.

**Impact:** Poor error messages for config issues.

**Effort:** Low (4-8 hours)

**Resolution:** Added comprehensive `Validate()` method to Config struct. Validates all configuration fields with clear, actionable error messages. Checks for positive durations, valid ranges, non-empty required fields, and reasonable limits.

**Resolved:** May 13, 2026

**Code Location:** `internal/core/warren.go` (Config.Validate method)

**Tests:** `internal/core/config_test.go` (9 test functions with 30+ sub-tests)

**Validation Rules:**
- PollInterval: positive, >= 100ms (avoid excessive CPU)
- MinConfidence: 0.0 to 1.0
- DBPath: non-empty
- ConfigDir: non-empty
- EventRetentionPeriod: positive
- EventPruningInterval: positive
- CacheTTL: positive, <= 1 hour (avoid stale data)
- RegistryPruneThreshold: positive, >= 1 hour (avoid premature pruning)

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
**High Priority:** 3 (1 resolved)  
**Medium Priority:** 4 (1 resolved)  
**Low Priority:** 10  
**Deferred:** 2  
**Won't Fix:** 2  
**Resolved:** 2

**Recommended for Phase 3:**
- Multi-server testing (#1)
- ~~File locking (#2)~~ ✅ **RESOLVED**
- E2E test infrastructure (#4)
- Metrics/observability (#6)
- Authentication (if network deployment needed) (#7)

**Resolved in Phase 2:**
- File locking for registry (#2) - May 13, 2026
- Event store compaction (#5) - May 13, 2026

**Can Wait:**
- Most low-priority items can be addressed as user demand dictates
- Phase 4 items are already planned in ROADMAP

---

*Document created: May 13, 2026*  
*Last updated: May 13, 2026*  
*Phase 2 status: Production ready with known limitations*
