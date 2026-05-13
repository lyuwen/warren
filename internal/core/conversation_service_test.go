package core

import (
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
