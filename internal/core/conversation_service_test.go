package core

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lfu/warren/internal/claude"
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
		`{"type":"user","uuid":"u1","parentUuid":"","isSidechain":false,"message":{"role":"user","content":"hi"}}`,
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
			`{"type":"assistant","uuid":"u2","parentUuid":"u1","isSidechain":false,"message":{"role":"assistant","content":"hello"}}`,
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
	writeJSONL(t, convFile, []string{`{"type":"user","uuid":"u1","isSidechain":false,"message":{"role":"user","content":"hi"}}`})

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
	writeJSONL(t, convFile, []string{`{"type":"assistant","uuid":"u2","isSidechain":false,"message":{"role":"assistant","content":"x"}}`})

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
	writeJSONL(t, convFile, []string{`{"type":"user","uuid":"u0","isSidechain":false,"message":{"role":"user","content":"hi"}}`})

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
				`{"type":"user","uuid":"x","isSidechain":false,"message":{"role":"user","content":"y"}}`,
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
			`{"type":"user","uuid":"u1","isSidechain":false,"message":{"role":"user","content":"hi"}}`,
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

// --- subscribeWithFetcher tests (internal polling primitive) ---
//
// These exercise the change-detection loop directly via the internal
// subscribeWithFetcher hook so we don't have to stand up a session mapper,
// filesystem, or SSH client to test the core polling/dedup/stop logic.

// fakeFetcher returns a controllable slice of messages. Calls to messages()
// are atomic; tests can mutate the slice between ticks.
type fakeFetcher struct {
	mu       sync.Mutex
	messages []*claude.Message
	calls    int64
	err      error
}

func (f *fakeFetcher) fetch() ([]*claude.Message, error) {
	atomic.AddInt64(&f.calls, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	// Return a copy so callers can't mutate our slice.
	out := make([]*claude.Message, len(f.messages))
	copy(out, f.messages)
	return out, nil
}

func (f *fakeFetcher) append(msg *claude.Message) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, msg)
}

