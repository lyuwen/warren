# Testing Report — Phase 2 Audit Batch 2

**Branch:** `test/phase2-audit-batch2`
**Implementer commit:** `73a6696` on `feat/phase2-audit-batch2`
**Date:** 2026-06-11
**Tester:** tester (dev-team-warren-v2)

## Scope

Tests for Phase 2 audit Batch 2 — both P1 items:

- **Item #3**: `ConnectionPool.Get` no longer serializes across SSH dial.
- **Item #4**: REST API enforces a CORS allow-list and binds to loopback by default.

Production code is the Implementer's responsibility — only test files were added on this branch.

## Files Added

- `internal/core/server_pool_test.go` — 4 tests for Item #3.
- `internal/web/server_test.go` — 9 tests (5 top-level functions, 4 with subtests) for Item #4.

No production files modified. No tests deleted or renamed.

## Test Design

### Item #3 — ConnectionPool concurrency

The Implementer chose the double-checked-locking + per-key promise (`connDial`) pattern and did **not** expose a test-only dialer hook. Tests therefore drive the real dial path through a `slowAcceptListener` helper: a `net.Listener` on `127.0.0.1:0` that accepts incoming TCP connections, holds them for a configurable duration to simulate a slow dial+handshake, then closes them. An atomic accept counter is the ground truth for "how many dials happened." The SSH handshake fails (the listener does not speak SSH); tests assert on timing and accept counts, not on `Get`'s return value.

All concurrency tests set `WARREN_SSH_INSECURE_HOSTKEY=1` so `hostKeyCallback` does not refuse before dial on machines without `~/.ssh/known_hosts`.

| Test | What it locks in |
| --- | --- |
| `TestConnectionPool_ConcurrentDifferentServers_NoSerialization` | Two concurrent `Get` calls against different servers complete in ~1× the listener hold (200ms), not 2×. Before the fix, the pool's mutex serialized this and the test would have measured ~400ms. Asserts `elapsed < 1.5 × hold`. |
| `TestConnectionPool_ConcurrentSameServer_SingleDial` | 16 concurrent `Get(sameServer)` calls released on a shared barrier result in exactly **1** TCP accept. Dedup invariant preserved by the per-key `connDial` ready channel. |
| `TestConnectionPool_FailedDialDoesNotBlockOtherServers` | A dead server (`203.0.113.1`, RFC 5737 TEST-NET-3) with a 600ms pool timeout does not stall a concurrent `Get(liveServer)` — the live call returns well before the dead-server timeout would have elapsed. |
| `TestConnectionPool_HostKeyCheckStillHonored` | Strict-default host-key flow from Batch 1 still fires after the refactor: with `~/.ssh/known_hosts` missing and `WARREN_SSH_INSECURE_HOSTKEY` unset, `Get` returns `ErrKnownHostsMissing` and the test listener's accept count stays at **0** (refusal happens before the network call). Uses `ssh-keygen` to seed a real private key so the auth-method guard does not fire first; skips if `ssh-keygen` is unavailable. |

### Item #4 — Web CORS + bind defaults

Tests target the public package surface (`DefaultBindAddr`, `EnvBindAddr`, `EnvCORSOrigins`, `corsMiddleware`, `resolveCORSOrigins`, `isLoopbackBind`, and `originHostsForBind` transitively via `NewServer`). They do not stand up real network listeners except in the WARN-log test (which uses port 0 so the OS assigns a free port and the listener is `Stop`ed immediately).

| Test | What it locks in |
| --- | --- |
| `TestWebServer_DefaultBind_Loopback` | Empty `Config.Addr` resolves to `127.0.0.1:8080`; `srv.addr` and `httpServer.Addr` both reflect it. Hard-coded constant check guards against an accidental default change. |
| `TestWebServer_BindFlagOverride` | `Config.Addr = "0.0.0.0:9090"` is honored verbatim end-to-end. |
| `TestWebServer_BindEnvVarOverride` | `WARREN_WEB_BIND` resolution (mirroring the `cmd/warren-web` shape) plus end-to-end `NewServer` adoption. |
| `TestWebServer_CORS_DefaultLoopbackOnly` | Subtests over allowed (`http://localhost:8080`, `http://127.0.0.1:8080`) and denied (`https://evil.example.com`, `http://attacker.test`, `https://example.com`) origins. Allowed origins get `Access-Control-Allow-Origin` + `Vary: Origin`; denied origins get neither. |
| `TestWebServer_CORS_NoOriginHeader` | Same-origin / non-browser callers (no `Origin` header) pass through cleanly; no CORS headers are tagged onto every response. |
| `TestWebServer_CORS_EnvVarAddsOrigin` | `WARREN_WEB_CORS_ORIGINS=https://example.com, https://other.example.com` (with whitespace) appends both to the loopback default; the default loopback origin still works, and an unrelated `https://evil.example.com` is still denied. |
| `TestWebServer_CORS_PreflightInPolicy` | In-policy `OPTIONS` preflight short-circuits with 204 and advertises allowed methods. |
| `TestWebServer_NonLoopbackBind_LogsWarning` | 4 subtests: `127.0.0.1:0` and `localhost:0` produce no WARN; `0.0.0.0:0` and `:0` produce a `WARN ... non-loopback ...` log line. Captures via `log.SetOutput`. Server is `Stop`ed immediately so no listener is left running. |
| `TestIsLoopbackBind_TableDriven` | 8 cases pin the loopback classifier: `127.0.0.1`, `localhost`, `[::1]` → true; `0.0.0.0`, `:8080`, `192.168.1.10`, malformed input → false. Locks in the predicate the WARN gate depends on. |
| `TestResolveCORSOrigins` | 4 cases for env parsing: empty, single, multiple-with-whitespace, empty-entries-skipped. Defaults are always preserved (append, not replace). |

