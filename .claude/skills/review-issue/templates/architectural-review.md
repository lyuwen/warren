# Architectural Review: Issue #<N> - <Short Title>

**Reviewer:** <agent or reviewer name>
**Date:** YYYY-MM-DD
**Branch:** `<branch-name>`
**Priority:** P<0|1|2|3>
**Status:** <✅ APPROVED FOR MERGE | 🟡 APPROVED WITH CAVEATS | 🔴 CHANGES REQUESTED | ❌ REJECTED>

---

## Executive Summary

**Verdict: <emoji> <ONE-LINE VERDICT>** — <one-sentence justification>.

<2–4 sentence summary describing what the change does and why the verdict was reached. Lead with strengths if approved, lead with the blocker if not.>

- ✅ <Strength 1>
- ✅ <Strength 2>
- ✅ <Strength 3>
- <🟡 / 🔴 Caveat or blocker, if any>

<For approvals:> **Ready for <immediate merge | merge after caveats addressed>.**
<For changes requested:> **Blocked on:** <one-line description of the blocker(s).>

---

## Scope

**Issue:** #<N> — <issue title>
**PR:** #<M> (if any)
**Diff:** `<base>..<branch>` — <N> files changed, +<X>/−<Y> lines.

**Files reviewed in depth:**
- `<file 1>`
- `<file 2>`
- `<file 3>`

**Files skimmed:** <list or "remaining diff is mechanical">.

---

## Architecture Assessment

### 1. Overall Design <✅ / 🟡 / 🔴 / status word>

<Describe the architectural shape of the change. Use an ASCII layer diagram when the change crosses layers — see issue-6 review for the canonical example.>

```
┌─────────────────────────────────────────────────┐
│              <Layer Name>                        │
│  ...                                             │
└─────────────────────────────────────────────────┘
```

**Assessment:** <One sentence on whether the design fits design-review.md.>

---

### 2. <Subsystem 2 — e.g., Core Integration> <status>

<Bullet-list the new/changed surface area. Quote signatures when small.>

```go
// <one-line description>
func <Type>.<Method>(...) ...
```

**Assessment:** <One sentence.>

---

### 3. <Subsystem 3 — e.g., Web/TUI/Parser/etc.> <status>

<As above.>

---

## Correctness Review

### What the issue asked for
- [ ] <Requirement 1>
- [ ] <Requirement 2>

### Edge cases considered
- ✅ <Case>: handled at `<file>:<line>`.
- 🟡 <Case>: handled but worth a follow-up — see [Caveats](#caveats).
- 🔴 <Case>: NOT handled — see [Blocking Issues](#blocking-issues).

### Design open questions touched
Per `CLAUDE.md`, Warren's design tracks five open questions. This diff:
- **Q1 (tmux integration cleanness):** <resolves | defers | worsens | N/A> — <why>.
- **Q2 (heuristic parser accuracy):** <...>
- **Q3 (polling interval):** <...>
- **Q4 (uncertainty display):** <...>
- **Q5 (event precedence):** <...>

---

## Testing

| Suite | Count | Status |
|-------|-------|--------|
| `go test ./internal/<pkg>/...` | <N> | <✅ pass / 🔴 fail> |
| `go vet ./internal/<pkg>/...` | — | <clean / issues> |
| Coverage (relevant pkgs) | <X.X%> | <vs. prior <Y.Y%>> |

**Real-Claude-Code fixtures:** <yes — list paths | no — flag as gap per CLAUDE.md guidance>.

**Test gaps:**
- <gap 1>
- <gap 2>

---

## Documentation

- [ ] README updated where relevant
- [ ] ROADMAP entry checked off (if part of a roadmap task)
- [ ] `docs/<feature>.md` added or updated
- [ ] `design-review.md` updated if architecture changed

---

## Blocking Issues

<Omit this section entirely if the verdict is ✅ APPROVED.>

### 🔴 <Blocker title>
- **Where:** `<file>:<line>`
- **Problem:**
  ```go
  <offending snippet>
  ```
  <Explanation.>
- **Why it blocks:** <impact — corruption, crash, security, design violation>.
- **Suggested fix:** <one-paragraph or bullet list>.

---

## Caveats / Follow-ups

### 🟡 <Caveat title>
- <Description and recommended follow-up issue/task.>

---

## What Looks Good

- ✅ <Item>
- ✅ <Item>

---

## Open Questions for the Author

1. <Question>
2. <Question>

---

## Final Recommendation

<One-paragraph close. State the verdict again and what the author should do next (merge / address caveats / address blockers and re-request review).>
