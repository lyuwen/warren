# Warren Project Guide

Warren is a central hub for supervising and interacting with distributed coding-agent sessions that run inside tmux across multiple local and remote servers.

## Project Status

**Current phase:** Phase 2 Complete ✅  
**Next phase:** Phase 3 - Interactive Hub

**Phase Completion:**
- ✅ Phase 1: Topology + Capture Validation (Complete)
- ✅ Phase 2: Central Read-Only Hub (Complete)
- ⏳ Phase 3: Interactive Hub (Not started)

## Key Documents

- **`design-review.md`** — The canonical design specification. Read this first to understand what Warren is, why it exists, and how it should work.
- **`ROADMAP.md`** — The implementation roadmap. Breaks the design into concrete phases and tasks with clear success criteria.

## Design Summary

Warren solves the problem of managing many Claude Code agent sessions distributed across multiple tmux environments. Instead of SSHing into each server and attaching to each tmux pane individually, Warren provides a central control surface where you can:

- See all agent sessions and their current state
- Understand what each agent is working on
- Identify which agents need attention (waiting for permission, asking questions, finished, errored)
- Act on agents directly (approve permissions, answer questions, send messages)
- Track what files and repos each agent has touched
- Manage plugin and permission state across environments

## Architecture

Warren has three layers:

1. **Warren Core** — The stable center. Handles server connections, tmux topology discovery, pane capture/control, activity parsing, state detection, and event storage.
2. **Desktop TUI** — The full operator console. Keyboard-first, deep workspace view.
3. **Web Interface** — The remote/mobile-friendly surface. Accessible from anywhere.

## Key Design Decisions

- **Tmux as first source of truth:** Warren targets real tmux panes and uses `tmux capture-pane` and `tmux send-keys` as the primary interface.
- **Local is just a special remote:** Local tmux environments are modeled as a special case of a server.
- **Events, not snapshots:** Activities and notifications are persisted as event streams in a database, not just current-state snapshots.
- **Explicit topology:** Warren models the full hierarchy: Server → TmuxSession → Window → Pane → AgentSession.
- **Simple permissions:** Permission management is a lightweight tracker and mode rotator, not a heavy policy engine.
- **Plugin inventory, not payload:** Warren tracks plugin state but doesn't bundle plugin files.

## Implementation Approach

Start with **Phase 1** (Topology + Capture Validation) to prove the tmux interface works. If plain `tmux capture-pane` and `tmux send-keys` are too fragile, stop and design a better integration layer before proceeding.

Then build **Phase 2** (Central Read-Only Hub) to make the workspace visible, followed by **Phase 3** (Interactive Hub) to add control capabilities.

**Phase 4** (Extended Operational Surfaces) adds plugin management and richer views, but is lower priority.

## Open Questions

The design explicitly flags these as investigation items:

1. Is plain tmux capture/control clean enough, or do we need a tmux plugin or socket protocol?
2. What is the real-world accuracy of heuristic parsing on Claude Code sessions?
3. What polling interval balances responsiveness and overhead?
4. How should Warren display uncertainty when state is ambiguous?
5. When both tmux-derived and Claude-derived events exist, which takes precedence?

## Working on Warren

When working on this project:

- **Read `design-review.md` first** to understand the full design.
- **Follow the phases in `ROADMAP.md`** — don't skip ahead.
- **Validate Phase 1 before building Phase 2** — the tmux interface is the foundation.
- **Treat activities and notifications as events** — append-only, immutable, queryable.
- **Keep permissions simple** — don't overbuild rules in the first version.
- **Test with real Claude Code sessions** — synthetic tests won't catch real parsing issues.

## Technology Choices (Provisional)

These are recommendations, not requirements:

- **Language:** Go (good SSH/terminal support, easy deployment, fast enough)
- **TUI framework:** Bubble Tea or tview
- **Web framework:** Go stdlib http or Gin
- **Database:** SQLite (local-first, simple, good enough for event storage)
- **Config format:** YAML for human-edited files, JSON for machine-generated state

## File Organization

