## Testing Report

### Summary
- Scope: Phase 2 audit Batch 1 test work on `test/phase2-audit-batch1`
- Branch head (before final report commit): `33f22c0`
- New/updated test files:
  - `internal/core/hostkey_test.go`
  - `internal/core/defaults_test.go`
  - `internal/core/cmd_legacy_db_test.go`
  - `internal/parser/activity_realui_test.go`
- Batch 1 target packages status:
  - `internal/core` ✅ passes under `go test ./internal/core/... -race -count=1`
  - `internal/parser` ✅ passes under `go test ./internal/parser/... -race -count=1`
- Full suite status:
  - `go test ./... -race -count=1 -timeout=180s` ❌ not fully green
  - Remaining failures are outside Batch 1 test scope in `internal/state`

### Coverage by Item

#### Item 1 — SSH host key strict default (P0 SECURITY)

Implemented in `internal/core/hostkey_test.go`.

Coverage map:

1. **known_hosts present + valid host → callback succeeds**
   - `TestConnectionPool_HostKeyCallback_KnownHostsPresentValid`

2. **known_hosts missing, env var unset → refuse-to-start, actionable message, no insecure fallback**
   - `TestConnectionPool_HostKeyCallback_MissingNoEnvVar_Refuses`
   - asserts `errors.Is(err, ErrKnownHostsMissing)`
   - asserts message includes `WARREN_SSH_INSECURE_HOSTKEY`
   - asserts message includes `ssh-keyscan`

3. **known_hosts missing, `WARREN_SSH_INSECURE_HOSTKEY=1` → callback succeeds, WARN logged**
   - `TestConnectionPool_HostKeyCallback_MissingWithEnvVar_InsecureWarns`
   - asserts WARN includes host:port

4. **known_hosts present but no entry for this host → callback factory still succeeds (handshake-time verification)**
   - `TestConnectionPool_HostKeyCallback_KnownHostsPresentButHostUnknown`
   - note: this matches the shipped implementation contract; host mismatch is enforced at callback/handshake time, not factory-construction time

5. **known_hosts permission-denied → unreadable-specific error**
   - `TestConnectionPool_HostKeyCallback_PermissionDeniedNoEnvVar_Refuses`
   - asserts `errors.Is(err, ErrKnownHostsUnreadable)`
   - asserts message includes `unreadable`

6. **known_hosts malformed/corrupt → malformed-specific error**
   - `TestConnectionPool_HostKeyCallback_MalformedNoEnvVar_Refuses`
   - asserts `errors.Is(err, ErrKnownHostsMalformed)`
   - asserts message includes `malformed`

WARN-once dedup coverage:
- `TestConnectionPool_HostKeyCallback_WarnsOncePerHostPort`
- `TestConnectionPool_HostKeyCallback_WarnsOncePerDifferentPorts`

Strict-overrides-insecure safety coverage:
- `TestConnectionPool_HostKeyCallback_EnvVarDoesNotDowngradePresent`

`Get()` early-return smoke:
- `TestConnectionPool_Get_EarlyReturnsBeforeDialWhenHostKeyUnavailable`
- seeds a real temp ed25519 key via `ssh-keygen` so auth-method discovery succeeds and the code deterministically reaches host-key gating before any TCP dial path
- skips if `ssh-keygen` is unavailable

Notes:
- Final implementation aligned to the later locked API contract:
  - pool-scoped `(*ConnectionPool).hostKeyCallback(...)`
  - sentinels `ErrKnownHostsMissing`, `ErrKnownHostsUnreadable`, `ErrKnownHostsMalformed`
  - pool-scoped dedup state (`p.insecureHostsWarned`)

#### Item 2 — Parser duplicate-event refactor (P0 CORRECTNESS)

Implemented in `internal/parser/activity_realui_test.go`.

Coverage map:

(a) **Per-line invariant over real fixtures**
- `TestParser_RealFixtures_NoDuplicatePerLine`
- fixtures used (all 5 from `internal/testdata/`):
  - `asking_permission_foundry2.txt`
  - `asking_question_foundry2.txt`
  - `idle_completed_foundry2.txt`
  - `idle_completed_localhost.txt`
  - `running_localhost.txt`

(b) **Precedence ladder table**
- `TestParser_PrecedenceTable`
- covers:
  - `● Read /etc/hosts` → file, not chat
  - `● Bash(ls -la)` → tool, not chat
  - `● Hello world` → chat (assistant)
  - `● What should I do next?` → question/prompt, not chat
  - `❯ run the tests` → chat (user)
  - `❯ 2. Yes, and don't ask again` → permission, not chat
  - `● Allowed by auto mode for Read` → permission, not chat
  - `Esc to cancel · Tab to amend` → permission
  - `● I'm running tests now` → chat, not tool
  - `● Executing command: foo is what I'd usually do` → chat, not tool

(c) **Real-fixture guard against synthetic-only blindspots**
- invariant tests above are fixture-based
- additional fixture smokes:
  - `TestParser_RealFixtures_EmitSomeEvents`
  - `TestParser_RealFixtures_PermissionFixtureEmitsPermission`
  - `TestParser_RealFixtures_QuestionFixtureEmitsQuestion`

