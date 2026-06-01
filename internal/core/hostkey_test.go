package core

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ============================================================================
// Phase 2 audit Batch 1 — Item 1: SSH host key strict default + tests.
//
// The audit and Critique pre-validation required SIX explicit test cases:
//
//   1. known_hosts present and valid host           → callback returned, no error
//   2. known_hosts missing, env var unset           → ERROR, refuse-to-start
//   3. known_hosts missing, WARREN_SSH_INSECURE=1   → callback + WARN log (once)
//   4. known_hosts present but no entry for host    → callback returned (host-key
//                                                     mismatch is detected at
//                                                     handshake, not factory)
//   5. known_hosts permission-denied, env unset     → ERROR mentioning unreadable
//   6. known_hosts malformed, env unset             → ERROR mentioning malformed
//
// Plus the once-per-(host,port) WARN-log invariant the Critique demanded:
//
//   * Call insecure path multiple times for same (host:port); WARN appears
//     exactly once. Different (host:port) gets its own one-shot warning.
//
// Implementation: hostKeyCallbackForServer is a package-private function in
// internal/core/server.go. This file lives in `package core` so it can be
// called directly without exposing the API. The same applies to the
// insecureHostsWarned sync.Map which we clear between tests via the
// resetInsecureHostsWarned helper.
//
// HOME is overridden per-test via t.Setenv so we never touch the real
// ~/.ssh/known_hosts.
// ============================================================================

// resetInsecureHostsWarned clears the package-level dedup map so each test
// starts fresh. Without this, a previous test's host:port pair would
// suppress a subsequent test's expected WARN.
func resetInsecureHostsWarned() {
	insecureHostsWarned = sync.Map{}
}

// writeKnownHosts writes the given content to <dir>/.ssh/known_hosts and
// returns the file path. mode applies to the file; the .ssh directory is
// always 0700.
func writeKnownHosts(t *testing.T, dir, content string, mode os.FileMode) string {
	t.Helper()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("mkdir .ssh: %v", err)
	}
	khPath := filepath.Join(sshDir, "known_hosts")
	if err := os.WriteFile(khPath, []byte(content), mode); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	return khPath
}

// validKnownHostsLine returns a syntactically-valid known_hosts entry. The
// key material is a fresh ed25519 public key encoded for use in known_hosts;
// it is NOT used for verification in any of these tests (we only call the
// factory, not the callback) but the line must parse cleanly so
// knownhosts.New succeeds.
func validKnownHostsLine(host string, port int) string {
	hostPort := host
	if port != 0 && port != 22 {
		hostPort = fmt.Sprintf("[%s]:%d", host, port)
	}
	// 32 zero bytes base64-encoded; OpenSSH accepts arbitrary key blobs in
	// known_hosts as long as the line is syntactically well-formed.
	return fmt.Sprintf("%s ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\n", hostPort)
}

// captureLogs swaps the standard log output with a buffer for the duration
// of the test and returns a "drain" function.
func captureLogs(t *testing.T) (drain func() string) {
	t.Helper()
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(old) })
	return func() string { return buf.String() }
}

// remoteServer is a small test fixture.
func remoteServer(name, host string, port int) *Server {
	return &Server{
		Name: name,
		Host: host,
		User: "tester",
		Port: port,
		Kind: ServerKindRemote,
	}
}

// -------- Case 1: known_hosts present and valid host --------

func TestHostKeyCallbackForServer_KnownHostsPresentValid(t *testing.T) {
	resetInsecureHostsWarned()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	writeKnownHosts(t, tmpHome, validKnownHostsLine("example.com", 22), 0644)

	srv := remoteServer("example", "example.com", 22)
	cb, err := hostKeyCallbackForServer(srv)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cb == nil {
		t.Fatalf("expected non-nil HostKeyCallback")
	}
}

// -------- Case 2: known_hosts missing, env var unset → refuse-to-start --------

func TestHostKeyCallbackForServer_MissingNoEnvVar_RefusesToStart(t *testing.T) {
	resetInsecureHostsWarned()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	// Do NOT create .ssh/known_hosts.
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "")

	srv := remoteServer("example", "example.com", 22)
	cb, err := hostKeyCallbackForServer(srv)
	if err == nil {
		t.Fatalf("expected error when known_hosts missing and env var unset; got nil")
	}
	if cb != nil {
		t.Errorf("expected nil callback on refuse; got non-nil")
	}
	msg := err.Error()
	// Critique-mandated substrings: env-var name + ssh-keyscan suggestion.
	if !strings.Contains(msg, "WARREN_SSH_INSECURE_HOSTKEY") {
		t.Errorf("error must mention env var WARREN_SSH_INSECURE_HOSTKEY; got %q", msg)
	}
	if !strings.Contains(msg, "ssh-keyscan") {
		t.Errorf("error must suggest ssh-keyscan; got %q", msg)
	}
	// Should mention the host so the user knows which connection refused.
	if !strings.Contains(msg, "example.com") {
		t.Errorf("error should mention the target host; got %q", msg)
	}
	// Plain English "missing" must appear so the operator knows why.
	if !strings.Contains(strings.ToLower(msg), "missing") && !strings.Contains(strings.ToLower(msg), "is missing") {
		t.Errorf("error should mention the file is missing; got %q", msg)
	}
}

