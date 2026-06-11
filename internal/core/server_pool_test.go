package core

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// Phase 2 audit Batch 2 — Item #3: ConnectionPool mutex held across dial.
//
// The pool now uses double-checked-locking + per-key in-flight promise
// (`connDial`) so p.mu is acquired only around the in-flight registry.
// The SSH dial + handshake run with p.mu RELEASED so:
//
//   (a) concurrent Get(serverA) / Get(serverB) proceed in parallel,
//   (b) concurrent Get(serverA) × N still results in exactly ONE dial,
//   (c) a slow/failing Get(deadServer) does NOT block Get(liveServer).
//
// The Implementer did not expose a test-only dialer hook, so these tests
// drive the dial path through real net.Listeners that count and time TCP
// accepts. The SSH handshake fails (the listener does not speak SSH), which
// is fine — we only care about when the dial happens, how often, and which
// servers it blocks.
//
// All tests set WARREN_SSH_INSECURE_HOSTKEY=1 so hostKeyCallback does not
// refuse before dial on machines without ~/.ssh/known_hosts.
// ============================================================================

// slowAcceptListener returns a TCP listener bound to 127.0.0.1:0 plus an
// atomic accept counter. The listener accepts incoming connections, sleeps
// `hold` to simulate a slow dial+handshake, then closes the connection.
// The SSH handshake on the client side fails with EOF/reset, which is
// expected — we observe the test invariants via accept timing and counts.
func slowAcceptListener(t *testing.T, hold time.Duration) (*net.TCPListener, *int64) {
	t.Helper()
	tcpAddr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
	ln, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	var accepts int64
	go func() {
		for {
			conn, err := ln.AcceptTCP()
			if err != nil {
				return // listener closed
			}
			atomic.AddInt64(&accepts, 1)
			go func(c *net.TCPConn) {
				defer c.Close()
				// Hold the connection open so the SSH client thinks the
				// dial succeeded; meanwhile the handshake will block on a
				// read until we drop the connection.
				time.Sleep(hold)
				// Drain any handshake bytes before close so the client
				// gets a clean EOF rather than a connection-reset race
				// that can flake under -race.
				_ = c.CloseRead()
				io.Copy(io.Discard, c)
			}(conn)
		}
	}()
	t.Cleanup(func() { _ = ln.Close() })
	return ln, &accepts
}

// serverFromListener builds a *Server pointing at the given listener's
// loopback address. Name is the dedup key for the pool.
func serverFromListener(name string, ln net.Listener) *Server {
	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	return &Server{
		Name: name,
		Host: host,
		User: "tester",
		Port: port,
		Kind: ServerKindRemote,
	}
}

// TestConnectionPool_ConcurrentDifferentServers_NoSerialization is the
// primary regression for Item #3. Two Get calls against different servers
// must overlap rather than serialize. The Batch 1 implementation held
// p.mu across the full dial+handshake, so this test would have observed
// wall time ≈ 2 × hold instead of ≈ 1 × hold.
func TestConnectionPool_ConcurrentDifferentServers_NoSerialization(t *testing.T) {
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")

	const hold = 200 * time.Millisecond

	lnA, _ := slowAcceptListener(t, hold)
	lnB, _ := slowAcceptListener(t, hold)
	srvA := serverFromListener("srv-a", lnA)
	srvB := serverFromListener("srv-b", lnB)

	pool := NewConnectionPool(2 * time.Second)
	defer pool.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	start := time.Now()
	go func() {
		defer wg.Done()
		_, _ = pool.Get(srvA)
	}()
	go func() {
		defer wg.Done()
		_, _ = pool.Get(srvB)
	}()
	wg.Wait()
	elapsed := time.Since(start)

	// If the dials are serialized, elapsed ≥ 2 × hold (~400ms). With the
	// fix, elapsed ≈ hold (~200ms). Use a generous ceiling that still
	// catches serialization (anything below 1.5 × hold) but tolerates
	// scheduler jitter on busy CI hardware.
	const ceiling = time.Duration(1.5 * float64(hold))
	if elapsed >= ceiling {
		t.Fatalf("expected concurrent dials to overlap (elapsed < %v), got %v — pool may be serializing across dial", ceiling, elapsed)
	}
}

// TestConnectionPool_ConcurrentSameServer_SingleDial verifies the
// in-flight promise deduplicates concurrent Get calls for the same server.
// The fix MUST preserve this — without it, racing callers would each
// initiate a dial and the pool would either leak connections or stomp on
// p.connections[key]. Listener accept count is the ground truth.
func TestConnectionPool_ConcurrentSameServer_SingleDial(t *testing.T) {
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")

	ln, accepts := slowAcceptListener(t, 150*time.Millisecond)
	srv := serverFromListener("srv-shared", ln)

	pool := NewConnectionPool(2 * time.Second)
	defer pool.Close()

	// Release N goroutines simultaneously on a shared barrier so they
	// hit Get during the same in-flight window.
	const N = 16
	var wg sync.WaitGroup
	wg.Add(N)
	gate := make(chan struct{})
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			<-gate
			_, _ = pool.Get(srv)
		}()
	}
	close(gate)
	wg.Wait()

	got := atomic.LoadInt64(accepts)
	if got != 1 {
		t.Fatalf("expected exactly 1 TCP accept for %d concurrent Get(sameServer), got %d — dedup broken", N, got)
	}
}

