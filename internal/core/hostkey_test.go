package core

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Phase 2 audit Batch 1 — Item 1: SSH host-key strict default + WARN-once tests.
//
// This suite targets the final locked API contract now implemented in
// internal/core/server.go:
//   - ConnectionPool.hostKeyCallback(server) is the test surface
//   - ErrKnownHostsMissing / ErrKnownHostsUnreadable / ErrKnownHostsMalformed
//     are the routing sentinels
//   - insecure WARN dedup state is pool-scoped (ConnectionPool field), not global

func writeKnownHosts(t *testing.T, dir, content string, mode os.FileMode) string {
	t.Helper()
	sshDir := filepath.Join(dir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir .ssh: %v", err)
	}
	khPath := filepath.Join(sshDir, "known_hosts")
	if err := os.WriteFile(khPath, []byte(content), mode); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	return khPath
}

func validKnownHostsLine(host string, port int) string {
	hostPort := host
	if port != 0 && port != 22 {
		hostPort = fmt.Sprintf("[%s]:%d", host, port)
	}
	return fmt.Sprintf("%s ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\n", hostPort)
}

func captureLogs(t *testing.T) func() string {
	t.Helper()
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(old) })
	return func() string { return buf.String() }
}

func remoteServer(name, host string, port int) *Server {
	return &Server{Name: name, Host: host, User: "tester", Port: port, Kind: ServerKindRemote}
}

func TestConnectionPool_HostKeyCallback_KnownHostsPresentValid(t *testing.T) {
	pool := NewConnectionPool(time.Second)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	writeKnownHosts(t, tmpHome, validKnownHostsLine("example.com", 22), 0o644)

	cb, err := pool.hostKeyCallback(remoteServer("example", "example.com", 22))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cb == nil {
		t.Fatalf("expected non-nil callback")
	}
}

func TestConnectionPool_HostKeyCallback_MissingNoEnvVar_Refuses(t *testing.T) {
	pool := NewConnectionPool(time.Second)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "")

	cb, err := pool.hostKeyCallback(remoteServer("example", "example.com", 22))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if cb != nil {
		t.Fatalf("expected nil callback on refuse")
	}
	if !errors.Is(err, ErrKnownHostsMissing) {
		t.Fatalf("expected errors.Is(..., ErrKnownHostsMissing), got %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "WARREN_SSH_INSECURE_HOSTKEY") {
		t.Errorf("error must mention env var; got %q", msg)
	}
	if !strings.Contains(msg, "ssh-keyscan") {
		t.Errorf("error must suggest ssh-keyscan; got %q", msg)
	}
}

func TestConnectionPool_HostKeyCallback_MissingWithEnvVar_InsecureWarns(t *testing.T) {
	pool := NewConnectionPool(time.Second)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")
	drain := captureLogs(t)

	cb, err := pool.hostKeyCallback(remoteServer("dev", "192.0.2.1", 2222))
	if err != nil {
		t.Fatalf("expected no error in insecure mode, got %v", err)
	}
	if cb == nil {
		t.Fatalf("expected non-nil callback")
	}
	out := drain()
	if !strings.Contains(out, "WARN") || !strings.Contains(out, "192.0.2.1:2222") {
		t.Errorf("expected WARN with host:port, got %q", out)
	}
}

func TestConnectionPool_HostKeyCallback_KnownHostsPresentButHostUnknown(t *testing.T) {
	pool := NewConnectionPool(time.Second)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	writeKnownHosts(t, tmpHome, validKnownHostsLine("other.example.com", 22), 0o644)

	cb, err := pool.hostKeyCallback(remoteServer("target", "target.example.com", 22))
	if err != nil {
		t.Fatalf("factory should succeed; callback enforces host match at handshake: %v", err)
	}
	if cb == nil {
		t.Fatalf("expected non-nil callback")
	}
}

func TestConnectionPool_HostKeyCallback_PermissionDeniedNoEnvVar_Refuses(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; chmod 0000 cannot deny reads")
	}
	pool := NewConnectionPool(time.Second)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "")
	khPath := writeKnownHosts(t, tmpHome, validKnownHostsLine("example.com", 22), 0o644)
	if err := os.Chmod(khPath, 0); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(khPath, 0o644) })

	cb, err := pool.hostKeyCallback(remoteServer("example", "example.com", 22))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if cb != nil {
		t.Fatalf("expected nil callback on refuse")
	}
	if !errors.Is(err, ErrKnownHostsUnreadable) {
		t.Fatalf("expected unreadable sentinel, got %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unreadable") {
		t.Errorf("error should say unreadable, got %q", err.Error())
	}
}