func (f *fakeFetcher) setError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// TestSubscribeWithFetcher_DetectsNewMessages verifies the poller emits the
// agentID when the message count grows.
func TestSubscribeWithFetcher_DetectsNewMessages(t *testing.T) {
	cs := NewConversationService()
	defer cs.Close()

	f := &fakeFetcher{
		messages: []*claude.Message{
			{Type: "user", UUID: "u1", Timestamp: time.Now()},
		},
	}

	ch, err := cs.subscribeWithFetcher("agent-1", f.fetch, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("subscribeWithFetcher: %v", err)
	}

	// Append a new message — next tick should notify.
	time.Sleep(30 * time.Millisecond)
	f.append(&claude.Message{Type: "assistant", UUID: "u2", Timestamp: time.Now().Add(time.Second)})

	select {
	case got := <-ch:
		if got != "agent-1" {
			t.Errorf("expected agent-1, got %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for notification (fetcher called %d times)", atomic.LoadInt64(&f.calls))
	}
}

// TestSubscribeWithFetcher_NoNotifyWhenUnchanged verifies that a stable
// message list does NOT spam the channel.
func TestSubscribeWithFetcher_NoNotifyWhenUnchanged(t *testing.T) {
	cs := NewConversationService()
	defer cs.Close()

	f := &fakeFetcher{
		messages: []*claude.Message{
			{Type: "user", UUID: "u1", Timestamp: time.Now()},
		},
	}

	ch, err := cs.subscribeWithFetcher("agent-stable", f.fetch, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("subscribeWithFetcher: %v", err)
	}

	// Wait for several poll ticks without changing the data.
	select {
	case got := <-ch:
		t.Errorf("unexpected notification on unchanged data: %q", got)
	case <-time.After(250 * time.Millisecond):
		// Good — no spurious notifications.
	}

	if atomic.LoadInt64(&f.calls) < 3 {
		t.Errorf("expected the fetcher to be polled multiple times, only called %d", atomic.LoadInt64(&f.calls))
	}
}

// TestSubscribeWithFetcher_DetectsTimestampAdvance verifies the poller also
// emits when the message count stays the same but the last-message timestamp
// advances (e.g., a streaming message getting updated in place).
func TestSubscribeWithFetcher_DetectsTimestampAdvance(t *testing.T) {
	cs := NewConversationService()
	defer cs.Close()

	now := time.Now()
	f := &fakeFetcher{
		messages: []*claude.Message{
			{Type: "user", UUID: "u1", Timestamp: now},
		},
	}

	ch, err := cs.subscribeWithFetcher("agent-ts", f.fetch, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("subscribeWithFetcher: %v", err)
	}

	// Replace the last message with the same count but a later timestamp.
	time.Sleep(30 * time.Millisecond)
	f.mu.Lock()
	f.messages = []*claude.Message{
		{Type: "user", UUID: "u1", Timestamp: now.Add(time.Second)},
	}
	f.mu.Unlock()

	select {
	case <-ch:
		// Good.
	case <-time.After(1 * time.Second):
		t.Fatal("expected notification when last-message timestamp advanced")
	}
}

// TestUnsubscribe_StopsPollingAndClosesChannel verifies that Unsubscribe stops
// the goroutine, closes the channel, and is safe to call twice.
func TestUnsubscribe_StopsPollingAndClosesChannel(t *testing.T) {
	cs := NewConversationService()
	defer cs.Close()

	f := &fakeFetcher{messages: []*claude.Message{{UUID: "u1", Timestamp: time.Now()}}}

	ch, err := cs.subscribeWithFetcher("agent-unsub", f.fetch, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("subscribeWithFetcher: %v", err)
	}

	// Let the poller spin up.
	time.Sleep(40 * time.Millisecond)

	cs.Unsubscribe("agent-unsub")

	// Channel should close.
	select {
	case _, ok := <-ch:
		if ok {
			// Drain any in-flight notification, then expect close.
			select {
			case _, ok2 := <-ch:
				if ok2 {
					t.Error("expected channel to be closed after Unsubscribe")
				}
			case <-time.After(200 * time.Millisecond):
				t.Error("expected channel to close after Unsubscribe")
			}
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("channel did not close within 500ms of Unsubscribe")
	}

	// Calling Unsubscribe again must be safe.
	cs.Unsubscribe("agent-unsub")
	cs.Unsubscribe("does-not-exist")

	// Polling should have stopped — record call count, wait, ensure it didn't grow.
	stopped := atomic.LoadInt64(&f.calls)
	time.Sleep(150 * time.Millisecond)
	now := atomic.LoadInt64(&f.calls)
	if now != stopped {
		t.Errorf("fetcher called %d more times after Unsubscribe (was %d, now %d)", now-stopped, stopped, now)
	}
}

// TestSubscribeWithFetcher_NonBlockingChannel ensures the poller never
// deadlocks when changes arrive faster than the consumer drains.
func TestSubscribeWithFetcher_NonBlockingChannel(t *testing.T) {
	cs := NewConversationService()
	defer cs.Close()

	f := &fakeFetcher{messages: []*claude.Message{{UUID: "u0", Timestamp: time.Now()}}}

	_, err := cs.subscribeWithFetcher("agent-flood", f.fetch, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("subscribeWithFetcher: %v", err)
	}

	// Generate far more changes than the channel buffer (10) without draining.
	for i := 0; i < 30; i++ {
		f.append(&claude.Message{UUID: "x", Timestamp: time.Now().Add(time.Duration(i+1) * time.Second)})
		time.Sleep(8 * time.Millisecond)
	}

	// If the poller had deadlocked, fetcher.calls would have stopped growing.
	// Sanity check: it should have been called many times.
	if got := atomic.LoadInt64(&f.calls); got < 10 {
		t.Errorf("fetcher should have been called many times, only got %d", got)
	}

	// Clean shutdown should also complete promptly even with a full channel.
	done := make(chan struct{})
	go func() { cs.Unsubscribe("agent-flood"); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Unsubscribe blocked after channel filled — likely deadlock in stopSubscription")
	}
}

// TestSubscribeToUpdates_DuplicateAgentID verifies a second Subscribe for the
// same agentID returns an error rather than silently replacing.
func TestSubscribeToUpdates_DuplicateAgentID(t *testing.T) {
	cs := NewConversationService()
	defer cs.Close()

	f := &fakeFetcher{}
	if _, err := cs.subscribeWithFetcher("dup", f.fetch, 50*time.Millisecond); err != nil {
		t.Fatalf("first subscribe: %v", err)
	}

	_, err := cs.subscribeWithFetcher("dup", f.fetch, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected error subscribing twice with same agentID")
	}
}

// TestClose_StopsAllSubscriptions verifies Close stops every active poller.
func TestClose_StopsAllSubscriptions(t *testing.T) {
	cs := NewConversationService()

	fs := []*fakeFetcher{
		{messages: []*claude.Message{{UUID: "a", Timestamp: time.Now()}}},
		{messages: []*claude.Message{{UUID: "b", Timestamp: time.Now()}}},
		{messages: []*claude.Message{{UUID: "c", Timestamp: time.Now()}}},
	}
	chans := make([]<-chan string, len(fs))
	for i, f := range fs {
		ch, err := cs.subscribeWithFetcher(string(rune('A'+i)), f.fetch, 20*time.Millisecond)
		if err != nil {
			t.Fatalf("subscribe %d: %v", i, err)
		}
		chans[i] = ch
	}

	// Let them spin up.
	time.Sleep(40 * time.Millisecond)

	done := make(chan struct{})
	go func() { cs.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close blocked — likely a stopSubscription deadlock")
	}

	// All channels should close.
	for i, ch := range chans {
		select {
		case _, ok := <-ch:
			if ok {
				// In-flight notification; expect close on next read.
				select {
				case _, ok2 := <-ch:
					if ok2 {
						t.Errorf("channel %d not closed after Close", i)
					}
				case <-time.After(200 * time.Millisecond):
					t.Errorf("channel %d not closed within 200ms after Close", i)
				}
			}
		case <-time.After(500 * time.Millisecond):
			t.Errorf("channel %d not closed within 500ms after Close", i)
		}
	}

	// Polling stopped.
	totals := make([]int64, len(fs))
	for i, f := range fs {
		totals[i] = atomic.LoadInt64(&f.calls)
	}
	time.Sleep(120 * time.Millisecond)
	for i, f := range fs {
		if got := atomic.LoadInt64(&f.calls); got != totals[i] {
			t.Errorf("fetcher %d kept polling after Close (was %d, now %d)", i, totals[i], got)
		}
	}
}

// TestSetPollInterval_AffectsNewSubscriptions verifies SetPollInterval changes
// the cadence used by SUBSEQUENT subscribes, not existing ones.
func TestSetPollInterval_AffectsNewSubscriptions(t *testing.T) {
	cs := NewConversationService()
	defer cs.Close()

	cs.SetPollInterval(10 * time.Millisecond)

	f := &fakeFetcher{messages: []*claude.Message{{UUID: "u1", Timestamp: time.Now()}}}
	// Pass interval=0 so it falls back to cs.pollInterval.
	_, err := cs.subscribeWithFetcher("fast", f.fetch, 0)
	if err != nil {
		t.Fatalf("subscribeWithFetcher: %v", err)
	}

	time.Sleep(120 * time.Millisecond)
	calls := atomic.LoadInt64(&f.calls)
	if calls < 5 {
		t.Errorf("expected ~12 calls in 120ms at 10ms interval, got %d", calls)
	}

	// Invalid intervals are ignored.
	cs.SetPollInterval(-1)
	cs.SetPollInterval(0)
}

// TestSubscribeToUpdates_ValidationErrors verifies the public API validates
// its required arguments.
func TestSubscribeToUpdates_ValidationErrors(t *testing.T) {
	cs := NewConversationService()
	defer cs.Close()

	if _, err := cs.SubscribeToUpdates("", nil, &Server{Kind: ServerKindLocal}, nil); err == nil {
		t.Error("expected error for empty agentID")
	}
	if _, err := cs.SubscribeToUpdates("agent", nil, nil, nil); err == nil {
		t.Error("expected error for nil server")
	}
}
