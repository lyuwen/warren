# Code Review: Phase 2 Audit Batch 2

**Reviewer:** Reviewer
**Date:** June 11, 2026
**Branches:**
- Feature: `feat/phase2-audit-batch2` @ `73a6696` (core) + `6bcd4da` (constant-derived early-out)
- Tests: `test/phase2-audit-batch2` @ `75954a6` (merge `3d7dc9c` brings feat into test branch)

**Scope:** Two P1 audit items.
- **#3:** ConnectionPool mutex no longer held across SSH dial+handshake (concurrency).
- **#4:** REST API CORS allow-list + loopback-only bind defaults (security).

---

## Verdict: **APPROVED**

All acceptance criteria are met. `go build`, `go vet`, and `go test ./internal/core/... ./internal/web/... -race -count=1` are clean (also re-run with `-count=3` for the new concurrency tests — no flakes observed in this environment). No regressions in Batch 1 work (SSH host-key strict default, parser refactor, warren.db defaults). Two MINOR suggestions noted below; neither blocks merge.

---

## Item 3: ConnectionPool Mutex Refactor (P1 CONCURRENCY)

### Implementation Review

**File:** `internal/core/server.go`

**What was delivered:**

✅ **Dial runs outside `p.mu`.** `Get()` releases `p.mu` (line 133) before calling `p.dial(server)` (line 137). The lock is reacquired only to mutate `p.pending` / `p.connections` (lines 139–146). The dial+handshake (TCP `Dial`, `ssh.NewClientConn`) is entirely lock-free.

✅ **Per-key in-flight promise (`connDial`).** First caller installs a `&connDial{ready: make(chan struct{})}` into `p.pending[key]` under the lock and performs the dial; subsequent same-key callers find the entry under the lock, release the lock, and wait on `<-d.ready` (lines 126–130). After the dial completes, the first caller writes `d.client`/`d.err` under `p.mu` and then `close(d.ready)`, establishing the happens-before for the waiting readers. Synchronization is correct.

✅ **Exactly-one dial per key.** Successful dial: `delete(p.pending, key)` + `p.connections[key] = client`. Failed dial: `delete(p.pending, key)` only (line 140) — the failed entry is NOT stored in `p.connections`, so the next `Get` for the same key will retry. Correct.

✅ **Host-key callback flow preserved.** `p.dial` calls `p.buildClientConfig` (line 155), which calls `p.hostKeyCallback` (line 210). The Batch 1 strict-default refusal still fires BEFORE the TCP dial (the callback error short-circuits `buildClientConfig`, which returns before `dialer.Dial`). This is verified end-to-end by `TestConnectionPool_HostKeyCheckStillHonored`.

✅ **Pool-scoped insecure-host WARN dedup preserved.** `p.insecureHostsWarned` `sync.Map` carries over from Batch 1 unchanged; the refactor doesn't disturb it.

✅ **`Close()` semantics preserved.** Still iterates `p.connections` under the lock; pending in-flight dials are not stored there so they cannot be double-closed.

**Code quality:**

✅ Readable — the comment block at lines 74–80 explains the double-checked-locking + promise pattern clearly.
✅ No scope creep — only the pool's locking discipline changed; the dial helpers are unchanged behavior.
✅ Consistent with existing patterns — uses `sync.Mutex`, `net.JoinHostPort`, identical error wrapping style as Batch 1.

### Test Review

**File:** `internal/core/server_pool_test.go` (4 tests)

✅ **`TestConnectionPool_ConcurrentDifferentServers_NoSerialization`** — drives two real `net.TCPListener`s with a 200ms accept hold and asserts wall-clock elapsed < 1.5×hold. This is the primary regression test for the refactor: under the old (Batch 1) code, this would observe ~400ms; under the fix, ~200ms. The mechanism (real listeners + atomic accept counter) is sound.

