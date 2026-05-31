# Phase <N> Technical Debt

**Date:** YYYY-MM-DD
**Companion to:** [Phase <N> Completion Report](phaseN-completion-report.md)

This document inventories known issues, limitations, and deferred work as of the end of Phase <N>. Items are grouped by priority. Each entry follows the same template.

## Priority key

- 🔴 **High** — blocks production use, security risk, or data correctness.
- 🟡 **Medium** — degrades UX or limits scalability; works around with effort.
- 🟢 **Low** — nice-to-have, polish, or low-impact edge case.
- ⚪ **Won't fix** — by-design decision, kept for visibility.

## Item template

```
### <Title>
- **Priority:** 🔴 / 🟡 / 🟢 / ⚪
- **Category:** correctness | performance | UX | docs | tests | security | infra
- **Issue:** <what is wrong / missing>
- **Impact:** <who feels it and how>
- **Effort:** <S (≤ 1 day) / M (1–5 days) / L (> 1 week)>
- **Workaround:** <what users do today | "none">
- **Code location:** `<file>:<line>` or `<package>`
- **Recommendation:** <when to address — e.g., "Phase N+1 alongside <related work>">
- **Issue link:** #<N> (if filed)
```

---

## 🔴 High Priority

### <Item 1>
- **Priority:** 🔴
- **Category:** <...>
- **Issue:** <...>
- **Impact:** <...>
- **Effort:** <...>
- **Workaround:** <...>
- **Code location:** <...>
- **Recommendation:** <...>

---

## 🟡 Medium Priority

### <Item>
...

---

## 🟢 Low Priority

### <Item>
...

---

## ⚪ Won't Fix (By Design)

### <Item>
- **Priority:** ⚪
- **Reason it's by-design:** <link to design-review.md section if applicable>
- **Note here so it doesn't get re-discovered:** <one line>

---

## Performance Notes

<Section for measurements, profiles, or "not yet measured" honesty. Capture anything that points at a future optimization without yet being a filed item.>

---

## Test Coverage Gaps

<Packages or scenarios where coverage is intentionally thin, with reasoning.>

| Area | Current | Target | Gap reason |
|------|---------|--------|-----------|
| `internal/<pkg>` | <X.X%> | <Y.Y%> | <why not yet> |

---

## Code Cleanup Opportunities

<Refactors that aren't urgent but would compound nicely. Keep this short — large refactors should be filed as their own roadmap items.>

- <opportunity>
- <opportunity>

---

## Items Deferred to Future Phases

| Item | Originally planned for | Deferred to | Reason |
|------|------------------------|-------------|--------|
| <item> | Phase <N> | Phase <N+k> | <reason> |