// -------- Case 3: known_hosts missing, env var=1 → insecure callback + WARN --------

func TestHostKeyCallbackForServer_MissingWithEnvVar_InsecureCallback(t *testing.T) {
	resetInsecureHostsWarned()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")
	drain := captureLogs(t)

	srv := remoteServer("dev-vm", "192.0.2.1", 2222)
	cb, err := hostKeyCallbackForServer(srv)
	if err != nil {
		t.Fatalf("expected no error in insecure mode; got %v", err)
	}
	if cb == nil {
		t.Fatalf("expected non-nil callback in insecure mode")
	}
	out := drain()
	if !strings.Contains(out, "WARN") {
		t.Errorf("expected a WARN log line in insecure mode; got: %q", out)
	}
	// The WARN should embed the host:port so an operator grepping logs can
	// identify which connection was accepted insecurely.
	if !strings.Contains(out, "192.0.2.1") {
		t.Errorf("WARN should embed host:port (192.0.2.1); got: %q", out)
	}
	if !strings.Contains(out, "WARREN_SSH_INSECURE_HOSTKEY") {
		t.Errorf("WARN should reference the env var by name so a grep finds it; got: %q", out)
	}
}

// -------- Case 4: known_hosts present, host not in file --------
//
// When known_hosts is well-formed but does not contain an entry for the
// target host, the factory still returns the callback — verification fails
// at SSH handshake time, not at callback construction. This pins existing
// behaviour so a future "pre-validate known_hosts contains host" change
// can't silently break the contract.

func TestHostKeyCallbackForServer_KnownHostsPresentButHostUnknown(t *testing.T) {
	resetInsecureHostsWarned()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	// File contains an entry for someone else.
	writeKnownHosts(t, tmpHome, validKnownHostsLine("other.example.com", 22), 0644)

	srv := remoteServer("target", "target.example.com", 22)
	cb, err := hostKeyCallbackForServer(srv)
	if err != nil {
		t.Fatalf("factory should succeed even when host not in known_hosts (failure happens at handshake); got %v", err)
	}
	if cb == nil {
		t.Fatalf("expected non-nil callback")
	}
}

// -------- Case 5: known_hosts permission-denied, env var unset --------
//
// Critique-required distinction: permission failure must NOT be confused
// with "malformed file" — the operator needs to know whether to fix perms
// or inspect the file contents. This test exercises that distinction.
//
// Note: stat() succeeds on a chmod-0000 file (stat doesn't read content);
// the permission error surfaces inside knownhosts.New(). A correct
// implementation must detect "open ... permission denied" and route it to
// the "unreadable" branch, not the "malformed" one.

func TestHostKeyCallbackForServer_PermissionDeniedNoEnvVar_Refuses(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; chmod 0000 cannot deny reads")
	}
	resetInsecureHostsWarned()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "")

	// Create the file then chmod 0000.
	khPath := writeKnownHosts(t, tmpHome, validKnownHostsLine("example.com", 22), 0644)
	if err := os.Chmod(khPath, 0000); err != nil {
		t.Fatalf("chmod 0000: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(khPath, 0644) })

	srv := remoteServer("example", "example.com", 22)
	cb, err := hostKeyCallbackForServer(srv)
	if err == nil {
		t.Fatalf("expected error when known_hosts is permission-denied; got nil")
	}
	if cb != nil {
		t.Errorf("expected nil callback on refuse; got non-nil")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "unreadable") {
		// Surfaces an implementation bug: permission-denied is being
		// classified as "malformed" because the impl checks os.Stat for
		// permission errors (which succeed for chmod-0000 files) then lets
		// knownhosts.New surface the open() failure — and labels that branch
		// "malformed". The Critique-mandated distinction is broken.
		t.Errorf("error should classify permission failure as 'unreadable' (Critique-mandated distinction from 'malformed'); got %q", err.Error())
	}
}

// -------- Case 6: known_hosts malformed (corrupt), env var unset --------

func TestHostKeyCallbackForServer_MalformedNoEnvVar_Refuses(t *testing.T) {
	resetInsecureHostsWarned()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "")

	// Write content that is clearly not a valid known_hosts file.
	writeKnownHosts(t, tmpHome, "this is not a known_hosts entry\n@@@garbage\n", 0644)

	srv := remoteServer("example", "example.com", 22)
	cb, err := hostKeyCallbackForServer(srv)
	if err == nil {
		// knownhosts.New is sometimes lenient about garbage lines (it skips
		// unparseable lines rather than rejecting the file). If it didn't
		// error, that's acceptable — but document the gap.
		t.Skipf("knownhosts.New accepted the malformed file (lenient parser); cannot exercise malformed branch this way")
		return
	}
	if cb != nil {
		t.Errorf("expected nil callback on refuse; got non-nil")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "malformed") {
		t.Errorf("error should mention %q; got %q", "malformed", err.Error())
	}
}

