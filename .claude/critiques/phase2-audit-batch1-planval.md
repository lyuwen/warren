# Plan Validation: Phase 2 Audit Remediation — Batch 1

**Mode:** Plan Validation (no code reviewed beyond verification reads)
**Branch:** `dev/phase2-audit-remediation` (no code yet)
**Inputs:** Architect's Task 2 brief; `pages/phase2-audit-report.html`; `docs/reviews/phase2-fixes-critique.md`; `docs/phase2-technical-debt.md`; current state of `internal/core/server.go`, `internal/parser/activity.go`, `cmd/warren-tui/main.go`, `cmd/warren-web/main.go`, `internal/core/warren.go`.

---

## Overall Verdict: **NEEDS REVISION** (across all three items)

None of the three plans is fundamentally wrong, but each has a substantive flaw that must be settled before code is written. **Most importantly: Item 2 Option A as proposed is technically impossible in Go's `regexp` package, which means the architect's "three options" set is really two options. The team must not pick A.** Items 1 and 3 have UX and architectural decisions that I want firmed up — not glossed over with "we'll figure it out in code."

I am not escalating to user. All three are resolvable in Architect-level revision.

---

## Item 1: SSH Host Key Fallback — NEEDS REVISION

### Plan adherence

The proposed fix (env-var gate + error-by-default) **does** address the audit finding and the previous critique's deferred P0 item. Mechanically, this resolves "silent MITM-vulnerable fallback."

### First-principles challenges

**1. Why an env var instead of per-server config? (BIGGEST CHALLENGE)**

`servers.yaml` already exists and carries per-server `SSHOptions map[string]string` (confirmed in `internal/core/server.go:31`). A binary global env var says "every SSH connection is insecure" or "every SSH connection is secure." That's the wrong axis — in the real world, an operator might want:

- Production servers: strict known_hosts
- A dev VM I just spun up: skip verification
- A CI runner: skip verification

An env var gives operators no way to express that. They'll set the env var globally because of the dev VM, and inherit MITM exposure on the production server in the same Warren process.

The **correct long-term design** is a per-server field, e.g. `insecure_host_key: true` in `servers.yaml`, defaulted to `false`. Env var is a global escape hatch only.

