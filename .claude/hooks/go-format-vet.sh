#!/usr/bin/env bash
# PostToolUse hook: run gofmt -w on the edited file, then `go vet` on its package.
# Triggered for Edit|Write|MultiEdit on *.go files.
#
# Exit 0 + stdout  → shown to Claude as info (formatting succeeded).
# Exit 2 + stderr  → fed back to Claude so it knows to fix vet failures.
#
# Failures in gofmt/vet are surfaced but never block the edit itself.
set -uo pipefail

input=$(cat)
file=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty')

# Only act on Go source files inside the project.
case "$file" in
  ""|*/vendor/*|*_generated.go) exit 0 ;;
  *.go) ;;
  *) exit 0 ;;
esac

# Skip files outside the project root (worktrees, /tmp, etc.).
case "$file" in
  "$CLAUDE_PROJECT_DIR"/*) ;;
  *) exit 0 ;;
esac

cd "$CLAUDE_PROJECT_DIR" || exit 0

# Format in place. gofmt only writes if changes are needed.
fmt_out=$(gofmt -w "$file" 2>&1) || true

# go vet on the package directory (relative to project root).
rel=${file#"$CLAUDE_PROJECT_DIR"/}
pkg_dir=$(dirname "$rel")
vet_out=$(go vet "./$pkg_dir/..." 2>&1)
vet_status=$?

if [ -n "$fmt_out" ]; then
  printf 'gofmt: %s\n' "$fmt_out"
fi

if [ $vet_status -ne 0 ]; then
  printf 'go vet failed in ./%s/...:\n%s\n' "$pkg_dir" "$vet_out" >&2
  exit 2
fi

if [ -n "$vet_out" ]; then
  printf 'go vet: %s\n' "$vet_out"
fi
exit 0
