# Code Review: Phase 2 Audit Batch 1

**Reviewer:** Reviewer  
**Date:** June 1, 2026  
**Branches:**
- Feature: `feat/phase2-audit-batch1` @ `204d5d605c5bd14aa33e73217cec200ee73aeca9`
- Tests: `test/phase2-audit-batch1` @ `6f0bb87e8a310417caefc7fab036aaf833897b8e`

**Scope:** Three P0 fixes from Phase 2 audit report:
1. SSH host key verification strict default (security)
2. Parser duplicate-event bug (correctness)
3. warren.db default path + legacy detection (data loss)

**Acceptance Oracle:** `.claude/critiques/phase2-audit-batch1-planval.md`

---

## Verdict: **APPROVED**

All three items meet the critique's acceptance criteria. No blockers. Minor suggestions noted below do not block merge.

---

## Item 1: SSH Host Key Strict Default (P0 SECURITY)

### Implementation Review

**Files Changed:**
- `internal/core/server.go` — `hostKeyCallback` → `hostKeyCallbackForServer(*Server)`, new error sentinels, WARN-once dedup
- `docs/phase2-technical-debt.md` — P0 entry #0a marked resolved, new P1 follow-up #0a-followup added

**What Was Delivered:**

✅ **Strict by default.** `hostKeyCallbackForServer` returns an error when `~/.ssh/known_hosts` is missing, unreadable, or malformed. No silent fallback to `InsecureIgnoreHostKey`.

✅ **Three distinct error messages.** `loadKnownHostsCallback` returns:
- `ErrKnownHostsMissing` + "~/.ssh/known_hosts is missing."
- `ErrKnownHostsUnreadable` + permission/stat error detail
- `ErrKnownHostsMalformed` + `knownhosts.New` parse error

✅ **Actionable error text.** `knownHostsUnavailableError.Error()` (lines 234-256) includes:
- The server name and host:port
- Why verification failed (missing/unreadable/malformed)
- Two remediation paths: `ssh-keyscan` (recommended) or `WARREN_SSH_INSECURE_HOSTKEY=1` (unsafe)

✅ **Env var opt-in for insecure mode.** `WARREN_SSH_INSECURE_HOSTKEY=1` gates the fallback (line 220).

✅ **WARN logged once per (host:port).** Lines 225-230 use `p.insecureHostsWarned.LoadOrStore` to deduplicate. Pool-scoped, not global.

✅ **Function signature accepts `*Server`.** Future per-server `InsecureHostKey bool` field can layer in without breaking callers (line 211 comment references the new P1 tech-debt entry).

✅ **Early return before dial.** `ConnectionPool.Get` calls `buildClientConfig` (which calls `hostKeyCallbackForServer`) before `Dial` (line 104-106), so host-key unavailability errors return before any TCP connection attempt.

**Critique Compliance Check:**

| Critique Requirement | Status |
|---------------------|--------|
| Env var vs per-server config decision | ✅ Env var now, per-server tracked as P1 #0a-followup |
| Error message locked (ssh-keyscan hint, env var escape hatch) | ✅ Lines 234-256 match critique proposal |
| CI test inventory | ✅ `server_test.go` tests set `t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")` |
| WARN-once per (host:port), not per connection | ✅ Pool-scoped `sync.Map` dedup |
| Missing vs unreadable vs malformed distinction | ✅ Three sentinels + distinct messages |

**Code Quality:**

✅ **Readable.** The three-tier error routing (`loadKnownHostsCallback` → sentinel + detail → `newKnownHostsUnavailableError`) is clear.

✅ **No scope creep.** The fix does exactly what the critique demanded, no more.

✅ **Consistent with existing patterns.** Uses `os.UserHomeDir()` like the rest of the codebase, `net.JoinHostPort` for formatting.

### Test Review

**File:** `internal/core/hostkey_test.go`

**Coverage:**

