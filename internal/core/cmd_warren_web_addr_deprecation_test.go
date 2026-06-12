package core_test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================================================
// Phase 2 audit Batch 2 — Item #4 cmd-level: --addr deprecation WARN.
//
// The Implementer kept --addr as a deprecated alias for --bind so existing
// scripts don't break (one-release deprecation window). When the operator
// passes --addr WITHOUT --bind, cmd/warren-web emits:
//
//   WARN: --addr is deprecated; use --bind. Honoring --addr=... for now.
//
// Without a wiring test, the deprecation WARN could be silently dropped in
// a refactor and the operator would never know they're on a deprecated path.
// Unit coverage in internal/web cannot exercise this — the WARN lives in
// package main. Strategy mirrors cmd_legacy_db_test.go: build, exec, observe
// stderr, then kill before the HTTP server holds the port.
// ============================================================================

// safeBuffer is a thread-safe bytes.Buffer wrapper. exec.Cmd writes to it
// from an internal goroutine, and these tests read while the child process
// is still running, so unprotected concurrent access trips -race.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestCmd_WarrenWeb_AddrDeprecationWarning verifies that passing --addr
// alone emits the deprecation WARN to stderr. The binary then proceeds to
// start the web server (we kill it before that conflicts with anything).
func TestCmd_WarrenWeb_AddrDeprecationWarning(t *testing.T) {
	bin := buildBinary(t, "warren-web")

	cwd := t.TempDir()
	tmpHome := t.TempDir()

	// Use --addr 127.0.0.1:0 so the kernel picks a free port; no conflict
	// with anything else on the test host. -db points at a path under
	// tmpHome so legacy-DB refuse-to-start cannot fire.
	cmd := exec.Command(bin,
		"-addr", "127.0.0.1:0",
		"-db", tmpHome+"/test.db",
	)
	cmd.Dir = cwd
	env := filterEnv(os.Environ(), "HOME", "WARREN_WEB_BIND")
	env = append(env, "HOME="+tmpHome)
	cmd.Env = env

	stderr := &safeBuffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	// Poll stderr for the WARN. The deprecation WARN fires inline from
	// main() before the HTTP server starts, so it should show up within
	// ~100ms — the 2s ceiling is just safety for slow CI runners.
	const wantSubstring = "--addr is deprecated"
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(stderr.String(), wantSubstring) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	out := stderr.String()
	if !strings.Contains(out, wantSubstring) {
		t.Errorf("expected stderr to contain deprecation WARN %q; got:\n%s", wantSubstring, out)
	}
	if !strings.Contains(out, "WARN") {
		t.Errorf("expected WARN prefix on deprecation log; got:\n%s", out)
	}
	if !strings.Contains(out, "127.0.0.1:0") {
		t.Errorf("expected deprecation WARN to echo the honored --addr value; got:\n%s", out)
	}
}

// TestCmd_WarrenWeb_BindFlagSilencesAddrDeprecation locks in the other
// half of the deprecation contract: when both --addr and --bind are
// supplied, --bind wins and the deprecation WARN is silenced. cmd/
// warren-web fires the WARN only when addrSet && !bindSet — this test
// pins that branch so an over-eager refactor doesn't start warning even
// when the user has already migrated.
func TestCmd_WarrenWeb_BindFlagSilencesAddrDeprecation(t *testing.T) {
	bin := buildBinary(t, "warren-web")

	cwd := t.TempDir()
	tmpHome := t.TempDir()

	cmd := exec.Command(bin,
		"-addr", "127.0.0.1:0",
		"-bind", "127.0.0.1:0",
		"-db", tmpHome+"/test.db",
	)
	cmd.Dir = cwd
	env := filterEnv(os.Environ(), "HOME", "WARREN_WEB_BIND")
	env = append(env, "HOME="+tmpHome)
	cmd.Env = env

	stderr := &safeBuffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	// Give the process ~500ms to fire any startup logs.
	time.Sleep(500 * time.Millisecond)

	out := stderr.String()
	if strings.Contains(out, "--addr is deprecated") {
		t.Errorf("deprecation WARN should be silenced when --bind is also set; got:\n%s", out)
	}
}
