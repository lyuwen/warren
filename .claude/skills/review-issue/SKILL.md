---
name: review-issue
description: Write a Warren-style architectural review for an issue, PR, or branch. Use when the user asks for an architectural review, critique, or merge gate on a specific issue/PR/branch. Produces a single markdown document at docs/reviews/architectural-review-issueN.md (or issue-N-<topic>-review.md) matching the existing house format — executive summary with verdict, layered architecture assessment, code/test/doc evidence, blocking issues, and final recommendation.
---

# Warren architectural review

Produce a review document that matches the format already established in `docs/reviews/` — see `docs/reviews/architectural-review-issue6.md` and `docs/reviews/architectural-review-issue5.md` as canonical examples.

## When invoked

Args may name an issue number, PR number, branch name, or scope ("topology UI", "remote support"). If none provided, ask the user which issue/PR/branch to review.

## Process

1. **Identify the scope.** Resolve the issue number, branch, and the diff under review.
   - If a number is given, find the matching issue title via `gh issue view <N>` and the linked PR/branch.
   - If a branch is given, run `git log main..<branch> --oneline` and `git diff main...<branch> --stat` to see what's in scope.
   - Confirm the scope with the user in one line before producing the report.

2. **Gather evidence.** For each subsystem touched by the diff:
   - List the files and line counts changed.
   - Read the most-changed files end-to-end (not just diff context).
   - Run `go test ./...` (or the relevant package subset) and record pass/fail + count.
   - Run `go vet` on the touched packages.
   - Skim the README/docs the diff updates; note any that should have been updated and weren't.

3. **Apply the rubric.** Score each of:
   - **Architecture** — layering, separation of concerns, fit with `design-review.md`.
   - **Correctness** — does the diff do what the issue asks? Edge cases handled?
   - **Tests** — coverage of the new code, regression risk, real-Claude-Code fixtures used where appropriate (per CLAUDE.md: "synthetic tests won't catch real parsing issues").
   - **Docs** — completion-report-ready? Any decision worth recording in `docs/`?
   - **Open questions** — does this diff resolve, defer, or worsen any of the five design open questions in CLAUDE.md?

4. **Write the report** to `docs/reviews/architectural-review-issue<N>.md` (or `docs/reviews/issue-<N>-<topic>-review.md` if `<N>-<topic>` is the established naming for that issue series). Use the template in `templates/architectural-review.md` in this skill as the starting structure.

5. **Set the verdict** as one of:
   - ✅ **APPROVED FOR MERGE** — ready, no blockers.
   - 🟡 **APPROVED WITH CAVEATS** — merge OK, follow-ups recommended; list them.
   - 🔴 **CHANGES REQUESTED** — blocking issues present; list them.
   - ❌ **REJECTED** — design-level problem; explain.

## Rules

- The verdict and the executive summary go at the top, not buried at the bottom.
- Every blocking finding cites `file:line` and quotes the offending snippet.
- Use ASCII layer diagrams (see issue-6 review) when the diff spans multiple layers — they make scope obvious at a glance.
- Match the bold/emoji conventions already in `docs/reviews/`: ✅ / 🟡 / 🔴 / ❌, **bold verdicts**, code-fenced Go snippets.
- Cite test counts and coverage when available; never hand-wave "tests look good."
- Do NOT modify the code under review from this skill — review only.

## Templates and references

- `templates/architectural-review.md` — section-by-section skeleton.
- `references/house-style.md` — the conventions distilled from existing reviews (verdict markers, section ordering, evidence requirements).

After writing, print the path of the generated review file and a one-line summary of the verdict.
