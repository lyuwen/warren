package web

import (
	"bytes"
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lfu/warren/internal/core"
)

// ============================================================================
// Phase 2 audit Batch 2 — Item #4: REST API CORS + loopback bind defaults.
//
// Locks in the security invariants the audit demanded:
//
//   - Default bind is 127.0.0.1:8080 (loopback only); --bind / WARREN_WEB_BIND
//     override.
//   - Default CORS allow-list contains only loopback origins; cross-origin
//     requests from arbitrary hosts get no CORS headers (browsers will block
//     the response).
//   - WARREN_WEB_CORS_ORIGINS appends operator-supplied origins.
//   - Non-loopback bind emits a WARN log on Start so operators can grep for
//     accidentally exposed surfaces.
//
// We deliberately do not Start() a listener in the bind-flag tests — opening
// real ports during go test invites flakes and conflicts with developers'
// running services. Instead we inspect the resolved config that NewServer
// (and the cmd-layer flag/env resolution it mirrors) would hand to
// http.Server.Addr.
// ============================================================================

// newTestWarren builds a Warren backed by a temp dir, isolated from the
// developer's $HOME / cwd. Used by tests that need a *core.Warren handle.
func newTestWarren(t *testing.T) *core.Warren {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := core.DefaultConfig()
	cfg.DBPath = filepath.Join(tmpDir, "test.db")
	cfg.ConfigDir = tmpDir
	w, err := core.NewWarren(cfg)
	if err != nil {
		t.Fatalf("new warren: %v", err)
	}
	return w
}

// ---------------------------------------------------------------------------
// Bind default + override
// ---------------------------------------------------------------------------

// TestWebServer_DefaultBind_Loopback verifies that a Config with an empty
// Addr resolves to the loopback-only DefaultBindAddr. This is the core
// security-posture default: the REST API must not be reachable on
// arbitrary network interfaces without explicit operator opt-in.
func TestWebServer_DefaultBind_Loopback(t *testing.T) {
	if DefaultBindAddr != "127.0.0.1:8080" {
		t.Fatalf("DefaultBindAddr regression: got %q, want %q", DefaultBindAddr, "127.0.0.1:8080")
	}

	srv := NewServer(&Config{Warren: newTestWarren(t)})
	if srv.addr != DefaultBindAddr {
		t.Fatalf("default bind: srv.addr = %q, want %q", srv.addr, DefaultBindAddr)
	}
	if srv.httpServer.Addr != DefaultBindAddr {
		t.Fatalf("default bind: httpServer.Addr = %q, want %q", srv.httpServer.Addr, DefaultBindAddr)
	}
}

// TestWebServer_BindFlagOverride verifies an explicit Addr on Config (the
// shape cmd/warren-web populates from --bind) is honored verbatim. This
// is the path operators take when they deliberately want a non-loopback
// bind such as `--bind 0.0.0.0:9090`.
func TestWebServer_BindFlagOverride(t *testing.T) {
	const custom = "0.0.0.0:9090"
	srv := NewServer(&Config{Addr: custom, Warren: newTestWarren(t)})
	if srv.addr != custom {
		t.Fatalf("--bind override: srv.addr = %q, want %q", srv.addr, custom)
	}
	if srv.httpServer.Addr != custom {
		t.Fatalf("--bind override: httpServer.Addr = %q, want %q", srv.httpServer.Addr, custom)
	}
}