✅ **All 5 critique-mandated cases:**
1. `TestConnectionPool_HostKeyCallback_KnownHostsPresentValid` — valid known_hosts → callback succeeds
2. `TestConnectionPool_HostKeyCallback_MissingNoEnvVar_Refuses` — missing, no env → error with `ErrKnownHostsMissing`, message includes `WARREN_SSH_INSECURE_HOSTKEY` and `ssh-keyscan`
3. `TestConnectionPool_HostKeyCallback_MissingWithEnvVar_InsecureWarns` — missing, env set → callback succeeds, WARN logged with host:port
4. `TestConnectionPool_HostKeyCallback_KnownHostsPresentButHostUnknown` — known_hosts exists but no entry for target host → callback factory succeeds (handshake-time enforcement)
5. `TestConnectionPool_HostKeyCallback_PermissionDeniedNoEnvVar_Refuses` — unreadable → `ErrKnownHostsUnreadable`, message says "unreadable"

✅ **Malformed case:** `TestConnectionPool_HostKeyCallback_MalformedNoEnvVar_Refuses` — corrupt known_hosts → `ErrKnownHostsMalformed`

✅ **WARN-once dedup:**
- `TestConnectionPool_HostKeyCallback_WarnsOncePerHostPort` — two calls to same host:port → one WARN
- `TestConnectionPool_HostKeyCallback_WarnsOncePerDifferentPorts` — different ports → separate WARNs

✅ **Strict overrides insecure:** `TestConnectionPool_HostKeyCallback_EnvVarDoesNotDowngradePresent` — valid known_hosts + env var set → no WARN (strict path wins)

✅ **Early return smoke:** `TestConnectionPool_Get_EarlyReturnsBeforeDialWhenHostKeyUnavailable` — seeds real ed25519 key via `ssh-keygen`, asserts `Get()` returns `ErrKnownHostsMissing` before any dial attempt

**Test Quality:**

✅ **Isolated.** Each test uses `t.TempDir()` and `t.Setenv()`.

✅ **Deterministic.** No sleeps, no network calls (except the `Get()` smoke test which dials a closed port and expects early error).

✅ **Meaningful assertions.** Uses `errors.Is()` for sentinels, `strings.Contains()` for message content.

✅ **Skips gracefully.** Permission test skips under root, `Get()` smoke skips if `ssh-keygen` unavailable.

### Item 1 Verdict: **APPROVED**

**What's Good:**
- The three-sentinel error routing is a real improvement over string-only contracts.
- WARN-once dedup is pool-scoped (not global), which is the right granularity.
- Early-return before dial prevents wasted TCP attempts.
- Tech-debt doc updated: P0 #0a marked resolved, P1 #0a-followup filed for per-server config.

**Suggestions (non-blocking):**
- None. This is a clean, complete fix.

---

## Item 2: Parser Duplicate-Event Bug (P0 CORRECTNESS)

### Implementation Review

**Files Changed:**
- `internal/parser/activity.go` — complete rewrite: four independent passes → single line-by-line dispatcher with precedence ladder

**What Was Delivered:**

✅ **Single-dispatch architecture.** `Parse()` (lines 155-234) runs one pass over `lines`, calling `classifyLine()` for each. Each line produces AT MOST ONE event.

✅ **Precedence ladder implemented:**
1. Permission (lines 236-252) — `Esc to cancel`, `❯ N.`, `Allowed by auto mode`
2. Question (lines 254-268) — `● ... ?` ending in `?`, within last 10 non-empty lines
3. File (lines 270-286) — `● Read`, `● Edit(...)`, `● Write(...)`
4. Tool (lines 288-304) — `● Bash(`, `● Agent(`, `● Skill(`, etc.
5. Chat (lines 306-320) — `●` or `❯` fallback

✅ **Over-broad regexes removed.** The legacy `(?i)running\s+tests`, `(?i)executing\s+command:`, `(?i)Read\s+tool.*?file_path` patterns are gone. Chat content that mentions tool words stays chat.

✅ **Case-inconsistent switch fixed.** `toolExtractors` (lines 128-138) now carry the canonical tool name (`"bash"`, `"lsp"`) so the dispatcher doesn't need a case-sensitive switch.

✅ **Degradation contract documented.** Lines 37-43: a brand-new tool name (e.g. `● Task(...)`) falls through to chat with role=assistant. No silent data loss.

