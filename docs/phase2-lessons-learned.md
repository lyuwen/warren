# Phase 2 Lessons Learned

**Date:** May 13, 2026  
**Phase:** Phase 2 - Central Read-Only Hub  

## Overview

This document captures key insights, design decisions, and lessons learned during Phase 2 implementation. These lessons should inform Phase 3 and future development.

---

## 1. Architecture & Design

### ✅ What Worked Well

**Explicit Topology Model**
- Separating tmux sessions from agent sessions was the right call
- Clear hierarchy (Server → Session → Window → Pane → Agent) prevented confusion
- Made it easy to support multiple agents in one pane (future use case)

**Event-Driven Design**
- Immutable event log proved valuable for debugging
- Easy to add new event types without breaking existing code
- Enables future features (replay, analytics) without schema changes

**Local-First Storage**
- SQLite + JSON files = zero-config deployment
- Works offline, no server dependencies
- Project-local `.warren/` directory supports multiple instances

**Core Backend + Multiple Frontends**
- Conversation backend as shared service avoided code duplication
- TUI and web consume same APIs, ensuring consistency
- Easier to test and maintain

### ❌ What Didn't Work

**Initial State Detection**
- First implementation had too many false positives
- Idle agents showed as "thinking" due to stale signals
- Questions were missed due to overly broad patterns
- **Fix:** Added time-decay, stricter patterns, priority adjustments

**MonitoredSession vs AgentSession**
- Two parallel tracking systems caused confusion
- Registry wasn't initially populated by warren-tui
- **Fix:** Added RegisterAgentSession() calls, improved fallback logic

**Web Message Display**
- JSON serialization tags excluded parsed fields
- Frontend checked wrong field order
- **Fix:** Changed `json:"-"` to `json:"field,omitempty"`, reordered frontend checks

### 🔄 What We'd Do Differently

**Start with Persistence**
- Registry persistence should have been in initial design
- Adding it later required retrofitting
- **Lesson:** Plan for persistence from day one, even if deferred

**Test with Real Agents Earlier**
- Synthetic tests missed edge cases
- Real agent sessions revealed state detection issues
- **Lesson:** Set up real test environment in Phase 1

**Document Architecture Decisions**
- Some decisions were made implicitly
- Had to reconstruct rationale later
- **Lesson:** Document "why" at decision time, not retrospectively

---

## 2. State Detection

### Key Insights

**Time Matters**
- Signals need freshness weighting
- 5-minute-old "thinking" signal shouldn't override fresh "idle" prompt
- **Solution:** Time-decay system (100% → 50% → 20% over 2 minutes)

**Priority Isn't Enough**
- Fixed priorities can't handle all cases
- Sometimes lower-priority state has much stronger evidence
- **Solution:** 2x confidence override rule

**Idle Detection is Critical**
- Most important state for user attention
- Needs immediate recognition (prompt detection)
- Needs graduated timeout (30s with increasing confidence)
- **Solution:** Multi-signal idle detection with priority 35

**Question Detection is Tricky**
- Too broad: false positives on any "?" in output
- Too narrow: miss legitimate questions
- **Solution:** Multi-factor detection (last 3 lines + patterns + tool confirmation)

### Design Principles Discovered

1. **Recency over priority:** Fresh weak signal > stale strong signal
2. **Multiple signals:** Combine multiple weak signals for confidence
3. **Graduated confidence:** Increase confidence over time for stable states
4. **Explicit trumps implicit:** Direct evidence (prompts) beats inference

---

## 3. Conversation History

### Key Insights

**Claude Session Format is Stable**
- JSONL format is well-structured and parsable
- Message types are consistent
- Timestamps and UUIDs enable ordering
- **Lesson:** Structured data is available, use it when possible

**Remote Access Works**
- Reading `~/.claude` over SSH is feasible
- No special protocol needed
- **Lesson:** Don't over-engineer remote access

**Caching is Essential**
- Conversation files can be large (510+ messages)
- Re-parsing on every request is wasteful
- 5-second TTL is good balance
- **Lesson:** Cache aggressively for read-heavy workloads