// TestWebServer_BindEnvVarOverride exercises the cmd/warren-web layer
// contract: WARREN_WEB_BIND supplies the default value for the --bind
// flag. We re-implement the same resolution step the cmd does so the
// test stays scoped to public package surface (cmd has no test package).
//
// This matches the WARREN_SSH_INSECURE_HOSTKEY pattern from Batch 1 and
// is required by the audit's acceptance criteria for Item #4.
func TestWebServer_BindEnvVarOverride(t *testing.T) {
	const want = "0.0.0.0:9090"
	t.Setenv(EnvBindAddr, want)

	// Mirror the cmd/warren-web logic: env wins over the in-code default
	// when no explicit flag has been provided.
	resolved := DefaultBindAddr
	if env := os.Getenv(EnvBindAddr); env != "" {
		resolved = env
	}
	if resolved != want {
		t.Fatalf("env-resolved bind: got %q, want %q", resolved, want)
	}

	// And verify NewServer accepts the env-resolved value end-to-end.
	srv := NewServer(&Config{Addr: resolved, Warren: newTestWarren(t)})
	if srv.addr != want {
		t.Fatalf("env override end-to-end: srv.addr = %q, want %q", srv.addr, want)
	}
}

// ---------------------------------------------------------------------------
// CORS allow-list behavior
// ---------------------------------------------------------------------------

// dummyHandler returns 200 OK with no body. Used as the inner handler
// when exercising corsMiddleware directly.
func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// TestWebServer_CORS_DefaultLoopbackOnly is the primary anti-regression
// for the CORS allow-list. Requests from loopback origins must receive
// `Access-Control-Allow-Origin`; requests from arbitrary external origins
// must NOT (the browser will then block the response).
func TestWebServer_CORS_DefaultLoopbackOnly(t *testing.T) {
	h := corsMiddleware(defaultCORSOrigins, dummyHandler())

	allowed := []string{
		"http://localhost:8080",
		"http://127.0.0.1:8080",
	}
	for _, origin := range allowed {
		t.Run("allowed/"+origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
			req.Header.Set("Origin", origin)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			got := rec.Header().Get("Access-Control-Allow-Origin")
			if got != origin {
				t.Fatalf("loopback origin %q must be allowed; got Access-Control-Allow-Origin = %q", origin, got)
			}
			// Vary: Origin is required when the response is dynamically
			// scoped to the request Origin, otherwise caches conflate
			// responses across origins.
			if vary := rec.Header().Get("Vary"); !strings.Contains(vary, "Origin") {
				t.Errorf("Vary: Origin missing for allowed origin %q; got %q", origin, vary)
			}
		})
	}

	denied := []string{
		"https://evil.example.com",
		"http://attacker.test",
		"https://example.com",
	}
	for _, origin := range denied {
		t.Run("denied/"+origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
			req.Header.Set("Origin", origin)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Fatalf("out-of-policy origin %q must NOT receive Access-Control-Allow-Origin; got %q", origin, got)
			}
		})
	}
}

// TestWebServer_CORS_NoOriginHeader verifies same-origin / non-browser
// callers pass through cleanly (no CORS headers set, request reaches the
// handler). This locks in the implementer's "Origin header absent → no-op"
// branch so a future refactor can't silently start tagging every response.
func TestWebServer_CORS_NoOriginHeader(t *testing.T) {
	h := corsMiddleware(defaultCORSOrigins, dummyHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("same-origin request without Origin header should pass through; got status %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin should be empty when no Origin header is sent; got %q", got)
	}
}

// TestWebServer_CORS_EnvVarAddsOrigin covers the operator override path:
// WARREN_WEB_CORS_ORIGINS=https://example.com,https://other.example.com
// adds those entries to the default loopback allow-list.
func TestWebServer_CORS_EnvVarAddsOrigin(t *testing.T) {
	t.Setenv(EnvCORSOrigins, "https://example.com, https://other.example.com")

	origins := resolveCORSOrigins(os.Getenv(EnvCORSOrigins))
	h := corsMiddleware(origins, dummyHandler())

	// Default loopback origins must STILL be allowed (env adds, not replaces).
	req := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:8080" {
		t.Fatalf("env override must not drop default loopback origins")
	}

	// New entry from env is allowed.
	req = httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.Header.Set("Origin", "https://example.com")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Fatalf("env-supplied origin must be allowed; got Access-Control-Allow-Origin = %q", got)
	}

	// Whitespace-trimmed second entry is allowed.
	req = httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.Header.Set("Origin", "https://other.example.com")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://other.example.com" {
		t.Fatalf("env-supplied (whitespace-trimmed) origin must be allowed; got %q", got)
	}

	// And an unrelated origin is still denied.
	req = httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("origin outside env override must remain denied; got %q", got)
	}
}

