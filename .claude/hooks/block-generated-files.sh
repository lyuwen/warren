#!/usr/bin/env bash
# PreToolUse hook: deny Edit|Write|MultiEdit on Warren's build artifacts and runtime state.
#
# Blocks:
#   - SQLite event store:        warren.db (any size — easy to accidentally corrupt)
#   - Built binaries:            warren, warren-tui, warren-web at project root
#   - Coverage output:           coverage.out
#   - Runtime state dir:         .warren/* (managed by the daemon)
#
# All other paths pass through.
set -uo pipefail

input=$(cat)
file=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')

if [ -z "$file" ]; then
  exit 0
fi

# Normalize to a path relative to the project root when possible.
rel="$file"
case "$file" in
  "$CLAUDE_PROJECT_DIR"/*) rel=${file#"$CLAUDE_PROJECT_DIR"/} ;;
esac

deny() {
  local reason="$1"
  jq -n --arg msg "$reason" '{
    hookSpecificOutput: { permissionDecision: "deny" },
    systemMessage: $msg
  }'
  exit 0
}

# Built binaries and DB at project root.
case "$rel" in
  warren.db|warren.db-*|warren|warren-tui|warren-web|coverage.out)
    deny "Refusing edit on build/state artifact \`$rel\` — regenerate via \`make build\` / \`make test-coverage\` instead."
    ;;
esac

# Warren runtime state directory.
case "$rel" in
  .warren|.warren/*)
    deny "Refusing edit on runtime state \`$rel\` — \`.warren/\` is managed by the warren daemon."
    ;;
esac

exit 0