**Recommendation:** Implement the env var (it's the minimum-viable fix and lets us ship Phase 3). But the plan must explicitly:
- Document `insecure_host_key` per-server as the proper long-term design
- File a P1 ticket in `docs/phase2-technical-debt.md` for the per-server config field
- Make the env var **fallback** behavior, not the primary mechanism (i.e. check per-server field first when it lands)

If the Architect does not want to implement per-server yet, fine — but the env var must be designed so adding per-server config later doesn't break it. That means the gating function should accept `*Server` and consult both, not be a global free function.

**2. CI / test breakage inventory — UNANSWERED**

The audit referenced "Docker-based SSH integration tests" (9 tests). Does any of those rely on the current silent fallback? The plan must answer this BEFORE Implementer starts, otherwise Tester will discover the breakage and we'll cycle. **Demand**: Architect must inventory whether any test path currently depends on `InsecureIgnoreHostKey` being the default, and either (a) bake `t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")` into those tests, or (b) populate a synthetic `known_hosts` fixture for them.

**3. WARN-level log on every connection in insecure mode — YES, but bounded**

Architect asked. Answer: yes, log when insecure mode is active. But do NOT log per-connection — that's spammy under reconnection storms. Compromise:

- Log once per `(host, port)` per process lifetime, at WARN level
- Include the host:port in the message so an operator grepping logs sees which servers were accepted insecurely

**4. The "unreadable" vs "missing" distinction matters**

`os.Stat` failing covers both "file doesn't exist" and "file exists but I lack permission." The error message should distinguish:
- Missing: tell user how to populate (ssh-keyscan)
- Permission error: tell user to fix perms

`knownhosts.New(khPath)` failing on a **malformed** known_hosts file is a third case (corrupt file). Currently falls back to insecure too. The fix must NOT silently accept that — log error, refuse to connect, suggest the user inspect `~/.ssh/known_hosts`.

**5. Error message (concrete proposal)**

```
SSH host key verification unavailable for "<server.Name>" (<host>:<port>):
  ~/.ssh/known_hosts is missing.

Warren refuses to connect because the server identity cannot be verified
(MITM vulnerable). To proceed, choose one:

  1. Populate known_hosts (recommended):
        ssh-keyscan -H <host> >> ~/.ssh/known_hosts

  2. Connect with verification disabled (UNSAFE; dev/CI only):
        export WARREN_SSH_INSECURE_HOSTKEY=1
```

If the file exists but is unreadable / malformed, swap "is missing" for "is unreadable: <err>" or "is malformed: <err>".

### Test strategy challenge

The plan says "Tester will use these." Be specific about what cases:

- known_hosts present and valid host → connect succeeds
- known_hosts missing, env var unset → return error with the message above, no dial attempted
- known_hosts missing, env var set → connect succeeds, WARN logged once
- known_hosts present but no entry for this host → error from `knownhosts` callback (existing behavior, no change needed but assert it still works)
- known_hosts permission-denied, env var unset → return permission-specific error

**The fifth case is the easy one to forget. Demand it explicitly in the test plan.**

### Item 1 Verdict: **NEEDS REVISION**

Required changes before Implementer starts:
- Decide explicitly: env-var-only now, with per-server config tracked in tech-debt? Or per-server config now? Architect picks, but **must record** the decision.
- Inventory existing test paths that rely on silent fallback. Plan their migration.
- Lock in the error message wording (above is a starting point).
- Lock in the once-per-host WARN-log rule.
- Add the malformed-known_hosts case to test plan.

---

## Item 2: Parser Duplicate Event Regex — NEEDS REVISION (critical: Option A is impossible)

### Plan adherence

The audit finding is real: I confirmed the bug by reading `internal/parser/activity.go:29`. The regex `^\s*●\s+[^(].+` matches `● Read /file`, `● Bash(...)`, `● Edit(...)`, etc., because the negative class `[^(]` only excludes the **single character immediately after the space**. For `● Read /foo`, that character is `R` — passes. So we get a chat event AND a typed event for every tool/file line. Confirmed.

### First-principles challenges

**1. Option A is technically impossible in Go's `regexp` package. (BLOCKING)**

Go's `regexp` is RE2-based. RE2 **does not support lookahead or lookbehind**. The Architect proposed "Tighten the chat regex with a negative lookahead/lookbehind." This cannot be done with Go stdlib `regexp`.

The team would have to either:
- Switch to `github.com/dlclark/regexp2` (non-stdlib, slower, adds dependency)
- Inline a "known tools" allowlist in code (i.e., not in the regex), which is effectively a degenerate form of Option C anyway

**Demand: drop Option A from the menu. Tell the team explicitly it's not viable.** If they pick Option A and silently work around it by adding code-level exclusion, that's the "superficial fix" pattern I'm here to halt.

**2. Option B (post-hoc dedup) is the wrong tool.**

Looking at the actual code, the four parsers have an architectural inconsistency: `parseChat` is line-by-line; `parseFileInteractions`, `parseToolUsage`, `parsePrompts` use `FindAllStringSubmatch` over the full content. Post-hoc dedup would require carrying line-position metadata that isn't currently captured. The fix would need a wrapper layer that re-discovers position. Brittle and unrelated to the real problem.

**3. Option C (single dispatch) is the right answer — but it's not a small fix.**

Option C requires unifying all four parsers into a single line-by-line pass with a defined precedence ladder. Proposed precedence (highest priority wins, **one event per line maximum**):

1. **Permission** — most specific anchors (`Esc to cancel · Tab to amend`, `Allowed by auto mode`, `❯ N. Option`)
2. **Question** — `●` + `?$` (last 10 non-empty lines only, per existing logic)
3. **File** — `● Read|Edit|Write`
4. **Tool** — `● Bash|Agent|Skill|LSP|WebSearch|WebFetch|Grep|Glob`
5. **Chat** — `●` or `❯` fallback (everything else with these prefixes)

**Degradation behavior on a new tool:** A future `● Task(...)` line would fall through to chat (level 5) and emit a chat event with role=assistant. **That is correct.** It's not classified as a "tool" event but Warren will still record the line. When the tool list is updated, it becomes a tool event. No data loss in the meantime. This is the answer to the audit's brittleness concern.

**The Architect should NOT lie about scope.** Option C touches the structure of `Parse()` and three of its helpers. It's not a one-line regex change. The Architect's plan must acknowledge this and tell the Implementer the expected diff size.

**4. Other regexes with similar over-matching — YES, several**

I read the file. Additional duplicate sources:

- `questionPatterns[0]` = `●\s+.+\?$`. An assistant chat line ending in `?` matches BOTH this AND `chatPatterns[1]`. Currently emits both. Same class of bug.
- `toolPatterns: regexp.MustCompile(`(?i)running\s+tests`)`. Matches the phrase "running tests" anywhere — if chat content says "I'm running tests now," that produces both a tool event AND a chat event.
- `toolPatterns: regexp.MustCompile(`(?i)executing\s+command:`)`. Same problem.
- Legacy filePatterns `(?i)Read\s+tool.*?file_path` etc.: when these match they OFTEN also match chatPatterns' `^assistant:` if the assistant said "I'll use the Read tool with file_path X." Same class.

**Demand: when the Implementer refactors to Option C, they fix all of these together.** This is exactly what option C unifies — one line, one event, by precedence. If they leave the legacy regexes as-is, the bug class persists.

**5. The Bash regex bug nobody mentioned**

`toolPatterns: regexp.MustCompile(`(?i)LSP\s+tool`)` paired with the switch-case `case strings.Contains(matchLower, "lsp"):`. Fine. But the parseToolUsage switch hits `case strings.Contains(match, "Bash"):` BEFORE the (?i) regex matches "bash tool" (legacy) — `match` is the literal matched text, "Bash" requires exact case. So a legacy match of `bash tool` lowercases and the switch case for "Bash" fails. `toolName` stays "unknown." Minor, not blocking, but **the Implementer should be told the switch is case-inconsistent with the regex.**

### Test strategy challenge

The plan says "real fixtures in `internal/testdata/` (5 captures) — confirm the fix produces N events per line, not N+1." Verified the fixtures exist:
```
asking_permission_foundry2.txt
asking_question_foundry2.txt
idle_completed_foundry2.txt
idle_completed_localhost.txt
running_localhost.txt
```

**That's necessary but not sufficient.** Two more requirements:

a) **Per-line invariant test:** parse each fixture, then for every line in the fixture content, assert `count_events_with_that_line_as_substring <= 1`. This is the "no duplicates" guarantee, encoded as a property test.

