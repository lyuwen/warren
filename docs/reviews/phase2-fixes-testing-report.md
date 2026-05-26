# Phase 2 Fixes — Testing Report

**Branch:** `test/phase2-fixes`
**Tester:** tester (dev-team-warren)
**Status:** Tests written against spec; pending merge with `feat/phase2-fixes`

## Summary

Tests added for all 4 Phase 2 fixes. Tests were written in parallel with the
Implementer's feat/ branch — they pass when run against an empty
`internal/core/` (the targeted symbols come from the implementer's branch)
only after merge.

## Test inventory

### Issue 1 — `findRepoRoot` (artifact_profile_test.go)

| Test | Purpose | Expected after fix |
| --- | --- | --- |
| `TestFindRepoRoot` (updated) | Original test, now seeds `.git/HEAD` | PASS |
| `TestFindRepoRoot_StaleGitDir` | Empty `.git/` dir must NOT be treated as repo (the actual bug) | PASS |
| `TestFindRepoRoot_GitWorktree` | `.git` as a regular file (worktree pointer) IS a repo | PASS |
| `TestFindRepoRoot_RegularRepoWithHEAD` | Canonical case with `.git/HEAD` | PASS |

Pre-existing `TestArtifactProfile_GetFilesByRepo`, `..._GetEditedFilesByRepo`,
`..._GetRelativePaths` were updated to seed `.git/HEAD` so they continue to
pass after the fix.

### Issue 2 — SSH `ConnectionPool` (server_test.go)

| Test | Purpose |
| --- | --- |
| `TestConnectionPool_LocalServerRejected` | `Get(localServer)` returns error |
| `TestConnectionPool_ConcurrentGet` | 25 goroutines call `Get` against unreachable host — must not race or panic (run with `-race`) |
| `TestConnectionPool_Close_MapCleared` | Pool seeded with a fake entry; `Close()` clears the map |
| `TestConnectionPool_LiveSSH` | Real SSH round-trip + connection caching; gated behind `WARREN_TEST_SSH=1` |

The pre-existing `TestConnectionPool_RemoteServerPlaceholder` in
`remote_test.go` will break once Issue #2 lands (it asserts that remote SSH
returns an error). Implementer/reviewer should update or remove it.

### Issue 3 — Registry save logging (registry_integration_test.go, NEW)

| Test | Purpose |
| --- | --- |
| `TestRegisterAgentSession_LoggingOnSaveFailure` | Captures `log` output; registryPath points under a file (forces save failure); asserts no error returned AND a warning is logged |
| `TestRegisterAgentSession_NoLogOnSuccess` | Successful save produces no warning noise |
| `TestRegisterAgentSession_EmptyPathNoSave` | Empty registryPath means no save attempted, no logs |

### Issue 4 — `SubscribeToUpdates` polling (conversation_service_test.go)

These tests assume the implementer exposes:

```
func (cs *ConversationService) SubscribeToFile(path string, interval time.Duration) (<-chan string, func(), error)
```

(Sent an open question to the implementer to confirm the API shape.)

| Test | Purpose |
| --- | --- |
| `TestSubscribeToFile_DetectsNewMessages` | Appending a line triggers a notification within 2s |
| `TestUnsubscribe_StopsPolling` | After `stop()`, further appends produce no notifications |
| `TestSubscribeToFile_ChannelDoesNotBlock` | 50 rapid appends do not deadlock the poller |
| `TestSubscribeToFile_NonExistentFile` | Late-created file handled either by returning err on Subscribe OR by emitting once it appears |

If the implementer settles on a different API (e.g., takes `agentID` and uses
internal session/pane registration), the four tests need a thin adapter; the
behaviors covered remain valid.

## How to run

```bash
cd /home/lfu/git-projects/warren/.worktrees/test
git merge feat/phase2-fixes        # bring in the implementation
go test ./internal/core/... -race
# Optional, live SSH:
WARREN_TEST_SSH=1 WARREN_TEST_SSH_HOST=... WARREN_TEST_SSH_USER=... \
  go test ./internal/core/ -run TestConnectionPool_LiveSSH
```

## Current run status (test branch ONLY, before merge)

- All new tests compile and run except the four `SubscribeToFile_*` tests,
  which fail to compile until the implementer adds the method.
- `TestFindRepoRoot`, `TestFindRepoRoot_StaleGitDir`, and
  `TestRegisterAgentSession_LoggingOnSaveFailure` FAIL — this is expected
  because the implementer's fixes are on the parallel `feat/` branch and not
  yet merged. They will pass after merge.
- `TestConnectionPool_*` (new ones) PASS even on the test branch because
  they exercise the existing pool plumbing.

## Coverage gaps & suggestions

- **SSH internals:** If the implementer factored out
  `buildClientConfig`/`defaultAuthMethods`, add targeted unit tests for: agent
  socket precedence when `SSH_AUTH_SOCK` set, key-file fallback, and error
  when no auth method is available. Not added here pending visibility into
  the implementation.
- **Conversation polling latency:** The 2s timeout in
  `TestSubscribeToFile_DetectsNewMessages` is forgiving; if the poller is
  meant to be near-real-time, tighten when API is stable.
- **Pre-existing test `TestConnectionPool_RemoteServerPlaceholder`** in
  `remote_test.go` asserts the *unimplemented* behavior and will fail after
  Issue #2 lands. Flag for reviewer.

## Concerns

- The implementer has not yet replied with the chosen `SubscribeToUpdates`
  API shape. If they pick something other than my assumed signature, the
  four polling tests need a small refactor.
- Pre-existing `go vet` warnings in `warren.go` (context leak in
  `NewWarren`) are unrelated to these fixes but worth noting to architect.
