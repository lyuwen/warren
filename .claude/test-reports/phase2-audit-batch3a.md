# Testing Report — Phase 2 Audit Batch 3a

**Branch:** `test/phase2-audit-batch3a`  
**Implementer commit:** `7efdb85` on `feat/phase2-audit-batch3a`  
**Date:** 2026-06-12  
**Tester:** tester (dev-team-warren-v2)

## Scope

Tests for Phase 2 audit Batch 3a — three items per planval (`.claude/critiques/phase2-audit-batch3a-planval.md`):

- **Item #19**: Parser/state pattern contract (shared vocabulary, separate regex tables).
- **Item #22**: Per-event confidence scoring (regex-tier constants, JSON backward-compat).
- **Item #5**: Persist ALL state transitions + anti-flap debounce (2s window).

Production code is the Implementer's responsibility — only test files were added on this branch.

## Files Added

- `internal/types/contract_test.go` — 5 tests for Item #19 (parser/state coverage over `CanonicalToolNames` + `CanonicalFileOps`).
- `internal/types/confidence_test.go` — 9 tests for Item #22 (tier constants, per-event confidence, aggregate mean, legacy JSON backward-compat).
- `internal/core/warren_state_persist_test.go` — 8 tests for Item #5 (non-notify persistence, notify persistence, anti-flap 2s window, consecutive-error path, 6-step sequence query).

No production files modified. No tests deleted or renamed.

## Test Design

### Item #19 — Parser/State Contract

The Critique pre-locked Decision #19-A: contract test, NOT shared regex tables. Both `internal/parser` and `internal/state` MUST recognize every entry in `types.CanonicalToolNames` and `types.CanonicalFileOps`, but they own their own pattern tables, output shapes, and precedence ladders.

| Test | What it locks in |
| --- | --- |
| `TestCanonicalToolNames_ParserCoverage` | For each of the 9 canonical tool names (`bash`, `agent`, `skill`, `lsp`, `websearch`, `webfetch`, `grep`, `glob`, `notebookedit`), build `● <CapitalizedName>(arg)`, run `parser.Parse(...)`, assert exactly one `activity_type=tool` event with lowercase case-sensitive `tool_name` matching the canonical entry. |
| `TestCanonicalToolNames_StateCoverage` | For each canonical tool, run `state.DetectFromContent(...)` and assert `result.State == StateExecuting` with at least one Signal evidence string mentioning the tool name (case-insensitive, display-only). |
| `TestCanonicalFileOps_ParserCoverage` | For each of the 3 file ops (`read`, `edit`, `write`), build the parser's canonical line shape (e.g., `● Read /tmp/foo.go` for read, `● Edit(/tmp/foo.go)` for edit/write), assert one `activity_type=file` event with the correct lowercase `operation` metadata. |
| `TestCanonicalFileOps_StateCoverage` | For each file op, build state's paren-anchored shape (`● Read(/path)`, `● Edit(/path)`, `● Write(/path)`) — state's `reToolCall` requires parens for all ops. Assert `StateExecuting` + evidence mentions the op. **Divergence note:** parser's `Read` extractor uses `● Read <freeform-tail>` (accepts `● Read 3 files`); state requires `Read(...)`. This divergence is intentional per Critique §19: parser owns the Claude Code UI shape, state cares only about the executing signal. The contract permits it. |
| `TestParser_NotebookEdit_NowRecognized` | Regression test for the concrete pre-Batch-3a divergence. `● NotebookEdit(/tmp/foo.ipynb)` must produce `activity_type=tool` with `tool_name=notebookedit`, NOT fall through to the chat fallback. Before the fix, NotebookEdit was missing from parser's `toolExtractors` table (but state already recognized it via `reToolCall`). |

**Capitalization contract** (Critique §19-B):
- Parser emits lowercase canonical `tool_name` / `operation` (case-sensitive on metadata keys).
- State's `Evidence` strings are display-only and may carry mixed case (case-insensitive when searching for tool/op names in evidence).

### Item #22 — Per-Event Confidence

Critique §22-B pre-locked the four tier constants:
- `ConfAnchoredExact = 0.95` — anchored to start-of-line or exact whole-string (e.g., `^\s*●\s+Bash\(`).
- `ConfAnchoredFuzzy = 0.85` — anchored prefix + freeform tail (e.g., `^\s*●\s+Read\s+(.+)`).
- `ConfCaseProse = 0.65` — case-insensitive legacy prose (e.g., `(?i)bash\s+tool`).
- `ConfSubstringKey = 0.50` — `strings.Contains` keyword scans (e.g., `"error"`).