// TestWebServer_CORS_PreflightInPolicy verifies OPTIONS preflights from
// allowed origins short-circuit with 204 No Content. Without this, the
// preflight would fall through to the static-file or method-not-allowed
// handler and the browser would refuse the actual request.
func TestWebServer_CORS_PreflightInPolicy(t *testing.T) {
	h := corsMiddleware(defaultCORSOrigins, dummyHandler())

	req := httptest.NewRequest(http.MethodOptions, "/api/agents", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("in-policy preflight: status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "POST") {
		t.Errorf("preflight should advertise allowed methods; got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Non-loopback bind WARN log
// ---------------------------------------------------------------------------

// TestWebServer_NonLoopbackBind_LogsWarning captures the default logger
// output across Server.Start() and confirms a WARN is emitted when the
// resolved bind is not loopback. The Start path also fires off the HTTP
// server in a goroutine; we Stop it immediately so the test does not
// hold a real listener open.
//
// Bind to 127.0.0.1:0 in the loopback-positive case so we can verify
// the absence of the WARN without conflicting with a developer's
// running service. Bind to a routable non-loopback address (0.0.0.0:0)
// to trigger the WARN branch.
func TestWebServer_NonLoopbackBind_LogsWarning(t *testing.T) {
	for _, tc := range []struct {
		name     string
		addr     string
		wantWarn bool
	}{
		{name: "loopback ipv4", addr: "127.0.0.1:0", wantWarn: false},
		{name: "loopback hostname", addr: "localhost:0", wantWarn: false},
		{name: "non-loopback 0.0.0.0", addr: "0.0.0.0:0", wantWarn: true},
		{name: "empty host (all interfaces)", addr: ":0", wantWarn: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			old := log.Writer()
			log.SetOutput(&buf)
			t.Cleanup(func() { log.SetOutput(old) })

			srv := NewServer(&Config{Addr: tc.addr, Warren: newTestWarren(t)})
			if err := srv.Start(); err != nil {
				t.Fatalf("Start: %v", err)
			}
			// Give the WARN log goroutine-free path a moment to flush;
			// Start() itself logs synchronously before returning, but
			// the WARN was emitted via log.Printf which writes
			// inline — no sleep should be needed.
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := srv.Stop(ctx); err != nil {
				t.Fatalf("Stop: %v", err)
			}

			out := buf.String()
			gotWarn := strings.Contains(out, "WARN") &&
				strings.Contains(out, "non-loopback")
			if gotWarn != tc.wantWarn {
				t.Fatalf("WARN log presence: got=%v want=%v\nlogs:\n%s", gotWarn, tc.wantWarn, out)
			}
		})
	}
}

// TestIsLoopbackBind_TableDriven locks in the loopback classifier the
// WARN gate relies on. If isLoopbackBind ever drifts (e.g. starts
// returning true for 0.0.0.0 because of a misread of IsLoopback), the
// non-loopback WARN goes silent and the security posture quietly
// regresses.
func TestIsLoopbackBind_TableDriven(t *testing.T) {
	cases := []struct {
		addr string
		want bool
	}{
		{"127.0.0.1:8080", true},
		{"127.0.0.1:0", true},
		{"localhost:8080", true},
		{"[::1]:8080", true},
		{"0.0.0.0:8080", false},
		{":8080", false}, // empty host == all interfaces
		{"192.168.1.10:8080", false},
		{"not a host:port", false}, // malformed → safe-side false
	}
	for _, tc := range cases {
		t.Run(tc.addr, func(t *testing.T) {
			if got := isLoopbackBind(tc.addr); got != tc.want {
				t.Errorf("isLoopbackBind(%q) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}

// TestResolveCORSOrigins covers the env parsing helper directly, including
// whitespace handling and empty-entry skipping.
func TestResolveCORSOrigins(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want []string
	}{
		{
			name: "empty env returns defaults",
			env:  "",
			want: defaultCORSOrigins,
		},
		{
			name: "single entry appended",
			env:  "https://app.example.com",
			want: append(append([]string{}, defaultCORSOrigins...), "https://app.example.com"),
		},
		{
			name: "multiple entries with whitespace",
			env:  " https://a.example.com , https://b.example.com ",
			want: append(append([]string{}, defaultCORSOrigins...), "https://a.example.com", "https://b.example.com"),
		},
		{
			name: "empty entries skipped",
			env:  ",,https://c.example.com,,",
			want: append(append([]string{}, defaultCORSOrigins...), "https://c.example.com"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveCORSOrigins(tc.env)
			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d (got=%v want=%v)", len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("entry %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestOriginHostsForBind locks in the auto-expand helper that keeps the
// default CORS allow-list useful when an operator binds to a non-default
// loopback port (e.g. --bind 127.0.0.1:8090). The default port short-
// circuits via DefaultBindAddr so a future change to that constant does
// not leave a stale magic-string in this helper (per the 6bcd4da follow-up).
//
// Non-loopback binds MUST return nil so operators are forced to opt in
// via WARREN_WEB_CORS_ORIGINS — silently auto-expanding for 0.0.0.0
// would silently widen the CORS surface.
func TestOriginHostsForBind(t *testing.T) {
	cases := []struct {
		name string
		addr string
		want []string
	}{
		{
			name: "default loopback port short-circuits",
			addr: "127.0.0.1:8080",
			want: nil,
		},
		{
			name: "default loopback port via localhost short-circuits",
			addr: "localhost:8080",
			want: nil,
		},
		{
			name: "non-default loopback port auto-expands ipv4",
			addr: "127.0.0.1:8090",
			want: []string{"http://localhost:8090", "http://127.0.0.1:8090"},
		},
		{
			name: "non-default loopback port auto-expands via localhost",
			addr: "localhost:9000",
			want: []string{"http://localhost:9000", "http://127.0.0.1:9000"},
		},
		{
			name: "non-default loopback port auto-expands ipv6 [::1]",
			addr: "[::1]:8090",
			want: []string{"http://localhost:8090", "http://127.0.0.1:8090"},
		},
		{
			name: "non-loopback bind returns nil (must opt in via env)",
			addr: "0.0.0.0:8090",
			want: nil,
		},
		{
			name: "private LAN bind returns nil",
			addr: "192.168.1.10:8090",
			want: nil,
		},
		{
			name: "empty host (all interfaces) returns nil",
			addr: ":8090",
			want: nil,
		},
		{
			name: "malformed addr returns nil",
			addr: "not a host:port",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := originHostsForBind(tc.addr)
			if len(got) != len(tc.want) {
				t.Fatalf("originHostsForBind(%q) = %v (len %d), want %v (len %d)", tc.addr, got, len(got), tc.want, len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("entry %d: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestOriginHostsForBind_TracksDefaultConstant locks in the 6bcd4da
// refactor: the "skip if port matches default" early-out derives the
// default port from DefaultBindAddr rather than hardcoding "8080". This
// test pins the invariant that the default port (whatever it is) is
// covered by defaultCORSOrigins and does not get duplicated into the
// auto-expansion.
func TestOriginHostsForBind_TracksDefaultConstant(t *testing.T) {
	_, defaultPort, err := net.SplitHostPort(DefaultBindAddr)
	if err != nil {
		t.Fatalf("DefaultBindAddr (%q) is malformed: %v", DefaultBindAddr, err)
	}
	got := originHostsForBind("127.0.0.1:" + defaultPort)
	if got != nil {
		t.Fatalf("originHostsForBind on default port %q must short-circuit (nil); got %v — would duplicate the default loopback origin", defaultPort, got)
	}
}