(d) **Case-inconsistent switch regression**
- `TestParser_ToolUsage_CaseInsensitive_LegacyBash`
- `TestParser_ToolUsage_CaseInsensitive_LegacyLSP`

Important outcome:
- These tests were initially written against the pre-refactor parser and failed exactly as expected (duplicate `chat` + typed event emission, over-broad legacy tool matches, lowercase `bash tool` producing `unknown`).
- After the single-dispatch parser refactor landed, all tests passed.

#### Item 3 — `warren.db` default path + legacy detection

Implemented in:
- `internal/core/defaults_test.go`
- `internal/core/cmd_legacy_db_test.go`

Helper coverage:

1. **`DefaultDBPath()` HOME-set branch**
   - `TestDefaultDBPath_WithHome`

2. **`DefaultDBPath()` HOME-empty fallback branch**
   - `TestDefaultDBPath_EmptyHome`
   - asserts no rooted `/.warren/...` expansion bug

3. **`DefaultConfigDir()` analogous coverage**
   - `TestDefaultConfigDir_WithHome`
   - `TestDefaultConfigDir_EmptyHome`

4. **`EnsureConfigDir()` create / idempotent / error path**
   - `TestEnsureConfigDir_CreatesMissing`
   - `TestEnsureConfigDir_NoopExisting`
   - `TestEnsureConfigDir_EmptyDirRejected`
   - `TestEnsureConfigDir_PermissionDenied`

Legacy DB helper coverage (final implementation uses `CheckLegacyDB(userOverrode, cwd, defaultPath)`):
- `TestCheckLegacyDB_TriggersWhenLegacyExistsAndNoOverride`
- `TestCheckLegacyDB_NoTriggerWhenUserOverrode`
- `TestCheckLegacyDB_NoTriggerWhenNewPathExists`
- `TestCheckLegacyDB_NoTriggerWhenNoLegacyFile`
- `TestCheckLegacyDB_EmptyArgs_Noops`

Cmd wiring / integration coverage:
- `TestCmd_WarrenWeb_LegacyDBRefusesToStart`
- `TestCmd_WarrenWeb_LegacyDBHonorsExplicitDBFlag`
- `TestCmd_WarrenTUI_LegacyDBRefusesToStart`

Notes:
- The final shipped implementation changed from an intermediate `LegacyDBCheck(userOverrodeDB, binaryHint)` helper to the locked API `CheckLegacyDB(userOverrode, cwd, defaultPath)`.
- Tests were updated accordingly.
- Current shipped UX uses a uniform `warren -db ./warren.db` escape-hatch string in the message, so cmd integration tests assert that exact text rather than per-binary names.

### Race Detector Results

Commands run:
- `go test ./internal/core/... ./internal/parser/... -race -count=1`
- `go test ./... -race -count=1 -timeout=180s`

Results:
- `internal/core` ✅ clean under `-race`
- `internal/parser` ✅ clean under `-race`
- Full suite ❌ not fully clean due to unrelated `internal/state` failures (see below)

Specific Batch 1 note:
- WARN-log dedup logic for SSH host-key insecure mode was exercised under `-race` via:
  - `TestConnectionPool_HostKeyCallback_WarnsOncePerHostPort`
  - `TestConnectionPool_HostKeyCallback_WarnsOncePerDifferentPorts`
- No race surfaced in the Batch 1 code paths.

### Remaining Failures / Gaps

#### Full-suite failures still present outside Batch 1 scope

`go test ./... -race -count=1 -timeout=180s` still fails in `internal/state`:
- `TestStateDetector_RealSessionCapture_PermissionApproved`
- `TestStateDetector_Performance_LargeContent`

These are **not introduced by the Batch 1 test work** and are outside the three-item testing brief assigned in Task #4.

#### Branch / integration note

Because `test/phase2-audit-batch1` diverged from `feat/phase2-audit-batch1`, I had to keep the test branch carrying both:
- the implementation merge commit(s)
- the tester-only regression files

Also note:
- the parser real-UI regression file (`internal/parser/activity_realui_test.go`) was temporarily absent from the later implementation commit and had to be restored onto the test branch so Item 2’s required coverage actually remained present on the delivered testing branch.

### Suggestions / Push-back

1. **State package failures should be triaged separately before claiming repo-wide green.**
   - Batch 1 core/parser work is green.
   - Repo-wide `go test ./... -race -count=1` is still red because of `internal/state`.

2. **Keep the restored parser regression file.**
   - This file is the main protection against the exact blindspot that let the audit bug through: tests that only used legacy `User:` / `Reading file:` style inputs.

3. **Preserve the sentinel-based SSH contract.**
   - The final implementation’s `errors.Is` sentinels made the host-key tests much more stable and precise.
   - This is a real improvement over the intermediate string-only contract.

4. **The `Get()` smoke test depends on `ssh-keygen` availability.**
   - It cleanly skips if `ssh-keygen` is absent.
   - That is acceptable because the 6-case matrix is fully covered by direct `hostKeyCallback` tests regardless.

### Ready for Review

- Batch 1 test coverage for Items 1–3 is implemented.
- `internal/core` and `internal/parser` pass under `-race`.
- Full-suite remaining failures are documented and isolated to `internal/state`.
- Branch is ready for reviewer inspection from the testing perspective.