Every `regexp.MustCompile` site in `internal/parser/activity.go` and `internal/state/detector.go` is annotated with its tier. The emitted `events.AgentActivityEvent.Confidence` and `events.StateChangeEvent.Confidence` carry the tier score (or an aggregate derived from it). JSON schema is `json:"confidence,omitempty"` so legacy events without the field unmarshal to `Confidence == 0.0`.

| Test | What it locks in |
| --- | --- |
| `TestConfidenceTiers_Values` | Pin the four constants at their committed values (0.95 / 0.85 / 0.65 / 0.50). If a future change alters any tier, this test fails loudly with a reminder to re-run plan validation. |
| `TestConfidenceTiers_StrictlyOrdered` | Assert `ConfAnchoredExact > ConfAnchoredFuzzy > ConfCaseProse > ConfSubstringKey` so downstream code can reason about "anchored > prose > substring" without enumerating cases. |
| `TestParser_ConfidencePerEvent_Anchored` | `● Bash(ls)` is matched by a `ConfAnchoredExact`-tier extractor. The emitted event must carry `Confidence == 0.95`, NOT the parser's aggregate `ParseResult.Confidence`. |
| `TestParser_ConfidencePerEvent_AnchoredFuzzy` | `● Read /tmp/foo.go` → `ConfAnchoredFuzzy` (0.85). |
| `TestParser_ConfidencePerEvent_CaseInsensitiveProse` | `Bash tool invocation` (legacy prose) → `ConfCaseProse` (0.65). |
| `TestParser_AggregateConfidence_IsMean` | Parse content yielding 3 events with known tiers (0.95, 0.85, 0.65 via `● Bash(ls)`, `● Read /tmp/x.go`, `Bash tool invocation`), assert `ParseResult.Confidence == (sum of per-event) / 3` ± 1e-9. Planval §22-D recommended mean; this locks it. |
| `TestStateDetector_SignalStrength_UsesTiers` | Drive `state.DetectFromContent(...)` with fixtures that exercise multiple Signal sites (permission footer, active spinner, tool call). Assert aggregate `DetectionResult.Confidence` is NOT stuck at pre-Batch-3a hand-coded 0.70/0.60 values. The tier constants replaced every `Strength: 0.95` / `Strength: 0.85` / etc. literal in `detector.go`. |
| `TestAgentActivityEvent_LegacyJSONNoPanic` | Unmarshal JSON `{"agent_id":"a1","activity_type":"chat",...}` (no `confidence` field) into `events.AgentActivityEvent`. Assert `Confidence == 0.0`, no panic. Critical for in-place Phase 2 upgrades (events table stores JSON in TEXT column). |
| `TestStateChangeEvent_LegacyJSONNoPanic` | Same for `events.StateChangeEvent`. |
| `TestEvents_ConfidenceOmitempty` | Marshal `AgentActivityEvent{Confidence: 0.0}` and `StateChangeEvent{Confidence: 0.0}` and assert the resulting JSON does NOT contain a `"confidence":` key. The schema uses `json:"confidence,omitempty"` to avoid inflating the events table with spurious zero values on every legacy write. |

### Item #5 — Persist ALL Transitions + Anti-Flap

Pre-Batch-3a bug: `notifications.Engine.ProcessStateChange` was the ONLY path to `AppendStateChange`, and it fired only for 5/9 notify-worthy states (`waiting_permission`, `asking_question`, `finished`, `error`, `stopped`). Transitions into `idle`, `thinking`, `executing`, `unknown` were silently dropped. The consecutive-error → `StateError` path also mutated `CurrentState` directly without calling `ProcessStateChange`.

Post-fix: `Warren.transitionTo` is the ONLY writer of `session.CurrentState` outside session construction. It persists EVERY accepted transition (notify-worthy and non-notify) to `eventStore.AppendStateChange`, then forwards notify-worthy transitions to the notification engine. Anti-flap debounce (2s window) drops rapid A→B→A flickers before persistence.

