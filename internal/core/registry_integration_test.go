package core

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestRegisterAgentSession_LoggingOnSaveFailure verifies that when the
// registry auto-save fails, RegisterAgentSession logs a warning (so operators
// can diagnose) but does NOT return the error (registration should still
// succeed if the in-memory write worked).
func TestRegisterAgentSession_LoggingOnSaveFailure(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	origOutput := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
	}()

	// Build a minimal Warren whose registryPath points at an unwriteable location.
	// Use a path under a file (not a dir) so any open-for-write fails.
	tmpDir := t.TempDir()
	notADir := filepath.Join(tmpDir, "not-a-dir")
	if err := os.WriteFile(notADir, []byte("x"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	badPath := filepath.Join(notADir, "registry.json")

	w := &Warren{
		sessionRegistry: NewAgentSessionRegistry(),
		registryPath:    badPath,
		mu:              sync.RWMutex{},
	}

	session := &AgentSession{
		ID:         "agent-test-1",
		Name:       "test agent",
		ServerName: "localhost",
		TmuxPaneID: "%99",
	}

	err := w.RegisterAgentSession(session)
	if err != nil {
		t.Fatalf("RegisterAgentSession should not return error on save failure, got: %v", err)
	}

	// The in-memory registration should still have succeeded.
	got, err := w.sessionRegistry.Get("agent-test-1")
	if err != nil {
		t.Errorf("session not in registry after RegisterAgentSession: %v", err)
	}
	if got == nil {
		t.Error("expected non-nil session from registry")
	}

	// The save failure should have been logged.
	logged := buf.String()
	if !strings.Contains(strings.ToLower(logged), "failed to") || !strings.Contains(strings.ToLower(logged), "save") {
		t.Errorf("expected a log message about a failed save; got: %q", logged)
	}
}

// TestRegisterAgentSession_NoLogOnSuccess verifies the happy path: a successful
// save produces no warning log noise.
func TestRegisterAgentSession_NoLogOnSuccess(t *testing.T) {
	var buf bytes.Buffer
	origOutput := log.Writer()
	origFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(origOutput)
		log.SetFlags(origFlags)
	}()

	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "registry.json")

	w := &Warren{
		sessionRegistry: NewAgentSessionRegistry(),
		registryPath:    registryPath,
		mu:              sync.RWMutex{},
	}

	session := &AgentSession{
		ID:         "agent-ok",
		Name:       "ok agent",
		ServerName: "localhost",
		TmuxPaneID: "%1",
	}

	if err := w.RegisterAgentSession(session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logged := buf.String()
	if strings.Contains(strings.ToLower(logged), "failed") {
		t.Errorf("expected no failure log on successful save, got: %q", logged)
	}

	// Registry file should exist.
	if _, err := os.Stat(registryPath); err != nil {
		t.Errorf("expected registry file at %s, got error: %v", registryPath, err)
	}
}

// TestRegisterAgentSession_EmptyPathNoSave verifies that when registryPath is
// empty, no save is attempted and no log noise is produced.
func TestRegisterAgentSession_EmptyPathNoSave(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	w := &Warren{
		sessionRegistry: NewAgentSessionRegistry(),
		registryPath:    "",
	}

	session := &AgentSession{
		ID:         "agent-no-path",
		ServerName: "localhost",
		TmuxPaneID: "%1",
	}

	if err := w.RegisterAgentSession(session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("expected no log output when registryPath empty, got: %q", buf.String())
	}
}
