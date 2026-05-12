---
name: user-decisions-phase2
description: User decisions and preferences for Phase 2 implementation
metadata:
  type: user
---

# User Decisions - Phase 2 Features

## Implementation Priority

**Decision**: A → B → C (state fixes first, then conversation display)

**Why**: State detection is blocking usability. Fix it first before adding new features.

## Remote Session Support

**Decision**: Yes, support reading ~/.claude from remote servers via SSH

**How to apply**: When implementing conversation reader, add SSH support for remote ~/.claude access. Don't skip this - it's a requirement.

## Conversation Depth

**Decision**: Display full history or as much as can be scraped

**How to apply**: Don't artificially limit conversation history. Show everything available. Add pagination/scrolling if needed for performance.

## UI Implementation Strategy

**Decision**: Treat conversation history as core backend feature, not separate UI implementations

**How to apply**: 
- Build shared conversation backend (internal/claude/ package)
- Both TUI and web consume the same API
- Don't duplicate conversation reading logic
- Front-end display differs by interface, but data access is unified

**Why**: Avoids code duplication, ensures consistency, easier to maintain.

## Phase Execution

**Phase A (State Detection)**: Start immediately, highest priority
**Phase B (Investigation)**: After Phase A complete
**Phase C (Conversation Display)**: After Phase B complete, implement both TUI and web

## Key Architectural Principle

Conversation history is a **core Warren capability**, not a UI feature. The backend provides the data, UIs consume it.
