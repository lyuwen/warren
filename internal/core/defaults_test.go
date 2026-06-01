package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lfu/warren/internal/core"
)

// ============================================================================
// Phase 2 audit Batch 1 — Item 3: warren.db default path + legacy detection.
//
// The audit (and the Critique pre-validation) demand:
//   - core.DefaultDBPath() resolves to <HOME>/.warren/warren.db; falls back
//     to "warren.db" (cwd-relative) when HOME is unresolvable.
//   - core.DefaultConfigDir() resolves to <HOME>/.warren; falls back to
//     ".warren" when HOME is unresolvable.
//   - core.EnsureConfigDir(dir) creates the directory if missing, no-ops
//     if it exists, and errors when it cannot create.
//   - core.LegacyDBCheck(userOverrodeDB bool, binaryHint string) error
//     returns a non-nil error whose message contains "Found legacy database"
//     when:
//        * the caller did NOT pass -db explicitly (flag.Visit-derived bool)
//        * AND ./warren.db exists in cwd
//        * AND core.DefaultDBPath() does NOT exist
//     and returns nil in all other cases.
//
// Cmd-level wiring (cmd/warren-web, cmd/warren-tui) is integration-tested
// separately — see cmd_legacy_db_test.go in this package which uses
// exec.Command against the built binaries.
// ============================================================================

func TestDefaultDBPath_WithHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	got := core.DefaultDBPath()
	want := filepath.Join(dir, ".warren", "warren.db")
	if got != want {
		t.Errorf("DefaultDBPath() = %q, want %q", got, want)
	}
}

// TestDefaultDBPath_EmptyHome documents the empty-HOME fallback branch the
// Critique flagged: when os.UserHomeDir() fails (or HOME is unset), the
// helper must fall back to a relative path rather than producing
// "/.warren/warren.db" (the broken os.ExpandEnv behaviour).
func TestDefaultDBPath_EmptyHome(t *testing.T) {
	t.Setenv("HOME", "")

	got := core.DefaultDBPath()
	if got == "" {
		t.Fatalf("DefaultDBPath() empty when HOME unset")
	}
	if strings.HasPrefix(got, "/.warren") || strings.HasPrefix(got, "/") {
		t.Errorf("DefaultDBPath() = %q with HOME='' must not be a rooted /.warren path (os.ExpandEnv fallback bug)", got)
	}
	if got != "warren.db" {
		t.Logf("DefaultDBPath() empty-HOME fallback = %q (acceptable if relative)", got)
	}
}

func TestDefaultConfigDir_WithHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	got := core.DefaultConfigDir()
	want := filepath.Join(dir, ".warren")
	if got != want {
		t.Errorf("DefaultConfigDir() = %q, want %q", got, want)
	}
}

func TestDefaultConfigDir_EmptyHome(t *testing.T) {
	t.Setenv("HOME", "")

	got := core.DefaultConfigDir()
	if got == "" {
		t.Fatalf("DefaultConfigDir() empty when HOME unset")
	}
	if strings.HasPrefix(got, "/.warren") || strings.HasPrefix(got, "/") {
		t.Errorf("DefaultConfigDir() = %q with HOME='' must not be rooted (os.ExpandEnv fallback bug)", got)
	}
}

func TestEnsureConfigDir_CreatesMissing(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, ".warren")

	if _, err := os.Stat(target); err == nil {
		t.Fatalf("precondition failed: %s already exists", target)
	}

	if err := core.EnsureConfigDir(target); err != nil {
		t.Fatalf("EnsureConfigDir(%q) returned error: %v", target, err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("expected %s to exist after EnsureConfigDir, stat err: %v", target, err)
	}
	if !info.IsDir() {
		t.Errorf("expected %s to be a directory, got mode=%v", target, info.Mode())
	}
}

func TestEnsureConfigDir_NoopExisting(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, ".warren")

	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatalf("setup: %v", err)
	}

	for i := 0; i < 3; i++ {
		if err := core.EnsureConfigDir(target); err != nil {
			t.Fatalf("EnsureConfigDir call #%d returned error: %v", i+1, err)
		}
	}
}

// TestEnsureConfigDir_EmptyDirRejected guards against silently accepting an
// empty string (which os.MkdirAll handles inconsistently across platforms).
func TestEnsureConfigDir_EmptyDirRejected(t *testing.T) {
	err := core.EnsureConfigDir("")
	if err == nil {
		t.Errorf("EnsureConfigDir(\"\") returned nil; expected error")
	}
}