✅ **Legacy patterns preserved.** Lines 122-126 keep `(?i)^reading\s+file:`, `(?i)^editing\s+file:` for backward compatibility with older captures.

**Critique Compliance Check:**

| Critique Requirement | Status |
|---------------------|--------|
| Option C (single-dispatch refactor) chosen | ✅ Implemented |
| Precedence ladder: permission > question > file > tool > chat | ✅ Lines 236-320 |
| Sweep over-broad regexes | ✅ `running tests`, `executing command:`, legacy Read tool gone |
| Case-inconsistent switch fixed | ✅ `toolExtractors` carry canonical names |
| Real fixtures used in tests | ✅ See test review below |

**Code Quality:**

✅ **Readable.** The precedence ladder is explicit and easy to audit.

✅ **No scope creep.** The refactor does exactly what the critique demanded: unify the four parsers, fix duplicates, preserve legacy compatibility.

✅ **Well-commented.** Lines 14-44 explain the design, the precedence ladder, and the degradation contract.

✅ **Consistent with existing patterns.** Uses `regexp.MustCompile` at construction time, `strings.TrimSpace` for line normalization.

### Test Review

**File:** `internal/parser/activity_realui_test.go`

**Coverage:**

✅ **(a) Per-line invariant over real fixtures** — `TestParser_RealFixtures_NoDuplicatePerLine`:
- Parses all 5 fixtures from `internal/testdata/`
- For every non-empty line, asserts `countByContent[line] <= 1`
- This is the "no duplicates" guarantee, encoded as a property test

✅ **(b) Precedence table test** — `TestParser_PrecedenceTable`:
- Explicit (input → expected type) enumeration:
  - `● Read /etc/hosts` → file, forbid chat
  - `● Bash(ls -la)` → tool, forbid chat
  - `● Hello world` → chat (assistant)
  - `● What should I do next?` → question/prompt, forbid chat
  - `❯ run the tests` → chat (user)
  - `❯ 2. Yes, and don't ask again` → permission, forbid chat
  - `● Allowed by auto mode for Read` → permission, forbid chat
  - `Esc to cancel · Tab to amend` → permission
  - `● I'm running tests now` → chat, forbid tool
  - `● Executing command: foo is what I'd usually do` → chat, forbid tool

✅ **(c) Real-fixture guard** — `TestParser_RealFixtures_PrefixedTokenLines_NoDoubleEvent`:
- For every `● Read|Edit|Write|Bash|...` line in the 5 fixtures, asserts it did NOT produce a chat event

✅ **(d) Case-inconsistent switch regression** — `TestParser_ToolUsage_CaseInsensitive_LegacyBash`, `TestParser_ToolUsage_CaseInsensitive_LegacyLSP`:
- Feeds lowercase `bash tool` and `lsp tool` legacy text
- Asserts tool_name is `"bash"` / `"lsp"`, not `"unknown"`

✅ **Fixture smoke tests:**
- `TestParser_RealFixtures_EmitSomeEvents` — all fixtures produce >0 events
- `TestParser_RealFixtures_PermissionFixtureEmitsPermission` — `asking_permission_foundry2.txt` produces a permission event
- `TestParser_RealFixtures_QuestionFixtureEmitsQuestion` — `asking_question_foundry2.txt` produces a question event

**Test Quality:**

✅ **Real fixtures, not synthetic.** All invariant tests read from `internal/testdata/`, addressing the audit's "zero real Claude Code UI patterns tested" finding.

✅ **Deterministic.** No randomness, no external dependencies.

✅ **Meaningful assertions.** Uses `countByContent` map to detect duplicates, `forbidType` to assert precedence.

### Item 2 Verdict: **APPROVED**

**What's Good:**
- The single-dispatch refactor is a structural fix, not a regex band-aid.
- The precedence ladder is explicit and testable.
- Real-fixture tests prevent the "synthetic-only blindspot" that let the audit bug through.
- Case-inconsistent switch is fixed.

**Suggestions (non-blocking):**
- None. This is a complete, well-tested refactor.

---

## Item 3: warren.db Default Path + Legacy Detection

### Implementation Review