## Test Run

```
$ go test ./internal/core/ ./internal/web/ -race -count=1
ok  	github.com/lfu/warren/internal/core	14.253s
ok  	github.com/lfu/warren/internal/web	1.223s
```

Batch 2 tests in isolation, race detector on:

```
$ go test ./internal/core/ ./internal/web/ \
    -run "TestConnectionPool_(ConcurrentDifferent|ConcurrentSame|FailedDial|HostKeyCheckStill)|TestWebServer|TestResolveCORSOrigins|TestIsLoopbackBind" \
    -race -count=1 -v
... 17 PASS, 0 FAIL ...
ok  	github.com/lfu/warren/internal/core	1.997s
ok  	github.com/lfu/warren/internal/web	1.106s
```

### Summary
- **Total tests added:** 13 top-level (`TestConnectionPool_*` × 4, `TestWebServer_*` × 7, `TestIsLoopbackBind_TableDriven`, `TestResolveCORSOrigins`).
- **Subtests:** 5 allowed/denied CORS origin cases, 4 WARN-log bind cases, 8 loopback-classifier cases, 4 env-parser cases.
- **Effective assertion count:** 30+ subtest invocations.
- **Passed:** all.
- **Failed:** 0.
- **Skipped:** `TestConnectionPool_HostKeyCheckStillHonored` will skip if `ssh-keygen` is unavailable (CI hardening — matches the existing `TestConnectionPool_Get_EarlyReturnsBeforeDialWhenHostKeyUnavailable` in `hostkey_test.go`).
- **Race detector:** clean across `./internal/core/...` and `./internal/web/...`.

### Coverage

- `internal/core/server.go`:
  - `ConnectionPool.Get` — covered for: dead-different-server isolation, same-server dedup, host-key short-circuit before dial.
  - `ConnectionPool.dial` — exercised indirectly via the listener-based tests.
  - `connDial` ready-channel synchronization — exercised by the 16-goroutine dedup test (race-clean).
- `internal/web/server.go`:
  - `DefaultBindAddr`, `EnvBindAddr`, `EnvCORSOrigins` — covered.
  - `corsMiddleware` — covered for: no-Origin pass-through, allowed origin (loopback + env-supplied), denied origin, in-policy preflight short-circuit.
  - `resolveCORSOrigins` — covered.
  - `isLoopbackBind` — covered.
  - `originHostsForBind` — covered transitively via `NewServer`.
  - `Server.Start` WARN log — covered for loopback (no-WARN) and non-loopback (WARN-emitted).

## Coverage Gaps / Suggestions

These are not regressions for Batch 2 but are worth tracking as future work:

1. **`Server.Start` HTTP error path:** the goroutine that calls `ListenAndServe` only logs to stdout via `fmt.Printf` if a non-`ErrServerClosed` error fires. There is no integration test that exercises a real bind failure (e.g. port in use). Low priority; the path is straightforward.
2. **CORS allow-list case-sensitivity:** the middleware compares Origins exactly. Browsers always send Origins in canonical lowercase, but a future proxy stripping/rewriting Origin would slip past silently. Not a Batch 2 concern.
3. **`originHostsForBind` non-default loopback port:** I cover it transitively via `NewServer`, but a focused unit test (`originHostsForBind("127.0.0.1:9090")` returns the auto-expanded `localhost:9090` + `127.0.0.1:9090` entries) would lock in the helper directly. Optional; can add if Reviewer wants explicit coverage.
4. **WARN log is one-shot per process:** the implementer's WARN fires inline from `Start()`, so a process that calls `Start` once gets one WARN. If `Start` is ever called multiple times (it currently is not), the WARN would repeat. No dedup test needed today.
5. **`--addr` deprecation WARN:** the cmd-layer deprecation log lives in `cmd/warren-web/main.go` and has no test package. Out of scope for Batch 2 since the audit's acceptance criteria focused on the `--bind` / env-var path; if Reviewer wants cmd-level coverage, a `cmd_warren_web_test.go` in the style of `cmd_legacy_db_test.go` would be the right shape.

## Concerns / Observations

- **No test-only dialer hook on `ConnectionPool`.** The Implementer's pattern works for these tests, but if Phase 3 work needs more granular dial-step assertions (e.g. "the handshake call happened with these config bytes"), a dependency-injectable `dialFunc` would be cleaner than the listener-based approach. I left a note in the test file header so a future maintainer doesn't re-invent the same harness.
- **`TestStateDetector_Performance_LargeContent` flakes under `-race`.** Pre-existing, unrelated to Batch 2 — implementer's commit message already calls it out. Worth filing as a separate technical-debt entry so it stops surfacing in `-race` runs and masking real regressions. Repro:
  ```
  go test ./internal/state/ -race -count=1 -run TestStateDetector_Performance_LargeContent
  ```
- **`TestConnectionPool_FailedDialDoesNotBlockOtherServers` uses real TEST-NET-3 routing.** RFC 5737 reserves `203.0.113.0/24` for documentation/examples, and we rely on the dial timing out at the pool's `Dialer.Timeout` (600ms in the test). On a network with aggressive ICMP unreachables this may return faster than 600ms; the test only asserts the live server finishes before the dead server's timeout, so a faster dead-server failure still passes — but it relies on the failure happening. If a future CI environment swallows the dead-server dial and returns success, the test would be silently wrong. Low risk on standard CI runners.

## Ready for Review

Tests committed on `test/phase2-audit-batch2`. All 13 new top-level tests pass with `-race` alongside the full Batch 1 + Batch 2 implementation. No production code modified.