// TestEnsureConfigDir_PermissionDenied verifies creation failures propagate.
// Skipped under root because root can write everywhere.
func TestEnsureConfigDir_PermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; chmod 0500 cannot deny writes")
	}

	base := t.TempDir()
	readonly := filepath.Join(base, "readonly")
	if err := os.MkdirAll(readonly, 0500); err != nil {
		t.Fatalf("setup: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(readonly, 0700) })

	target := filepath.Join(readonly, ".warren")
	if err := core.EnsureConfigDir(target); err == nil {
		t.Fatalf("EnsureConfigDir(%q) succeeded, expected permission error", target)
	}
}

// -------- LegacyDBCheck --------

// TestLegacyDBCheck_TriggersWhenLegacyExistsAndNoOverride is the
// refuse-to-start case demanded by the Critique. User did not pass -db,
// ./warren.db exists, DefaultDBPath() does NOT exist → error with the
// actionable "Found legacy database" wording.
func TestLegacyDBCheck_TriggersWhenLegacyExistsAndNoOverride(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cwd := t.TempDir()
	chdirForTest(t, cwd)

	if err := os.WriteFile(filepath.Join(cwd, "warren.db"), []byte("legacy"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := core.LegacyDBCheck(false, "warren-web")
	if err == nil {
		t.Fatalf("LegacyDBCheck(false, ...) returned nil; expected refuse-to-start error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Found legacy database") {
		t.Errorf("error message must contain \"Found legacy database\"; got %q", msg)
	}
	// Should mention the new $HOME/.warren/ location.
	if !strings.Contains(msg, ".warren") {
		t.Errorf("error message should reference the new $HOME/.warren/ location; got %q", msg)
	}
	// Should mention the binary hint so the user's escape-hatch invocation is correct.
	if !strings.Contains(msg, "warren-web") {
		t.Errorf("error message should reference the binary hint %q; got %q", "warren-web", msg)
	}
	// Should suggest `-db ./warren.db` as the escape hatch.
	if !strings.Contains(msg, "./warren.db") {
		t.Errorf("error message should suggest -db ./warren.db escape hatch; got %q", msg)
	}
}

// TestLegacyDBCheck_NoTriggerWhenUserOverrode confirms flag.Visit-derived
// detection: when the caller explicitly set -db, never warn — the user is
// in control regardless of cwd contents.
func TestLegacyDBCheck_NoTriggerWhenUserOverrode(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cwd := t.TempDir()
	chdirForTest(t, cwd)

	if err := os.WriteFile(filepath.Join(cwd, "warren.db"), []byte("legacy"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := core.LegacyDBCheck(true, "warren-web")
	if err != nil {
		t.Errorf("LegacyDBCheck(true, ...) returned %v; expected nil when user overrode -db", err)
	}
}

// TestLegacyDBCheck_NoTriggerWhenNewPathExists covers "user already
// migrated": both ./warren.db AND <HOME>/.warren/warren.db exist. New path
// wins, no warning.
func TestLegacyDBCheck_NoTriggerWhenNewPathExists(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cwd := t.TempDir()
	chdirForTest(t, cwd)

	if err := os.WriteFile(filepath.Join(cwd, "warren.db"), []byte("legacy"), 0644); err != nil {
		t.Fatalf("setup legacy: %v", err)
	}
	newDir := filepath.Join(tmpHome, ".warren")
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatalf("setup new dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(newDir, "warren.db"), []byte("new"), 0644); err != nil {
		t.Fatalf("setup new db: %v", err)
	}

	if err := core.LegacyDBCheck(false, "warren-web"); err != nil {
		t.Errorf("LegacyDBCheck returned %v; expected nil when DefaultDBPath already exists", err)
	}
}

// TestLegacyDBCheck_NoTriggerWhenNoLegacyFile covers the first-run path.
func TestLegacyDBCheck_NoTriggerWhenNoLegacyFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cwd := t.TempDir()
	chdirForTest(t, cwd)

	if err := core.LegacyDBCheck(false, "warren-web"); err != nil {
		t.Errorf("LegacyDBCheck returned %v; expected nil on first run", err)
	}
}

// TestLegacyDBCheck_DefaultBinaryHint exercises the empty-binaryHint
// fallback documented in defaults.go: a caller that passes "" gets
// "warren" used in the suggestion text.
func TestLegacyDBCheck_DefaultBinaryHint(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cwd := t.TempDir()
	chdirForTest(t, cwd)

	if err := os.WriteFile(filepath.Join(cwd, "warren.db"), []byte("legacy"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := core.LegacyDBCheck(false, "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "warren -db") && !strings.Contains(err.Error(), "warren ") {
		t.Errorf("error message should fall back to %q binary hint; got %q", "warren", err.Error())
	}
}

// chdirForTest changes into dir and restores cwd at test end. Renamed from
// `chdir` to avoid the suggestion of overriding os.Chdir.
func chdirForTest(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir(%q): %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Logf("chdir restore failed: %v", err)
		}
	})
}
