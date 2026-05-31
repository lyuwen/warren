# Phase <N> Completion Report

**Date:** YYYY-MM-DD
**Status:** ✅ Complete
**Duration:** ~<X> weeks (<start date> → <end date>)
**Phase Lead:** <name>

---

## Executive Summary

<2–3 paragraphs. What did Phase N set out to do? What was delivered? Were the success criteria from ROADMAP.md met? Lead with the headline outcome.>

**Success criteria from ROADMAP.md:**
- [x] <criterion 1>
- [x] <criterion 2>
- [ ] <unmet criterion — see Known Issues>

---

## Features Delivered

### <Feature 1 — e.g., Topology Discovery>
- **Status:** ✅ Complete
- **Tests:** <N> tests, all passing
- **Key files:** `internal/<pkg>/...`
- **Key decisions:** <bullet — see Architecture Decisions for detail>

### <Feature 2>
...

### <Feature 3>
...

---

## Architecture Decisions

### <Decision 1 — e.g., Tmux capture-pane as primary interface>
- **Chose:** <option>
- **Rejected:** <alternatives>
- **Rationale:** <why>
- **Trade-offs accepted:** <what we give up>
- **Reversibility:** <high / medium / low>

### <Decision 2>
...

---

## Testing Summary

| Suite | Count | Pass | Coverage |
|-------|-------|------|----------|
| `./internal/tmux/...` | <N> | <N> | <X.X%> |
| `./internal/parser/...` | <N> | <N> | <X.X%> |
| `./internal/state/...` | <N> | <N> | <X.X%> |
| `./internal/web/...` | <N> | <N> | <X.X%> |
| `./internal/tui/...` | <N> | <N> | <X.X%> |
| **Total** | **<N>** | **<N>** | **<X.X%>** |

**Real-Claude-Code fixtures used:** <yes/no — list paths in internal/testdata/>.

**Notable test gaps:** <none | bullet list referencing technical-debt doc>.

---

## Performance Metrics

<Fill in if measured; otherwise write "Not formally measured this phase — see [technical debt](phaseN-technical-debt.md#performance).">

| Metric | Value | Notes |
|--------|-------|-------|
| Pane capture latency (p50/p99) | <ms> / <ms> | <conditions> |
| Memory at <N> sessions | <MB> | <conditions> |
| Polling interval | <ms> | <configurable? default?> |

---

## Code Statistics

- Commits: <N>
- Files changed: <N>
- Lines added / removed: +<X> / −<Y>
- Go source LOC at end of phase: <N>

---

## Production Readiness

| Criterion | Status | Notes |
|-----------|--------|-------|
| All planned features implemented | ✅ / ⚠️ / ❌ | |
| All tests passing | ✅ / ⚠️ / ❌ | |
| Documentation complete | ✅ / ⚠️ / ❌ | |
| Known critical bugs | ✅ none / ⚠️ <N> | see technical debt |
| Backwards compatibility | ✅ / ⚠️ / ❌ / N/A | |
| Deployment story | ✅ / ⚠️ / ❌ | |

**Verdict:** <Ready for daily use | Ready for evaluation | Not ready — blockers below>.

---

## Known Issues and Limitations

Brief list; full detail lives in [phaseN-technical-debt.md](phaseN-technical-debt.md).

- 🔴 <high-priority item>
- 🟡 <medium-priority item>
- 🟢 <low-priority item>

---

## Lessons Learned

Summary only — full retrospective in [phaseN-lessons-learned.md](phaseN-lessons-learned.md).

- ✅ <what worked>
- ⚠️ <what didn't>
- 💡 <key insight for Phase N+1>

---

## Next Steps

Phase <N+1> will address:
- <item from ROADMAP>
- <item from ROADMAP>
- <carryover from this phase's technical debt>

See [ROADMAP.md](../ROADMAP.md) for the full Phase <N+1> plan.
