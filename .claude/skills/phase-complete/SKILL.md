---
name: phase-complete
description: Generate the full set of Warren phase wrap-up deliverables for Phase N — the three required docs (completion report, lessons learned, technical debt), plus the synchronized updates to ROADMAP.md, README.md, CLAUDE.md, and pages/index.html. Use when the user says "wrap up phase N", "phase N is done", "let's close phase N", or asks you to produce the phase completion docs. Mirrors the procedure documented in CLAUDE.md. Pairs naturally with the phase-wrap-up-auditor subagent — run that after this skill to verify the result.
---

# Warren phase wrap-up

CLAUDE.md defines the phase wrap-up process. This skill executes it. The goal is six synchronized artifacts in a single coherent commit.

## When invoked

Args may be a phase number ("2") or "Phase N". If absent, infer from the most recent `docs/phase*-status.md` or `docs/phase*-completion-report.md` and confirm with the user before generating files.

## Deliverables (in order)

For phase `N`, produce:

1. `docs/phaseN-completion-report.md` — from `templates/completion-report.md`
2. `docs/phaseN-lessons-learned.md` — from `templates/lessons-learned.md`
3. `docs/phaseN-technical-debt.md` — from `templates/technical-debt.md`
4. Updates to `ROADMAP.md`:
   - All Phase N tasks marked `[x]`.
   - Add a "Phase N Wrap-Up Documentation" section linking the three docs above.
   - Update test coverage numbers to current.
5. Updates to `README.md`:
   - "Project Status" → Phase N ✅ complete.
   - Add Phase N highlights to "What's New".
   - Prune fixed items from "Known Limitations".
   - Refresh test coverage stats.
6. Updates to `CLAUDE.md`:
   - "Project Status" → Phase N complete, Phase N+1 next.
   - Add any new key documents to "Key Documents".
7. Updates to `pages/index.html`:
   - Phase N marked complete with progress bars.
   - Statistics (tests, lines of code, commits) refreshed.
   - Add Phase N features to the showcase.
   - Update recent commits log.

## Process

1. **Gather facts.** Before writing anything:
   - `git log --since="<phase start date>" --oneline | wc -l` for commit count.
   - `git diff --stat <phase-start-commit>..HEAD | tail -1` for files/insertions.
   - `go test -cover ./... 2>&1 | tee /tmp/coverage.txt` for current coverage.
   - `find . -name '*.go' -not -path './vendor/*' | xargs wc -l | tail -1` for Go LOC.
   - Read `ROADMAP.md` to extract what Phase N actually planned to deliver.
   - Read any `docs/phaseN-status.md`, `docs/phaseN-test-scenarios.md`, or similar prep docs already in the repo — pull material from them; do not duplicate them.

2. **Write the three docs.** Fill the three templates with real numbers. Cite commits by short SHA where helpful. Do not invent metrics — if you didn't measure something, say so.

3. **Update the four existing files.** Each is a surgical edit, not a rewrite:
   - `ROADMAP.md`: change `[ ]` → `[x]` for Phase N tasks only; add the wrap-up links section directly under the Phase N heading.
   - `README.md`: update only the named sections; preserve formatting.
   - `CLAUDE.md`: update the "Project Status" block at the top and the "Key Documents" list.
   - `pages/index.html`: find the Phase N section/card and toggle its state; refresh the stats block.

4. **Self-check before exit.** Print:
   - Paths of all created files.
   - Paths of all modified files (with one-line summaries of what changed).
   - The current test/coverage numbers used.
   - A reminder: "Run the phase-wrap-up-auditor subagent next to verify."

## Rules

- **Use actual data.** Coverage, test counts, commit counts, LOC — measure, don't guess.
- **Reuse what's already there.** If `docs/phaseN-status.md` or `docs/phaseN-test-scenarios.md` exist, mine them for the completion report instead of starting blank.
- **Match prior phase's structure.** If `docs/phase1-completion-report.md` or `docs/phase2-completion-report.md` exist, mirror their section ordering and tone — consistency across phases matters.
- **Don't claim "production ready" without evidence.** The completion-report template has a "Production Readiness" section with a checklist; fill it honestly, including ⚠️ and ❌ items.
- **Don't commit.** Stop after writing files. The user reviews and commits.

## Templates

- `templates/completion-report.md` — executive summary, features, architecture decisions, testing, performance, production readiness, lessons-link, next steps.
- `templates/lessons-learned.md` — what went well (evidence), what could improve (root causes), architecture decisions revisited, recommendations for Phase N+1.
- `templates/technical-debt.md` — prioritized item list; each item: issue / impact / effort / recommendation / workaround / code location.

## After running

Recommend the user invoke the `phase-wrap-up-auditor` subagent on the result. That agent runs the verification checklist and surfaces gaps before the wrap-up commit lands.