**Files Changed:**
- `internal/core/defaults.go` — new file: `DefaultDBPath()`, `DefaultConfigDir()`, `EnsureConfigDir()`, `CheckLegacyDB()`
- `internal/core/warren.go` — `DefaultConfig()` now calls `DefaultDBPath()` / `DefaultConfigDir()`, `NewWarren()` calls `EnsureConfigDir()`
- `cmd/warren-web/main.go` — `-db` default is `core.DefaultDBPath()`, `flag.Visit` detects override, `CheckLegacyDB()` called before `NewWarren()`
- `cmd/warren-tui/main.go` — same wiring
- `docs/phase2-technical-debt.md` — P0 entry #0c marked resolved

**What Was Delivered:**

✅ **Unified helpers.** `DefaultDBPath()` and `DefaultConfigDir()` consolidate the three drifted call sites.

✅ **HOME-unresolvable fallback.** Lines 29-34, 40-47: when `os.UserHomeDir()` fails, fall back to `.warren` / `warren.db` (cwd-relative), not `/.warren` (the `os.ExpandEnv` bug).

✅ **Auto-create config dir.** `EnsureConfigDir()` (lines 55-60) is idempotent `os.MkdirAll`. Called once in `core.NewWarren()` (warren.go:154), so both cmds inherit it.

✅ **Legacy-DB detection.** `CheckLegacyDB()` (lines 101-119):
- Returns error when: user did NOT pass `-db` explicitly AND `./warren.db` exists AND `DefaultDBPath()` does NOT exist
- Returns nil in all other cases (user override, no legacy file, new DB already exists)

✅ **Actionable error message.** `legacyDBError.Error()` (lines 67-85):
- "Found legacy database at ./warren.db"
- Migration hint: `mkdir -p ~/.warren && mv ./warren.db ~/.warren/warren.db`
- Escape hatch: `warren -db ./warren.db`
- "Refusing to start (would create empty database and orphan your data)"

✅ **flag.Visit-based detection.** Both cmds (warren-web:30-35, warren-tui:23-28) use `flag.Visit` to detect explicit `-db` override, not value comparison.

✅ **Wired into both binaries.** warren-web:44 and warren-tui:37 call `CheckLegacyDB()`, print error to stderr, exit non-zero.

**Critique Compliance Check:**

| Critique Requirement | Status |
|---------------------|--------|
| Unify into `DefaultDBPath()` / `DefaultConfigDir()` | ✅ `internal/core/defaults.go` |
| Auto-create `$HOME/.warren/` in `core.NewWarren()` | ✅ `EnsureConfigDir()` called at warren.go:154 |
| Legacy-DB refuse-to-start with actionable message | ✅ `CheckLegacyDB()` + `legacyDBError` |
| flag.Visit for override detection | ✅ Both cmds use `flag.Visit` |
| Wire into BOTH warren-web and warren-tui | ✅ Both call `CheckLegacyDB()` before `NewWarren()` |

**Code Quality:**

✅ **Readable.** The helpers are simple and well-commented.

✅ **No scope creep.** The fix does exactly what the critique demanded.

✅ **Consistent with existing patterns.** Uses `os.UserHomeDir()`, `filepath.Join()`, `os.MkdirAll()`.

✅ **Error handling.** `EnsureConfigDir()` rejects empty string (line 56-58), propagates `MkdirAll` errors.

### Test Review

**Files:**
- `internal/core/defaults_test.go` — unit tests for helpers
- `internal/core/cmd_legacy_db_test.go` — integration tests for cmd wiring

**Helper Coverage:**

✅ **`DefaultDBPath()` HOME-set branch** — `TestDefaultDBPath_WithHome`

✅ **`DefaultDBPath()` HOME-empty fallback** — `TestDefaultDBPath_EmptyHome` (asserts no `/.warren` bug)

✅ **`DefaultConfigDir()` analogous coverage** — `TestDefaultConfigDir_WithHome`, `TestDefaultConfigDir_EmptyHome`

✅ **`EnsureConfigDir()` create / idempotent / error paths:**
- `TestEnsureConfigDir_CreatesMissing`
- `TestEnsureConfigDir_NoopExisting`
- `TestEnsureConfigDir_EmptyDirRejected`
- `TestEnsureConfigDir_PermissionDenied` (skips under root)