✅ **`TestConnectionPool_ConcurrentSameServer_SingleDial`** — 16 goroutines released through a barrier hit `Get(sameServer)` simultaneously; asserts `accepts == 1` via atomic counter. This is the ground-truth dedup test, not a stub.

✅ **`TestConnectionPool_FailedDialDoesNotBlockOtherServers`** — uses RFC 5737 TEST-NET-3 (`203.0.113.1`) as a dial sink that times out via the pool's own `Dialer.Timeout = 600ms`. Live server uses a real loopback listener with 80ms hold. Asserts live's elapsed < deadTimeout AND that dead actually errored (sanity check on test setup validity). Correct and resilient.

✅ **`TestConnectionPool_HostKeyCheckStillHonored`** — locks in that the Batch 1 strict-default flow is unchanged: missing `~/.ssh/known_hosts` + no `WARREN_SSH_INSECURE_HOSTKEY` → `ErrKnownHostsMissing` returned BEFORE the listener observes any TCP accept. Verifies the early-return-before-dial wiring survived the refactor.

**Test quality:**

✅ Isolated — `t.TempDir()`, `t.Setenv()`, `t.Cleanup` on listeners.
✅ Deterministic — wall-clock-based but with generous ceilings; tests passed cleanly under `-race -count=3`.
✅ Skip-aware — `ssh-keygen` lookup gracefully skips on environments without it.

### Item 3 Verdict: **APPROVED**

**What's good:**
- The double-checked-locking + promise pattern is correctly implemented; happens-before is established via `close(d.ready)` and the read-after-channel-receive idiom.
- Failed-dial cleanup is correct (entry removed from `pending`, NOT stored in `connections`).
- Tests use real listeners and wall-clock observation — the dedup count and serialization tests can't be faked by a buggy implementation.

**Suggestions (non-blocking):**
- *MINOR (server_pool_test.go:127)* — the ceiling `1.5 × hold = 300ms` is tight for the no-serialization test. Passed cleanly here under `-count=3 -race`, but on heavily loaded CI it could flake. A future hardening pass could either widen the ceiling to ~1.8×, or replace the wall-clock check with a synchronization primitive (e.g. ensure both listener accepts happen before either returns). Not urgent.

---

## Item 4: REST API CORS + Loopback Bind (P1 SECURITY)

### Implementation Review

**Files:** `internal/web/server.go`, `cmd/warren-web/main.go`

**What was delivered:**

✅ **Loopback-only default.** `DefaultBindAddr = "127.0.0.1:8080"` (server.go:24). `NewServer` substitutes it when `Config.Addr == ""` (line 62–64). This is the security-posture default the audit required.

✅ **`--bind` flag + `WARREN_WEB_BIND` env.** `cmd/warren-web/main.go:21–24` reads `WARREN_WEB_BIND` as the default for `--bind`. `--bind` flag is documented (main.go:28). Test `TestWebServer_BindEnvVarOverride` mirrors the cmd resolution step.