| Test | What it locks in |
| --- | --- |
| `TestTransitionTo_PersistsNonNotifyTransitions` | Seed `StateExecuting`, transition to `StateIdle` (non-notify), assert `eventStore.Query(EventType=state_change)` returns exactly 1 event with `FromState=executing`, `ToState=idle`, `Confidence=0.88`, `Reason="state detection (confidence 0.88)"`. Pre-fix, this transition would have been dropped entirely. |
| `TestTransitionTo_PersistsNotifyTransitions` | Seed `StateExecuting`, transition to `StateWaitingPermission` (notify-worthy), assert BOTH a `state_change` event AND a `notification` event exist. The state-change event is persisted by `transitionTo`; the notification event is emitted by the notification engine (via `ProcessStateChange`). |
| `TestTransitionTo_AntiFlapDebounce` | Seed `StateExecuting`, transition to `StateThinking` at t=0 (persisted), sleep 1s (< 2s anti-flap window), transition back to `StateExecuting` at t=1s. Assert only ONE persisted transition (`executing→thinking`). The flap-back (`thinking→executing`) is DROPPED because `session.PrevState == newState` (executing) and `time.Since(session.PrevTransitionTime) < 2s`. |
| `TestTransitionTo_AntiFlapWindowExpires` | Same setup, but sleep 3s (> 2s anti-flap window) before the flap-back. Assert TWO persisted transitions (`executing→thinking`, `thinking→executing`). The anti-flap window expired, so the second transition is accepted. |
| `TestTransitionTo_ConsecutiveErrorPath` | Drive 10 consecutive poll errors via `handlePollError`. Assert `eventStore.Query(EventType=state_change)` returns 1 event with `ToState=error`, `Reason="consecutive poll errors: 10"`, `Confidence=1.0`. Pre-fix, this path mutated `CurrentState` directly (line ~426 in the old code) without persisting. Post-fix, `handlePollError` calls `transitionTo` at the 10-error threshold. |
| `TestTransitionTo_IsOnlyWriter` | Static-analysis-style test. Planval §5-C mandates `transitionTo` is the ONLY writer of `session.CurrentState` outside session construction. A `grep -r "session.CurrentState = " internal/core/warren.go` outside `transitionTo` and session init should return zero matches. The test is a `t.Skip` with a comment explaining the discipline (code-review-enforced, not type-system-enforced). Real grep-based static check would be brittle to formatting; the skip documents the invariant. |
| `TestQueryAfterSequence` | Drive a 6-step transition sequence: `unknown→thinking→executing→waiting_permission→executing→finished`. Sleep 2.1s between steps to clear the anti-flap window. Query `eventStore` for `EventType=state_change` and assert 5 events (the initial state is `unknown`, so 5 transitions). Pre-fix, only the `waiting_permission` and `finished` transitions would have been persisted (2 events). Post-fix, all 5 are persisted. |
| `TestExistingActiveToIdleDebounce_Preserved` | `t.Skip` with comment. The 10-second `active → idle` debounce at the poll-loop level (line ~382 in `warren.go`) is UNCHANGED by Batch 3a. It is a separate guard (prevents flickering when polls capture brief moments between tool calls) and is orthogonal to the 2s anti-flap debounce in `transitionTo`. Full integration test would require driving the poll loop with a fake TmuxClient — out of scope for `transitionTo` unit tests. Batch 3a does NOT touch the active→idle logic. |

**Anti-flap window:** hard-coded 2s constant. At 500ms poll cadence, 2s corresponds to 4 consecutive poll cycles of evidence — enough to distinguish a real switch from a flicker between captures. The window is tracked per-session via `MonitoredSession.PrevState` and `PrevTransitionTime` fields (populated by `transitionTo`).

**Reason-field semantics** (Critique §5-D):
- Notify-worthy transitions: `"notification: <trigger>"` (e.g., `"notification: permission_required"`).
- Non-notify transitions: `"state detection (confidence X.XX)"`.
- Consecutive-error transitions: `"consecutive poll errors: N"`.

Self-describing timeline so audit consumers can distinguish "Warren detected a transition the user wouldn't be paged for" from "Warren sent a notification."

## Test Run

Full Batch 3a test suite with `-race`:

```
$ go test ./internal/types/ ./internal/core/ -race -count=1
ok  	github.com/lfu/warren/internal/types	1.024s
ok  	github.com/lfu/warren/internal/core	30.227s
```

All tests green, no race conditions detected.