func TestConnectionPool_HostKeyCallback_MalformedNoEnvVar_Refuses(t *testing.T) {
	pool := NewConnectionPool(time.Second)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "")
	writeKnownHosts(t, tmpHome, "example.com ssh-rsa not-valid-base64-!!!!\n", 0o644)

	cb, err := pool.hostKeyCallback(remoteServer("example", "example.com", 22))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if cb != nil {
		t.Fatalf("expected nil callback on refuse")
	}
	if !errors.Is(err, ErrKnownHostsMalformed) {
		t.Fatalf("expected malformed sentinel, got %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "malformed") {
		t.Errorf("error should say malformed, got %q", err.Error())
	}
}

func TestConnectionPool_HostKeyCallback_WarnsOncePerHostPort(t *testing.T) {
	pool := NewConnectionPool(time.Second)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")
	drain := captureLogs(t)

	srvA := remoteServer("a", "10.0.0.1", 22)
	srvB := remoteServer("b", "10.0.0.2", 22)
	for i := 0; i < 3; i++ {
		if _, err := pool.hostKeyCallback(srvA); err != nil {
			t.Fatalf("A call %d: %v", i, err)
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := pool.hostKeyCallback(srvB); err != nil {
			t.Fatalf("B call %d: %v", i, err)
		}
	}
	out := drain()
	if strings.Count(out, "10.0.0.1:22") != 1 {
		t.Errorf("expected one WARN for 10.0.0.1:22, got log:\n%s", out)
	}
	if strings.Count(out, "10.0.0.2:22") != 1 {
		t.Errorf("expected one WARN for 10.0.0.2:22, got log:\n%s", out)
	}
}

func TestConnectionPool_HostKeyCallback_WarnsOncePerDifferentPorts(t *testing.T) {
	pool := NewConnectionPool(time.Second)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")
	drain := captureLogs(t)

	for i := 0; i < 2; i++ {
		_, _ = pool.hostKeyCallback(remoteServer("a22", "10.0.0.5", 22))
		_, _ = pool.hostKeyCallback(remoteServer("a2222", "10.0.0.5", 2222))
	}
	out := drain()
	// Include the trailing space before the opening parenthesis from the WARN
	// format so :22 does not spuriously match :2222.
	if strings.Count(out, "10.0.0.5:22 ") != 1 {
		t.Errorf("expected one WARN for :22, got log:\n%s", out)
	}
	if strings.Count(out, "10.0.0.5:2222 ") != 1 {
		t.Errorf("expected one WARN for :2222, got log:\n%s", out)
	}
}

func TestConnectionPool_HostKeyCallback_EnvVarDoesNotDowngradePresent(t *testing.T) {
	pool := NewConnectionPool(time.Second)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")
	writeKnownHosts(t, tmpHome, validKnownHostsLine("example.com", 22), 0o644)
	drain := captureLogs(t)

	cb, err := pool.hostKeyCallback(remoteServer("example", "example.com", 22))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cb == nil {
		t.Fatalf("expected non-nil callback")
	}
	if out := drain(); strings.Contains(out, "MITM-vulnerable") || strings.Contains(out, "WARN") {
		t.Errorf("valid known_hosts should suppress insecure warning, got:\n%s", out)
	}
}

func TestConnectionPool_Get_EarlyReturnsBeforeDialWhenHostKeyUnavailable(t *testing.T) {
	pool := NewConnectionPool(100 * time.Millisecond)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "")
	// Seed a REAL temporary private key so Get() reaches host-key validation
	// rather than failing earlier on the auth-method guard. Intentionally do
	// NOT create ~/.ssh/known_hosts — the missing-file sentinel is the point
	// of this test. Skip if ssh-keygen is unavailable.
	sshDir := filepath.Join(tmpHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir .ssh: %v", err)
	}
	key := filepath.Join(sshDir, "id_ed25519")
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available; cannot deterministically reach host-key path in Get()")
	}
	cmd := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ssh-keygen failed: %v\n%s", err, string(out))
	}

	_, err := pool.Get(remoteServer("closed", "127.0.0.1", 1))
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrKnownHostsMissing) {
		t.Fatalf("expected known_hosts missing sentinel before any dial path, got %v", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "dial ") {
		t.Errorf("expected early host-key error before dial attempt, got %q", err.Error())
	}
}