```
warren/
├── CLAUDE.md              # This file
├── design-review.md       # Canonical design spec
├── ROADMAP.md            # Implementation roadmap
├── cmd/                  # CLI entry points
│   ├── warren/           # Main Warren daemon/CLI
│   └── warren-web/       # Web interface server
├── internal/             # Internal packages
│   ├── core/             # Warren Core logic
│   ├── tmux/             # Tmux interface
│   ├── parser/           # Activity parser
│   ├── state/            # State detection
│   ├── events/           # Event store
│   ├── tui/              # TUI implementation
│   └── web/              # Web interface
├── pkg/                  # Public packages (if any)
├── docs/                 # Additional documentation
└── pages/                # Store project status and progress in HTML for visualization
```

## Getting Started (Once Implementation Begins)

1. Read `design-review.md` to understand the design
2. Read `ROADMAP.md` Phase 1 to understand the first tasks
3. Set up a test environment with tmux and Claude Code
4. Implement Phase 1.1-1.4 to validate the tmux interface
5. If Phase 1 succeeds, proceed to Phase 2
6. If Phase 1 fails, stop and redesign the tmux integration layer

## Design Principles to Preserve

- **Centralize attention, not execution** — agents run where they belong, Warren just observes and controls
- **Model reality explicitly** — tmux session ≠ agent session, don't collapse them
- **Events over snapshots** — history matters, state transitions matter
- **Simple first, complex later** — don't overbuild permissions or plugins too early
- **Validate interfaces early** — prove tmux capture/control works before building on it

## Project progress visualization

Visualize the progress in HTML.

- Always update the HTML page before committing major progress
- Put the HTML page in pages/index.html

## Phase Completion Process

When completing a major phase (Phase 1, Phase 2, etc.), follow this wrap-up process:

### Required Documentation

Create these three documents in `docs/`:

1. **`phaseN-completion-report.md`** — Comprehensive completion report
   - Executive summary (what was delivered)
   - Feature breakdown with status
   - Architecture overview and key decisions
   - Testing summary (coverage, results, test counts)
   - Performance metrics (if measured)
   - Production readiness checklist
   - Known issues and limitations
   - Next steps

2. **`phaseN-lessons-learned.md`** — Retrospective analysis
   - What went well (with evidence)
   - What could improve (with root causes)
   - Architecture decisions (rationale and trade-offs)
   - Design patterns that worked
   - Team collaboration insights
   - Recommendations for next phase

3. **`phaseN-technical-debt.md`** — Technical debt inventory
   - Known limitations (prioritized: high/medium/low)
   - Future enhancements
   - Performance optimizations needed
   - Code cleanup opportunities
   - Test coverage gaps
   - Each item should include: issue, impact, effort, recommendation, workaround

### ROADMAP.md Updates

After creating wrap-up docs, update `ROADMAP.md`:

- Mark all phase tasks as `[x]` complete
- Add "Phase N Wrap-Up Documentation" section with links to the three docs
- Update test coverage numbers
- Update documentation status

### README.md Updates

Update the main README:

- Update "Project Status" section
- Add new features to "What's New" section
- Update "Known Limitations" (remove fixed issues)
- Update test coverage statistics

### Progress Visualization

Update `pages/index.html`:

- Mark phase as complete with progress bars
- Update statistics (tests, lines of code, commits)
- Add completed features to showcase
- Update recent commits log

### Example

See Phase 2 wrap-up as reference:
- [Phase 2 Completion Report](docs/phase2-completion-report.md)
- [Phase 2 Lessons Learned](docs/phase2-lessons-learned.md)
- [Phase 2 Technical Debt](docs/phase2-technical-debt.md)

This process ensures knowledge is preserved and future phases benefit from past experience.

- Mark all phase tasks as complete with `[x]`
- Add phase status: `**Status:** ✅ **COMPLETE**`
- Add wrap-up documentation section with links to the three docs above
- Add brief Phase N+1 planning section if not already present

### CLAUDE.md Updates

- Update "Project Status" section with current phase
- Add any new key documents to "Key Documents" section
- Update "Getting Started" if process changed
- Document any new conventions or practices learned

### Review Checklist

Before marking a phase complete, verify:

- [ ] All planned features implemented and tested
- [ ] All tests passing (or failures documented)
- [ ] Production readiness assessed
- [ ] All three wrap-up documents created
- [ ] ROADMAP.md updated with completion status
- [ ] README.md updated with new features
- [ ] Getting started guide updated if needed
- [ ] HTML progress page updated (pages/index.html)
- [ ] Technical debt documented and prioritized
- [ ] Lessons learned captured for next phase

### Template Structure

Use this structure for completion reports:

```markdown
# Phase N Completion Report

**Date:** YYYY-MM-DD
**Status:** ✅ Complete
**Duration:** ~X weeks

## Executive Summary
[2-3 paragraphs: what was delivered, success criteria met]

## Features Delivered
[Section per major feature with status, tests, key decisions]

## Architecture Decisions
[Document key choices with rationale and trade-offs]

## Testing Summary
[Coverage, test counts, what was tested]

## Performance Metrics
[If measured: latency, memory, throughput]

## Production Readiness
[Checklist: deployment requirements, known issues]

## Lessons Learned
[Brief summary, link to full lessons-learned doc]

## Next Steps
[What Phase N+1 will address]
```

## When to Update This Guide

Update `CLAUDE.md` when:

- The project structure changes significantly
- New key documents are added
- Technology choices are finalized
- Implementation reveals design flaws that require spec changes

Always keep `design-review.md` as the canonical design source. If implementation diverges from the design, update the design doc first, then update the roadmap and this guide.

## Phase Wrap-Up Process

**At the end of each phase, create comprehensive wrap-up documentation:**

### Required Documents

1. **Phase Completion Report** (`docs/phaseN-completion-report.md`)
   - Executive summary of objectives and success criteria
   - Detailed review of all features delivered
   - Architecture decisions and rationale
   - Performance metrics and test coverage
   - Code statistics (files changed, lines added, commits)
   - Known issues and technical debt summary
   - Deployment status and requirements
   - Sign-off and readiness for next phase

2. **Lessons Learned** (`docs/phaseN-lessons-learned.md`)
   - What worked well (architecture, design, process)
   - What didn't work (issues, mistakes, rework)
   - What we'd do differently next time
   - Key insights by category (architecture, testing, UX, performance, security)
   - Design principles discovered
   - Recommendations for next phase

3. **Technical Debt Log** (`docs/phaseN-technical-debt.md`)
   - Known issues and limitations
   - Prioritized by impact (high/medium/low)
   - Effort estimates for each item
   - Recommendations for when to address
   - Workarounds for users
   - Code locations for developers
   - Items deferred to future phases
   - Items that won't be fixed (by design)

### Update Existing Documents

4. **ROADMAP.md**
   - Mark all phase tasks as complete
   - Add references to wrap-up documentation
   - Update test coverage numbers
   - Update documentation list

5. **CLAUDE.md** (this file)
   - Update project status
   - Update phase completion status
   - Document any new key documents
   - Update technology choices if changed

6. **README.md**
   - Update project status
   - Add new features to highlights
   - Update known limitations
   - Update test coverage numbers

### Optional Documents

7. **Progress Visualization** (`pages/index.html`)
   - Update statistics (tests, lines of code, files)
   - Mark phase as complete
   - Add new features to showcase
   - Update recent commits log

### Process

1. **Review all work** - Go through commits, PRs, and completed tasks
2. **Document decisions** - Capture architecture decisions and rationale
3. **Capture insights** - Write down lessons learned while fresh
4. **Track debt** - Document known issues and limitations
5. **Update references** - Link wrap-up docs from ROADMAP and CLAUDE.md
6. **Commit and push** - Make wrap-up documentation part of the phase deliverable

### Why This Matters

- **Knowledge preservation:** Captures context and rationale for future developers
- **Continuous improvement:** Lessons learned inform next phase
- **Technical debt tracking:** Prevents issues from being forgotten
- **Onboarding:** New team members can understand project history
- **Decision audit trail:** Documents why choices were made

### Example

See Phase 2 wrap-up documentation:
- [Phase 2 Completion Report](docs/phase2-completion-report.md)
- [Phase 2 Lessons Learned](docs/phase2-lessons-learned.md)
- [Phase 2 Technical Debt](docs/phase2-technical-debt.md)
