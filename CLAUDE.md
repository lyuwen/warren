# Warren Project Guide

Warren is a central hub for supervising and interacting with distributed coding-agent sessions that run inside tmux across multiple local and remote servers.

## Project Status

**Current phase:** Design complete, implementation not started.

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
└── docs/                 # Additional documentation
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

## When to Update This Guide

Update `CLAUDE.md` when:

- The project structure changes significantly
- New key documents are added
- Technology choices are finalized
- Implementation reveals design flaws that require spec changes

Always keep `design-review.md` as the canonical design source. If implementation diverges from the design, update the design doc first, then update the roadmap and this guide.