✅ **`--addr` deprecation WARN.** `main.go:39–50`: after `flag.Parse()`, `flag.Visit` populates `addrSet`/`bindSet`. If `addrSet && !bindSet`, exactly one `log.Printf("WARN: --addr is deprecated; use --bind. ...")` fires, then `bindAddr = *addr`. WARN is emitted ONCE at startup, not per request — confirmed by reading the call path (it's in `main()` before the HTTP server starts).

✅ **Strict default CORS allow-list (loopback only).** `defaultCORSOrigins` (server.go:37–40) contains `http://localhost:8080` and `http://127.0.0.1:8080` only.

✅ **`WARREN_WEB_CORS_ORIGINS` appends, does NOT replace.** `resolveCORSOrigins` (line 129–140) starts with `append([]string{}, defaultCORSOrigins...)`, then comma-splits env value with `strings.TrimSpace` and empty-entry skipping. Defaults are never dropped.

✅ **`corsMiddleware` rejection semantics — NO 403, NO headers for out-of-policy.** (server.go:149–176)
- Same-origin / no-`Origin` header → request passes through cleanly, no CORS headers set.
- `Origin` in allow-list → set `Access-Control-Allow-Origin`, `Vary: Origin`, `Allow-Methods`, `Allow-Headers`. Preflight OPTIONS short-circuits with 204.
- `Origin` NOT in allow-list → NO CORS headers, request still flows to `next.ServeHTTP`. The browser will block the response client-side. There is NO 403, NO structured rejection — exactly the contract the architect requested.

✅ **`originHostsForBind` auto-expand for non-default loopback ports.** (server.go:206–225)
- Non-loopback host (e.g. `0.0.0.0`, `192.168.x.x`, empty host) → returns `nil`. Forces operators to opt in via `WARREN_WEB_CORS_ORIGINS` for non-loopback exposure.
- Loopback host on default port → `nil` (already covered by `defaultCORSOrigins`).
- Loopback host on custom port → `["http://localhost:PORT", "http://127.0.0.1:PORT"]`.
- IPv6 `[::1]` is normalized to loopback equivalents.

✅ **`DefaultBindAddr`-derived early-out (commit `6bcd4da`).** server.go:214 uses `net.SplitHostPort(DefaultBindAddr)` to derive the default port for the short-circuit check. **Magic-string check: confirmed clean.** Grep for `8080` in `internal/web/server.go` shows three remaining occurrences, all expected and acceptable:
  1. `DefaultBindAddr` constant (line 24) — the canonical source.
  2. `defaultCORSOrigins` entries (lines 38–39) — see MINOR note below.
  3. A comment-only example on line 179 — not runtime.

✅ **Non-loopback bind WARN.** `isLoopbackBind` (server.go:181–199) correctly identifies loopback (`127.0.0.0/8` via `IsLoopback`, `localhost`, `::1`), rejects empty host and unparseable addrs. `Server.Start()` (line 238–240) emits `log.Printf("WARN: warren-web bound to non-loopback address %q; ...")` exactly once, before launching the HTTP goroutine.

✅ **Middleware order.** `corsMiddleware` wraps the mux at line 113. There is no separate logging or recovery middleware in the current codebase, so the architect's "CORS early but after logging/recovery" criterion is vacuously satisfied for now. (If logging/recovery is added later, it should sit outside the CORS wrap; flagged informationally only.)

✅ **Backwards compatibility.** For requests without an `Origin` header (the existing in-policy path), behavior is byte-identical: middleware sets no headers, `next.ServeHTTP` runs. Existing same-origin clients are unaffected.

**Code quality:**

✅ Readable — clear comment block at lines 142–148 explains the rejection semantics.
✅ No scope creep — only CORS + bind plumbing changed; no handler logic was touched.
✅ Consistent with existing patterns — uses `os.Getenv`, `strings.Split`/`TrimSpace`, `net.SplitHostPort`, `log.Printf`.
✅ Constants are well-named (`EnvBindAddr`, `EnvCORSOrigins`) and exported for cmd-layer use.

### Test Review

**File:** `internal/web/server_test.go` (9 tests, 30+ subtests)

✅ **`TestWebServer_DefaultBind_Loopback`** — pins `DefaultBindAddr == "127.0.0.1:8080"` and verifies it flows through to `srv.httpServer.Addr` when `Config.Addr` is empty.

✅ **`TestWebServer_BindFlagOverride`** — explicit `Config.Addr = "0.0.0.0:9090"` is honored verbatim.

✅ **`TestWebServer_BindEnvVarOverride`** — mirrors the cmd resolution (`os.Getenv(EnvBindAddr)` → flag default) end-to-end and asserts the resolved addr reaches `httpServer.Addr`.

✅ **`TestWebServer_CORS_DefaultLoopbackOnly`** — table-driven allow (loopback) / deny (`evil.example.com`, etc.). Asserts `Access-Control-Allow-Origin` is set for allowed and EMPTY for denied. Also checks `Vary: Origin` for allowed.

✅ **`TestWebServer_CORS_NoOriginHeader`** — same-origin path passes through with status 200, no `Access-Control-Allow-Origin` header. Locks in the "no Origin → no-op" branch.

✅ **`TestWebServer_CORS_EnvVarAddsOrigin`** — verifies (a) defaults still allowed, (b) new env entries allowed, (c) whitespace-trimmed second entry allowed, (d) unrelated origin still denied. All four assertions present.

✅ **`TestWebServer_CORS_PreflightInPolicy`** — OPTIONS with in-policy `Origin` returns 204 + advertises allowed methods.

✅ **`TestWebServer_NonLoopbackBind_LogsWarning`** — captures `log.Writer()` across `Start`/`Stop` for four cases (`127.0.0.1:0`, `localhost:0`, `0.0.0.0:0`, `:0`). Asserts WARN presence/absence matches expectation. This is the right shape — Start is exercised on a real (port-zero) listener so the actual code path runs.

✅ **`TestIsLoopbackBind_TableDriven`** — eight cases covering IPv4, IPv6, hostname, empty host, private LAN, malformed input.

✅ **`TestResolveCORSOrigins`** — four cases: empty env, single entry, whitespace handling, empty-entry skipping.

✅ **`TestOriginHostsForBind`** — nine cases covering default-port short-circuit, non-default loopback expansion (IPv4/hostname/IPv6), non-loopback returns nil, private LAN nil, empty host nil, malformed nil.

✅ **`TestOriginHostsForBind_TracksDefaultConstant`** — pins the 6bcd4da invariant: derives `defaultPort` from `net.SplitHostPort(DefaultBindAddr)` and asserts `originHostsForBind` short-circuits for it. If `DefaultBindAddr` ever changes, this test catches a stale magic-string in the helper.

**Test quality:**

✅ Isolated — each test uses `t.Setenv` and/or `t.TempDir`.
✅ Deterministic — no sleeps, no external network. The Start/Stop WARN test uses `:0` for port-zero binds so it cannot conflict with developer services.
✅ Meaningful assertions — checks exact header values, status codes, and presence/absence rather than "did it produce something".
✅ Direct unit coverage for the parsing helpers (`resolveCORSOrigins`, `originHostsForBind`, `isLoopbackBind`) prevents future drift.

### Item 4 Verdict: **APPROVED**

**What's good:**
- CORS rejection semantics are exactly right: no 403, no structured response, no headers for out-of-policy origins. The browser is the enforcement point, as it should be.
- The 6bcd4da follow-up is a real improvement: derives the default port from `DefaultBindAddr` rather than hardcoding `"8080"`. The new `TestOriginHostsForBind_TracksDefaultConstant` test pins this invariant.
- Non-loopback bind WARN is loud, accurate, and one-shot at startup — easy to grep for in production logs.
- `WARREN_WEB_CORS_ORIGINS` APPENDS rather than replaces, so default loopback access is never accidentally broken by an operator override.
- Test coverage hits every code path including parser edge cases (whitespace, empty entries) and classifier corner cases (IPv6, malformed).

**Suggestions (non-blocking):**

- *MINOR — magic-port consistency in `defaultCORSOrigins`* (`internal/web/server.go:38–39`).
  `defaultCORSOrigins` still has hardcoded `"http://localhost:8080"` / `"http://127.0.0.1:8080"`. These are semantically tied to `DefaultBindAddr`'s port. If `DefaultBindAddr` ever changes (the same scenario commit `6bcd4da` defended against in `originHostsForBind`), `defaultCORSOrigins` would silently drift and the bundled UI would lose its default-port CORS coverage.

  Suggested follow-up (P3, NOT a blocker for this batch): derive `defaultCORSOrigins` from `DefaultBindAddr` at package init via `net.SplitHostPort`, the same pattern 6bcd4da used. Could also add an init-time invariant test that asserts both default origins reference the same port as `DefaultBindAddr`. Cheap; closes the last magic-string drift surface.

- *MINOR — no cmd-level test for `--addr` deprecation WARN.*
  Already acknowledged by the architect as P3/non-blocking; defer to Batch 4 alongside the TUI keynav cmd-level wiring tests. Manual code-read confirms WARN fires once at startup (cmd/warren-web/main.go:48), not per-request.

---

## Cross-Cutting Concerns

### Build & Vet

✅ `go build ./...` — clean (no output).
✅ `go vet ./...` — clean (no output).

### Race Tests (Batch 2 target packages)

```
go test ./internal/core/... ./internal/web/... -race -count=1
ok      github.com/lfu/warren/internal/core     13.982s
ok      github.com/lfu/warren/internal/web      1.221s
```

Re-ran with `-count=3` for the new concurrency/CORS tests — no flakes observed in this environment:
```
go test ./internal/core/... -race -count=3 -run 'TestConnectionPool_(Concurrent|Failed|HostKey).*'
ok      github.com/lfu/warren/internal/core     4.068s

go test ./internal/web/... -race -count=3
ok      github.com/lfu/warren/internal/web      1.727s
```

### Pre-existing `internal/state` flake

✅ Confirmed: `TestStateDetector_Performance_LargeContent` fails with the wall-clock threshold (got ~79ms, expects <50ms) under `-race`. This is a pre-existing flake unrelated to Batch 2 — the failure mode is identical to what's documented in the test report. Not a blocker for Batch 2; should be tracked separately as test-hygiene tech debt (raise the threshold or drop the test from `-race` runs).

### No Regressions to Batch 1 Work

✅ **SSH host-key strict default** — `TestConnectionPool_HostKeyCheckStillHonored` explicitly exercises the early-return-before-dial path. The full `hostkey_test.go` suite still passes.
✅ **Parser** — `internal/parser` tests unchanged and green (verified by clean `go test ./internal/core/... ./internal/parser/...` runs in the broader test report).
✅ **warren.db defaults** — no code paths in `internal/core/defaults.go` or `internal/core/warren.go` touched by Batch 2.

### No Scope Creep

✅ Batch 2 changes are limited to:
- `internal/core/server.go` — pool refactor + adjacent doc updates only.
- `internal/web/server.go` — bind/CORS additions; existing handler bodies untouched.
- `cmd/warren-web/main.go` — flag plumbing + WARN; no other changes.
- Tests: only the two new test files (`server_pool_test.go`, `server_test.go`).

No drive-by refactors. No unrelated cleanups. Exactly what was asked for.

---

## Acceptance Criteria Matrix

| Criterion | Status |
|-----------|--------|
| `go build ./...` clean | ✅ |
| `go vet ./...` clean | ✅ |
| `go test ./internal/core/... ./internal/web/... -race -count=1` clean | ✅ |
| `ssh.Dial` NOT called with `p.mu` held | ✅ server.go:133–137 |
| Same-key dedup via in-flight promise | ✅ server.go:126–130 |
| Failed-dial cleanup (no orphan in `connections`) | ✅ server.go:140–145 |
| Batch 1 host-key flow preserved | ✅ TestConnectionPool_HostKeyCheckStillHonored |
| Default bind = `127.0.0.1:8080` | ✅ server.go:24 |
| `--bind` flag with `--addr` deprecation WARN | ✅ main.go:28–50 |
| `WARREN_WEB_BIND` + `WARREN_WEB_CORS_ORIGINS` env | ✅ server.go:28,33 + main.go:22 |
| Strict default CORS allow-list (loopback only) | ✅ server.go:37–40 |
| Out-of-policy CORS = no headers, NO 403 | ✅ server.go:154–164 |
| Env CORS appends, doesn't replace | ✅ server.go:129–140 |
| `originHostsForBind` returns nil for non-loopback | ✅ server.go:206–225 + tests |
| `originHostsForBind` early-out derived from `DefaultBindAddr` (6bcd4da) | ✅ server.go:214 + TestOriginHostsForBind_TracksDefaultConstant |
| Non-loopback bind WARN at Start | ✅ server.go:238–240 |
| No regressions to Batch 1 | ✅ |
| No scope creep | ✅ |

---

## Summary

This is a clean, complete implementation of the two P1 audit items. Item #3 correctly hoists the SSH dial out of the pool's mutex via a double-checked-locking + per-key promise pattern; synchronization is sound (channel-close establishes happens-before for the shared `connDial` fields) and the failed-dial cleanup path is correct. Item #4 lands a strict-by-default CORS posture with loopback-only bind, an env-driven append-only allow-list, and a non-loopback WARN. The 6bcd4da follow-up removed the last `"8080"` magic string in `originHostsForBind` and added a regression test (`TestOriginHostsForBind_TracksDefaultConstant`) that pins the invariant going forward.

Tests are thorough and the right shape: real `net.Listener`s with atomic accept counters and wall-clock observation for the concurrency tests, direct unit tests for the CORS parsing helpers, and end-to-end Start/Stop coverage for the WARN log. No race-detector failures observed under `-count=3` in this environment.

Two MINOR suggestions, both P3/non-blocking:
1. Derive `defaultCORSOrigins` ports from `DefaultBindAddr` to close the last magic-string drift surface (same pattern 6bcd4da applied to `originHostsForBind`).
2. Add a cmd-level test for the `--addr` deprecation WARN — defer to Batch 4 alongside other cmd-level wiring tests.

Pre-existing `internal/state` perf flake is confirmed unrelated to Batch 2 and should be tracked separately.

### Next Steps

- ✅ Merge `feat/phase2-audit-batch2` and `test/phase2-audit-batch2` to `dev/phase2-audit-remediation`.
- ⏳ File the `defaultCORSOrigins` derivation as a P3 follow-up in the tech-debt log (cheap; closes the consistency loop with 6bcd4da).
- ⏳ Add cmd-level `--addr` deprecation WARN test in Batch 4.
- ⏳ Triage `TestStateDetector_Performance_LargeContent` flake separately (out of Batch 2 scope).

---

**Verdict: APPROVED**

No blockers. Ready to merge.

---

## Addendum: Test commit `7cac304` (2026-06-11)

**Update:** Tester pushed an additional commit on `test/phase2-audit-batch2` adding 4 tests that close two coverage gaps I had noted as MINOR/P3 deferrals. The verdict remains **APPROVED** — these tests strengthen the batch; nothing regressed.

**Updated test branch tip:** `7cac304` (`test/phase2-audit-batch2`)
**New test count:** 17 top-level / 40+ subtests (was 13 / 30+)
**Files touched:**
- `internal/web/server_test.go` (+2 tests, +94 LoC)
- `internal/core/cmd_warren_web_addr_deprecation_test.go` (new, +145 LoC, reuses `buildBinary`/`filterEnv` helpers from `cmd_legacy_db_test.go`)

### New tests reviewed

✅ **`TestOriginHostsForBind`** (`internal/web/server_test.go`) — 9-case table covering:
- Default-port short-circuit via `127.0.0.1:8080` and `localhost:8080`
- Non-default loopback auto-expand via `127.0.0.1:8090`, `localhost:9000`, `[::1]:8090`
- `nil` return for non-loopback `0.0.0.0:8090`, private LAN `192.168.1.10:8090`, empty-host `:8090`, and malformed input

  Notably exercises the threat-model invariant: **non-loopback hosts MUST return nil so the operator is forced to opt in via `WARREN_WEB_CORS_ORIGINS`.** This is the right shape — if `originHostsForBind` ever drifted to auto-expand for `0.0.0.0`, the CORS surface would silently widen, and this test catches that.

✅ **`TestOriginHostsForBind_TracksDefaultConstant`** (`internal/web/server_test.go`) — pins commit `6bcd4da`: derives `defaultPort` from `net.SplitHostPort(DefaultBindAddr)` and asserts `originHostsForBind` short-circuits for it. If `DefaultBindAddr` ever changes, this test catches a stale magic-string regression in the helper. Directly addresses the magic-string concern in my Task #4 brief.

✅ **`TestCmd_WarrenWeb_AddrDeprecationWarning`** (`internal/core/cmd_warren_web_addr_deprecation_test.go`) — full cmd-exec test:
- Builds `warren-web` via `buildBinary` (reused from `cmd_legacy_db_test.go`).
- Execs with `-addr 127.0.0.1:0` (port 0 = kernel-picked, no test-host conflicts) and a `-db` path under a `tmpHome` to prevent legacy-DB refuse-to-start from firing.
- Filters `HOME` / `WARREN_WEB_BIND` from inherited env and pins them to test-local values.
- Polls stderr until the WARN substring appears (50ms poll, 2s ceiling); asserts:
  - `"--addr is deprecated"` substring present
  - `"WARN"` prefix present
  - The echoed `127.0.0.1:0` value present (verifies the WARN actually carries the operator's input, not a stub message)
- Kills the process in `t.Cleanup`.

  **Race-safety detail (good):** the test introduces a `safeBuffer` (mutex-wrapped `bytes.Buffer`) for `cmd.Stderr` because `exec.Cmd` writes to it from an internal goroutine while the test polls reads. This is exactly the right fix; without it, `-race` would flag the buffer access. Looking at the implementation it's correct — `Write` and `String` both take `b.mu`. Subtle but important; if the Tester hadn't added this, the test would be a `-race` flake-in-waiting.

✅ **`TestCmd_WarrenWeb_BindFlagSilencesAddrDeprecation`** (`internal/core/cmd_warren_web_addr_deprecation_test.go`) — locks in the other half of the contract: when BOTH `-addr` and `-bind` are passed, the WARN must be silenced because the cmd's gate is `addrSet && !bindSet`. Pins the precise branch logic from `cmd/warren-web/main.go:47` so a future "let's always warn when --addr is provided" refactor can't silently regress.

  500ms wall-clock wait before reading stderr is fine here — the WARN is inline and synchronous; if it fires it'll be in the buffer well before 500ms. The test could theoretically be flaky on a heavily loaded CI runner if 500ms isn't enough for the binary to start AND log, but the `-count=3 -race` run here completed in ~7.3s for both new cmd tests combined with no failures.

### Build / vet / test re-verification at tip

```
go build ./... → clean
go vet ./... → clean
go test ./internal/core/... ./internal/web/... -race -count=1
  ok  github.com/lfu/warren/internal/core  16.015s
  ok  github.com/lfu/warren/internal/web    1.231s

go test ./internal/core/... -race -count=3 -run 'TestCmd_WarrenWeb_(AddrDeprecation|BindFlagSilences).*'
  ok  github.com/lfu/warren/internal/core   7.296s
```

No flakes observed across 3 runs of the new exec-based tests.

### Impact on my previous MINOR suggestions

- **Suggestion #2 from my original review** ("no cmd-level test for `--addr` deprecation WARN — defer to Batch 4") is now **CLOSED** by `TestCmd_WarrenWeb_AddrDeprecationWarning` + `TestCmd_WarrenWeb_BindFlagSilencesAddrDeprecation`.
- **Suggestion #1** (derive `defaultCORSOrigins` from `DefaultBindAddr` to fully close the magic-string drift surface) is **NOT** addressed by this commit and stands. Still P3/non-blocking.

### Final updated verdict

**APPROVED.** Test coverage on this batch is now meaningfully stronger than what I originally certified. The Tester correctly identified and proactively fixed the `safeBuffer` race-safety concern that the exec-based test pattern introduces — that's a sign of a careful test, not a mechanical one.

Recommend merging both branches as-is.