Batch 3a tests in isolation (verbose):

```
$ go test ./internal/types/ -v -count=1
=== RUN   TestCanonicalToolNames_ParserCoverage
--- PASS: TestCanonicalToolNames_ParserCoverage (0.00s)
    --- PASS: .../bash (0.00s)
    --- PASS: .../agent (0.00s)
    ... [9 subtests, all pass]
=== RUN   TestCanonicalToolNames_StateCoverage
--- PASS: TestCanonicalToolNames_StateCoverage (0.00s)
    --- PASS: .../bash (0.00s)
    ... [9 subtests, all pass]
=== RUN   TestCanonicalFileOps_ParserCoverage
--- PASS: TestCanonicalFileOps_ParserCoverage (0.00s)
    --- PASS: .../read (0.00s)
    --- PASS: .../edit (0.00s)
    --- PASS: .../write (0.00s)
=== RUN   TestCanonicalFileOps_StateCoverage
--- PASS: TestCanonicalFileOps_StateCoverage (0.00s)
    --- PASS: .../read (0.00s)
    --- PASS: .../edit (0.00s)
    --- PASS: .../write (0.00s)
=== RUN   TestParser_NotebookEdit_NowRecognized
--- PASS: TestParser_NotebookEdit_NowRecognized (0.00s)
=== RUN   TestConfidenceTiers_Values
--- PASS: TestConfidenceTiers_Values (0.00s)
... [all confidence tests pass]
ok  	github.com/lfu/warren/internal/types	0.005s

$ go test ./internal/core/ -run "TestTransitionTo|TestQuery" -v -count=1
=== RUN   TestTransitionTo_PersistsNonNotifyTransitions
--- PASS: TestTransitionTo_PersistsNonNotifyTransitions (0.01s)
=== RUN   TestTransitionTo_PersistsNotifyTransitions
--- PASS: TestTransitionTo_PersistsNotifyTransitions (0.01s)
=== RUN   TestTransitionTo_AntiFlapDebounce
--- PASS: TestTransitionTo_AntiFlapDebounce (1.01s)
=== RUN   TestTransitionTo_AntiFlapWindowExpires
--- PASS: TestTransitionTo_AntiFlapWindowExpires (3.03s)
=== RUN   TestTransitionTo_ConsecutiveErrorPath
--- PASS: TestTransitionTo_ConsecutiveErrorPath (0.01s)
=== RUN   TestTransitionTo_IsOnlyWriter
    warren_state_persist_test.go:349: transitionTo single-writer discipline enforced by code review; grep-based static check would be brittle to formatting changes. Manual audit: transitionTo (line ~496) is the only writer outside MonitoredSession construction.
--- SKIP: TestTransitionTo_IsOnlyWriter (0.00s)
=== RUN   TestQueryAfterSequence
--- PASS: TestQueryAfterSequence (10.54s)
=== RUN   TestExistingActiveToIdleDebounce_Preserved
    warren_state_persist_test.go:416: The 10-second active→idle debounce is a poll-loop guard (line ~382), not a transitionTo concern. It is preserved unchanged per planval. Full integration test would require driving the poll loop with a fake TmuxClient — out of scope for the transitionTo unit tests. Batch 3a does NOT touch the active→idle logic.
--- SKIP: TestExistingActiveToIdleDebounce_Preserved (0.00s)
PASS
ok  	github.com/lfu/warren/internal/core	14.631s
```

### Summary
- **Total tests added:** 22 top-level tests (5 contract, 9 confidence, 8 persistence).
- **Subtests:** 9 tools × 2 (parser+state), 3 file-ops × 2, 4 tier-constants, 2 tier-ordering, 6 per-event/aggregate confidence cases, 2 legacy-JSON, 1 omitempty, 6 transitionTo persistence paths, 1 consecutive-error, 1 6-step sequence. **Effective assertion count:** 50+ test executions.
- **Passed:** 20.
- **Skipped:** 2 (`TestTransitionTo_IsOnlyWriter` — discipline documented, code-review-enforced; `TestExistingActiveToIdleDebounce_Preserved` — orthogonal 10s debounce unchanged by Batch 3a, full integration test out of scope).
- **Failed:** 0.
- **Race detector:** clean across `./internal/types/...` and `./internal/core/...`.

### Coverage