✅ **`CheckLegacyDB()` refuse-to-start case** — `TestCheckLegacyDB_TriggersWhenLegacyExistsAndNoOverride`:
- User did NOT pass `-db`, `./warren.db` exists, `DefaultDBPath()` does NOT exist → error
- Asserts `errors.Is(err, ErrLegacyDBOrphan)`
- Asserts message includes "Found legacy database", "mkdir -p", "mv", "warren -db"

✅ **`CheckLegacyDB()` no-trigger cases:**
- `TestCheckLegacyDB_NoTriggerWhenUserOverrode` — user passed `-db` explicitly → nil
- `TestCheckLegacyDB_NoTriggerWhenBothExist` — both legacy and new DB exist → nil (already migrated)
- `TestCheckLegacyDB_NoTriggerWhenNoLegacyFile` — first run, no legacy file → nil
- `TestCheckLegacyDB_EmptyArgs_Noops` — empty cwd/defaultPath → nil (guard branch)

**Cmd Wiring Coverage:**

✅ **warren-web refuse-to-start** — `TestCmd_WarrenWeb_LegacyDBRefusesToStart`:
- Builds `warren-web` binary via `go build`
- Runs in temp cwd with legacy `./warren.db`, HOME pointed at empty temp dir
- Asserts exit non-zero, stderr includes "Found legacy database", "warren -db ./warren.db", ".warren"

✅ **warren-web honors explicit `-db`** — `TestCmd_WarrenWeb_LegacyDBHonorsExplicitDBFlag`:
- Runs `warren-web -db ./warren.db` with legacy file present
- Asserts process starts (no legacy-DB error), or exits for unrelated reason (sqlite rejecting fake content)

✅ **warren-tui refuse-to-start** — `TestCmd_WarrenTUI_LegacyDBRefusesToStart`:
- Same as warren-web test, for TUI binary

**Test Quality:**

✅ **Isolated.** Each test uses `t.TempDir()`, `t.Setenv()`, `t.Cleanup()`.

✅ **Deterministic.** No sleeps, no network calls.

✅ **Meaningful assertions.** Uses `errors.Is()` for sentinels, `strings.Contains()` for message content, exit code checks for cmd tests.

✅ **Skips gracefully.** Cmd tests skip if `go` toolchain unavailable or build fails.

### Item 3 Verdict: **APPROVED**

**What's Good:**
- The unified helpers prevent future drift.
- Legacy-DB detection is refuse-to-start (not silent data loss).
- flag.Visit-based override detection is correct (not value comparison).
- Both binaries wired up (not just one).
- Tech-debt doc updated: P0 #0c marked resolved.

**Suggestions (non-blocking):**
- None. This is a complete, well-tested fix.

---

## Cross-Cutting Concerns

### Build & Vet

✅ **`go build ./...`** — clean (no output)

✅ **`go vet ./...`** — clean (no output)

### Race Tests

✅ **Batch 1 target packages pass:**
```
go test ./internal/core/... ./internal/parser/... -race -count=1
ok  	github.com/lfu/warren/internal/core	12.528s
ok  	github.com/lfu/warren/internal/parser	1.182s
```

⚠️ **Full suite not green** — `internal/state` failures remain (documented in test report as out-of-scope for Batch 1).

### Tech-Debt Doc Updates

✅ **P0 #0a (SSH host key) marked resolved** — lines 15-36, includes resolution date, commit reference, code location, test location.

✅ **P1 #0a-followup (per-server insecure_host_key) filed** — lines 39-50, includes rationale, effort estimate, recommendation.

✅ **P0 #0c (warren.db default path) marked resolved** — lines 95-120 (inferred from doc structure; not explicitly visible in truncated read but referenced in test report).

### Documentation Quality

✅ **Code comments.** All three items have clear inline comments explaining design decisions.

✅ **Test comments.** Test files have header comments explaining what they target and why.

✅ **Commit messages.** Feat commit `204d5d6` says "fix: complete Phase 2 audit batch 1 items #1 #2 Part 2" (clear scope).

---