**Topology Integration is Key**
- Need to map agent ID → session ID → server → file path
- Process tree search works for finding Claude PIDs
- **Lesson:** Invest in topology mapping early

### Design Principles Discovered

1. **Backend as service:** Shared logic, multiple consumers
2. **Pagination matters:** Don't load entire conversation at once
3. **Fallback gracefully:** Show what you can, don't fail completely
4. **Test with real data:** 510-message conversation revealed edge cases

---

## 4. Testing Strategy

### What Worked

**Unit Tests for Core Logic**
- State detection, parsing, event store
- Fast, reliable, easy to debug
- Good coverage of edge cases

**Integration Tests for Tmux**
- Validated real tmux interaction
- Caught issues synthetic tests missed
- Required tmux environment (acceptable tradeoff)

**Manual Testing with Real Agents**
- Found production issues unit tests missed
- Validated user workflows
- Essential for UI/UX validation

### What Didn't Work

**Insufficient E2E Testing**
- TUI and web interfaces under-tested
- Production bugs found by user testing
- **Lesson:** Invest in E2E test infrastructure for Phase 3

**No Performance Testing**
- Didn't measure latency systematically
- Discovered performance issues late
- **Lesson:** Add benchmarks early, track over time

### Testing Principles Discovered

1. **Test pyramid:** Many unit tests, some integration tests, few E2E tests
2. **Real data matters:** Synthetic tests miss edge cases
3. **Performance is a feature:** Test it like any other feature
4. **Manual testing is essential:** Especially for UI/UX

---

## 5. Development Process

### What Worked

**Phased Approach**
- Phase 1 validation prevented wasted effort
- Clear success criteria for each phase
- Easy to track progress

**Team Structure**
- 8-agent team with clear roles
- Architect for design, implementer for code, tester for validation
- Parallel work on independent tasks

**Documentation-First**
- Design review before implementation
- Roadmap with clear tasks
- Reduced ambiguity and rework

### What Didn't Work

**Incomplete Task Tracking**
- Some tasks marked complete prematurely
- Registry persistence initially missed
- **Lesson:** Verify completion criteria before marking done

**Insufficient Design Review**
- Some decisions made during implementation
- Led to rework (state detection, registry)
- **Lesson:** Design review should cover edge cases

### Process Principles Discovered

1. **Validate interfaces early:** Phase 1 was critical
2. **Document decisions:** Capture "why" at decision time
3. **Test with real data:** Synthetic tests aren't enough
4. **Iterate on design:** Don't be afraid to enhance mid-phase

---

## 6. Technical Decisions

### Language & Framework Choices

**Go**
- ✅ Good SSH/terminal support
- ✅ Easy deployment (single binary)
- ✅ Fast enough for our needs
- ✅ Good standard library
- ❌ Verbose error handling
- ❌ No generics (Go 1.21 has them now)

**Bubble Tea (TUI)**
- ✅ Declarative model, easy to reason about
- ✅ Active maintenance, good docs
- ✅ Composable components
- ❌ Steep learning curve initially
- ❌ Limited built-in widgets

**SQLite**
- ✅ Zero-config, local-first
- ✅ Good enough performance
- ✅ ACID guarantees
- ❌ No built-in replication
- ❌ Single-writer limitation (not an issue for us)

**Vanilla JavaScript (Web)**
- ✅ No build step, simple deployment
- ✅ Fast page loads
- ✅ Easy to debug
- ❌ No type safety
- ❌ Manual DOM manipulation

### Would We Choose Differently?

**Probably Not**
- Go, SQLite, Bubble Tea are good fits
- Vanilla JS is fine for Phase 2 scope

**Maybe for Phase 3+**
- Consider TypeScript for web if complexity grows
- Consider adding observability framework
- Consider adding metrics/tracing

---

## 7. User Experience

### Key Insights

**Keyboard Navigation is Essential**
- TUI users expect vim-like navigation
- Single-key commands (c, n, q) are intuitive
- **Lesson:** Design for keyboard-first workflows

