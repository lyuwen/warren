package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewConversationService(t *testing.T) {
	service := NewConversationService()

	if service == nil {
		t.Fatal("Expected non-nil service")
	}

	if service.cacheTTL != 5*time.Second {
		t.Errorf("Expected default cacheTTL 5s, got %v", service.cacheTTL)
	}

	if service.sessionMapper == nil {
		t.Error("Expected non-nil sessionMapper")
	}

	if service.conversationReader == nil {
		t.Error("Expected non-nil conversationReader")
	}

	if service.cache == nil {
		t.Error("Expected non-nil cache")
	}

	if service.sshClients == nil {
		t.Error("Expected non-nil sshClients map")
	}
}

func TestNewConversationServiceWithTTL(t *testing.T) {
	tests := []struct {
		name     string
		cacheTTL time.Duration
	}{
		{"1 second", 1 * time.Second},
		{"5 seconds", 5 * time.Second},
		{"30 seconds", 30 * time.Second},
		{"1 minute", 1 * time.Minute},
		{"1 hour", 1 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewConversationServiceWithTTL(tt.cacheTTL)

			if service == nil {
				t.Fatal("Expected non-nil service")
			}

			if service.cacheTTL != tt.cacheTTL {
				t.Errorf("Expected cacheTTL %v, got %v", tt.cacheTTL, service.cacheTTL)
			}
		})
	}
}

func TestConversationService_ClearCache(t *testing.T) {
	service := NewConversationService()

	// Add some entries to cache
	service.cache.entries["test1"] = &cacheEntry{
		filePath: "test1",
		expiry:   time.Now().Add(1 * time.Hour),
	}
	service.cache.entries["test2"] = &cacheEntry{
		filePath: "test2",
		expiry:   time.Now().Add(1 * time.Hour),
	}

	if len(service.cache.entries) != 2 {
		t.Errorf("Expected 2 cache entries, got %d", len(service.cache.entries))
	}

	// Clear cache
	service.ClearCache()

	if len(service.cache.entries) != 0 {
		t.Errorf("Expected 0 cache entries after clear, got %d", len(service.cache.entries))
	}
}

// --- SubscribeToUpdates polling tests ---
//
// These tests assume the implementer exposes a file-watcher API on
// ConversationService that the public SubscribeToUpdates can be built on top
// of. Expected API surface (will coordinate with implementer):
//
//   func (cs *ConversationService) SubscribeToFile(filePath string, interval time.Duration) (<-chan string, func(), error)
//
//   - returns a channel that receives the agent/file identifier when the file
//     grows (new messages appended)
//   - the second return value is a stop function that halts the polling
//     goroutine
//   - polling at the provided interval
//
// If the implementer chooses a different shape (e.g., a Subscribe method that
// takes a registered agentID), these tests will need a small adapter; the
// behaviors covered (detect change, stop on unsubscribe, no deadlock) remain
// valid.

func writeJSONL(t *testing.T, path string, lines []string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
}

// TestSubscribeToFile_DetectsNewMessages verifies that appending data to the
// watched file causes the subscription channel to emit a notification.
func TestSubscribeToFile_DetectsNewMessages(t *testing.T) {
	tmp := t.TempDir()
	convFile := filepath.Join(tmp, "session.jsonl")

	// Seed with a minimal valid line so the file exists.
	writeJSONL(t, convFile, []string{
		`{"type":"user","uuid":"u1","parentUuid":"","isSidechain":false}`,
	})

	cs := NewConversationService()
	ch, stop, err := cs.SubscribeToFile(convFile, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("SubscribeToFile: %v", err)
	}
	defer stop()

	// Append a new message after a short delay.
	go func() {
		time.Sleep(150 * time.Millisecond)
		writeJSONL(t, convFile, []string{
			`{"type":"assistant","uuid":"u2","parentUuid":"u1","isSidechain":false}`,
		})
	}()

	select {
	case id := <-ch:
		if id == "" {
			t.Error("expected non-empty notification identifier")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for change notification")
	}
}

// TestUnsubscribe_StopsPolling verifies that calling the stop function halts
// the polling goroutine and no further notifications are delivered.
func TestUnsubscribe_StopsPolling(t *testing.T) {
	tmp := t.TempDir()
	convFile := filepath.Join(tmp, "session.jsonl")
	writeJSONL(t, convFile, []string{`{"type":"user","uuid":"u1","isSidechain":false}`})

	cs := NewConversationService()
	ch, stop, err := cs.SubscribeToFile(convFile, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("SubscribeToFile: %v", err)
	}

	// Drain any startup notification (some implementations emit on first scan).
	select {
	case <-ch:
	case <-time.After(150 * time.Millisecond):
	}

	stop()

	// Append after stopping — should not produce a notification.
	writeJSONL(t, convFile, []string{`{"type":"assistant","uuid":"u2","isSidechain":false}`})

	select {
	case id, ok := <-ch:
		if ok {
			t.Errorf("expected no notification after stop, got: %q", id)
		}
		// Channel closed is acceptable too.
	case <-time.After(500 * time.Millisecond):
		// Good — no notification arrived.
	}
}

// TestSubscribeToFile_ChannelDoesNotBlock fires many rapid changes and
// verifies the poller doesn't deadlock even if the consumer is slow.
func TestSubscribeToFile_ChannelDoesNotBlock(t *testing.T) {
	tmp := t.TempDir()
	convFile := filepath.Join(tmp, "session.jsonl")
	writeJSONL(t, convFile, []string{`{"type":"user","uuid":"u0","isSidechain":false}`})

	cs := NewConversationService()
	ch, stop, err := cs.SubscribeToFile(convFile, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("SubscribeToFile: %v", err)
	}
	defer stop()

	// Rapidly append messages without draining the channel.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			writeJSONL(t, convFile, []string{
				`{"type":"user","uuid":"x","isSidechain":false}`,
			})
			time.Sleep(10 * time.Millisecond)
		}
		close(done)
	}()

	select {
	case <-done:
		// Writer finished — poller didn't deadlock.
	case <-time.After(5 * time.Second):
		t.Fatal("writer goroutine deadlocked, poller likely blocking on channel send")
	}

	// Drain whatever the channel buffered without panicking.
	drainCount := 0
drainLoop:
	for {
		select {
		case <-ch:
			drainCount++
		default:
			break drainLoop
		}
	}
	t.Logf("drained %d notifications", drainCount)
}

// TestSubscribeToFile_NonExistentFile verifies sensible behavior when the
// target file doesn't exist yet (it may be created later by Claude Code).
// The subscription should either return an error OR succeed and emit a
// notification once the file appears.
func TestSubscribeToFile_NonExistentFile(t *testing.T) {
	tmp := t.TempDir()
	convFile := filepath.Join(tmp, "not-yet.jsonl")

	cs := NewConversationService()
	ch, stop, err := cs.SubscribeToFile(convFile, 50*time.Millisecond)
	if err != nil {
		// Acceptable: implementation requires file to exist up-front.
		return
	}
	defer stop()

	// Create the file after the fact.
	go func() {
		time.Sleep(100 * time.Millisecond)
		writeJSONL(t, convFile, []string{
			`{"type":"user","uuid":"u1","isSidechain":false}`,
		})
	}()

	select {
	case <-ch:
		// Good — implementation handles late-creation case.
	case <-time.After(1 * time.Second):
		// Also acceptable — implementation may choose to not emit until first
		// stat succeeds and a subsequent change is detected.
		t.Log("no notification within 1s for late-created file (acceptable)")
	}
}