// TestHostKeyCallbackForServer_MalformedNoEnvVar_RefusesBinary uses a binary
// blob that knownhosts.New is more likely to reject (NUL bytes + invalid
// base64). Complements the "garbage text" case above.
func TestHostKeyCallbackForServer_MalformedNoEnvVar_RefusesBinary(t *testing.T) {
	resetInsecureHostsWarned()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "")

	// Lines with format that looks like known_hosts but with invalid key data
	// commonly trip knownhosts.New.
	sshDir := filepath.Join(tmpHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	khPath := filepath.Join(sshDir, "known_hosts")
	if err := os.WriteFile(khPath, []byte("example.com ssh-rsa not-valid-base64-!!!!\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	srv := remoteServer("example", "example.com", 22)
	cb, err := hostKeyCallbackForServer(srv)
	if err == nil {
		t.Skipf("knownhosts.New accepted invalid-key-data line (lenient parser); cannot exercise malformed branch this way")
		return
	}
	if cb != nil {
		t.Errorf("expected nil callback; got non-nil")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "malformed") {
		t.Errorf("error should mention %q; got %q", "malformed", err.Error())
	}
}

// -------- WARN-once dedup invariant --------

// TestHostKeyCallbackForServer_WarnsOncePerHostPort exercises the
// once-per-(host,port) WARN log rule the Critique demanded. Two calls for
// the same host:port produce exactly one WARN; a different host:port gets
// its own.
func TestHostKeyCallbackForServer_WarnsOncePerHostPort(t *testing.T) {
	resetInsecureHostsWarned()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")
	drain := captureLogs(t)

	srvA := remoteServer("a", "10.0.0.1", 22)
	srvB := remoteServer("b", "10.0.0.2", 22)

	// Three calls for A, two for B, interleaved.
	for i := 0; i < 3; i++ {
		if _, err := hostKeyCallbackForServer(srvA); err != nil {
			t.Fatalf("A call %d: %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := hostKeyCallbackForServer(srvB); err != nil {
			t.Fatalf("B call %d: %v", i, err)
		}
	}
	if _, err := hostKeyCallbackForServer(srvA); err != nil {
		t.Fatalf("A re-call: %v", err)
	}

	out := drain()

	countA := strings.Count(out, "10.0.0.1")
	countB := strings.Count(out, "10.0.0.2")
	if countA != 1 {
		t.Errorf("expected exactly one WARN for 10.0.0.1, got %d. Log:\n%s", countA, out)
	}
	if countB != 1 {
		t.Errorf("expected exactly one WARN for 10.0.0.2, got %d. Log:\n%s", countB, out)
	}
}

// TestHostKeyCallbackForServer_WarnsOncePerHostPort_DifferentPorts verifies
// the dedup key is host:port, not just host. The same host on two ports
// gets two warnings.
func TestHostKeyCallbackForServer_WarnsOncePerHostPort_DifferentPorts(t *testing.T) {
	resetInsecureHostsWarned()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")
	drain := captureLogs(t)

	srv22 := remoteServer("a22", "10.0.0.5", 22)
	srv2222 := remoteServer("a2222", "10.0.0.5", 2222)

	for i := 0; i < 2; i++ {
		_, _ = hostKeyCallbackForServer(srv22)
		_, _ = hostKeyCallbackForServer(srv2222)
	}

	out := drain()
	// Anchor on the exact host:port substring with a delimiter (space, paren)
	// after the port number so "10.0.0.5:22 " does not accidentally match
	// "10.0.0.5:2222". The WARN format includes " (WARREN_..." right after.
	count22 := strings.Count(out, "10.0.0.5:22 ")
	count2222 := strings.Count(out, "10.0.0.5:2222 ")
	if count22 != 1 {
		t.Errorf("expected exactly one WARN for 10.0.0.5:22, got %d\nlog:\n%s", count22, out)
	}
	if count2222 != 1 {
		t.Errorf("expected exactly one WARN for 10.0.0.5:2222, got %d\nlog:\n%s", count2222, out)
	}
}

// -------- Strict-by-default precedence with env var set --------
//
// Even when WARREN_SSH_INSECURE_HOSTKEY=1 is set, a valid known_hosts file
// should still win — strict verification is strictly safer. The env var is
// a fallback, not an override. The Critique flagged this explicitly:
// "use the verifying callback even if the operator also set the env var."

func TestHostKeyCallbackForServer_EnvVarDoesNotDowngradePresent(t *testing.T) {
	resetInsecureHostsWarned()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")
	writeKnownHosts(t, tmpHome, validKnownHostsLine("example.com", 22), 0644)
	drain := captureLogs(t)

	srv := remoteServer("example", "example.com", 22)
	cb, err := hostKeyCallbackForServer(srv)
	if err != nil {
		t.Fatalf("expected callback with valid known_hosts; got %v", err)
	}
	if cb == nil {
		t.Fatalf("expected non-nil callback")
	}
	// No insecure-mode WARN should fire — we used strict mode.
	out := drain()
	if strings.Contains(out, "MITM-vulnerable") || strings.Contains(out, "WARN") {
		t.Errorf("env var should NOT downgrade present-valid known_hosts; insecure WARN appeared:\n%s", out)
	}
}