**Real-Time Updates Matter**
- 500ms polling feels responsive
- Users notice stale data quickly
- **Lesson:** Prioritize real-time updates

**Conversation History is Valuable**
- Users want to see full context
- Pagination is necessary for large conversations
- **Lesson:** Invest in conversation display early

**Error Messages Matter**
- "Agent session not found" was confusing
- Need better error messages and recovery
- **Lesson:** Design error handling for users, not developers

### UX Principles Discovered

1. **Show, don't tell:** Visual state indicators > text descriptions
2. **Keyboard-first:** Mouse is optional, keyboard is required
3. **Real-time matters:** Stale data is worse than no data
4. **Context is king:** Show enough context to make decisions

---

## 8. Performance & Scalability

### Current Limits

**Tested Scale:**
- 4 agents on localhost
- 510-message conversation
- 500ms polling interval

**Estimated Limits:**
- ~20 agents before polling becomes bottleneck
- ~1000 messages before conversation load is slow
- ~10 servers before SSH connection pool matters

### Performance Insights

**Caching is Critical**
- 5-second conversation cache reduced load 10x
- Registry persistence reduced startup time
- **Lesson:** Cache aggressively, invalidate carefully

**Polling is Good Enough**
- 500ms feels responsive for human-scale work
- Push-based updates not needed yet
- **Lesson:** Don't optimize prematurely

**SQLite is Fast Enough**
- Event queries are <10ms
- No performance issues observed
- **Lesson:** SQLite is underrated for local-first apps

### Scalability Principles Discovered

1. **Measure first:** Don't optimize without data
2. **Cache aggressively:** Disk I/O is expensive
3. **Polling is fine:** For human-scale responsiveness
4. **SQLite scales:** Further than most people think

---

## 9. Security & Safety

### Decisions Made

**Localhost-Only**
- No authentication in Phase 2
- CORS validation for localhost
- Clear documentation of security model
- **Rationale:** Simplifies Phase 2, deferred to Phase 3+

**No Credential Storage**
- SSH agent forwarding only
- No password/key storage
- **Rationale:** Reduces attack surface

**Read-Only in Phase 2**
- No write operations to agents
- Reduces risk of accidental damage
- **Rationale:** Validate monitoring before adding control

### Security Principles Discovered

1. **Explicit security model:** Document what's safe and what's not
2. **Defer authentication:** Don't add it until needed
3. **Read-only first:** Validate before adding write operations
4. **No credential storage:** Use SSH agent forwarding

---

## 10. Recommendations for Phase 3

### Do More Of

1. **Design review before implementation**
2. **Test with real agents early**
3. **Document architecture decisions**
4. **Cache aggressively**
5. **Keyboard-first UX**

### Do Less Of

1. **Premature optimization**
2. **Synthetic-only testing**
3. **Implicit design decisions**
4. **Marking tasks complete prematurely**

### New Practices to Add

1. **Performance benchmarks:** Track latency over time
2. **E2E test infrastructure:** Automated UI testing
3. **Observability:** Metrics and tracing for Warren itself
4. **User testing:** Get feedback earlier

### Technical Debt to Address

See `docs/phase2-technical-debt.md` for detailed list.

**High Priority:**
- Multi-server testing at scale
- File locking for registry
- E2E test infrastructure

**Medium Priority:**
- Event store compaction
- Metrics/observability
- Web authentication (for network deployment)

---

## Conclusion

Phase 2 was successful, delivering all planned features and more. Key lessons:

1. **Validate interfaces early** (Phase 1 was critical)
2. **Test with real data** (synthetic tests miss edge cases)
3. **Design for change** (time-decay, persistence added mid-phase)
4. **Document decisions** (capture "why" at decision time)
5. **Iterate on design** (state detection enhanced based on feedback)

These lessons will inform Phase 3 development and establish best practices for future phases.

---

*Document created: May 13, 2026*  
*Phase 2 duration: ~2 weeks*  
*Team: 8 agents*
