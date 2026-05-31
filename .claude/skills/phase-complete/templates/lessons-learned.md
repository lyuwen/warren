# Phase <N> Lessons Learned

**Date:** YYYY-MM-DD
**Companion to:** [Phase <N> Completion Report](phaseN-completion-report.md)

---

## What Went Well

### <Theme 1 — e.g., Tmux-first architecture held up>
- **Evidence:** <commit / file / test that proves it>.
- **Why it worked:** <root cause — not just "we did X">.
- **Keep doing:** <concrete behavior to preserve in Phase N+1>.

### <Theme 2>
...

---

## What Could Improve

### <Issue 1 — e.g., Parser fragility surfaced late>
- **Symptom:** <what we observed>.
- **Root cause:** <why it happened — be honest>.
- **Cost:** <rework hours, bugs shipped, time lost>.
- **Better next time:** <concrete process change, not "be more careful">.

### <Issue 2>
...

---

## Architecture Decisions Revisited

For each major decision recorded in the completion report, look back: did it hold?

### <Decision 1>
- **Original rationale:** <one line>.
- **In hindsight:** <still right | partially wrong | regret — explain>.
- **Action:** <leave as-is | refactor candidate | document as known limitation>.

---

## Design Patterns That Worked

- **<Pattern>** — used in `<file>`. Worth lifting into other subsystems because <reason>.
- **<Pattern>** — ...

## Design Patterns That Didn't

- **<Anti-pattern>** — used in `<file>`. Should be retired because <reason>. Migration path: <one line>.

---

## Team / Process Insights

<Even if a solo project, capture process observations: branch hygiene, review cadence, doc-as-you-go vs. doc-at-end, etc.>

- <observation>
- <observation>

---

## Five Open Questions — Status

CLAUDE.md tracks five open design questions. Phase <N>'s impact on each:

| # | Question | Status after Phase <N> |
|---|----------|------------------------|
| Q1 | Is plain tmux capture/control clean enough? | <resolved / clearer / still open> — <one line> |
| Q2 | Heuristic parser accuracy on real Claude Code? | <...> |
| Q3 | Polling interval trade-offs? | <...> |
| Q4 | How to display state uncertainty? | <...> |
| Q5 | Precedence when tmux- and Claude-derived events conflict? | <...> |

---

## Recommendations for Phase <N+1>

1. **<Recommendation>** — <why, with reference to a lesson above>.
2. **<Recommendation>** — ...
3. **<Recommendation>** — ...

---

## Anti-recommendations

Things we considered for Phase N+1 but decided against, with reasons. Saves the next reviewer from re-litigating.

- <Idea> — rejected because <reason>.