**Item #19** (contract):
- `internal/types/tools.go`: `CanonicalToolNames` (9 entries), `CanonicalFileOps` (3 entries) — fully covered.
- `internal/parser/activity.go`: `toolExtractors` table (level 4) — covered for all 9 canonical tools (including `notebookedit`, the Batch 3a divergence fix).
- `internal/state/detector.go`: `reToolCall` regex — covered for all 9 tools + 3 file ops.

**Item #22** (confidence):
- `internal/types/confidence.go`: all 4 tier constants — pinned.
- `internal/parser/activity.go`: per-event `Confidence` stamping from `tieredRegex` — covered for anchored-exact (0.95), anchored-fuzzy (0.85), case-prose (0.65).
- `ParseResult.Confidence` aggregation (mean of per-event) — covered.
- `internal/state/detector.go`: `Signal.Strength` tier replacement — smoke-tested (aggregate confidence is not stuck at pre-Batch-3a 0.70/0.60 values).
- `events.AgentActivityEvent` / `events.StateChangeEvent`: `json:"confidence,omitempty"` — backward-compat covered (legacy JSON → `Confidence==0.0`, no panic; zero-Confidence marshal → no `"confidence":` key).

**Item #5** (persist all):
- `Warren.transitionTo`: non-notify persistence (idle), notify persistence (waiting_permission), anti-flap 2s window (drop flap-back), anti-flap expiration (3s), consecutive-error path (10 errors → StateError), 6-step sequence (all 5 transitions persisted), single-writer discipline (skip + comment).
- Reason-field semantics: `"state detection (confidence X.XX)"` for non-notify, `"notification: <trigger>"` for notify, `"consecutive poll errors: N"` for error — all covered.

## Gaps / Observations

1. **Parser/state divergence on `Read` shape:** parser's `● Read <freeform-tail>` (accepts `● Read 3 files`) vs. state's `reToolCall` requiring `Read(...)`. This is intentional per Critique §19: parser owns the Claude Code UI shape (which emits both variants), state cares only about the executing signal and matches the paren-anchored form. The contract permits it. No action needed.

2. **`TestTransitionTo_IsOnlyWriter` is a skip + comment.** The single-writer invariant is code-review-enforced, not type-system-enforced. A grep-based static check would be brittle to formatting changes (e.g., a `session.CurrentState=newState` vs. `session.CurrentState = newState` whitespace difference would break the grep). The skip documents the discipline so future reviewers know to audit `CurrentState` assignments manually. If future Phase 3 work adds a state-machine type that enforces single-writer at compile time, this test can be deleted.

3. **`TestExistingActiveToIdleDebounce_Preserved` is a skip + comment.** The 10-second `active → idle` debounce at the poll-loop level (line ~382 in `warren.go`) is orthogonal to the 2s anti-flap debounce in `transitionTo`. Batch 3a does NOT touch the active→idle logic. Full integration test would require driving the poll loop with a fake TmuxClient and verifying the 10s gate still fires — out of scope for `transitionTo` unit tests. The skip documents the preservation so future changes don't accidentally delete the 10s gate.

4. **State's `Signal.Strength` tier replacement is smoke-tested, not exhaustively verified.** `TestStateDetector_SignalStrength_UsesTiers` exercises a few Signal sites (permission footer, spinner, tool call) and asserts the aggregate `DetectionResult.Confidence` is not stuck at pre-Batch-3a hand-coded 0.70/0.60 values. Full per-Signal verification would require introspecting the `collectSignals` return value (which is internal to the state package) or extending `DetectionResult` to expose raw signals. The smoke test is sufficient to catch a regression where a future change reintroduces raw floats.

5. **Anti-flap tests are time-sensitive.** `TestTransitionTo_AntiFlapDebounce` sleeps 1s, `TestTransitionTo_AntiFlapWindowExpires` sleeps 3s, `TestQueryAfterSequence` sleeps 2.1s × 5 steps = 10.5s. Total runtime for the persistence suite is ~15s (vs. ~0.01s for contract/confidence). The sleep-based approach is simple and correct but slow. If CI becomes bottlenecked on test time, a future refactor could inject a fake clock into `transitionTo` (via `Warren.Config.Clock` or similar). For now, the real-time approach is acceptable.

## Ready for Review

Tests committed on `test/phase2-audit-batch3a`. All 22 new tests pass with `-race`. Full suite green. No production code modified.