b) **Precedence table test:** explicitly enumerate (input line → expected event type) for at least one representative line from each precedence level. E.g.:
- `● Read /etc/hosts` → file event, NOT chat
- `● Bash(ls)` → tool event, NOT chat
- `● Hello world` → chat event
- `● What should I do next?` → question event, NOT chat
- `❯ run the tests` → chat event with role=user

c) **Audit said:** "All tests use legacy patterns; zero real Claude Code UI patterns tested." The fix-validating tests **must read from `internal/testdata/`**. If the Tester writes synthetic strings only, the same blindspot recurs. Architect must instruct Tester explicitly.

### Item 2 Verdict: **NEEDS REVISION**

Required changes before Implementer starts:
- **Strike Option A.** Tell the team it's not viable in Go's stdlib regexp.
- **Recommend Option C explicitly** as the chosen path, with the precedence ladder above.
- **Scope warning:** this is a structural refactor of `Parse()`, not a regex tweak. Set Implementer expectations.
- **Sweep the other over-broad regexes** (questionPatterns[0], `running tests`, `executing command:`, legacy file/tool patterns) in the same refactor.
- **Test plan** must include the per-line invariant + precedence table tests, both reading from `internal/testdata/`.

---

## Item 3: warren.db Default Path — NEEDS REVISION

### Plan adherence

