# Warren review house style

Conventions distilled from `docs/reviews/architectural-review-issue3.md` through `issue6.md` and the `issue-N-<topic>-review.md` series. Follow these when writing a new review.

## Verdict markers
- ✅ **APPROVED FOR MERGE** — no blockers.
- 🟡 **APPROVED WITH CAVEATS** — merge OK; list follow-ups.
- 🔴 **CHANGES REQUESTED** — blocking issues; must fix before merge.
- ❌ **REJECTED** — design problem; not fixable by patching the diff.

## Section ordering (non-negotiable)
1. Title and meta block (Reviewer / Date / Branch / Priority / Status).
2. Executive Summary — verdict on the FIRST line.
3. Scope.
4. Architecture Assessment (one subsection per affected layer/subsystem, each with its own status emoji).
5. Correctness Review (requirements checklist + edge cases + design-question impact).
6. Testing (table of suites, coverage, fixture note).
7. Documentation (checklist).
8. Blocking Issues — omit entirely if approved.
9. Caveats / Follow-ups.
10. What Looks Good.
11. Open Questions for the Author.
12. Final Recommendation.

## Evidence requirements
- Every blocking finding: `file:line` reference AND a quoted snippet ≤ 5 lines.
- Every test claim: count of tests and pass/fail status.
- Every coverage claim: number, ideally compared to the prior baseline.
- ASCII layer diagrams when the diff spans ≥ 2 layers — see issue-6.

## Tone
- Concise, direct, no hedging on verdict. The summary line is binary.
- Quote, don't paraphrase, when calling out problems.
- Strengths get bullets too — reviews are not just blocker lists.
- Address the author in 2nd person only in "Open Questions for the Author".

## What NOT to do
- Don't bury the verdict.
- Don't omit the Architecture Assessment even on small diffs — say "N/A — single-file mechanical change" and move on.
- Don't propose rewrites in a review; propose follow-up issues.
- Don't claim "tests look good" without a count.
- Don't modify the code from this skill — review only.

## File naming
- Primary pattern: `docs/reviews/architectural-review-issue<N>.md`.
- Topic-led pattern (also used in the repo): `docs/reviews/issue-<N>-<short-topic>-review.md`.
- Pick whichever series is already in use for that issue number; don't mix.

## Related docs
- `design-review.md` — canonical design; cite when assessing architectural fit.
- `CLAUDE.md` — five open questions checklist for the Correctness Review section.
- `ROADMAP.md` — for verifying the diff matches an actual planned task.
