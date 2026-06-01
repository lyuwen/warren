package core_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// Phase 2 audit Batch 1 — Item 3 cmd-level integration: legacy-DB refuse-to-start.
//
// Unit tests in defaults_test.go cover core.LegacyDBCheck exhaustively. These
// tests verify the WIRING: warren-web and warren-tui actually call
// LegacyDBCheck at startup and exit non-zero with the actionable message.
// Without wiring tests, an Implementer could ship the helper without
// hooking it up and the silent-data-loss bug remains.
//
// Strategy: build the binary into a t.TempDir(), then exec it in a temp
// cwd with HOME pointed at another temp dir. Assert exit code and stderr.
// ============================================================================

// buildBinary compiles cmd/<name> into the test's TempDir and returns the
// absolute path. Skips the test if the go toolchain is unavailable or the
// build itself fails.
func buildBinary(t *testing.T, cmdPkg string) string {
	t.Helper()

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}

	repoRoot, err := repoRoot()
	if err != nil {
		t.Skipf("locate repo root: %v", err)
	}
	cmdDir := filepath.Join(repoRoot, "cmd", cmdPkg)
	if _, err := os.Stat(cmdDir); err != nil {
		t.Skipf("cmd dir not found: %v", err)
	}

	outDir := t.TempDir()
	binName := cmdPkg
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(outDir, binName)

	build := exec.Command("go", "build", "-o", binPath, "./cmd/"+cmdPkg)
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "CGO_ENABLED=1") // sqlite needs cgo
	var stderr bytes.Buffer
	build.Stderr = &stderr
	if err := build.Run(); err != nil {
		t.Skipf("go build cmd/%s failed: %v\nstderr:\n%s", cmdPkg, err, stderr.String())
	}
	return binPath
}

// repoRoot ascends from cwd to find the directory containing go.mod.
func repoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", cwd)
		}
		dir = parent
	}
}

// runWithEnv launches bin in cwd with HOME pointed at tmpHome. Returns
// (exitCode, combined-stderr-string).
func runWithEnv(t *testing.T, bin, cwd, tmpHome string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = cwd
	env := filterEnv(os.Environ(), "HOME")
	env = append(env, "HOME="+tmpHome)
	cmd.Env = env

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	exit := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exit = ee.ExitCode()
		} else {
			t.Logf("exec error (not ExitError): %v", err)
			exit = -1
		}
	}
	return exit, stderr.String()
}

// filterEnv returns a copy of env with all entries starting with any of the
// given prefixes removed (used to drop HOME so callers can override it).
func filterEnv(env []string, keysToDrop ...string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		drop := false
		for _, k := range keysToDrop {
			if strings.HasPrefix(kv, k+"=") {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, kv)
		}
	}
	return out
}

// TestCmd_WarrenWeb_LegacyDBRefusesToStart verifies the wiring: warren-web
// detects an orphan ./warren.db, refuses to start, and prints the actionable
// migration hint to stderr.
func TestCmd_WarrenWeb_LegacyDBRefusesToStart(t *testing.T) {
	bin := buildBinary(t, "warren-web")

	cwd := t.TempDir()
	tmpHome := t.TempDir()

	if err := os.WriteFile(filepath.Join(cwd, "warren.db"), []byte("legacy"), 0644); err != nil {
		t.Fatalf("setup legacy: %v", err)
	}

	exit, stderr := runWithEnv(t, bin, cwd, tmpHome)

	if exit == 0 {
		t.Fatalf("warren-web exited 0 with legacy ./warren.db present; expected non-zero refuse-to-start.\nstderr:\n%s", stderr)
	}
	if !strings.Contains(stderr, "Found legacy database") {
		t.Errorf("stderr missing %q substring:\n%s", "Found legacy database", stderr)
	}
	if !strings.Contains(stderr, "warren-web") {
		t.Errorf("stderr should mention the binary name %q:\n%s", "warren-web", stderr)
	}
	if !strings.Contains(stderr, ".warren") {
		t.Errorf("stderr should reference the new $HOME/.warren/ location:\n%s", stderr)
	}
}

// TestCmd_WarrenWeb_LegacyDBHonorsExplicitDBFlag verifies that passing -db
// explicitly bypasses the legacy check. flag.Visit-based detection must
// distinguish "user passed -db" from "user accepted the default".
func TestCmd_WarrenWeb_LegacyDBHonorsExplicitDBFlag(t *testing.T) {
	bin := buildBinary(t, "warren-web")

	cwd := t.TempDir()
	tmpHome := t.TempDir()

	if err := os.WriteFile(filepath.Join(cwd, "warren.db"), []byte("legacy"), 0644); err != nil {
		t.Fatalf("setup legacy: %v", err)
	}

	cmd := exec.Command(bin,
		"-db", "./warren.db",
		"-addr", "127.0.0.1:0",
	)
	cmd.Dir = cwd
	env := filterEnv(os.Environ(), "HOME")
	env = append(env, "HOME="+tmpHome)
	cmd.Env = env

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		out := stderr.String()
		if strings.Contains(out, "Found legacy database") {
			t.Errorf("warren-web -db ./warren.db still triggered legacy-DB check (flag.Visit detection broken):\n%s\nexit err: %v", out, err)
			return
		}
		// Exited for an unrelated reason (sqlite rejecting the fake content,
		// port conflict, etc.) — acceptable: it got past LegacyDBCheck.
		t.Logf("process exited unrelated to legacy check: %v\nstderr:\n%s", err, out)
	case <-time.After(750 * time.Millisecond):
		// Still running — bypassed legacy check and is now serving.
	}
}

// TestCmd_WarrenTUI_LegacyDBRefusesToStart mirrors warren-web for the TUI.
func TestCmd_WarrenTUI_LegacyDBRefusesToStart(t *testing.T) {
	bin := buildBinary(t, "warren-tui")

	cwd := t.TempDir()
	tmpHome := t.TempDir()

	if err := os.WriteFile(filepath.Join(cwd, "warren.db"), []byte("legacy"), 0644); err != nil {
		t.Fatalf("setup legacy: %v", err)
	}

	exit, stderr := runWithEnv(t, bin, cwd, tmpHome)

	if exit == 0 {
		t.Fatalf("warren-tui exited 0 with legacy ./warren.db present; expected non-zero refuse-to-start.\nstderr:\n%s", stderr)
	}
	if !strings.Contains(stderr, "Found legacy database") {
		t.Errorf("stderr missing %q substring:\n%s", "Found legacy database", stderr)
	}
	if !strings.Contains(stderr, "warren-tui") {
		t.Errorf("stderr should mention the binary name %q:\n%s", "warren-tui", stderr)
	}
}