Changing the defaults in `cmd/warren-web/main.go:20` and `internal/core/warren.go:87` to match `cmd/warren-tui/main.go:16` does fix the audit's "orphan DB at project root" finding. Verified the three call sites.

### First-principles challenges

**1. Migration UX — Architect raised the right question, here's the answer**

A user with existing `./warren.db` will upgrade, restart, and silently get a fresh empty DB at `$HOME/.warren/warren.db`. Their event history, registry, and notifications appear lost (the file still exists in cwd, but Warren doesn't read it). This is data loss from the user's mental model.

**Demand: implement the legacy-DB warning.** At startup, if the user did NOT pass `-db` explicitly AND `./warren.db` exists in cwd AND `$HOME/.warren/warren.db` does NOT exist, log a clear warning and refuse to start (do not silently create a new DB). Concrete proposal:

```
Found legacy database at ./warren.db (cwd-relative).
Warren now uses $HOME/.warren/warren.db by default.

To preserve your data:
    mkdir -p $HOME/.warren && mv ./warren.db $HOME/.warren/warren.db

Or keep the legacy location with:
    warren-web -db ./warren.db

Refusing to start (would create empty database and orphan your data).
```

"Refuse to start" is the right call here because **silent data loss is worse than a startup failure with an actionable message**. The user can clear the warning in seconds.

Detection mechanism: compare the user's `*dbPath` flag value against `core.DefaultDBPath()`. If equal AND cwd `warren.db` exists, trigger the check.

**2. Auto-create `$HOME/.warren/` — yes, but at the right layer**

`warren-tui/main.go:19-24` does `os.MkdirAll(warrenDir, 0755)`. The fix must do the same in `warren-web/main.go`. **But: this directory creation belongs in `core.NewWarren()` or in a helper, not duplicated across cmds.** Currently both cmds have their own `discoverAndRegisterSessions` duplication too — there's a pattern of cmd-level copy-paste in this codebase that the fix should not propagate.

**Recommendation:** Make `core.EnsureConfigDir(configDir) error` and call it once in `core.NewWarren()` after Validate. Both cmds get the behavior automatically. No duplicate `MkdirAll` literals.

**3. Unify into a helper — YES, this is mandatory not optional**

Architect asked "or is duplicating an `os.Getenv("HOME")` literal acceptable for now?" Answer: **No.** The whole reason we're fixing this is that the duplication drifted. Three call sites, three subtly different forms (`"warren.db"`, `os.ExpandEnv("$HOME/.warren/warren.db")`, the web default). Fixing them in lockstep without consolidation just delays the next drift.

**Proposed:**

```go
// internal/core/defaults.go
func DefaultDBPath() string {
    home, err := os.UserHomeDir()
    if err != nil || home == "" {
        // fall back to cwd, with a comment about why
        return "warren.db"
    }
    return filepath.Join(home, ".warren", "warren.db")
}

func DefaultConfigDir() string {
    home, err := os.UserHomeDir()
    if err != nil || home == "" {
        return ".warren"
    }
    return filepath.Join(home, ".warren")
}
```

Then:
- `DefaultConfig()` (`warren.go:87`) calls `DefaultDBPath()` and `DefaultConfigDir()`
- Both cmds call those for their `flag.String` defaults so `--help` shows the resolved path
- Neither cmd hand-rolls `os.ExpandEnv("$HOME/...")` again

**Use `os.UserHomeDir()`, NOT `os.ExpandEnv("$HOME/...")`.** The latter blindly substitutes empty string if `$HOME` is unset, producing `/.warren/warren.db`. `UserHomeDir` returns an error you can detect.

**4. Tests**

Architect's suggestion: `t.Setenv("HOME", t.TempDir())`. Correct for Linux/macOS. Tests needed:

- `DefaultDBPath` returns `<tmpdir>/.warren/warren.db` when HOME is set
- `DefaultDBPath` falls back to `warren.db` (cwd-relative) when HOME is empty — and that fact is documented
- Legacy-DB detection triggers when `./warren.db` exists AND user didn't override AND new path doesn't exist
- Legacy-DB detection does NOT trigger when user passes `-db` explicitly (even if matches the default by coincidence — see point 5)

**5. Edge: user explicitly passes `-db $HOME/.warren/warren.db`**

If the user passes the same string as the default, `flag` cannot distinguish "user passed it" from "user accepted the default." To detect this properly, use `flag.Visit` (which iterates only flags the user set) instead of comparing values. **The plan must mention this** or it'll be a subtle bug.

### Item 3 Verdict: **NEEDS REVISION**

Required changes before Implementer starts:
- Implement `core.DefaultDBPath()` and `core.DefaultConfigDir()` helpers; remove all literal duplications.
- Add legacy-DB detection that refuses to start with the actionable error above.
- Use `flag.Visit` (not value comparison) to detect "user did not override."
- Move `MkdirAll` into `core.NewWarren()` (or a `core.EnsureConfigDir` helper) so both cmds inherit it.
- Tests must cover the empty-HOME fallback path explicitly.

---

## Cross-Cutting Concerns

**1. Batch coherence.** Three items are genuinely independent in code. OK to land together. But the SSH item is by far the heaviest, and Item 2 (parser) is a structural refactor not a regex tweak. If Implementer scopes Item 2 too small, the batch will need rework. **Architect should brief Implementer that Item 2 is a refactor, not a one-line fix.**

**2. P0 SSH item is on the "Phase 3 entry criteria" promise from the previous critique.** Lines 50 and 125 of `docs/reviews/phase2-fixes-critique.md` are explicit: "MUST file as a P0 security ticket and resolve before the first Phase 3 user-callable SSH path lands." Verified that promise. This batch is the resolution. If the team papers over with a half-fix (e.g. env var that defaults to insecure, or doesn't actually return errors, or doesn't log warnings), my next critique will be REJECTED with no further negotiation. The Architect should set this expectation with the Implementer explicitly.

**3. "Claimed complete but actually partial" pattern.** The audit flagged five such items in Phase 2 (server grouping, manual registration, content-based discovery, claimed-vs-actual test counts, claimed-vs-actual API endpoint counts). My job on the post-implementation pass will be to verify NONE of these three Batch 1 items are shipped half-done. Specifically I will check:
- Is `WARREN_SSH_INSECURE_HOSTKEY` actually consulted on every connection, or only at startup?
- Does the parser refactor unify all four parsers, or just patch the chat one?
- Did the legacy-DB detection actually wire up to BOTH `warren-web` and `warren-tui`, or only one?

**4. Tech-debt doc updates required regardless of which option is chosen for Item 1.** The previous critique's P0 ticket (`docs/phase2-technical-debt.md:15-25`) must be **closed out** in the same commit that lands the fix. The P1 mutex item (`:33`) is **not in this batch** — that's fine, but the Architect should confirm to the team-lead that mutex is in a later batch, not silently dropped. The user warned us about papering over.

---

## Summary Verdicts

| Item | Verdict | Critical Blocker |
|------|---------|-----|
| 1. SSH host key | **NEEDS REVISION** | Decide env-var vs per-server config explicitly; lock error message; inventory CI tests; design once-per-host WARN log |
| 2. Parser duplicates | **NEEDS REVISION** | Option A is impossible in Go RE2 — strike it; commit to Option C as a structural refactor; sweep all over-broad regexes; test from real fixtures |
| 3. warren.db default | **NEEDS REVISION** | Unify into `core.DefaultDBPath()`/`DefaultConfigDir()`; add legacy-DB refuse-to-start check; use `flag.Visit` for override detection |

**No item rises to ESCALATE TO USER.** All three are resolvable by Architect-level plan revision, no user input required. The user gave us latitude on remediation approach when they accepted the Phase 2 fixes critique deferral — these are implementation decisions within that envelope.

## Bottom Line

The single most important fix to the plan: **drop Option A from Item 2.** It's not a viable choice in Go's standard library, and offering it on the menu invites the Implementer to pick the easiest-looking option and ship a regex hack that compiles but doesn't actually fix the underlying architectural inconsistency (line-based vs. content-based parsing). Commit to Option C, set scope expectations, and the team can land a real fix instead of another superficial patch.