// TestConnectionPool_FailedDialDoesNotBlockOtherServers locks in the
// concurrency property the audit called out: a slow or failing dial to
// one host must not stall an unrelated host. Before the fix, the dead
// host's dial blocked every other Get behind p.mu.
//
// "Dead" server: unreachable TEST-NET-1 address (203.0.113.x). The pool's
// short Dialer.Timeout (200ms) ensures the dial fails deterministically
// in test wall-clock without hanging the suite.
// "Live" server: a real loopback listener with a short hold.
func TestConnectionPool_FailedDialDoesNotBlockOtherServers(t *testing.T) {
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "1")

	const deadTimeout = 600 * time.Millisecond
	pool := NewConnectionPool(deadTimeout)
	defer pool.Close()

	// 203.0.113.0/24 is RFC 5737 TEST-NET-3; routable test address that
	// reliably refuses to complete a TCP handshake. The pool's Dialer
	// Timeout caps the dial at deadTimeout.
	deadServer := &Server{
		Name: "dead-server",
		Host: "203.0.113.1",
		User: "tester",
		Port: 22,
		Kind: ServerKindRemote,
	}

	lnLive, _ := slowAcceptListener(t, 80*time.Millisecond)
	liveServer := serverFromListener("live-server", lnLive)

	type result struct {
		who     string
		elapsed time.Duration
		err     error
	}
	resCh := make(chan result, 2)

	go func() {
		s := time.Now()
		_, err := pool.Get(deadServer)
		resCh <- result{who: "dead", elapsed: time.Since(s), err: err}
	}()
	// Tiny stagger so the dead-server goroutine reliably acquires p.mu
	// first; the test then asserts the live goroutine does NOT inherit
	// that latency.
	time.Sleep(5 * time.Millisecond)
	go func() {
		s := time.Now()
		_, err := pool.Get(liveServer)
		resCh <- result{who: "live", elapsed: time.Since(s), err: err}
	}()

	var live, dead result
	for i := 0; i < 2; i++ {
		r := <-resCh
		if r.who == "live" {
			live = r
		} else {
			dead = r
		}
	}

	// Live server's dial+handshake fails (listener doesn't speak SSH),
	// but it must return well before the dead server's timeout would
	// have completed if the two were serialized.
	if live.elapsed >= deadTimeout {
		t.Fatalf("live server Get took %v ≥ dead-server timeout %v — pool is still serializing across dial", live.elapsed, deadTimeout)
	}
	// Sanity: the dead server's dial must actually have hit its timeout
	// (otherwise the test is meaningless — there's no slow caller to
	// block behind).
	if dead.err == nil {
		t.Fatal("expected dead-server Get to error, got nil — test setup invalid")
	}
}

// TestConnectionPool_HostKeyCheckStillHonored is a smoke test that the
// Batch 1 host-key strict-default flow still fires after the Batch 2
// refactor. Without WARREN_SSH_INSECURE_HOSTKEY, hostKeyCallback must
// refuse the dial before any network activity if ~/.ssh/known_hosts is
// unavailable. Detailed coverage of every error path lives in
// hostkey_test.go; this test just locks in the wiring.
func TestConnectionPool_HostKeyCheckStillHonored(t *testing.T) {
	// Force known_hosts resolution to fail by pointing HOME at an empty
	// dir. We deliberately do NOT set WARREN_SSH_INSECURE_HOSTKEY so the
	// strict default applies.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("WARREN_SSH_INSECURE_HOSTKEY", "")

	// Seed a real ed25519 key so Get() reaches host-key validation rather
	// than failing earlier on the auth-method guard. Intentionally do NOT
	// create ~/.ssh/known_hosts — strict-default refusal is what we are
	// testing. Mirrors the setup in hostkey_test.go's
	// TestConnectionPool_Get_EarlyReturnsBeforeDialWhenHostKeyUnavailable.
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available; cannot deterministically reach host-key path in Get()")
	}
	sshDir := filepath.Join(tmpHome, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir .ssh: %v", err)
	}
	key := filepath.Join(sshDir, "id_ed25519")
	if out, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key).CombinedOutput(); err != nil {
		t.Skipf("ssh-keygen failed: %v\n%s", err, string(out))
	}

	ln, accepts := slowAcceptListener(t, 50*time.Millisecond)
	srv := serverFromListener("strict-host-key", ln)

	pool := NewConnectionPool(2 * time.Second)
	defer pool.Close()

	_, err := pool.Get(srv)
	if err == nil {
		t.Fatal("expected Get to fail with strict host-key default + missing known_hosts; got nil")
	}
	if !errors.Is(err, ErrKnownHostsMissing) {
		t.Fatalf("expected ErrKnownHostsMissing sentinel, got: %v", err)
	}
	// Strict mode must refuse BEFORE dialing — listener accept count
	// must stay zero. This is the wiring invariant the refactor must
	// preserve: the host-key check happens inside dial() (called
	// outside the lock) but still BEFORE any network activity.
	if got := atomic.LoadInt64(accepts); got != 0 {
		t.Fatalf("strict host-key default should refuse before TCP dial; listener accepted %d connections", got)
	}
}
