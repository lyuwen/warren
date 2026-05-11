# Warren

Warren is a central hub for supervising and interacting with distributed coding-agent sessions that run inside tmux across multiple local and remote servers.

## Project Status

**Phase 1 Implementation Complete** - The tmux interface layer has been validated and is ready for use.

## Quick Start

### Build

```bash
go build -o warren ./cmd/warren
```

### Usage

```bash
# Discover tmux topology
./warren topology

# Capture content from a pane
./warren capture <pane-id>

# Send text to a pane
./warren send <pane-id> "your text here"

# Test control loop
./warren test-loop <pane-id>
```

## Phase 1 Implementation

Phase 1 validates the tmux interface model. The following components are implemented:

### Core Components

- **Server Model** (`internal/core/`)
  - Server entity with local/remote support
  - Server registry with YAML persistence
  - Connection pool for SSH connections (placeholder)

### Tmux Interface (`internal/tmux/`)

- **Topology Discovery**
  - List sessions, windows, and panes
  - Build complete topology tree: Server → Session → Window → Pane
  - Support for both local and remote execution

- **Pane Capture**
  - Capture pane content with configurable scrollback
  - ANSI escape sequence stripping
  - Configurable capture options

- **Pane Control**
  - Send text to panes
  - Send keystroke sequences (Enter, Tab, Ctrl+C, etc.)
  - Pane validation before operations

- **Control Loop**
  - High-level capture → validate → action → verify workflow
  - Built-in validators and verifiers
  - Helper methods for common operations (approve permission, answer questions)
  - State change detection

## Architecture

```
warren/
├── cmd/
│   └── warren/          # CLI tool for Phase 1 testing
├── internal/
│   ├── core/            # Server model and registry
│   └── tmux/            # Tmux interface layer
│       ├── executor.go      # Command execution (local/remote)
│       ├── topology.go      # Topology discovery
│       ├── capture.go       # Pane content capture
│       ├── control.go       # Pane control (send text/keys)
│       └── control_loop.go  # High-level control workflows
└── docs/
    ├── CLAUDE.md        # Project guide
    ├── design-review.md # Design specification
    └── ROADMAP.md       # Implementation roadmap
```

## Testing

```bash
# Run unit tests
go test ./...

# Run integration tests (requires tmux)
go test -v ./internal/tmux/...

# Run only unit tests (skip integration)
go test -short ./...
```

## Phase 1 Validation Results

✅ **Server Model**: Local server detection and registry persistence working
✅ **Topology Discovery**: Successfully discovers sessions, windows, and panes
✅ **Pane Capture**: Captures content with ANSI stripping and scrollback support
✅ **Pane Control**: Sends text and keystrokes reliably
✅ **Control Loop**: Capture → validate → action → verify workflow validated

**Conclusion**: The tmux interface is clean and reliable enough to build Phase 2 on top of it.

## Next Steps

Phase 2 will implement:
- Agent session registry and discovery
- Event store for activities and notifications
- Activity parser for Claude Code sessions
- State detection (idle, thinking, waiting_permission, etc.)
- Basic TUI and web interface

## Documentation

- [CLAUDE.md](CLAUDE.md) - Project guide and working instructions
- [design-review.md](design-review.md) - Complete design specification
- [ROADMAP.md](ROADMAP.md) - Implementation roadmap with all phases

## License

TBD