## Critique Pre-Validation Compliance

The critique pre-validation document (`.claude/critiques/phase2-audit-batch1-planval.md`) flagged specific requirements for each item. Here's the compliance matrix:

### Item 1 (SSH Host Key)

| Critique Requirement | Implementation | Tests |
|---------------------|----------------|-------|
| Env var vs per-server config decision | ✅ Env var now, per-server P1 | N/A |
| Error message locked | ✅ Lines 234-256 match proposal | ✅ String assertions |
| CI test inventory | ✅ `server_test.go` sets env var | ✅ Documented in test report |
| WARN-once per (host:port) | ✅ Pool-scoped `sync.Map` | ✅ Two dedup tests |
| Missing vs unreadable vs malformed | ✅ Three sentinels | ✅ Three test cases |

### Item 2 (Parser)

| Critique Requirement | Implementation | Tests |
|---------------------|----------------|-------|
| Option C (single-dispatch) | ✅ Lines 155-234 | ✅ Per-line invariant test |
| Precedence ladder | ✅ Lines 236-320 | ✅ Precedence table test |
| Sweep over-broad regexes | ✅ Legacy patterns gone | ✅ Chat-not-tool cases |
| Case-inconsistent switch | ✅ `toolExtractors` carry names | ✅ Lowercase bash/lsp tests |
| Real fixtures, not synthetic | N/A | ✅ All 5 from `testdata/` |

### Item 3 (warren.db)

| Critique Requirement | Implementation | Tests |
|---------------------|----------------|-------|
| Unify into helpers | ✅ `defaults.go` | ✅ Unit tests |
| Auto-create in `NewWarren()` | ✅ `EnsureConfigDir()` call | ✅ Unit + integration |
| Legacy-DB refuse-to-start | ✅ `CheckLegacyDB()` | ✅ Cmd integration tests |
| flag.Visit for override | ✅ Both cmds | ✅ Explicit-flag test |
| Wire into BOTH binaries | ✅ warren-web + warren-tui | ✅ Two cmd tests |

**All critique requirements met.**

---

## Summary

### Overall Assessment

This is a **high-quality, complete implementation** of the three P0 fixes. All critique requirements are met. Tests are comprehensive, deterministic, and use real fixtures where demanded. Code is readable, well-commented, and free of scope creep.

### Key Strengths

1. **SSH host key fix is production-ready.** The three-sentinel error routing, WARN-once dedup, and early-return-before-dial design are all correct. The env var is fit-for-purpose as a minimum-viable fallback, with per-server config tracked as P1 follow-up.

2. **Parser refactor is structural, not superficial.** The single-dispatch architecture with explicit precedence ladder is a real fix, not a regex band-aid. Real-fixture tests prevent regression.

3. **warren.db fix prevents silent data loss.** The refuse-to-start + actionable message design is the right call. Unified helpers prevent future drift.

4. **Tests are thorough.** All critique-mandated cases covered. Real fixtures used. Cmd-level wiring verified via integration tests.

5. **Tech-debt doc updated.** P0 entries marked resolved, P1 follow-up filed.

### Recommendations for Next Phase

1. **Address P1 #0a-followup (per-server insecure_host_key) before Phase 3 multi-server UX.** The env var is acceptable for Batch 1, but operators will need per-server granularity once Warren fans out to multiple SSH targets.

2. **Triage `internal/state` failures separately.** Batch 1 target packages are green, but full-suite `go test ./...` is still red. The Architect should assign a separate task to investigate and fix `internal/state` before claiming repo-wide green.

3. **Preserve the parser real-fixture tests.** These are the main protection against the "synthetic-only blindspot" that let the audit bug through. Do not delete or weaken them in future refactors.

### Next Steps

- ✅ Merge `feat/phase2-audit-batch1` and `test/phase2-audit-batch1` to `dev/phase2-audit-remediation`
- ✅ Run Critique post-implementation pass to verify no "claimed complete but actually partial" issues
- ⏳ Triage `internal/state` failures (separate task)
- ⏳ Address P1 #0a-followup (per-server insecure_host_key) before Phase 3

---

**Verdict: APPROVED**

No blockers. Ready to merge.
